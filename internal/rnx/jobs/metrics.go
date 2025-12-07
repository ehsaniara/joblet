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

// ANSI color codes for telemetry output
const (
	metricsColorCyan   = "\033[36m"
	metricsColorYellow = "\033[33m"
	metricsColorReset  = "\033[0m"
)

func NewMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <job-uuid>",
		Short: "View job resource metrics and eBPF telemetry events",
		Long: `View resource usage metrics for a running or completed job.

This command shows CPU, memory, I/O, network, and GPU metrics collected
during job execution via the unified telemetry stream.

For COMPLETED jobs: Shows all metrics from start to finish, then exits
For RUNNING jobs: Shows all metrics from start to current, then continues
                  streaming live updates until job completes

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

eBPF Telemetry (--tel flag):
Use --tel to include eBPF visibility events for security monitoring:
  • EXEC: Process executions (fork/exec syscalls)
  • NET: Outgoing network connections (connect syscall)
  • ACCEPT: Incoming network connections (accept syscall)
  • SEND/RECV: Socket data transfers (sendto/recvfrom syscalls)
  • MMAP: Memory mappings with executable permissions
  • MPROTECT: Memory protection changes adding exec permission

Examples:
  # View only resource metrics
  rnx job metrics f47ac10b

  # View metrics + all eBPF telemetry events
  rnx job metrics f47ac10b --tel

  # Filter specific event types with grep
  rnx job metrics f47ac10b --tel | grep EXEC
  rnx job metrics f47ac10b --tel | grep NET
  rnx job metrics f47ac10b --tel | grep ACCEPT
  rnx job metrics f47ac10b --tel | grep MMAP

  # Output as JSON (one event per line)
  rnx --json job metrics f47ac10b

Metrics Include:
  • CPU: Usage percentage
  • Memory: Current usage and limit
  • Disk I/O: Read/write bytes
  • Network: RX/TX bytes
  • GPU: Utilization and memory (if allocated)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetrics(cmd, args)
		},
	}

	cmd.Flags().Bool("tel", false, "Include eBPF telemetry events (process executions + network connections)")

	return cmd
}

func runMetrics(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	showTel, _ := cmd.Flags().GetBool("tel")

	// Build list of event types to stream
	eventTypes := []string{"metrics"}
	if showTel {
		eventTypes = append(eventTypes, "exec", "connect")
	}

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

	// Use StreamJobTelemetry for live telemetry
	stream, err := jobClient.StreamJobTelemetry(ctx, jobID, eventTypes)
	if err != nil {
		return fmt.Errorf("couldn't start reading metrics: %v", err)
	}

	if !common.JSONOutput {
		if showTel {
			fmt.Fprintf(os.Stderr, "Streaming metrics + eBPF telemetry (Ctrl+C to stop)...\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "Streaming metrics (Ctrl+C to stop)...\n\n")
		}
	}

	for {
		event, e := stream.Recv()
		if e == io.EOF {
			if eventCount == 0 {
				return fmt.Errorf("no telemetry available for job %s (metrics collection may not be enabled)", jobID)
			}
			return nil // Clean exit at end of stream
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				// This is an expected error due to our cancellation
				return nil
			}

			if s, ok := status.FromError(e); ok {
				return fmt.Errorf("problem reading telemetry: %v", s.Message())
			}

			return fmt.Errorf("error receiving telemetry stream: %v", e)
		}

		eventCount++

		if common.JSONOutput {
			if err := outputTelemetryJSON(event); err != nil {
				return fmt.Errorf("couldn't format output as JSON: %v", err)
			}
		} else {
			outputTelemetryEventHuman(event)
		}

		// Stream continues:
		// - For completed jobs: shows all historical events then exits at EOF
		// - For running jobs: shows historical + live events until job completes or Ctrl+C
	}
}

// outputTelemetryJSON outputs a telemetry event as a JSON object (one per line for streaming)
func outputTelemetryJSON(event *pb.TelemetryEvent) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(event)
}

// outputTelemetryEventHuman outputs a telemetry event in human-readable format
func outputTelemetryEventHuman(event *pb.TelemetryEvent) {
	timestamp := time.Unix(0, event.Timestamp).Format("15:04:05.000")

	switch event.Type {
	case "metrics":
		metrics := event.GetMetrics()
		if metrics == nil {
			return
		}

		fmt.Printf("\n═══ Metrics at %s ═══\n", timestamp)
		fmt.Printf("Job ID: %s\n\n", event.JobId)

		// CPU
		fmt.Printf("CPU: %.2f%%\n", metrics.CpuPercent)

		// Memory
		fmt.Printf("\nMemory:\n")
		fmt.Printf("  Current: %s\n", formatBytesInt(metrics.MemoryBytes))
		if metrics.MemoryLimit > 0 {
			percent := float64(metrics.MemoryBytes) / float64(metrics.MemoryLimit) * 100
			fmt.Printf("  Limit: %s (%.1f%% used)\n", formatBytesInt(metrics.MemoryLimit), percent)
		}

		// Disk I/O
		fmt.Printf("\nDisk I/O:\n")
		fmt.Printf("  Read: %s\n", formatBytesInt(metrics.DiskReadBytes))
		fmt.Printf("  Write: %s\n", formatBytesInt(metrics.DiskWriteBytes))

		// Network
		fmt.Printf("\nNetwork:\n")
		fmt.Printf("  RX: %s\n", formatBytesInt(metrics.NetRecvBytes))
		fmt.Printf("  TX: %s\n", formatBytesInt(metrics.NetSentBytes))

		// GPU (if present)
		if metrics.GpuPercent > 0 || metrics.GpuMemoryBytes > 0 {
			fmt.Printf("\nGPU:\n")
			fmt.Printf("  Utilization: %.1f%%\n", metrics.GpuPercent)
			fmt.Printf("  Memory: %s\n", formatBytesInt(metrics.GpuMemoryBytes))
		}

	case "exec":
		if exec := event.GetExec(); exec != nil {
			fmt.Printf("[%s] %sEXEC%s    pid=%d ppid=%d %s\n",
				timestamp,
				metricsColorCyan,
				metricsColorReset,
				exec.Pid,
				exec.Ppid,
				exec.Binary,
			)
		}

	case "connect":
		if conn := event.GetConnect(); conn != nil {
			fmt.Printf("[%s] %sNET%s     pid=%d %s:%d (%s)\n",
				timestamp,
				metricsColorYellow,
				metricsColorReset,
				conn.Pid,
				conn.Address,
				conn.Port,
				conn.Protocol,
			)
		}
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
