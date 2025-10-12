package metrics

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Collector gathers resource usage metrics for a job by reading cgroup statistics.
// It periodically samples CPU, memory, I/O, and GPU metrics, then publishes them
// for real-time streaming and persistence.
type Collector struct {
	jobID          string
	cgroupPath     string
	sampleInterval time.Duration
	limits         *domain.ResourceLimits
	gpuIndices     []int

	// We keep the previous sample around to calculate rates like bytes/sec and IOPS
	previousSample *domain.JobMetricsSample
	previousTime   time.Time

	// Lifecycle management - context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Publisher sends metrics to the storage layer via pub/sub
	metricsPublisher MetricsPublisher

	logger *logger.Logger
}

// MetricsPublisher is the interface for publishing metrics (pub/sub)
type MetricsPublisher interface {
	PublishMetrics(ctx context.Context, sample *domain.JobMetricsSample) error
}

// NewCollector creates a new metrics collector for a job
func NewCollector(
	jobID string,
	cgroupPath string,
	sampleInterval time.Duration,
	limits *domain.ResourceLimits,
	gpuIndices []int,
	publisher MetricsPublisher,
) *Collector {
	ctx, cancel := context.WithCancel(context.Background())

	return &Collector{
		jobID:            jobID,
		cgroupPath:       cgroupPath,
		sampleInterval:   sampleInterval,
		limits:           limits,
		gpuIndices:       gpuIndices,
		ctx:              ctx,
		cancel:           cancel,
		metricsPublisher: publisher,
		logger:           logger.WithField("component", "metrics-collector").WithField("jobID", jobID),
	}
}

// Start begins collecting metrics at the configured interval
func (c *Collector) Start() error {
	c.logger.Info("starting metrics collection", "interval", c.sampleInterval, "cgroupPath", c.cgroupPath)

	c.wg.Add(1)
	go c.collectionLoop()

	return nil
}

// Stop gracefully stops metrics collection
func (c *Collector) Stop() error {
	c.logger.Info("stopping metrics collection")
	c.cancel()
	c.wg.Wait()
	c.logger.Info("metrics collection stopped")
	return nil
}

// collectionLoop runs the periodic metrics collection
func (c *Collector) collectionLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.sampleInterval)
	defer ticker.Stop()

	// Collect initial sample immediately
	c.collectAndPublish()

	for {
		select {
		case <-ticker.C:
			c.collectAndPublish()
		case <-c.ctx.Done():
			c.logger.Debug("collection loop terminated")
			return
		}
	}
}

// collectAndPublish collects a metrics sample and publishes it
func (c *Collector) collectAndPublish() {
	sample, err := c.CollectSample()
	if err != nil {
		c.logger.Warn("failed to collect metrics", "error", err)
		return
	}

	if c.metricsPublisher != nil {
		if err := c.metricsPublisher.PublishMetrics(c.ctx, sample); err != nil {
			c.logger.Warn("failed to publish metrics", "error", err)
		}
	}

	// Store for next rate calculation
	c.previousSample = sample
	c.previousTime = sample.Timestamp
}

// CollectSample collects a single metrics sample
func (c *Collector) CollectSample() (*domain.JobMetricsSample, error) {
	now := time.Now()

	sample := &domain.JobMetricsSample{
		JobID:          c.jobID,
		Timestamp:      now,
		SampleInterval: c.sampleInterval,
		CgroupPath:     c.cgroupPath,
		Limits:         c.limits,
		GPUAllocation:  c.gpuIndices,
	}

	// Collect CPU metrics
	cpuMetrics, err := c.collectCPUMetrics()
	if err != nil {
		c.logger.Debug("failed to collect CPU metrics", "error", err)
		cpuMetrics = &domain.CPUMetrics{}
	}
	sample.CPU = *cpuMetrics

	// Collect memory metrics
	memMetrics, err := c.collectMemoryMetrics()
	if err != nil {
		c.logger.Debug("failed to collect memory metrics", "error", err)
		memMetrics = &domain.MemoryMetrics{}
	}
	sample.Memory = *memMetrics

	// Collect I/O metrics
	ioMetrics, err := c.collectIOMetrics()
	if err != nil {
		c.logger.Debug("failed to collect I/O metrics", "error", err)
		ioMetrics = &domain.IOMetrics{}
	}
	sample.IO = *ioMetrics

	// Collect process metrics
	procMetrics, err := c.collectProcessMetrics()
	if err != nil {
		c.logger.Debug("failed to collect process metrics", "error", err)
		procMetrics = &domain.ProcessMetrics{}
	}
	sample.Process = *procMetrics

	// Collect GPU metrics if GPU allocated
	if len(c.gpuIndices) > 0 {
		gpuMetrics, err := c.collectGPUMetrics()
		if err != nil {
			c.logger.Debug("failed to collect GPU metrics", "error", err)
		} else {
			sample.GPU = gpuMetrics
		}
	}

	return sample, nil
}

// collectCPUMetrics reads CPU statistics from the cgroup v2 cpu controller.
// This gives us usage time, throttling info, and CPU pressure metrics.
func (c *Collector) collectCPUMetrics() (*domain.CPUMetrics, error) {
	metrics := &domain.CPUMetrics{}

	// Read cpu.stat - contains usage and throttling statistics
	cpuStatPath := filepath.Join(c.cgroupPath, "cpu.stat")
	data, err := os.ReadFile(cpuStatPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cpu.stat: %w", err)
	}

	// Parse the key-value format from cpu.stat
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "usage_usec":
			metrics.UsageUSec = value
		case "user_usec":
			metrics.UserUSec = value
		case "system_usec":
			metrics.SystemUSec = value
		case "nr_periods":
			metrics.NrPeriods = value
		case "nr_throttled":
			metrics.NrThrottled = value
		case "throttled_usec":
			metrics.ThrottledUSec = value
		}
	}

	// Calculate CPU usage as a percentage by comparing with the previous sample.
	// The raw usage values are cumulative, so we need the delta to get current usage.
	if c.previousSample != nil && c.previousTime.Before(time.Now()) {
		timeDelta := time.Since(c.previousTime).Seconds()
		if timeDelta > 0 {
			usageDelta := float64(metrics.UsageUSec - c.previousSample.CPU.UsageUSec)
			// Convert from microseconds to percentage of wall clock time
			metrics.UsagePercent = (usageDelta / 1000000.0 / timeDelta) * 100.0

			// Calculate how much time was spent throttled (when CPU limit was hit)
			if metrics.NrPeriods > 0 {
				throttleDelta := float64(metrics.ThrottledUSec - c.previousSample.CPU.ThrottledUSec)
				totalPeriodTime := timeDelta * 1000000.0 // Convert to microseconds
				if totalPeriodTime > 0 {
					metrics.ThrottlePercent = (throttleDelta / totalPeriodTime) * 100.0
				}
			}
		}
	}

	// CPU pressure shows when processes are waiting for CPU time (contention indicator)
	pressurePath := filepath.Join(c.cgroupPath, "cpu.pressure")
	if pressureData, err := os.ReadFile(pressurePath); err == nil {
		c.parsePSI(string(pressureData), &metrics.PressureSome10, &metrics.PressureSome60, &metrics.PressureSome300,
			&metrics.PressureFull10, &metrics.PressureFull60, &metrics.PressureFull300)
	}

	return metrics, nil
}

// collectMemoryMetrics reads memory statistics from the cgroup v2 memory controller.
// This includes current usage, limits, breakdown by type (anon/file), and fault counters.
func (c *Collector) collectMemoryMetrics() (*domain.MemoryMetrics, error) {
	metrics := &domain.MemoryMetrics{}

	// Current memory usage - this is the live total
	currentPath := filepath.Join(c.cgroupPath, "memory.current")
	if data, err := os.ReadFile(currentPath); err == nil {
		metrics.Current, _ = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}

	// Memory limit - can be "max" for unlimited, otherwise a byte value
	maxPath := filepath.Join(c.cgroupPath, "memory.max")
	if data, err := os.ReadFile(maxPath); err == nil {
		maxStr := strings.TrimSpace(string(data))
		if maxStr != "max" {
			metrics.Max, _ = strconv.ParseUint(maxStr, 10, 64)
		}
	}

	// Calculate usage percentage if there's a limit set
	if metrics.Max > 0 {
		metrics.UsagePercent = (float64(metrics.Current) / float64(metrics.Max)) * 100.0
	}

	// memory.stat provides detailed breakdown of memory usage
	statPath := filepath.Join(c.cgroupPath, "memory.stat")
	if data, err := os.ReadFile(statPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			value, _ := strconv.ParseUint(fields[1], 10, 64)
			switch fields[0] {
			case "anon":
				metrics.Anon = value // Anonymous memory (heap, stack, etc.)
			case "file":
				metrics.File = value // File-backed memory (page cache)
			case "kernel_stack":
				metrics.KernelStack = value
			case "slab":
				metrics.Slab = value // Kernel slab allocator memory
			case "sock":
				metrics.Sock = value // Socket buffers
			case "shmem":
				metrics.Shmem = value // Shared memory
			case "file_mapped":
				metrics.FileMapped = value // Memory-mapped files
			case "file_dirty":
				metrics.FileDirty = value // Dirty pages waiting to be written
			case "file_writeback":
				metrics.FileWriteback = value // Pages currently being written back
			case "pgfault":
				metrics.PgFault = value // Minor page faults (found in cache)
			case "pgmajfault":
				metrics.PgMajFault = value // Major page faults (required disk I/O)
			}
		}
	}

	// memory.events tracks important events like OOM conditions
	eventsPath := filepath.Join(c.cgroupPath, "memory.events")
	if data, err := os.ReadFile(eventsPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			value, _ := strconv.ParseUint(fields[1], 10, 64)
			switch fields[0] {
			case "oom":
				metrics.OOMEvents = value // Out-of-memory events
			case "oom_kill":
				metrics.OOMKill = value // Processes killed due to OOM
			}
		}
	}

	// Memory pressure indicates when the system is struggling to keep up with memory demands
	pressurePath := filepath.Join(c.cgroupPath, "memory.pressure")
	if pressureData, err := os.ReadFile(pressurePath); err == nil {
		c.parsePSI(string(pressureData), &metrics.PressureSome10, &metrics.PressureSome60, &metrics.PressureSome300,
			&metrics.PressureFull10, &metrics.PressureFull60, &metrics.PressureFull300)
	}

	return metrics, nil
}

// collectIOMetrics reads I/O statistics from the cgroup v2 io controller.
// This tracks read/write bytes and operations, both total and per-device.
func (c *Collector) collectIOMetrics() (*domain.IOMetrics, error) {
	metrics := &domain.IOMetrics{
		Devices: make(map[string]*domain.DeviceIOMetrics),
	}

	// Read io.stat
	ioStatPath := filepath.Join(c.cgroupPath, "io.stat")
	data, err := os.ReadFile(ioStatPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read io.stat: %w", err)
	}

	// io.stat format: "device_id rbytes=N wbytes=N rios=N wios=N dbytes=N dios=N"
	// Each line is a different block device
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		device := parts[0]
		deviceMetrics := &domain.DeviceIOMetrics{Device: device}

		// Parse the key=value pairs for this device
		for _, part := range parts[1:] {
			kv := strings.Split(part, "=")
			if len(kv) != 2 {
				continue
			}

			value, _ := strconv.ParseUint(kv[1], 10, 64)
			switch kv[0] {
			case "rbytes":
				deviceMetrics.ReadBytes = value
				metrics.TotalReadBytes += value
			case "wbytes":
				deviceMetrics.WriteBytes = value
				metrics.TotalWriteBytes += value
			case "rios":
				deviceMetrics.ReadOps = value
				metrics.TotalReadOps += value
			case "wios":
				deviceMetrics.WriteOps = value
				metrics.TotalWriteOps += value
			case "dbytes":
				deviceMetrics.DiscardBytes = value
				metrics.TotalDiscardBytes += value
			case "dios":
				deviceMetrics.DiscardOps = value
				metrics.TotalDiscardOps += value
			}
		}

		metrics.Devices[device] = deviceMetrics
	}

	// Calculate bytes/sec and IOPS by comparing with the previous sample
	// (cumulative counters need deltas to get rates)
	if c.previousSample != nil && c.previousTime.Before(time.Now()) {
		timeDelta := time.Since(c.previousTime).Seconds()
		if timeDelta > 0 {
			readBytesDelta := float64(metrics.TotalReadBytes - c.previousSample.IO.TotalReadBytes)
			writeBytesDelta := float64(metrics.TotalWriteBytes - c.previousSample.IO.TotalWriteBytes)
			readOpsDelta := float64(metrics.TotalReadOps - c.previousSample.IO.TotalReadOps)
			writeOpsDelta := float64(metrics.TotalWriteOps - c.previousSample.IO.TotalWriteOps)

			metrics.ReadBPS = readBytesDelta / timeDelta
			metrics.WriteBPS = writeBytesDelta / timeDelta
			metrics.ReadIOPS = readOpsDelta / timeDelta
			metrics.WriteIOPS = writeOpsDelta / timeDelta
		}
	}

	// I/O pressure shows when processes are waiting for I/O operations
	pressurePath := filepath.Join(c.cgroupPath, "io.pressure")
	if pressureData, err := os.ReadFile(pressurePath); err == nil {
		c.parsePSI(string(pressureData), &metrics.PressureSome10, &metrics.PressureSome60, &metrics.PressureSome300,
			&metrics.PressureFull10, &metrics.PressureFull60, &metrics.PressureFull300)
	}

	return metrics, nil
}

// collectProcessMetrics reads process statistics from the cgroup v2 pids controller.
// This gives us process count, limits, and fork bomb protection events.
func (c *Collector) collectProcessMetrics() (*domain.ProcessMetrics, error) {
	metrics := &domain.ProcessMetrics{}

	// Current number of processes in the cgroup
	currentPath := filepath.Join(c.cgroupPath, "pids.current")
	if data, err := os.ReadFile(currentPath); err == nil {
		metrics.Current, _ = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}

	// Maximum allowed processes (fork bomb protection)
	maxPath := filepath.Join(c.cgroupPath, "pids.max")
	if data, err := os.ReadFile(maxPath); err == nil {
		maxStr := strings.TrimSpace(string(data))
		if maxStr != "max" {
			metrics.Max, _ = strconv.ParseUint(maxStr, 10, 64)
		}
	}

	// Events when process limit was hit
	eventsPath := filepath.Join(c.cgroupPath, "pids.events")
	if data, err := os.ReadFile(eventsPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "max" {
				metrics.Events, _ = strconv.ParseUint(fields[1], 10, 64)
				break
			}
		}
	}

	return metrics, nil
}

// parsePSI parses Pressure Stall Information (PSI) metrics from cgroup files.
// PSI tracks resource contention - how often processes are stalled waiting for resources.
func (c *Collector) parsePSI(data string, some10, some60, some300, full10, full60, full300 *float64) {
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		var avg10, avg60, avg300 float64
		for _, field := range fields[1:] {
			kv := strings.Split(field, "=")
			if len(kv) != 2 {
				continue
			}

			switch kv[0] {
			case "avg10":
				avg10, _ = strconv.ParseFloat(kv[1], 64)
			case "avg60":
				avg60, _ = strconv.ParseFloat(kv[1], 64)
			case "avg300":
				avg300, _ = strconv.ParseFloat(kv[1], 64)
			}
		}

		if fields[0] == "some" {
			*some10 = avg10
			*some60 = avg60
			*some300 = avg300
		} else if fields[0] == "full" {
			*full10 = avg10
			*full60 = avg60
			*full300 = avg300
		}
	}
}

// collectGPUMetrics collects GPU metrics using nvidia-smi for all GPUs allocated to this job.
// Returns comprehensive stats including utilization, memory, temperature, and power draw.
func (c *Collector) collectGPUMetrics() ([]domain.GPUMetrics, error) {
	if len(c.gpuIndices) == 0 {
		return nil, nil
	}

	var gpuMetrics []domain.GPUMetrics

	for _, gpuIndex := range c.gpuIndices {
		metrics, err := c.collectSingleGPUMetrics(gpuIndex)
		if err != nil {
			c.logger.Warn("failed to collect metrics for GPU", "index", gpuIndex, "error", err)
			continue
		}
		gpuMetrics = append(gpuMetrics, *metrics)
	}

	return gpuMetrics, nil
}

// collectSingleGPUMetrics queries nvidia-smi for comprehensive metrics from a single GPU.
// We query 24 different fields in one call to minimize overhead.
func (c *Collector) collectSingleGPUMetrics(gpuIndex int) (*domain.GPUMetrics, error) {
	cmd := exec.Command("nvidia-smi",
		fmt.Sprintf("--id=%d", gpuIndex),
		"--query-gpu=index,uuid,name,compute_cap,driver_version,utilization.gpu,memory.used,memory.total,memory.free,encoder.stats.sessionCount,decoder.stats.sessionCount,clocks.sm,clocks.mem,pcie.link.gen.current,pcie.link.width.current,temperature.gpu,temperature.memory,power.draw,power.limit,fan.speed,ecc.errors.corrected.volatile.total,ecc.errors.uncorrected.volatile.total,retired_pages.count,compute_mode",
		"--format=csv,noheader,nounits")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	// Parse the CSV output - nvidia-smi returns one line per GPU
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse nvidia-smi output: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no data returned from nvidia-smi")
	}

	record := records[0]
	if len(record) < 24 {
		return nil, fmt.Errorf("insufficient fields in nvidia-smi output: got %d, expected 24", len(record))
	}

	metrics := &domain.GPUMetrics{
		Index:             gpuIndex,
		UUID:              strings.TrimSpace(record[1]),
		Name:              strings.TrimSpace(record[2]),
		ComputeCapability: strings.TrimSpace(record[3]),
		DriverVersion:     strings.TrimSpace(record[4]),
		Utilization:       parseFloat64(record[5]),
		MemoryUsed:        parseUint64(record[6]) * 1024 * 1024, // nvidia-smi reports in MiB, we want bytes
		MemoryTotal:       parseUint64(record[7]) * 1024 * 1024,
		MemoryFree:        parseUint64(record[8]) * 1024 * 1024,
		EncoderUtil:       parseFloat64(record[9]),
		DecoderUtil:       parseFloat64(record[10]),
		SMClock:           parseFloat64(record[11]),
		MemoryClock:       parseFloat64(record[12]),
		Temperature:       parseFloat64(record[15]),
		TemperatureMemory: parseFloat64(record[16]),
		PowerDraw:         parseFloat64(record[17]),
		PowerLimit:        parseFloat64(record[18]),
		FanSpeed:          parseFloat64(record[19]),
		ECCErrorsSingle:   parseUint64(record[20]),
		ECCErrorsDouble:   parseUint64(record[21]),
		RetiredPages:      parseUint64(record[22]),
		ComputeMode:       strings.TrimSpace(record[23]),
	}

	// Calculate GPU memory usage percentage
	if metrics.MemoryTotal > 0 {
		metrics.MemoryPercent = (float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal)) * 100.0
	}

	// Collect PCIe throughput (note: static queries return 0, real-time monitoring needed)
	pcieTx, pcieRx := c.collectPCIeThroughput(gpuIndex)
	metrics.PCIeThroughputTx = pcieTx
	metrics.PCIeThroughputRx = pcieRx

	// Collect process information running on this GPU
	processCount, processMemory := c.collectGPUProcessInfo(gpuIndex)
	metrics.ProcessesCount = uint64(processCount)
	metrics.ProcessesMemory = processMemory

	return metrics, nil
}

// collectPCIeThroughput collects PCIe throughput for a GPU
func (c *Collector) collectPCIeThroughput(gpuIndex int) (tx, rx float64) {
	// PCIe throughput requires real-time monitoring (nvidia-smi dmon)
	// Static queries don't provide meaningful throughput data
	return 0, 0
}

// collectGPUProcessInfo queries nvidia-smi for processes currently using this GPU.
// Returns the count of processes and their total GPU memory usage.
func (c *Collector) collectGPUProcessInfo(gpuIndex int) (count int, memory uint64) {
	cmd := exec.Command("nvidia-smi",
		fmt.Sprintf("--id=%d", gpuIndex),
		"--query-compute-apps=pid,used_memory",
		"--format=csv,noheader,nounits")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return 0, 0
	}

	count = len(records)
	for _, record := range records {
		if len(record) >= 2 {
			mem := parseUint64(record[1])
			memory += mem * 1024 * 1024 // Convert MiB to bytes
		}
	}

	return count, memory
}

// parseFloat64 safely parses a float from nvidia-smi output.
// nvidia-smi returns "N/A" for unavailable metrics, which we treat as 0.
func parseFloat64(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "N/A" || s == "[N/A]" || s == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// parseUint64 safely parses an unsigned integer from nvidia-smi output.
func parseUint64(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "N/A" || s == "[N/A]" || s == "" {
		return 0
	}
	val, _ := strconv.ParseUint(s, 10, 64)
	return val
}
