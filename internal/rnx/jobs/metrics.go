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

func NewMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <job-uuid>",
		Short: "View job resource metrics (CPU, memory, disk I/O, network)",
		Long: `View resource usage metrics for a running or completed job.

This command shows CPU, memory, I/O, network, and GPU metrics collected
during job execution via cgroups (sampled every ~5 seconds).

For COMPLETED jobs: Shows all metrics from start to finish, then exits
For RUNNING jobs: Shows all metrics from start to current, then continues
                  streaming live updates until job completes

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

For eBPF telematics events (process execution, network connections, etc.),
use the separate 'rnx job telematics' command.

Examples:
  # View resource metrics
  rnx job metrics f47ac10b

  # Output as JSON (one event per line)
  rnx --json job metrics f47ac10b

Metrics Include:
  - CPU: Usage percentage
  - Memory: Current usage and limit
  - Disk I/O: Read/write bytes
  - Network: RX/TX bytes
  - GPU: Utilization and memory (if allocated)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetrics(cmd, args)
		},
	}

	return cmd
}

func runMetrics(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for Ctrl+C to allow interrupting long-running jobs
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nStopping metrics stream...")
		cancel()
	}()

	eventCount := 0

	// Connect to joblet server
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	// Use StreamJobMetrics for live metrics
	stream, err := jobClient.StreamJobMetrics(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't start reading metrics: %v", err)
	}

	if !common.JSONOutput {
		fmt.Fprintf(os.Stderr, "Streaming metrics (Ctrl+C to stop)...\n\n")
	}

	for {
		event, e := stream.Recv()
		if e == io.EOF {
			if eventCount == 0 {
				return fmt.Errorf("no metrics available for job %s (metrics collection may not be enabled)", jobID)
			}
			return nil // Clean exit at end of stream
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				// This is an expected error due to our cancellation
				return nil
			}

			if s, ok := status.FromError(e); ok {
				return fmt.Errorf("problem reading metrics: %v", s.Message())
			}

			return fmt.Errorf("error receiving metrics stream: %v", e)
		}

		eventCount++

		if common.JSONOutput {
			if err := outputMetricsJSON(event); err != nil {
				return fmt.Errorf("couldn't format output as JSON: %v", err)
			}
		} else {
			outputMetricsEventHuman(event)
		}

		// Stream continues:
		// - For completed jobs: shows all historical metrics then exits at EOF
		// - For running jobs: shows historical + live metrics until job completes or Ctrl+C
	}
}

// outputMetricsJSON outputs a metrics event as a JSON object (one per line for streaming)
func outputMetricsJSON(event *pb.JobMetricsEvent) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(event)
}

// outputMetricsEventHuman outputs a metrics event in human-readable format
func outputMetricsEventHuman(event *pb.JobMetricsEvent) {
	timestamp := time.Unix(0, event.Timestamp).Format("15:04:05.000")

	fmt.Printf("\n=== Metrics at %s ===\n", timestamp)
	fmt.Printf("Job ID: %s\n\n", event.JobId)

	// CPU
	fmt.Printf("CPU: %.2f%%\n", event.CpuPercent)

	// Memory
	fmt.Printf("\nMemory:\n")
	fmt.Printf("  Current: %s\n", formatBytesInt(event.MemoryBytes))
	if event.MemoryLimit > 0 {
		percent := float64(event.MemoryBytes) / float64(event.MemoryLimit) * 100
		fmt.Printf("  Limit: %s (%.1f%% used)\n", formatBytesInt(event.MemoryLimit), percent)
	}

	// Disk I/O
	fmt.Printf("\nDisk I/O:\n")
	fmt.Printf("  Read: %s\n", formatBytesInt(event.DiskReadBytes))
	fmt.Printf("  Write: %s\n", formatBytesInt(event.DiskWriteBytes))

	// Network
	fmt.Printf("\nNetwork:\n")
	fmt.Printf("  RX: %s\n", formatBytesInt(event.NetRecvBytes))
	fmt.Printf("  TX: %s\n", formatBytesInt(event.NetSentBytes))

	// GPU (if present)
	if event.GpuPercent > 0 || event.GpuMemoryBytes > 0 {
		fmt.Printf("\nGPU:\n")
		fmt.Printf("  Utilization: %.1f%%\n", event.GpuPercent)
		fmt.Printf("  Memory: %s\n", formatBytesInt(event.GpuMemoryBytes))
	}
}

// formatBytesUint converts uint64 bytes to human-readable format
func formatBytesUint(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatBytesInt converts int64 bytes to human-readable format
func formatBytesInt(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	return formatBytesUint(uint64(bytes))
}
