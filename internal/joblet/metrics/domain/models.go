package domain

import (
	"time"
)

// JobMetricsSample represents a single metrics sample for a job at a specific point in time
type JobMetricsSample struct {
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`

	// Metadata
	SampleInterval time.Duration   `json:"sample_interval"`
	CgroupPath     string          `json:"cgroup_path,omitempty"`
	Limits         *ResourceLimits `json:"limits,omitempty"`
	GPUAllocation  []int           `json:"gpu_allocation,omitempty"`

	// Metrics
	CPU     CPUMetrics      `json:"cpu"`
	Memory  MemoryMetrics   `json:"memory"`
	IO      IOMetrics       `json:"io"`
	Network *NetworkMetrics `json:"network,omitempty"`
	Process ProcessMetrics  `json:"process"`
	GPU     []GPUMetrics    `json:"gpu,omitempty"`
}

// ResourceLimits contains configured resource limits for a job
type ResourceLimits struct {
	CPU    int32 `json:"cpu"`    // CPU percentage limit
	Memory int64 `json:"memory"` // Memory limit in bytes
	IO     int32 `json:"io"`     // I/O limit in bytes/sec
}

// CPUMetrics contains CPU usage statistics from cgroup
type CPUMetrics struct {
	// From cpu.stat
	UsageUSec     uint64 `json:"usage_usec"`     // Total CPU time in microseconds
	UserUSec      uint64 `json:"user_usec"`      // User mode CPU time
	SystemUSec    uint64 `json:"system_usec"`    // Kernel mode CPU time
	NrPeriods     uint64 `json:"nr_periods"`     // Number of enforcement periods
	NrThrottled   uint64 `json:"nr_throttled"`   // Number of throttled periods
	ThrottledUSec uint64 `json:"throttled_usec"` // Total throttled time

	// Calculated
	UsagePercent    float64 `json:"usage_percent"`    // Current CPU usage %
	ThrottlePercent float64 `json:"throttle_percent"` // Percentage of time throttled

	// From cpu.pressure (PSI)
	PressureSome10  float64 `json:"pressure_some_10,omitempty"`  // 10s average
	PressureSome60  float64 `json:"pressure_some_60,omitempty"`  // 60s average
	PressureSome300 float64 `json:"pressure_some_300,omitempty"` // 300s average
	PressureFull10  float64 `json:"pressure_full_10,omitempty"`  // 10s average
	PressureFull60  float64 `json:"pressure_full_60,omitempty"`  // 60s average
	PressureFull300 float64 `json:"pressure_full_300,omitempty"` // 300s average
}

// MemoryMetrics contains memory usage statistics from cgroup
type MemoryMetrics struct {
	// From memory.current and memory.max
	Current      uint64  `json:"current"`       // Current memory usage in bytes
	Max          uint64  `json:"max"`           // Memory limit in bytes
	UsagePercent float64 `json:"usage_percent"` // Memory usage percentage

	// From memory.stat
	Anon          uint64 `json:"anon"`           // Anonymous memory
	File          uint64 `json:"file"`           // File-backed memory (cache)
	KernelStack   uint64 `json:"kernel_stack"`   // Kernel stack memory
	Slab          uint64 `json:"slab"`           // Kernel slab memory
	Sock          uint64 `json:"sock"`           // Socket buffer memory
	Shmem         uint64 `json:"shmem"`          // Shared memory
	FileMapped    uint64 `json:"file_mapped"`    // Memory-mapped files
	FileDirty     uint64 `json:"file_dirty"`     // Dirty page cache
	FileWriteback uint64 `json:"file_writeback"` // Pages under writeback
	PgFault       uint64 `json:"pgfault"`        // Page fault count
	PgMajFault    uint64 `json:"pgmajfault"`     // Major page fault count

	// From memory.events
	OOMEvents uint64 `json:"oom_events,omitempty"` // OOM killer invocations
	OOMKill   uint64 `json:"oom_kill,omitempty"`   // Processes killed by OOM

	// From memory.pressure (PSI)
	PressureSome10  float64 `json:"pressure_some_10,omitempty"`
	PressureSome60  float64 `json:"pressure_some_60,omitempty"`
	PressureSome300 float64 `json:"pressure_some_300,omitempty"`
	PressureFull10  float64 `json:"pressure_full_10,omitempty"`
	PressureFull60  float64 `json:"pressure_full_60,omitempty"`
	PressureFull300 float64 `json:"pressure_full_300,omitempty"`
}

// IOMetrics contains I/O statistics from cgroup
type IOMetrics struct {
	// Per-device metrics (keyed by device major:minor)
	Devices map[string]*DeviceIOMetrics `json:"devices,omitempty"`

	// Aggregated metrics
	TotalReadBytes    uint64 `json:"total_read_bytes"`
	TotalWriteBytes   uint64 `json:"total_write_bytes"`
	TotalReadOps      uint64 `json:"total_read_ops"`
	TotalWriteOps     uint64 `json:"total_write_ops"`
	TotalDiscardBytes uint64 `json:"total_discard_bytes,omitempty"`
	TotalDiscardOps   uint64 `json:"total_discard_ops,omitempty"`

	// Calculated rates
	ReadBPS   float64 `json:"read_bps"`   // Read bandwidth (bytes/sec)
	WriteBPS  float64 `json:"write_bps"`  // Write bandwidth (bytes/sec)
	ReadIOPS  float64 `json:"read_iops"`  // Read IOPS
	WriteIOPS float64 `json:"write_iops"` // Write IOPS

	// From io.pressure (PSI)
	PressureSome10  float64 `json:"pressure_some_10,omitempty"`
	PressureSome60  float64 `json:"pressure_some_60,omitempty"`
	PressureSome300 float64 `json:"pressure_some_300,omitempty"`
	PressureFull10  float64 `json:"pressure_full_10,omitempty"`
	PressureFull60  float64 `json:"pressure_full_60,omitempty"`
	PressureFull300 float64 `json:"pressure_full_300,omitempty"`
}

// DeviceIOMetrics contains per-device I/O statistics
type DeviceIOMetrics struct {
	Device       string `json:"device"`           // Device identifier (major:minor)
	ReadBytes    uint64 `json:"rbytes"`           // Bytes read
	WriteBytes   uint64 `json:"wbytes"`           // Bytes written
	ReadOps      uint64 `json:"rios"`             // Read operations
	WriteOps     uint64 `json:"wios"`             // Write operations
	DiscardBytes uint64 `json:"dbytes,omitempty"` // Bytes discarded
	DiscardOps   uint64 `json:"dios,omitempty"`   // Discard operations
}

// NetworkMetrics contains network statistics (if network isolation enabled)
type NetworkMetrics struct {
	Interfaces map[string]*NetworkInterfaceMetrics `json:"interfaces,omitempty"`

	// Aggregated metrics
	TotalRxBytes   uint64 `json:"total_rx_bytes"`
	TotalTxBytes   uint64 `json:"total_tx_bytes"`
	TotalRxPackets uint64 `json:"total_rx_packets"`
	TotalTxPackets uint64 `json:"total_tx_packets"`
	TotalRxErrors  uint64 `json:"total_rx_errors"`
	TotalTxErrors  uint64 `json:"total_tx_errors"`
	TotalRxDropped uint64 `json:"total_rx_dropped"`
	TotalTxDropped uint64 `json:"total_tx_dropped"`

	// Calculated rates
	RxBPS float64 `json:"rx_bps"` // Receive bandwidth (bytes/sec)
	TxBPS float64 `json:"tx_bps"` // Transmit bandwidth (bytes/sec)
}

// NetworkInterfaceMetrics contains per-interface network statistics
type NetworkInterfaceMetrics struct {
	Interface string `json:"interface"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxDropped uint64 `json:"tx_dropped"`
}

// ProcessMetrics contains process-related statistics
type ProcessMetrics struct {
	// From pids.current and pids.max
	Current uint64 `json:"current"` // Current number of processes/threads
	Max     uint64 `json:"max"`     // Maximum PIDs allowed
	Events  uint64 `json:"events"`  // PID limit hit count

	// From /proc/[pid]/status and /proc/[pid]/stat
	Threads  uint64 `json:"threads"`  // Total thread count
	Running  uint64 `json:"running"`  // Running processes
	Sleeping uint64 `json:"sleeping"` // Sleeping processes
	Stopped  uint64 `json:"stopped"`  // Stopped processes
	Zombie   uint64 `json:"zombie"`   // Zombie processes

	// From /proc/[pid]/fd and /proc/[pid]/limits
	OpenFDs uint64 `json:"open_fds,omitempty"` // Open file descriptors
	MaxFDs  uint64 `json:"max_fds,omitempty"`  // FD limit
}

// GPUMetrics contains GPU statistics (NVIDIA/CUDA)
type GPUMetrics struct {
	// Device identification
	Index             int    `json:"index"`              // GPU device index
	UUID              string `json:"uuid"`               // GPU unique identifier
	Name              string `json:"name"`               // GPU model name
	ComputeCapability string `json:"compute_capability"` // CUDA compute capability
	DriverVersion     string `json:"driver_version"`     // NVIDIA driver version

	// Utilization metrics
	Utilization   float64 `json:"utilization"`            // GPU core utilization (0-100%)
	MemoryUsed    uint64  `json:"memory_used"`            // GPU memory used (bytes)
	MemoryTotal   uint64  `json:"memory_total"`           // GPU memory total (bytes)
	MemoryFree    uint64  `json:"memory_free"`            // GPU memory free (bytes)
	MemoryPercent float64 `json:"memory_percent"`         // Memory utilization percentage
	EncoderUtil   float64 `json:"encoder_util,omitempty"` // Video encoder utilization
	DecoderUtil   float64 `json:"decoder_util,omitempty"` // Video decoder utilization

	// Performance metrics
	SMClock          float64 `json:"sm_clock,omitempty"`           // Streaming Multiprocessor clock (MHz)
	MemoryClock      float64 `json:"memory_clock,omitempty"`       // Memory clock speed (MHz)
	PCIeThroughputRx float64 `json:"pcie_throughput_rx,omitempty"` // PCIe receive throughput (MB/s)
	PCIeThroughputTx float64 `json:"pcie_throughput_tx,omitempty"` // PCIe transmit throughput (MB/s)

	// Thermal & Power
	Temperature       float64 `json:"temperature"`                  // GPU temperature (Celsius)
	TemperatureMemory float64 `json:"temperature_memory,omitempty"` // Memory temperature
	PowerDraw         float64 `json:"power_draw"`                   // Current power draw (Watts)
	PowerLimit        float64 `json:"power_limit"`                  // Power limit (Watts)
	FanSpeed          float64 `json:"fan_speed,omitempty"`          // Fan speed percentage

	// Error & Health
	ECCErrorsSingle uint64 `json:"ecc_errors_single,omitempty"` // Single-bit ECC errors
	ECCErrorsDouble uint64 `json:"ecc_errors_double,omitempty"` // Double-bit ECC errors
	XIDErrors       uint64 `json:"xid_errors,omitempty"`        // XID error events
	RetiredPages    uint64 `json:"retired_pages,omitempty"`     // Retired memory pages
	ThrottleReasons uint64 `json:"throttle_reasons,omitempty"`  // Throttling reason bitmask

	// Process metrics
	ProcessesCount  uint64 `json:"processes_count,omitempty"`  // Number of processes using GPU
	ProcessesMemory uint64 `json:"processes_memory,omitempty"` // Memory used by job processes
	ComputeMode     string `json:"compute_mode,omitempty"`     // Compute mode (exclusive/shared)
}

// MetricsConfig contains configuration for metrics collection
type MetricsConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	DefaultSampleRate time.Duration `yaml:"default_sample_rate" json:"default_sample_rate"`
	Storage           StorageConfig `yaml:"storage" json:"storage"`
}

// StorageConfig contains metrics storage settings
type StorageConfig struct {
	Directory string          `yaml:"directory" json:"directory"`
	Retention RetentionConfig `yaml:"retention" json:"retention"`
}

// RetentionConfig contains retention policy settings
type RetentionConfig struct {
	Days int `yaml:"days" json:"days"`
}
