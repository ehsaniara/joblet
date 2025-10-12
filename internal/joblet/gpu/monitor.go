package gpu

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/pkg/logger"
)

// GPUMetrics represents GPU performance and utilization metrics
type GPUMetrics struct {
	Index        int       `json:"index"`
	MemoryUsed   int64     `json:"memory_used_mb"`
	MemoryTotal  int64     `json:"memory_total_mb"`
	MemoryFree   int64     `json:"memory_free_mb"`
	Utilization  float64   `json:"utilization_percent"`
	Temperature  float64   `json:"temperature_celsius"`
	PowerDraw    float64   `json:"power_draw_watts"`
	JobID        string    `json:"job_id,omitempty"`
	LastUpdated  time.Time `json:"last_updated"`
	ProcessCount int       `json:"process_count"`
}

// GPUMonitor provides GPU metrics collection and monitoring
type GPUMonitor struct {
	manager    *Manager
	logger     *logger.Logger
	interval   time.Duration
	metrics    map[int]*GPUMetrics
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	isRunning  bool
	runningMux sync.RWMutex
}

// NewGPUMonitor creates a new GPU monitoring service
func NewGPUMonitor(manager *Manager, interval time.Duration) *GPUMonitor {
	if interval == 0 {
		interval = 10 * time.Second // Default 10 second interval
	}

	return &GPUMonitor{
		manager:  manager,
		logger:   logger.New().WithField("component", "gpu-monitor"),
		interval: interval,
		metrics:  make(map[int]*GPUMetrics),
	}
}

// Start begins the GPU monitoring service
func (m *GPUMonitor) Start(ctx context.Context) error {
	m.runningMux.Lock()
	defer m.runningMux.Unlock()

	if m.isRunning {
		return fmt.Errorf("GPU monitor is already running")
	}

	if !m.manager.IsEnabled() {
		m.logger.Debug("GPU support disabled, monitor will not start")
		return nil
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.isRunning = true

	m.logger.Info("starting GPU monitoring service", "interval", m.interval)

	// Collect initial metrics
	if err := m.collectMetrics(); err != nil {
		m.logger.Warn("failed to collect initial GPU metrics", "error", err)
	}

	// Start background goroutine for periodic collection
	go m.monitoringLoop()

	return nil
}

// Stop stops the GPU monitoring service
func (m *GPUMonitor) Stop() {
	m.runningMux.Lock()
	defer m.runningMux.Unlock()

	if !m.isRunning {
		return
	}

	m.logger.Info("stopping GPU monitoring service")
	m.cancel()
	m.isRunning = false
}

// IsRunning returns whether the monitor is currently running
func (m *GPUMonitor) IsRunning() bool {
	m.runningMux.RLock()
	defer m.runningMux.RUnlock()
	return m.isRunning
}

// GetMetrics returns current GPU metrics for all GPUs
func (m *GPUMonitor) GetMetrics() map[int]*GPUMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to avoid concurrent modification
	result := make(map[int]*GPUMetrics)
	for k, v := range m.metrics {
		metricsCopy := *v // Copy the struct
		result[k] = &metricsCopy
	}

	return result
}

// GetMetricsForGPU returns metrics for a specific GPU
func (m *GPUMonitor) GetMetricsForGPU(index int) (*GPUMetrics, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics, exists := m.metrics[index]
	if !exists {
		return nil, fmt.Errorf("no metrics available for GPU %d", index)
	}

	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// monitoringLoop runs the periodic metrics collection
func (m *GPUMonitor) monitoringLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("GPU monitoring loop stopped")
			return
		case <-ticker.C:
			if err := m.collectMetrics(); err != nil {
				m.logger.Warn("failed to collect GPU metrics", "error", err)
			}
		}
	}
}

// collectMetrics collects current GPU metrics using nvidia-smi
func (m *GPUMonitor) collectMetrics() error {
	start := time.Now()

	// Execute nvidia-smi command to get metrics
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=index,memory.used,memory.total,memory.free,utilization.gpu,temperature.gpu,power.draw",
		"--format=csv,noheader,nounits")

	// Set timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute nvidia-smi: %w", err)
	}

	// Parse CSV output
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse nvidia-smi CSV output: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	newMetrics := make(map[int]*GPUMetrics)

	for _, record := range records {
		if len(record) < 7 {
			m.logger.Warn("invalid nvidia-smi record", "record", record)
			continue
		}

		metrics, err := m.parseMetricsRecord(record, now)
		if err != nil {
			m.logger.Warn("failed to parse metrics record", "record", record, "error", err)
			continue
		}

		// Add job information if GPU is allocated
		if allocation, err := m.manager.GetJobAllocation(""); err == nil && allocation != nil {
			for _, gpuIndex := range allocation.GPUIndices {
				if gpuIndex == metrics.Index {
					metrics.JobID = allocation.JobID
					break
				}
			}
		}

		// Get process count for this GPU
		metrics.ProcessCount = m.getGPUProcessCount(metrics.Index)

		newMetrics[metrics.Index] = metrics
	}

	// Update metrics map
	m.metrics = newMetrics

	duration := time.Since(start)
	m.logger.Debug("GPU metrics collection completed",
		"gpus", len(newMetrics),
		"duration", duration)

	return nil
}

// parseMetricsRecord parses a single CSV record into GPUMetrics
func (m *GPUMonitor) parseMetricsRecord(record []string, timestamp time.Time) (*GPUMetrics, error) {
	index, err := strconv.Atoi(strings.TrimSpace(record[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid GPU index: %w", err)
	}

	memoryUsed, err := m.parseMemoryValue(record[1])
	if err != nil {
		return nil, fmt.Errorf("invalid memory used: %w", err)
	}

	memoryTotal, err := m.parseMemoryValue(record[2])
	if err != nil {
		return nil, fmt.Errorf("invalid memory total: %w", err)
	}

	memoryFree, err := m.parseMemoryValue(record[3])
	if err != nil {
		return nil, fmt.Errorf("invalid memory free: %w", err)
	}

	utilization, err := m.parseFloatValue(record[4])
	if err != nil {
		return nil, fmt.Errorf("invalid utilization: %w", err)
	}

	temperature, err := m.parseFloatValue(record[5])
	if err != nil {
		return nil, fmt.Errorf("invalid temperature: %w", err)
	}

	powerDraw, err := m.parseFloatValue(record[6])
	if err != nil {
		return nil, fmt.Errorf("invalid power draw: %w", err)
	}

	return &GPUMetrics{
		Index:       index,
		MemoryUsed:  memoryUsed,
		MemoryTotal: memoryTotal,
		MemoryFree:  memoryFree,
		Utilization: utilization,
		Temperature: temperature,
		PowerDraw:   powerDraw,
		LastUpdated: timestamp,
	}, nil
}

// parseMemoryValue parses memory values, handling special cases like "N/A"
func (m *GPUMonitor) parseMemoryValue(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "N/A" || value == "[N/A]" {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

// parseFloatValue parses float values, handling special cases like "N/A"
func (m *GPUMonitor) parseFloatValue(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "N/A" || value == "[N/A]" {
		return 0.0, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.0, err
	}

	return parsed, nil
}

// getGPUProcessCount gets the number of processes running on a GPU
func (m *GPUMonitor) getGPUProcessCount(gpuIndex int) int {
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=gpu_bus_id",
		"--format=csv,noheader,nounits", "-i", fmt.Sprintf("%d", gpuIndex))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return 0
	}

	return len(lines)
}

// CheckGPUHealth performs health checks on all GPUs
func (m *GPUMonitor) CheckGPUHealth() map[int]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	health := make(map[int]string)

	for index, metrics := range m.metrics {
		status := "healthy"

		// Check for high temperature (>85°C is concerning)
		if metrics.Temperature > 85.0 {
			status = "warning: high temperature"
		}

		// Check for very high temperature (>95°C is critical)
		if metrics.Temperature > 95.0 {
			status = "critical: overheating"
		}

		// Check for memory issues (>95% usage)
		if metrics.MemoryTotal > 0 {
			memoryUsagePercent := float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
			if memoryUsagePercent > 95.0 {
				status = "warning: high memory usage"
			}
		}

		// Check if metrics are stale (older than 30 seconds)
		if time.Since(metrics.LastUpdated) > 30*time.Second {
			status = "warning: stale metrics"
		}

		health[index] = status
	}

	return health
}
