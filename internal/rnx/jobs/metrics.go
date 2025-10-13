package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	pb "github.com/ehsaniara/joblet/api/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics <job-uuid>",
		Short: "View job resource metrics",
		Long: `View resource usage metrics for a running or completed job.

This command shows CPU, memory, I/O, network, and process metrics collected
during job execution. Metrics are stored as time-series data.

For COMPLETED jobs: Shows all metrics from start to finish, then exits
For RUNNING jobs: Shows all metrics from start to current, then continues
                  streaming live updates until job completes

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

Examples:
  # View metrics for a completed job (shows complete history)
  rnx job metrics f47ac10b-58cc-4372-a567-0e02b2c3d479

  # Monitor a running job (shows history + live stream)
  rnx job metrics a1b2c3d4

  # Output as JSON (one sample per line)
  rnx --json job metrics f47ac10b

Metrics Include:
  • CPU: Usage %, user/system time, throttling
  • Memory: Current/peak usage, anonymous/file cache, page faults
  • I/O: Read/write bandwidth, IOPS, total bytes
  • Network: RX/TX bytes/packets, bandwidth
  • Process: Count, threads, open file descriptors
  • GPU: Utilization, memory, temperature, power (if allocated)`,
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

	sampleCount := 0

	// Step 1: Try to fetch historical metrics from joblet-persist first
	historicalCount, err := streamHistoricalMetrics(ctx, jobID)
	if err != nil {
		// If persist is unavailable or has no data, that's OK - we'll just stream live metrics
		// Only fail if it's a critical error (e.g., config issue)
		if !errors.Is(err, io.EOF) && !isUnavailableError(err) {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: couldn't fetch historical metrics: %v\n", err)
		}
	}
	sampleCount += historicalCount

	// Step 2: Stream live metrics from joblet-core
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	stream, err := jobClient.StreamJobMetrics(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't start reading metrics: %v", err)
	}

	for {
		sample, e := stream.Recv()
		if e == io.EOF {
			if sampleCount == 0 {
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

		sampleCount++

		if common.JSONOutput {
			if err := outputMetricsJSON(sample); err != nil {
				return fmt.Errorf("couldn't format output as JSON: %v", err)
			}
		} else {
			outputMetricsHuman(sample)
		}

		// Stream continues:
		// - For completed jobs: shows all historical metrics then exits at EOF
		// - For running jobs: shows historical + live metrics until job completes or Ctrl+C
	}
}

// outputMetricsJSON outputs a metrics sample as a JSON object (one per line for streaming)
func outputMetricsJSON(sample *pb.JobMetricsSample) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(sample)
}

// outputMetricsHuman outputs metrics in a human-readable format
func outputMetricsHuman(sample *pb.JobMetricsSample) {
	timestamp := time.Unix(sample.Timestamp, 0).Format("15:04:05")

	fmt.Printf("\n═══ Metrics Sample at %s ═══\n", timestamp)
	fmt.Printf("Job ID: %s\n", sample.JobId)
	fmt.Printf("Sample Interval: %ds\n\n", sample.SampleIntervalSeconds)

	// CPU Metrics
	if sample.Cpu != nil {
		fmt.Println("CPU:")
		fmt.Printf("  Usage: %.2f%%\n", sample.Cpu.UsagePercent)
		if sample.Cpu.ThrottlePercent > 0 {
			fmt.Printf("  Throttled: %.2f%%\n", sample.Cpu.ThrottlePercent)
		}
		fmt.Printf("  User Time: %d μs\n", sample.Cpu.UserUsec)
		fmt.Printf("  System Time: %d μs\n", sample.Cpu.SystemUsec)
		if sample.Cpu.PressureSome10 > 0 {
			fmt.Printf("  Pressure (10s avg): %.2f%%\n", sample.Cpu.PressureSome10)
		}
	}

	// Memory Metrics
	if sample.Memory != nil {
		fmt.Println("\nMemory:")
		fmt.Printf("  Current: %s (%.2f%%)\n",
			formatBytesUint(sample.Memory.Current),
			sample.Memory.UsagePercent)
		fmt.Printf("  Peak: %s\n", formatBytesUint(sample.Memory.Max))
		fmt.Printf("  Anonymous: %s\n", formatBytesUint(sample.Memory.Anon))
		fmt.Printf("  File Cache: %s\n", formatBytesUint(sample.Memory.File))
		if sample.Memory.PgMajFault > 0 {
			fmt.Printf("  Major Page Faults: %d\n", sample.Memory.PgMajFault)
		}
		if sample.Memory.OomEvents > 0 {
			fmt.Printf("  OOM Events: %d\n", sample.Memory.OomEvents)
		}
	}

	// I/O Metrics
	if sample.Io != nil {
		fmt.Println("\nI/O:")
		fmt.Printf("  Read: %s/s (%.0f ops/s)\n",
			formatBytesFloat(sample.Io.ReadBPS),
			sample.Io.ReadIOPS)
		fmt.Printf("  Write: %s/s (%.0f ops/s)\n",
			formatBytesFloat(sample.Io.WriteBPS),
			sample.Io.WriteIOPS)
		fmt.Printf("  Total Read: %s\n", formatBytesUint(sample.Io.TotalReadBytes))
		fmt.Printf("  Total Write: %s\n", formatBytesUint(sample.Io.TotalWriteBytes))
	}

	// Network Metrics
	if sample.Network != nil {
		fmt.Println("\nNetwork:")
		fmt.Printf("  RX: %s/s (%d packets/s)\n",
			formatBytesFloat(sample.Network.RxBPS),
			sample.Network.TotalRxPackets)
		fmt.Printf("  TX: %s/s (%d packets/s)\n",
			formatBytesFloat(sample.Network.TxBPS),
			sample.Network.TotalTxPackets)
		fmt.Printf("  Total RX: %s\n", formatBytesUint(sample.Network.TotalRxBytes))
		fmt.Printf("  Total TX: %s\n", formatBytesUint(sample.Network.TotalTxBytes))
	}

	// Process Metrics
	if sample.Process != nil {
		fmt.Println("\nProcesses:")
		fmt.Printf("  Count: %d (max: %d)\n", sample.Process.Current, sample.Process.Max)
		if sample.Process.Threads > 0 {
			fmt.Printf("  Threads: %d\n", sample.Process.Threads)
		}
		if sample.Process.OpenFDs > 0 {
			fmt.Printf("  Open FDs: %d/%d\n", sample.Process.OpenFDs, sample.Process.MaxFDs)
		}
	}

	// GPU Metrics
	if len(sample.Gpu) > 0 {
		fmt.Println("\nGPU:")
		for _, gpu := range sample.Gpu {
			fmt.Printf("  GPU %d (%s):\n", gpu.Index, gpu.Name)
			fmt.Printf("    Utilization: %.1f%%\n", gpu.Utilization)
			fmt.Printf("    Memory: %s / %s (%.1f%%)\n",
				formatBytesUint(gpu.MemoryUsed),
				formatBytesUint(gpu.MemoryTotal),
				gpu.MemoryPercent)
			if gpu.Temperature > 0 {
				fmt.Printf("    Temperature: %.1f°C\n", gpu.Temperature)
			}
			if gpu.PowerDraw > 0 {
				fmt.Printf("    Power: %.1fW / %.1fW\n", gpu.PowerDraw, gpu.PowerLimit)
			}
		}
	}

	// Resource Limits
	if sample.Limits != nil {
		fmt.Println("\nResource Limits:")
		if sample.Limits.Cpu > 0 {
			fmt.Printf("  CPU: %d%%\n", sample.Limits.Cpu)
		}
		if sample.Limits.Memory > 0 {
			fmt.Printf("  Memory: %s\n", formatBytesUint(uint64(sample.Limits.Memory)))
		}
		if sample.Limits.Io > 0 {
			fmt.Printf("  I/O: %s/s\n", formatBytesUint(uint64(sample.Limits.Io)))
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

// formatBytesFloat converts float64 bytes to human-readable format
func formatBytesFloat(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := float64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", bytes/div, "KMGTPE"[exp])
}

// streamHistoricalMetrics fetches and displays historical metrics from joblet-persist
// Returns the count of metrics samples displayed
func streamHistoricalMetrics(ctx context.Context, jobID string) (int, error) {
	persistClient, err := common.NewPersistClient()
	if err != nil {
		return 0, fmt.Errorf("couldn't connect to joblet-persist: %w", err)
	}
	defer persistClient.Close()

	// Query all historical metrics (no limit)
	req := &persistpb.QueryMetricsRequest{
		JobId:  jobID,
		Limit:  0, // No limit - fetch all
		Offset: 0,
	}

	stream, err := persistClient.QueryMetrics(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("couldn't query historical metrics: %w", err)
	}

	sampleCount := 0

	// Read all historical metrics
	for {
		metricSample, e := stream.Recv()
		if e == io.EOF {
			// End of historical metrics - this is expected
			return sampleCount, nil
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return sampleCount, nil
			}
			// Check if it's a "not found" error - means no historical metrics exist
			if s, ok := status.FromError(e); ok && s.Code() == codes.NotFound {
				return sampleCount, nil // Silently continue to live metrics
			}
			return sampleCount, fmt.Errorf("error receiving historical metrics: %v", e)
		}

		sampleCount++

		// Convert IPC metric to joblet metric format
		sample := convertIPCMetricToJobMetric(metricSample)

		// Output the historical metric sample
		if common.JSONOutput {
			if err := outputMetricsJSON(sample); err != nil {
				return sampleCount, fmt.Errorf("couldn't format metric as JSON: %v", err)
			}
		} else {
			outputMetricsHuman(sample)
		}
	}
}

// convertIPCMetricToJobMetric converts an IPC Metric to joblet JobMetricsSample
// The IPC Metric format is simpler - just CPU, memory, GPU, disk, and network basics
func convertIPCMetricToJobMetric(ipcMetric *ipcpb.Metric) *pb.JobMetricsSample {
	sample := &pb.JobMetricsSample{
		JobId:                 ipcMetric.JobId,
		Timestamp:             ipcMetric.Timestamp / 1000000, // Convert nanoseconds to seconds
		SampleIntervalSeconds: 1,                             // IPC metrics don't include interval, default to 1s
	}

	if ipcMetric.Data == nil {
		return sample
	}

	// Convert CPU metrics
	if ipcMetric.Data.CpuUsage > 0 {
		sample.Cpu = &pb.JobCPUMetrics{
			UsagePercent: ipcMetric.Data.CpuUsage * 100, // Convert cores to percentage
		}
	}

	// Convert Memory metrics
	if ipcMetric.Data.MemoryUsage > 0 {
		sample.Memory = &pb.JobMemoryMetrics{
			Current: uint64(ipcMetric.Data.MemoryUsage),
		}
	}

	// Convert GPU metrics (simple format from IPC)
	if ipcMetric.Data.GpuUsage > 0 {
		sample.Gpu = []*pb.JobGPUMetrics{
			{
				Index:       0,
				Name:        "GPU",
				Utilization: ipcMetric.Data.GpuUsage * 100, // Convert 0-1 to percentage
			},
		}
	}

	// Convert I/O metrics
	if ipcMetric.Data.DiskIo != nil {
		sample.Io = &pb.JobIOMetrics{
			TotalReadBytes:  uint64(ipcMetric.Data.DiskIo.ReadBytes),
			TotalWriteBytes: uint64(ipcMetric.Data.DiskIo.WriteBytes),
		}
	}

	// Convert Network metrics
	if ipcMetric.Data.NetworkIo != nil {
		sample.Network = &pb.JobNetworkMetrics{
			TotalRxBytes:   uint64(ipcMetric.Data.NetworkIo.RxBytes),
			TotalTxBytes:   uint64(ipcMetric.Data.NetworkIo.TxBytes),
			TotalRxPackets: uint64(ipcMetric.Data.NetworkIo.RxPackets),
			TotalTxPackets: uint64(ipcMetric.Data.NetworkIo.TxPackets),
		}
	}

	return sample
}
