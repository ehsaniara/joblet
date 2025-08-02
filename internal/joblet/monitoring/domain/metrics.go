package domain

import (
	"time"
)

// SystemMetrics represents a snapshot of system resource metrics
type SystemMetrics struct {
	Timestamp time.Time        `json:"timestamp"`
	Host      HostInfo         `json:"host"`
	CPU       CPUMetrics       `json:"cpu"`
	Memory    MemoryMetrics    `json:"memory"`
	Disk      []DiskMetrics    `json:"disk"`
	Network   []NetworkMetrics `json:"network"`
	IO        IOMetrics        `json:"io"`
	Processes ProcessMetrics   `json:"processes"`
	Cloud     *CloudInfo       `json:"cloud,omitempty"`
}

// HostInfo contains basic host information
type HostInfo struct {
	Hostname     string        `json:"hostname"`
	OS           string        `json:"os"`
	Kernel       string        `json:"kernel"`
	Architecture string        `json:"architecture"`
	Uptime       time.Duration `json:"uptime"`
	BootTime     time.Time     `json:"boot_time"`
}

// CPUMetrics contains CPU usage statistics
type CPUMetrics struct {
	UsagePercent float64    `json:"usage_percent"`
	LoadAverage  [3]float64 `json:"load_average"` // 1min, 5min, 15min
	Cores        int        `json:"cores"`
	PerCoreUsage []float64  `json:"per_core_usage"`
	StealTime    float64    `json:"steal_time"`
	UserTime     float64    `json:"user_time"`
	SystemTime   float64    `json:"system_time"`
	IdleTime     float64    `json:"idle_time"`
	IOWaitTime   float64    `json:"iowait_time"`
}

// MemoryMetrics contains memory usage statistics
type MemoryMetrics struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	CachedBytes    uint64  `json:"cached_bytes"`
	BufferedBytes  uint64  `json:"buffered_bytes"`
	SwapTotal      uint64  `json:"swap_total"`
	SwapUsed       uint64  `json:"swap_used"`
	SwapFree       uint64  `json:"swap_free"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskMetrics contains disk usage statistics for a mount point
type DiskMetrics struct {
	MountPoint      string  `json:"mount_point"`
	Device          string  `json:"device"`
	FileSystem      string  `json:"filesystem"`
	TotalBytes      uint64  `json:"total_bytes"`
	UsedBytes       uint64  `json:"used_bytes"`
	FreeBytes       uint64  `json:"free_bytes"`
	UsagePercent    float64 `json:"usage_percent"`
	InodesTotal     uint64  `json:"inodes_total"`
	InodesUsed      uint64  `json:"inodes_used"`
	InodesFree      uint64  `json:"inodes_free"`
	ReadIOPS        uint64  `json:"read_iops"`
	WriteIOPS       uint64  `json:"write_iops"`
	ReadThroughput  uint64  `json:"read_throughput"`
	WriteThroughput uint64  `json:"write_throughput"`
}

// NetworkMetrics contains network interface statistics
type NetworkMetrics struct {
	Interface       string  `json:"interface"`
	BytesReceived   uint64  `json:"bytes_received"`
	BytesSent       uint64  `json:"bytes_sent"`
	PacketsReceived uint64  `json:"packets_received"`
	PacketsSent     uint64  `json:"packets_sent"`
	ErrorsReceived  uint64  `json:"errors_received"`
	ErrorsSent      uint64  `json:"errors_sent"`
	DropsReceived   uint64  `json:"drops_received"`
	DropsSent       uint64  `json:"drops_sent"`
	RxThroughputBPS float64 `json:"rx_throughput_bps"`
	TxThroughputBPS float64 `json:"tx_throughput_bps"`
	RxPacketsPerSec float64 `json:"rx_packets_per_sec"`
	TxPacketsPerSec float64 `json:"tx_packets_per_sec"`
}

// IOMetrics contains block device I/O statistics
type IOMetrics struct {
	ReadsCompleted  uint64  `json:"reads_completed"`
	WritesCompleted uint64  `json:"writes_completed"`
	ReadBytes       uint64  `json:"read_bytes"`
	WriteBytes      uint64  `json:"write_bytes"`
	ReadTime        uint64  `json:"read_time_ms"`
	WriteTime       uint64  `json:"write_time_ms"`
	IOTime          uint64  `json:"io_time_ms"`
	WeightedIOTime  uint64  `json:"weighted_io_time_ms"`
	QueueDepth      float64 `json:"queue_depth"`
	Utilization     float64 `json:"utilization_percent"`
}

// ProcessMetrics contains process-related statistics
type ProcessMetrics struct {
	TotalProcesses   int           `json:"total_processes"`
	RunningProcesses int           `json:"running_processes"`
	ZombieProcesses  int           `json:"zombie_processes"`
	TotalThreads     int           `json:"total_threads"`
	TopByCPU         []ProcessInfo `json:"top_by_cpu"`
	TopByMemory      []ProcessInfo `json:"top_by_memory"`
}

// ProcessInfo contains information about a single process
type ProcessInfo struct {
	PID           int       `json:"pid"`
	PPID          int       `json:"ppid"`
	Name          string    `json:"name"`
	Command       string    `json:"command"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryBytes   uint64    `json:"memory_bytes"`
	MemoryPercent float64   `json:"memory_percent"`
	Status        string    `json:"status"`
	StartTime     time.Time `json:"start_time"`
}

// CloudInfo contains cloud environment information
type CloudInfo struct {
	Provider       string            `json:"provider"`
	Region         string            `json:"region"`
	Zone           string            `json:"zone"`
	InstanceID     string            `json:"instance_id"`
	InstanceType   string            `json:"instance_type"`
	HypervisorType string            `json:"hypervisor_type"`
	Metadata       map[string]string `json:"metadata"`
}

// MonitoringConfig represents monitoring system configuration
type MonitoringConfig struct {
	Enabled    bool             `json:"enabled" yaml:"enabled"`
	Collection CollectionConfig `json:"collection" yaml:"collection"`
}

// CollectionConfig represents collection settings
type CollectionConfig struct {
	SystemInterval  time.Duration `json:"system_interval" yaml:"system_interval"`
	ProcessInterval time.Duration `json:"process_interval" yaml:"process_interval"`
	CloudDetection  bool          `json:"cloud_detection" yaml:"cloud_detection"`
}
