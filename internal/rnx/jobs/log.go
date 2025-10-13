package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "github.com/ehsaniara/joblet/api/gen"
	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
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

	// Step 1: Try to fetch historical logs from joblet-persist first
	if err := streamHistoricalLogs(ctx, jobID); err != nil {
		// If persist is unavailable, not implemented, or has no data, that's OK - we'll just stream live logs
		// Only show warning for unexpected errors
		if !errors.Is(err, io.EOF) && !isUnavailableError(err) && !isNotImplementedError(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: couldn't fetch historical logs: %v\n", err)
		}
	}

	// Step 2: Stream live logs from joblet-core
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	stream, err := jobClient.GetJobLogs(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't start reading logs: %v", err)
	}

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

// streamHistoricalLogs fetches and displays historical logs from joblet-persist
func streamHistoricalLogs(ctx context.Context, jobID string) error {
	persistClient, err := common.NewPersistClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet-persist: %w", err)
	}
	defer persistClient.Close()

	// Query all historical logs (no limit)
	req := &persistpb.QueryLogsRequest{
		JobId: jobID,
		// stream: UNSPECIFIED means both STDOUT and STDERR
		Limit:  0, // No limit - fetch all
		Offset: 0,
	}

	stream, err := persistClient.QueryLogs(ctx, req)
	if err != nil {
		return fmt.Errorf("couldn't query historical logs: %w", err)
	}

	// Read all historical log lines
	for {
		logLine, e := stream.Recv()
		if e == io.EOF {
			// End of historical logs - this is expected
			return nil
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			// Check for expected/non-critical errors
			if s, ok := status.FromError(e); ok {
				switch s.Code() {
				case codes.NotFound:
					// No historical logs exist - silently continue to live logs
					return nil
				case codes.Unimplemented:
					// QueryLogs not implemented yet - silently continue to live logs
					return nil
				case codes.Unknown:
					// Check if it's the "not implemented yet" message
					if strings.Contains(s.Message(), "not implemented") {
						return nil
					}
				}
			}
			return fmt.Errorf("error receiving historical log: %v", e)
		}

		// Output the historical log line
		if common.JSONOutput {
			logEntry := map[string]interface{}{
				"timestamp": time.Unix(0, logLine.Timestamp).Format(time.RFC3339),
				"data":      logLine.Content,
				"stream":    logLine.Stream.String(),
				"sequence":  logLine.Sequence,
			}
			encoder := json.NewEncoder(os.Stdout)
			if err := encoder.Encode(logEntry); err != nil {
				return fmt.Errorf("couldn't format log as JSON: %v", err)
			}
		} else {
			fmt.Printf("%s", logLine.Content)
		}
	}
}

// isUnavailableError checks if the error is due to service being unavailable
func isUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.Unavailable
}

// isNotImplementedError checks if the error is due to feature not being implemented
func isNotImplementedError(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	if !ok {
		return false
	}
	// Check for Unimplemented code or Unknown with "not implemented" message
	return s.Code() == codes.Unimplemented ||
		(s.Code() == codes.Unknown && strings.Contains(s.Message(), "not implemented"))
}
