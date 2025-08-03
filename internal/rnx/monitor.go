package rnx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	pb "joblet/api/gen"
)

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor system metrics",
		Long:  "Monitor system metrics including CPU, memory, disk, network, and processes",
	}

	cmd.AddCommand(newMonitorStatusCmd())
	cmd.AddCommand(newMonitorTopCmd())
	cmd.AddCommand(newMonitorWatchCmd())

	return cmd
}

func newMonitorStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current system status",
		Long: `Display comprehensive system status including:
- Host information (OS, kernel, uptime)
- CPU metrics (usage, cores, load average)
- Memory usage (total, used, available, swap)
- Disk usage by mount point
- Network interfaces and traffic
- Process statistics

Examples:
  rnx monitor status
  rnx monitor status --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorStatus(jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

func newMonitorTopCmd() *cobra.Command {
	var metricTypes []string

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show current system metrics",
		Long: `Display current system metrics in a condensed format.
Optionally filter by metric types.

Available metric types:
- cpu: CPU usage and load
- memory: Memory and swap usage  
- disk: Disk usage by mount point
- network: Network interface statistics
- io: I/O operations and throughput
- process: Process statistics

Examples:
  rnx monitor top
  rnx monitor top --filter=cpu,memory
  rnx monitor top --filter=disk,network`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorTop(metricTypes)
		},
	}

	cmd.Flags().StringSliceVar(&metricTypes, "filter", nil, "Comma-separated list of metric types to display")

	return cmd
}

func newMonitorWatchCmd() *cobra.Command {
	var (
		interval    int
		metricTypes []string
		compact     bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch system metrics in real-time",
		Long: `Stream system metrics in real-time with automatic refresh.
Press Ctrl+C to stop watching.

Available metric types:
- cpu: CPU usage and load
- memory: Memory and swap usage
- disk: Disk usage by mount point  
- network: Network interface statistics
- io: I/O operations and throughput
- process: Process statistics

Examples:
  rnx monitor watch
  rnx monitor watch --interval=2
  rnx monitor watch --interval=5 --filter=cpu,memory
  rnx monitor watch --compact`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorWatch(interval, metricTypes, compact)
		},
	}

	cmd.Flags().IntVar(&interval, "interval", 5, "Update interval in seconds")
	cmd.Flags().StringSliceVar(&metricTypes, "filter", nil, "Comma-separated list of metric types to display")
	cmd.Flags().BoolVar(&compact, "compact", false, "Use compact single-line format for network display")

	return cmd
}

// Command implementations

func runMonitorStatus(jsonOutput bool) error {
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := jobClient.GetSystemStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get system status: %v", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	displaySystemStatus(resp)
	return nil
}

func runMonitorTop(metricTypes []string) error {
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use GetSystemStatus to get the current metrics
	resp, err := jobClient.GetSystemStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get system status: %v", err)
	}

	// Convert to SystemMetricsRes format for display
	metrics := &pb.SystemMetricsRes{
		Timestamp: resp.Timestamp,
		Host:      resp.Host,
		Cpu:       resp.Cpu,
		Memory:    resp.Memory,
		Disks:     resp.Disks,
		Networks:  resp.Networks,
		Io:        resp.Io,
		Processes: resp.Processes,
		Cloud:     resp.Cloud,
	}

	displaySystemMetrics(metrics)
	return nil
}

func runMonitorWatch(interval int, metricTypes []string, compact bool) error {
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &pb.StreamMetricsReq{
		IntervalSeconds: int32(interval),
		MetricTypes:     metricTypes,
	}

	stream, err := jobClient.StreamSystemMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start metrics stream: %v", err)
	}

	fmt.Printf("Watching system metrics (interval: %ds). Press Ctrl+C to stop.\n\n", interval)

	for {
		resp, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("stream error: %v", err)
		}

		// Clear screen and move cursor to top
		fmt.Print("\033[2J\033[H")
		fmt.Printf("System Metrics - %s (refreshing every %ds)\n\n", resp.Timestamp, interval)

		displaySystemMetricsWithOptions(resp, compact)
	}
}

// Display functions

func displaySystemStatus(status *pb.SystemStatusRes) {
	fmt.Printf("System Status - %s\n", status.Timestamp)
	fmt.Printf("Available: %v\n\n", status.Available)

	// Host Information
	if status.Host != nil {
		fmt.Printf("Host Information:\n")
		fmt.Printf("  Hostname:     %s\n", status.Host.Hostname)
		fmt.Printf("  OS:           %s\n", status.Host.Os)
		fmt.Printf("  Kernel:       %s\n", status.Host.KernelVersion)
		fmt.Printf("  Architecture: %s\n", status.Host.Architecture)
		fmt.Printf("  Uptime:       %s\n", formatDurationFromSeconds(status.Host.Uptime))
		fmt.Printf("  Boot Time:    %s\n", status.Host.BootTime)
		fmt.Println()
	}

	// CPU Information
	if status.Cpu != nil {
		fmt.Printf("CPU:\n")
		fmt.Printf("  Cores:        %d\n", status.Cpu.Cores)
		fmt.Printf("  Usage:        %.1f%%\n", status.Cpu.UsagePercent)
		fmt.Printf("  Load Average: %.2f, %.2f, %.2f\n",
			status.Cpu.LoadAverage[0], status.Cpu.LoadAverage[1], status.Cpu.LoadAverage[2])
		fmt.Printf("  User Time:    %.1f%%\n", status.Cpu.UserTime)
		fmt.Printf("  System Time:  %.1f%%\n", status.Cpu.SystemTime)
		fmt.Printf("  Idle Time:    %.1f%%\n", status.Cpu.IdleTime)
		fmt.Printf("  I/O Wait:     %.1f%%\n", status.Cpu.IoWaitTime)
		fmt.Println()
	}

	// Memory Information
	if status.Memory != nil {
		fmt.Printf("Memory:\n")
		fmt.Printf("  Total:        %s\n", formatBytes(status.Memory.TotalBytes))
		fmt.Printf("  Used:         %s (%.1f%%)\n",
			formatBytes(status.Memory.UsedBytes), status.Memory.UsagePercent)
		fmt.Printf("  Available:    %s\n", formatBytes(status.Memory.AvailableBytes))
		fmt.Printf("  Free:         %s\n", formatBytes(status.Memory.FreeBytes))
		fmt.Printf("  Cached:       %s\n", formatBytes(status.Memory.CachedBytes))
		fmt.Printf("  Buffered:     %s\n", formatBytes(status.Memory.BufferedBytes))
		if status.Memory.SwapTotal > 0 {
			fmt.Printf("  Swap Total:   %s\n", formatBytes(status.Memory.SwapTotal))
			fmt.Printf("  Swap Used:    %s\n", formatBytes(status.Memory.SwapUsed))
		}
		fmt.Println()
	}

	// Disk Information
	if len(status.Disks) > 0 {
		fmt.Printf("Disks:\n")
		for _, disk := range status.Disks {
			fmt.Printf("  %s (%s):\n", disk.MountPoint, disk.Device)
			fmt.Printf("    Filesystem:   %s\n", disk.Filesystem)
			fmt.Printf("    Total:        %s\n", formatBytes(disk.TotalBytes))
			fmt.Printf("    Used:         %s (%.1f%%)\n",
				formatBytes(disk.UsedBytes), disk.UsagePercent)
			fmt.Printf("    Available:    %s\n", formatBytes(disk.FreeBytes))
			fmt.Printf("    Inodes:       %d/%d\n", disk.InodesUsed, disk.InodesTotal)
		}
		fmt.Println()
	}

	// Network Information
	if len(status.Networks) > 0 {
		fmt.Printf("Network Interfaces:\n")
		for _, net := range status.Networks {
			fmt.Printf("  %s:\n", net.Interface)
			fmt.Printf("    RX: %s (%d packets, %d errors)\n",
				formatBytes(net.BytesReceived), net.PacketsReceived, net.ErrorsIn)
			fmt.Printf("    TX: %s (%d packets, %d errors)\n",
				formatBytes(net.BytesSent), net.PacketsSent, net.ErrorsOut)
			fmt.Printf("    Rate: ↓ %s/s ↑ %s/s\n",
				formatBytes(int64(net.ReceiveRate)), formatBytes(int64(net.TransmitRate)))
		}
		fmt.Println()
	}

	// Process Information
	if status.Processes != nil {
		fmt.Printf("Processes:\n")
		fmt.Printf("  Total:    %d\n", status.Processes.TotalProcesses)
		fmt.Printf("  Running:  %d\n", status.Processes.RunningProcesses)
		fmt.Printf("  Zombie:   %d\n", status.Processes.ZombieProcesses)
		fmt.Printf("  Threads:  %d\n", status.Processes.TotalThreads)
		fmt.Println()
	}

	// Cloud Information
	if status.Cloud != nil {
		fmt.Printf("Cloud Environment:\n")
		fmt.Printf("  Provider:     %s\n", status.Cloud.Provider)
		fmt.Printf("  Region:       %s\n", status.Cloud.Region)
		fmt.Printf("  Zone:         %s\n", status.Cloud.Zone)
		fmt.Printf("  Instance ID:  %s\n", status.Cloud.InstanceID)
		fmt.Printf("  Instance Type: %s\n", status.Cloud.InstanceType)
		fmt.Println()
	}

}

func displaySystemMetrics(metrics *pb.SystemMetricsRes) {
	fmt.Printf("System Metrics - %s\n\n", metrics.Timestamp)

	// CPU
	if metrics.Cpu != nil {
		fmt.Printf("CPU Usage: %.1f%% (%d cores) | Load: %.2f %.2f %.2f\n",
			metrics.Cpu.UsagePercent, metrics.Cpu.Cores,
			metrics.Cpu.LoadAverage[0], metrics.Cpu.LoadAverage[1], metrics.Cpu.LoadAverage[2])
	}

	// Processes
	if metrics.Processes != nil {
		fmt.Printf("Processes: %d total, %d running, %d threads\n",
			metrics.Processes.TotalProcesses,
			metrics.Processes.RunningProcesses,
			metrics.Processes.TotalThreads)

		// Show top processes table if data is available
		if len(metrics.Processes.TopByCPU) > 0 {
			fmt.Printf("Top Processes by CPU:\n")
			fmt.Printf("  %-6s │ %-16s │ %6s │ %6s │ %8s │ %s\n",
				"PID", "Name", "CPU%", "Mem%", "Memory", "Command")
			fmt.Printf("  ───────┼──────────────────┼────────┼────────┼──────────┼──────────\n")

			count := 0
			for _, proc := range metrics.Processes.TopByCPU {
				if count >= 10 {
					break
				}

				// Truncate name if too long
				name := proc.Name
				if len(name) > 16 {
					name = name[:13] + "..."
				}

				// Truncate command if too long
				command := proc.Command
				if len(command) > 35 {
					command = command[:32] + "..."
				}

				fmt.Printf("  %-6d │ %-16s │ %5.1f%% │ %5.1f%% │ %8s │ %s\n",
					proc.Pid,
					name,
					proc.CpuPercent,
					proc.MemoryPercent,
					formatBytes(proc.MemoryBytes),
					command)
				count++
			}
		}
	}

	// Memory
	if metrics.Memory != nil {
		fmt.Printf("Memory: %s/%s (%.1f%%) | Available: %s\n",
			formatBytes(metrics.Memory.UsedBytes),
			formatBytes(metrics.Memory.TotalBytes),
			metrics.Memory.UsagePercent,
			formatBytes(metrics.Memory.AvailableBytes))
	}

	// Disks (show top 3 by usage)
	if len(metrics.Disks) > 0 {
		fmt.Printf("Disks: ")
		count := 0
		for _, disk := range metrics.Disks {
			if count > 0 {
				fmt.Printf(" | ")
			}
			fmt.Printf("%s: %.1f%%", disk.MountPoint, disk.UsagePercent)
			count++
			if count >= 3 {
				break
			}
		}
		fmt.Println()
	}

	// Network (show active interfaces in organized table format)
	if len(metrics.Networks) > 0 {
		fmt.Printf("Network Interfaces:\n")
		fmt.Printf("  %-12s │ %12s │ %12s │ %10s │ %10s │ %20s │ %s\n",
			"Interface", "RX Rate", "TX Rate", "RX Total", "TX Total", "Packets", "Errors")
		fmt.Printf("  ─────────────────┼──────────────┼──────────────┼────────────┼────────────┼──────────────────────┼────────\n")

		activeCount := 0
		for _, net := range metrics.Networks {
			if net.BytesReceived > 0 || net.BytesSent > 0 {
				// Format packet info
				packetInfo := ""
				if net.PacketsReceived > 0 || net.PacketsSent > 0 {
					packetInfo = fmt.Sprintf("%d↓ %d↑", net.PacketsReceived, net.PacketsSent)
				} else {
					packetInfo = "─"
				}

				// Format error info
				errorInfo := ""
				if net.ErrorsIn > 0 || net.ErrorsOut > 0 {
					errorInfo = fmt.Sprintf("⚠ %d↓ %d↑", net.ErrorsIn, net.ErrorsOut)
				} else {
					errorInfo = "─"
				}

				fmt.Printf("  %-12s │ %12s │ %12s │ %10s │ %10s │ %20s │ %s\n",
					net.Interface,
					formatBytes(int64(net.ReceiveRate))+"/s",
					formatBytes(int64(net.TransmitRate))+"/s",
					formatBytes(net.BytesReceived),
					formatBytes(net.BytesSent),
					packetInfo,
					errorInfo)

				activeCount++
			}
		}
		if activeCount == 0 {
			fmt.Printf("  No active interfaces\n")
		}
	}

	// I/O Statistics
	if metrics.Io != nil {
		fmt.Printf("I/O: %s read (%d ops) | %s write (%d ops)\n",
			formatBytes(metrics.Io.ReadBytes),
			metrics.Io.TotalReads,
			formatBytes(metrics.Io.WriteBytes),
			metrics.Io.TotalWrites)
	}

	// Processes
	if metrics.Processes != nil {
		fmt.Printf("Processes: %d total, %d running, %d threads\n",
			metrics.Processes.TotalProcesses,
			metrics.Processes.RunningProcesses,
			metrics.Processes.TotalThreads)
	}

	fmt.Println()
}

func displaySystemMetricsWithOptions(metrics *pb.SystemMetricsRes, compact bool) {
	fmt.Printf("System Metrics - %s\n\n", metrics.Timestamp)

	// CPU
	if metrics.Cpu != nil {
		cpuLine := fmt.Sprintf("CPU Usage: %.1f%% (%d cores)", metrics.Cpu.UsagePercent, metrics.Cpu.Cores)

		// Add user/system/idle breakdown if available
		if metrics.Cpu.UserTime > 0 || metrics.Cpu.SystemTime > 0 || metrics.Cpu.IdleTime > 0 {
			cpuLine += fmt.Sprintf(" [User: %.1f%%, System: %.1f%%, Idle: %.1f%%]",
				metrics.Cpu.UserTime, metrics.Cpu.SystemTime, metrics.Cpu.IdleTime)
		}

		// Add I/O wait if significant
		if metrics.Cpu.IoWaitTime > 0.1 {
			cpuLine += fmt.Sprintf(" [I/O Wait: %.1f%%]", metrics.Cpu.IoWaitTime)
		}

		// Add load averages
		cpuLine += fmt.Sprintf(" | Load: %.2f %.2f %.2f",
			metrics.Cpu.LoadAverage[0], metrics.Cpu.LoadAverage[1], metrics.Cpu.LoadAverage[2])

		// Add per-core usage if available and compact enough (<=8 cores)
		if len(metrics.Cpu.PerCoreUsage) > 0 && len(metrics.Cpu.PerCoreUsage) <= 8 {
			cpuLine += " | Cores: "
			for i, usage := range metrics.Cpu.PerCoreUsage {
				if i > 0 {
					cpuLine += ","
				}
				cpuLine += fmt.Sprintf("%.0f%%", usage)
			}
		}

		fmt.Println(cpuLine)
	}

	// Memory
	if metrics.Memory != nil {
		fmt.Printf("Memory: %s/%s (%.1f%%) | Available: %s\n",
			formatBytes(metrics.Memory.UsedBytes),
			formatBytes(metrics.Memory.TotalBytes),
			metrics.Memory.UsagePercent,
			formatBytes(metrics.Memory.AvailableBytes))
	}

	// Disks (show top 3 by usage)
	if len(metrics.Disks) > 0 {
		fmt.Printf("Disks: ")
		count := 0
		for _, disk := range metrics.Disks {
			if count > 0 {
				fmt.Printf(" | ")
			}
			fmt.Printf("%s: %.1f%%", disk.MountPoint, disk.UsagePercent)
			count++
			if count >= 3 {
				break
			}
		}
		fmt.Println()
	}

	// Network - compact vs detailed display
	if len(metrics.Networks) > 0 {
		if compact {
			// Original compact single-line format
			fmt.Printf("Network: ")
			count := 0
			for _, net := range metrics.Networks {
				if net.BytesReceived > 0 || net.BytesSent > 0 {
					if count > 0 {
						fmt.Printf(" | ")
					}
					fmt.Printf("%s: ↓%s/s ↑%s/s",
						net.Interface,
						formatBytes(int64(net.ReceiveRate)),
						formatBytes(int64(net.TransmitRate)))
					count++
				}
			}
			if count == 0 {
				fmt.Printf("No active interfaces")
			}
			fmt.Println()
		} else {
			// Detailed organized table format
			fmt.Printf("Network Interfaces:\n")
			fmt.Printf("  %-20s │ %12s │ %12s │ %10s │ %10s │ %20s │ %s\n",
				"Interface", "RX Rate", "TX Rate", "RX Total", "TX Total", "Packets", "Errors")
			fmt.Printf("  ─────────────────────┼──────────────┼──────────────┼────────────┼────────────┼──────────────────────┼────────\n")

			activeCount := 0
			for _, net := range metrics.Networks {
				if net.BytesReceived > 0 || net.BytesSent > 0 {
					if activeCount >= 10 {
						break
					}

					// Format packet info
					packetInfo := ""
					if net.PacketsReceived > 0 || net.PacketsSent > 0 {
						packetInfo = fmt.Sprintf("%d↓ %d↑", net.PacketsReceived, net.PacketsSent)
					} else {
						packetInfo = "─"
					}

					// Format error info
					errorInfo := ""
					if net.ErrorsIn > 0 || net.ErrorsOut > 0 {
						errorInfo = fmt.Sprintf("⚠ %d↓ %d↑", net.ErrorsIn, net.ErrorsOut)
					} else {
						errorInfo = "─"
					}

					fmt.Printf("  %-20s │ %12s │ %12s │ %10s │ %10s │ %20s │ %s\n",
						net.Interface,
						formatBytes(int64(net.ReceiveRate))+"/s",
						formatBytes(int64(net.TransmitRate))+"/s",
						formatBytes(net.BytesReceived),
						formatBytes(net.BytesSent),
						packetInfo,
						errorInfo)

					activeCount++
				}
			}
			if activeCount == 0 {
				fmt.Printf("  No active interfaces\n")
			}
		}
	}

	// I/O Statistics
	if metrics.Io != nil {
		fmt.Printf("I/O: %s read (%d ops) | %s write (%d ops)\n",
			formatBytes(metrics.Io.ReadBytes),
			metrics.Io.TotalReads,
			formatBytes(metrics.Io.WriteBytes),
			metrics.Io.TotalWrites)
	}

	// Top Processes by CPU
	if metrics.Processes != nil && len(metrics.Processes.TopByCPU) > 0 {
		fmt.Printf("Top Processes by CPU:\n")
		fmt.Printf("  %-6s │ %-16s │ %6s │ %6s │ %8s │ %s\n",
			"PID", "Name", "CPU%", "Mem%", "Memory", "Command")
		fmt.Printf("  ───────┼──────────────────┼────────┼────────┼──────────┼──────────\n")

		count := 0
		for _, proc := range metrics.Processes.TopByCPU {
			if count >= 10 {
				break
			}

			// Truncate name if too long
			name := proc.Name
			if len(name) > 16 {
				name = name[:13] + "..."
			}

			// Truncate command if too long
			command := proc.Command
			if len(command) > 35 {
				command = command[:32] + "..."
			}

			fmt.Printf("  %-6d │ %-16s │ %5.1f%% │ %5.1f%% │ %8s │ %s\n",
				proc.Pid,
				name,
				proc.CpuPercent,
				proc.MemoryPercent,
				formatBytes(proc.MemoryBytes),
				command)
			count++
		}
	}

	fmt.Println()
}

// Utility functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDurationFromSeconds(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	hours := (duration % (24 * time.Hour)) / time.Hour
	minutes := (duration % time.Hour) / time.Minute

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}
