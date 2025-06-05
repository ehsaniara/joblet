package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"io"
	"job-worker/pkg/client"
	"os"
	"os/signal"
	"syscall"
)

func newStreamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream <job-id>",
		Short: "Stream job logs",
		Args:  cobra.ExactArgs(1),
		RunE:  runStream,
	}

	cmd.Flags().BoolVarP(&streamParams.follow, "follow", "f", true, "Follow the log stream (can be terminated with Ctrl+C)")

	return cmd
}

type streamCmdParams struct {
	follow bool
}

var streamParams = &streamCmdParams{}

func runStream(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived termination signal. Closing stream...")
		cancel()
	}()

	jobClient, err := client.NewJobClient(cfg.ServerAddr)
	if err != nil {
		return err
	}
	defer jobClient.Close()

	stream, err := jobClient.GetJobsStream(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to start log stream: %v", err)
	}

	fmt.Printf("Streaming logs for job %s (Press Ctrl+C to exit):\n", jobID)

	for {
		chunk, e := stream.Recv()
		if e == io.EOF {
			return nil // Clean exit at end of stream
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				// This is an expected error due to our cancellation
				return nil
			}

			if s, ok := status.FromError(e); ok {
				return fmt.Errorf("stream error: %v", s.Message())
			}

			return fmt.Errorf("error receiving stream: %v", e)
		}

		fmt.Printf("%s", chunk.Payload)
	}
}
