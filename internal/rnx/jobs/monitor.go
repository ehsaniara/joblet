package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"joblet/internal/rnx/common"
	"strings"
	"time"

	pb "joblet/api/gen"

	"github.com/spf13/cobra"
)

func NewMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor remote joblet server metrics",
		Long: `Monitor comprehensive remote joblet server metrics including CPU, memory, disk, network, processes, and volumes.

Remote monitoring provides detailed insights into joblet server resources:
- Server resource utilization (CPU, memory, I/O) 
- Server network interface statistics and throughput
- Server disk usage for all mount points
- Joblet volume usage and availability on the server
- Server processes and resource consumption
- Server cloud environment detection

All commands connect to the remote joblet server and support JSON output for dashboards.`,
	}

	cmd.AddCommand(NewMonitorStatusCmd())
	cmd.AddCommand(NewMonitorTopCmd())
	cmd.AddCommand(NewMonitorWatchCmd())

	return cmd
}

func NewMonitorStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show remote joblet server status",
		Long: `Display comprehensive remote joblet server status including:
- Server host information (OS, kernel, uptime, cloud environment)
- Server CPU metrics (usage, cores, load average, per-core usage)
- Server memory usage (total, used, available, cached, swap)
- Server disk usage for all mount points and joblet volumes
- Server network interfaces with traffic statistics and rates
- Server process statistics (running, sleeping, top consumers)
- Server cloud environment detection (provider, region, instance type)

The --json flag outputs server data in UI-compatible format for dashboards and monitoring tools.

Examples:
  rnx monitor status                    # Human-readable server status
  rnx monitor status --json            # JSON server data for APIs/UIs
  rnx --node=production monitor status # Monitor specific server`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorStatus(common.JSONOutput)
		},
	}

	return cmd
}

func NewMonitorTopCmd() *cobra.Command {
	var metricTypes []string

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show current remote server metrics",
		Long: `Display current remote joblet server metrics in a condensed format with top server processes.
Optionally filter by specific server metric types. Supports JSON output for monitoring integrations.

Available server metric types for filtering:
- cpu: Server CPU usage, load averages, and per-core utilization
- memory: Server memory and swap usage with caching statistics  
- disk: Server disk usage by mount point including joblet volumes
- network: Server network interface statistics with throughput rates
- io: Server I/O operations, throughput, and block device statistics
- process: Server process statistics with top CPU/memory consumers

Examples:
  rnx monitor top                          # Show all server metrics
  rnx monitor top --filter=cpu,memory      # Show only server CPU and memory
  rnx monitor top --json                   # JSON server data for dashboards
  rnx monitor top --filter=disk,network    # Server disk and network only`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorTop(metricTypes, common.JSONOutput)
		},
	}

	cmd.Flags().StringSliceVar(&metricTypes, "filter", nil, "Comma-separated list of metric types to display")

	return cmd
}

func NewMonitorWatchCmd() *cobra.Command {
	var (
		interval    int
		metricTypes []string
		compact     bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch remote server metrics in real-time",
		Long: `Stream remote joblet server metrics in real-time with configurable refresh intervals.
Press Ctrl+C to stop watching. Perfect for live server monitoring and troubleshooting.

Available server metric types for filtering:
- cpu: Server CPU usage, load averages, and per-core utilization
- memory: Server memory and swap usage with detailed breakdowns
- disk: Server disk usage for all mount points and joblet volumes  
- network: Server network interface statistics with live throughput
- io: Server I/O operations, throughput, and utilization
- process: Live server process statistics with top consumers

The --json flag streams JSON objects with server data for real-time monitoring integrations.

Examples:
  rnx monitor watch                            # Watch all server metrics (5s interval)
  rnx monitor watch --interval=2               # Faster server refresh (2s interval)
  rnx monitor watch --filter=cpu,memory        # Monitor specific server metrics only
  rnx monitor watch --compact                  # Use compact server display format
  rnx monitor watch --json --interval=10       # JSON server stream for monitoring tools`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorWatch(interval, metricTypes, compact, common.JSONOutput)
		},
	}

	cmd.Flags().IntVar(&interval, "interval", 5, "Update interval in seconds")
	cmd.Flags().StringSliceVar(&metricTypes, "filter", nil, "Comma-separated list of metric types to display")
	cmd.Flags().BoolVar(&compact, "compact", false, "Use compact single-line format for network display")

	return cmd
}

// Command implementations

func runMonitorStatus(jsonOutput bool) error {
	jobClient, err := common.NewJobClient()
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
		// Transform to UI-expected format
		uiData := transformToUIFormat(resp)
		data, err := json.MarshalIndent(uiData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	displaySystemStatus(resp)
	return nil
}

func runMonitorTop(metricTypes []string, jsonOutput bool) error {
	jobClient, err := common.NewJobClient()
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

	if jsonOutput {
		// Transform to UI-expected format
		uiData := transformToUIFormat(resp)
		data, err := json.MarshalIndent(uiData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	displaySystemMetrics(metrics)
	return nil
}

func runMonitorWatch(interval int, metricTypes []string, compact bool, jsonOutput bool) error {
	jobClient, err := common.NewJobClient()
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

	if !jsonOutput {
		fmt.Printf("Watching system metrics (interval: %ds). Press Ctrl+C to stop.\n\n", interval)
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("stream error: %v", err)
		}

		if jsonOutput {
			// Convert to SystemStatusRes format for UI transformation
			statusResp := &pb.SystemStatusRes{
				Timestamp: resp.Timestamp,
				Available: true,
				Host:      resp.Host,
				Cpu:       resp.Cpu,
				Memory:    resp.Memory,
				Disks:     resp.Disks,
				Networks:  resp.Networks,
				Io:        resp.Io,
				Processes: resp.Processes,
				Cloud:     resp.Cloud,
			}

			// Transform to UI-expected format
			uiData := transformToUIFormat(statusResp)
			data, err := json.MarshalIndent(uiData, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %v", err)
			}
			fmt.Println(string(data))
		} else {
			// Clear screen and move cursor to top
			fmt.Print("\033[2J\033[H")
			fmt.Printf("\nSystem Metrics - %s (refreshing every %ds)\n\n", resp.Timestamp, interval)
			displaySystemMetricsWithOptions(resp, compact)
		}
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
		fmt.Printf("  Sleeping: %d\n", status.Processes.SleepingProcesses)
		fmt.Printf("  Stopped:  %d\n", status.Processes.StoppedProcesses)
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
		fmt.Printf("Processes: %d total, %d running, %d sleeping, %d threads\n",
			metrics.Processes.TotalProcesses,
			metrics.Processes.RunningProcesses,
			metrics.Processes.SleepingProcesses,
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
		fmt.Printf("Processes: %d total, %d running, %d sleeping, %d threads\n",
			metrics.Processes.TotalProcesses,
			metrics.Processes.RunningProcesses,
			metrics.Processes.SleepingProcesses,
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

// UI Format structures for frontend compatibility
type UIFormat struct {
	HostInfo      UIHostInfo      `json:"hostInfo"`
	CPUInfo       UICPUInfo       `json:"cpuInfo"`
	MemoryInfo    UIMemoryInfo    `json:"memoryInfo"`
	DisksInfo     UIDisksInfo     `json:"disksInfo"`
	NetworkInfo   UINetworkInfo   `json:"networkInfo"`
	ProcessesInfo UIProcessesInfo `json:"processesInfo"`
}

type UIHostInfo struct {
	Hostname      string `json:"hostname"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	Release       string `json:"release"`
	Uptime        int64  `json:"uptime"`
	CloudProvider string `json:"cloudProvider"`
	InstanceType  string `json:"instanceType"`
	Region        string `json:"region"`
}

type UICPUInfo struct {
	Cores        int32     `json:"cores"`
	Threads      int32     `json:"threads"`
	Model        string    `json:"model"`
	Frequency    int32     `json:"frequency"`
	Usage        float64   `json:"usage"`
	LoadAverage  []float64 `json:"loadAverage"`
	PerCoreUsage []float64 `json:"perCoreUsage"`
	Temperature  int32     `json:"temperature"`
}

type UIMemoryInfo struct {
	Total     int64      `json:"total"`
	Used      int64      `json:"used"`
	Available int64      `json:"available"`
	Percent   float64    `json:"percent"`
	Buffers   int64      `json:"buffers"`
	Cached    int64      `json:"cached"`
	Swap      UISwapInfo `json:"swap"`
}

type UISwapInfo struct {
	Total   int64   `json:"total"`
	Used    int64   `json:"used"`
	Percent float64 `json:"percent"`
}

type UIDisksInfo struct {
	Disks      []UIDiskInfo `json:"disks"`
	TotalSpace int64        `json:"totalSpace"`
	UsedSpace  int64        `json:"usedSpace"`
}

type UIDiskInfo struct {
	Name       string  `json:"name"`
	Mountpoint string  `json:"mountpoint"`
	Filesystem string  `json:"filesystem"`
	Size       int64   `json:"size"`
	Used       int64   `json:"used"`
	Available  int64   `json:"available"`
	Percent    float64 `json:"percent"`
	ReadBps    int64   `json:"readBps"`
	WriteBps   int64   `json:"writeBps"`
	IOPS       int64   `json:"iops"`
}

type UINetworkInfo struct {
	Interfaces   []UINetworkInterface `json:"interfaces"`
	TotalRxBytes int64                `json:"totalRxBytes"`
	TotalTxBytes int64                `json:"totalTxBytes"`
}

type UINetworkInterface struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Speed       int32    `json:"speed"`
	MTU         int32    `json:"mtu"`
	IPAddresses []string `json:"ipAddresses"`
	MacAddress  string   `json:"macAddress"`
	RxBytes     int64    `json:"rxBytes"`
	TxBytes     int64    `json:"txBytes"`
	RxPackets   int64    `json:"rxPackets"`
	TxPackets   int64    `json:"txPackets"`
	RxErrors    int64    `json:"rxErrors"`
	TxErrors    int64    `json:"txErrors"`
}

type UIProcessesInfo struct {
	Processes      []UIProcessInfo `json:"processes"`
	TotalProcesses int32           `json:"totalProcesses"`
}

type UIProcessInfo struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	Command     string  `json:"command"`
	User        string  `json:"user"`
	CPU         float64 `json:"cpu"`
	Memory      float64 `json:"memory"`
	MemoryBytes int64   `json:"memoryBytes"`
	Status      string  `json:"status"`
	StartTime   string  `json:"startTime"`
	Threads     int32   `json:"threads"`
}

// transformToUIFormat converts the protobuf response to UI-expected format
func transformToUIFormat(resp *pb.SystemStatusRes) *UIFormat {
	// Calculate total space for disks (excluding duplicates and snaps)
	var totalSpace, usedSpace int64
	seenDevices := make(map[string]bool)
	uiDisks := []UIDiskInfo{}

	for _, disk := range resp.Disks {
		// Skip snap mounts and duplicates
		if strings.Contains(disk.MountPoint, "/snap/") {
			continue
		}

		if !seenDevices[disk.Device] {
			seenDevices[disk.Device] = true
			totalSpace += disk.TotalBytes
			usedSpace += disk.UsedBytes
		}

		// Only include main mount points in UI
		if disk.MountPoint == "/" || disk.MountPoint == "/boot" {
			uiDisks = append(uiDisks, UIDiskInfo{
				Name:       disk.Device,
				Mountpoint: disk.MountPoint,
				Filesystem: disk.Filesystem,
				Size:       disk.TotalBytes,
				Used:       disk.UsedBytes,
				Available:  disk.FreeBytes,
				Percent:    disk.UsagePercent,
				ReadBps:    0, // TODO: Calculate actual read BPS from metrics
				WriteBps:   0, // TODO: Calculate actual write BPS from metrics
				IOPS:       0, // TODO: Calculate actual IOPS from metrics
			})
		}
	}

	// Volume statistics are now provided by the server-side monitoring service
	// and included in the disk metrics above

	// Calculate total network bytes
	var totalRxBytes, totalTxBytes int64
	uiInterfaces := []UINetworkInterface{}

	for _, net := range resp.Networks {
		totalRxBytes += net.BytesReceived
		totalTxBytes += net.BytesSent

		// Only include active interfaces
		if net.BytesReceived > 0 || net.BytesSent > 0 {
			// Determine interface type and status
			interfaceType := "ethernet"
			if strings.Contains(net.Interface, "wlan") || strings.Contains(net.Interface, "wifi") {
				interfaceType = "wireless"
			}

			uiInterfaces = append(uiInterfaces, UINetworkInterface{
				Name:        net.Interface,
				Type:        interfaceType,
				Status:      "up",
				Speed:       1000,       // TODO: Get actual interface speed
				MTU:         1500,       // TODO: Get actual MTU
				IPAddresses: []string{}, // TODO: Get actual IP addresses
				MacAddress:  "unknown",  // TODO: Get actual MAC address
				RxBytes:     net.BytesReceived,
				TxBytes:     net.BytesSent,
				RxPackets:   net.PacketsReceived,
				TxPackets:   net.PacketsSent,
				RxErrors:    net.ErrorsIn,
				TxErrors:    net.ErrorsOut,
			})
		}
	}

	// Get top processes (limit to match UI expectation)
	uiProcesses := []UIProcessInfo{}
	for i, proc := range resp.Processes.TopByCPU {
		if i >= 10 { // Limit to top 10
			break
		}

		// Convert status to lowercase for UI consistency
		status := strings.ToLower(proc.Status)
		if status == "s" {
			status = "sleeping"
		} else if status == "r" {
			status = "running"
		}

		uiProcesses = append(uiProcesses, UIProcessInfo{
			PID:         proc.Pid,
			Name:        proc.Name,
			Command:     proc.Command,
			User:        "unknown", // TODO: Get actual process user
			CPU:         proc.CpuPercent,
			Memory:      proc.MemoryPercent,
			MemoryBytes: proc.MemoryBytes,
			Status:      status,
			StartTime:   proc.StartTime,
			Threads:     0, // TODO: Get actual thread count
		})
	}

	// Calculate swap percentage
	swapPercent := 0.0
	if resp.Memory.SwapTotal > 0 {
		swapPercent = float64(resp.Memory.SwapUsed) / float64(resp.Memory.SwapTotal) * 100
	}

	return &UIFormat{
		HostInfo: UIHostInfo{
			Hostname:      resp.Host.Hostname,
			Platform:      resp.Host.Os,
			Arch:          resp.Host.Architecture,
			Release:       resp.Host.KernelVersion,
			Uptime:        resp.Host.Uptime,
			CloudProvider: resp.Cloud.Provider,
			InstanceType:  resp.Cloud.InstanceType,
			Region:        resp.Cloud.Region,
		},
		CPUInfo: UICPUInfo{
			Cores:        resp.Cpu.Cores,
			Threads:      resp.Cpu.Cores * 2,            // Placeholder assumption
			Model:        "Intel Core i7",               // Placeholder
			Frequency:    2400,                          // Placeholder
			Usage:        resp.Cpu.UsagePercent / 100.0, // Convert to 0-1 range
			LoadAverage:  resp.Cpu.LoadAverage,
			PerCoreUsage: resp.Cpu.PerCoreUsage,
			Temperature:  45, // Placeholder
		},
		MemoryInfo: UIMemoryInfo{
			Total:     resp.Memory.TotalBytes,
			Used:      resp.Memory.UsedBytes,
			Available: resp.Memory.AvailableBytes,
			Percent:   resp.Memory.UsagePercent,
			Buffers:   resp.Memory.BufferedBytes,
			Cached:    resp.Memory.CachedBytes,
			Swap: UISwapInfo{
				Total:   resp.Memory.SwapTotal,
				Used:    resp.Memory.SwapUsed,
				Percent: swapPercent,
			},
		},
		DisksInfo: UIDisksInfo{
			Disks:      uiDisks,
			TotalSpace: totalSpace,
			UsedSpace:  usedSpace,
		},
		NetworkInfo: UINetworkInfo{
			Interfaces:   uiInterfaces,
			TotalRxBytes: totalRxBytes,
			TotalTxBytes: totalTxBytes,
		},
		ProcessesInfo: UIProcessesInfo{
			Processes:      uiProcesses,
			TotalProcesses: resp.Processes.TotalProcesses,
		},
	}
}
