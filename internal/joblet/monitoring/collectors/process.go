package collectors

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// ProcessCollector collects process metrics from /proc
type ProcessCollector struct {
	logger       *logger.Logger
	lastCPUStats map[int]*processCPUStats
	lastTime     time.Time
	systemCPU    *systemCPUStats
}

type processCPUStats struct {
	utime  uint64
	stime  uint64
	cutime uint64
	cstime uint64
}

type systemCPUStats struct {
	totalTime uint64
}

// NewProcessCollector creates a new process metrics collector
func NewProcessCollector() *ProcessCollector {
	return &ProcessCollector{
		logger:       logger.WithField("component", "process-collector"),
		lastCPUStats: make(map[int]*processCPUStats),
	}
}

// Collect gathers current process metrics
func (c *ProcessCollector) Collect() (*domain.ProcessMetrics, error) {
	processes, err := c.getProcessList()
	if err != nil {
		return nil, fmt.Errorf("failed to get process list: %w", err)
	}

	currentTime := time.Now()
	currentSystemCPU, err := c.getSystemCPUTime()
	if err != nil {
		c.logger.Warn("failed to get system CPU time", "error", err)
	}

	// Calculate CPU percentages if we have previous data
	if c.lastTime.Before(currentTime) && c.systemCPU != nil && currentSystemCPU != nil {
		timeDelta := currentTime.Sub(c.lastTime).Seconds()
		systemDelta := float64(currentSystemCPU.totalTime - c.systemCPU.totalTime)

		if systemDelta > 0 {
			c.calculateCPUPercentages(processes, timeDelta, systemDelta)
		}
	}

	// Count process states
	totalProcesses := len(processes)
	runningProcesses := 0
	zombieProcesses := 0
	totalThreads := 0

	for _, proc := range processes {
		switch proc.Status {
		case "R": // Running
			runningProcesses++
		case "Z": // Zombie
			zombieProcesses++
		}
		totalThreads += int(proc.numThreads)
	}

	// Sort processes by CPU and memory usage for top lists
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})
	topByCPU := c.getTopProcesses(processes, 10)

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].MemoryBytes > processes[j].MemoryBytes
	})
	topByMemory := c.getTopProcesses(processes, 10)

	metrics := &domain.ProcessMetrics{
		TotalProcesses:   totalProcesses,
		RunningProcesses: runningProcesses,
		ZombieProcesses:  zombieProcesses,
		TotalThreads:     totalThreads,
		TopByCPU:         topByCPU,
		TopByMemory:      topByMemory,
	}

	// Store current stats for next calculation
	c.lastCPUStats = make(map[int]*processCPUStats)
	for _, proc := range processes {
		c.lastCPUStats[proc.PID] = &processCPUStats{
			utime:  proc.utime,
			stime:  proc.stime,
			cutime: proc.cutime,
			cstime: proc.cstime,
		}
	}
	c.lastTime = currentTime
	c.systemCPU = currentSystemCPU

	return metrics, nil
}

type processInfo struct {
	domain.ProcessInfo
	utime      uint64
	stime      uint64
	cutime     uint64
	cstime     uint64
	numThreads uint64
}

// getProcessList reads process information from /proc
func (c *ProcessCollector) getProcessList() (processes []*processInfo, err error) {
	// Add panic recovery to prevent test failures
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in getProcessList: %v", r)
			c.logger.Error("recovered from panic in process collection", "panic", r)
		}
	}()

	processes = []*processInfo{}

	// In CI environments, be extra conservative
	inCI := os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
	maxProcesses := 1000
	if inCI {
		maxProcesses = 100 // Much smaller limit in CI to avoid resource issues
		c.logger.Debug("running in CI environment, using conservative process limits", "limit", maxProcesses)
	}

	// Read /proc directory directly instead of using WalkDir to avoid
	// potential hangs in CI environments with unusual /proc structures
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// On non-Linux systems or restricted containers, /proc might not exist or be accessible
		if os.IsNotExist(err) || os.IsPermission(err) {
			c.logger.Debug("/proc directory not accessible", "error", err)
			return processes, nil
		}
		return nil, fmt.Errorf("failed to read /proc directory: %w", err)
	}

	// Limit number of processes to avoid memory issues and long processing times
	processCount := 0

	for _, entry := range entries {
		if processCount >= maxProcesses {
			c.logger.Debug("reached maximum process limit", "limit", maxProcesses)
			break
		}

		// Only look at numeric directories (PIDs)
		if !entry.IsDir() || !c.isNumeric(entry.Name()) {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		process, err := c.readProcessInfo(pid)
		if err != nil {
			// Process might have disappeared, skip
			continue
		}

		if process != nil {
			processes = append(processes, process)
			processCount++
		}
	}

	return processes, nil
}

// readProcessInfo reads detailed information about a specific process
func (c *ProcessCollector) readProcessInfo(pid int) (proc *processInfo, err error) {
	// Add panic recovery for parsing edge cases
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic reading process %d: %v", pid, r)
			proc = nil
		}
	}()

	// Read /proc/PID/stat
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return nil, err
	}

	// Handle processes with spaces in names (e.g., "(test process)")
	// The name is enclosed in parentheses and can contain spaces
	statStr := string(statData)
	nameStart := strings.Index(statStr, "(")
	nameEnd := strings.LastIndex(statStr, ")")

	if nameStart == -1 || nameEnd == -1 || nameEnd <= nameStart {
		return nil, fmt.Errorf("invalid stat format for pid %d", pid)
	}

	// Extract fields after the name
	fieldsAfterName := strings.Fields(statStr[nameEnd+1:])
	if len(fieldsAfterName) < 22 {
		return nil, fmt.Errorf("insufficient fields in stat file for pid %d", pid)
	}

	// Parse basic info
	ppid, _ := strconv.Atoi(fieldsAfterName[1])
	state := fieldsAfterName[0]
	utime, _ := strconv.ParseUint(fieldsAfterName[11], 10, 64)
	stime, _ := strconv.ParseUint(fieldsAfterName[12], 10, 64)
	cutime, _ := strconv.ParseUint(fieldsAfterName[13], 10, 64)
	cstime, _ := strconv.ParseUint(fieldsAfterName[14], 10, 64)
	numThreads, _ := strconv.ParseUint(fieldsAfterName[17], 10, 64)
	_, _ = strconv.ParseUint(fieldsAfterName[20], 10, 64) // vsize - not used currently
	rss, _ := strconv.ParseUint(fieldsAfterName[21], 10, 64)

	// Read /proc/PID/comm for process name
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	commData, err := os.ReadFile(commPath)
	name := "unknown"
	if err == nil {
		name = strings.TrimSpace(string(commData))
	}

	// Read /proc/PID/cmdline for command
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	cmdlineData, err := os.ReadFile(cmdlinePath)
	command := name
	if err == nil && len(cmdlineData) > 0 {
		// Replace null bytes with spaces
		cmdline := string(cmdlineData)
		cmdline = strings.ReplaceAll(cmdline, "\x00", " ")
		command = strings.TrimSpace(cmdline)
		if command == "" {
			command = fmt.Sprintf("[%s]", name)
		}
	}

	// Get process start time
	startTime := time.Time{} // Default to zero time if we can't read it
	if len(fieldsAfterName) > 19 {
		startTimeJiffies, _ := strconv.ParseUint(fieldsAfterName[19], 10, 64)
		// Convert jiffies to time (approximate)
		// This is a simplified conversion - in reality we'd need boot time and HZ
		bootTime := time.Now().Add(-time.Hour * 24) // Rough approximation
		startTime = bootTime.Add(time.Duration(startTimeJiffies) * 10 * time.Millisecond)
	}

	// Convert RSS from pages to bytes (assuming 4KB pages)
	const pageSize = 4096
	memoryBytes := rss * pageSize

	process := &processInfo{
		ProcessInfo: domain.ProcessInfo{
			PID:         pid,
			PPID:        ppid,
			Name:        name,
			Command:     command,
			MemoryBytes: memoryBytes,
			Status:      state,
			StartTime:   startTime,
		},
		utime:      utime,
		stime:      stime,
		cutime:     cutime,
		cstime:     cstime,
		numThreads: numThreads,
	}

	return process, nil
}

// calculateCPUPercentages calculates CPU percentages for all processes
func (c *ProcessCollector) calculateCPUPercentages(processes []*processInfo, timeDelta, systemDelta float64) {
	// Get total system memory for percentage calculations
	totalMemory := c.getTotalMemory()

	for _, proc := range processes {
		// Calculate CPU percentage
		if lastStats, exists := c.lastCPUStats[proc.PID]; exists {
			totalCPUTime := float64(proc.utime + proc.stime - lastStats.utime - lastStats.stime)
			if systemDelta > 0 {
				proc.CPUPercent = (totalCPUTime / systemDelta) * 100.0
			}
		}

		// Calculate memory percentage
		if totalMemory > 0 {
			proc.MemoryPercent = float64(proc.MemoryBytes) / float64(totalMemory) * 100.0
		}
	}
}

// getSystemCPUTime gets total system CPU time
func (c *ProcessCollector) getSystemCPUTime() (*systemCPUStats, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read first line of /proc/stat")
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return nil, fmt.Errorf("unexpected format in /proc/stat")
	}

	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil, fmt.Errorf("insufficient CPU fields in /proc/stat")
	}

	var totalTime uint64
	for i := 1; i < len(fields); i++ {
		val, _ := strconv.ParseUint(fields[i], 10, 64)
		totalTime += val
	}

	return &systemCPUStats{totalTime: totalTime}, nil
}

// getTotalMemory gets total system memory in bytes
func (c *ProcessCollector) getTotalMemory() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024 // Convert KB to bytes
			}
		}
	}
	return 0
}

// getTopProcesses returns the top N processes from a sorted list
func (c *ProcessCollector) getTopProcesses(processes []*processInfo, n int) []domain.ProcessInfo {
	if len(processes) < n {
		n = len(processes)
	}

	result := make([]domain.ProcessInfo, n)
	for i := 0; i < n; i++ {
		result[i] = processes[i].ProcessInfo
	}

	return result
}

// isNumeric checks if a string contains only digits
func (c *ProcessCollector) isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
