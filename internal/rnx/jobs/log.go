package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
)

func NewLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <job-uuid>",
		Short: "Stream job logs",
		Long: `Stream logs from a running or completed job in real-time.

This command follows the log stream for running jobs and shows
all output for completed jobs. Use Ctrl+C to stop following a log stream.

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

Examples:
  # Stream logs from a running job (full UUID)
  rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479

  # Stream logs from a running job (short-form UUID)
  rnx job log f47ac10b

  # View logs from a completed job (short-form UUID)
  rnx job log a1b2c3d4

  # Stop following with Ctrl+C for running jobs`,
		Args: cobra.ExactArgs(1),
		RunE: runLog,
	}

	return cmd
}

func runLog(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nStopping log stream...")
		cancel()
	}()

	// Connect to joblet server
	// Note: GetJobLogs automatically streams BOTH historical (from persist) and live logs
	// The server handles fetching history first, then streaming live updates seamlessly
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	stream, err := jobClient.GetJobLogs(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't start reading logs: %v", err)
	}

	// Stream all logs (server provides historical + live seamlessly)
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
				return fmt.Errorf("problem reading logs: %v", s.Message())
			}

			return fmt.Errorf("error receiving log stream: %v", e)
		}

		if common.JSONOutput {
			if err := outputLogChunkJSON(chunk); err != nil {
				return fmt.Errorf("couldn't format output as JSON: %v", err)
			}
		} else {
			fmt.Printf("%s", chunk.Payload)
		}
	}
}

// outputLogChunkJSON outputs a log chunk as a JSON object (one per line for streaming)
func outputLogChunkJSON(chunk *pb.DataChunk) error {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      string(chunk.Payload),
	}

	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(logEntry)
}
