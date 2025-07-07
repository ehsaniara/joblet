package rnx

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <job-id>",
		Short: "Stream job logs",
		Args:  cobra.ExactArgs(1),
		RunE:  runLog,
	}

	cmd.Flags().BoolVarP(&logParams.follow, "follow", "f", true, "Follow the log stream (can be terminated with Ctrl+C)")

	return cmd
}

type logCmdParams struct {
	follow bool
}

var logParams = &logCmdParams{}

func runLog(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived termination signal. Closing log stream...")
		cancel()
	}()

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	stream, err := jobClient.GetJobLogs(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to start log stream: %v", err)
	}

	//fmt.Printf("Logs for job %s (Press Ctrl+C to exit if streaming):\n", jobID)

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
				return fmt.Errorf("log stream error: %v", s.Message())
			}

			return fmt.Errorf("error receiving log stream: %v", e)
		}

		fmt.Printf("%s", chunk.Payload)
	}
}
