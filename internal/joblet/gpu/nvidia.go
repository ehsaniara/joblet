package gpu

import (
	"bufio"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// NvidiaDiscovery implements GPU discovery for NVIDIA GPUs using /proc filesystem
type NvidiaDiscovery struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewNvidiaDiscovery creates a new NVIDIA GPU discovery service
func NewNvidiaDiscovery(platform platform.Platform) *NvidiaDiscovery {
	return &NvidiaDiscovery{
		platform: platform,
		logger:   logger.New().WithField("component", "nvidia-discovery"),
	}
}

// DiscoverGPUs discovers NVIDIA GPUs using the /proc filesystem
func (n *NvidiaDiscovery) DiscoverGPUs() ([]*GPU, error) {
	n.logger.Debug("starting NVIDIA GPU discovery")

	// First try /proc/driver/nvidia/gpus/ (most reliable)
	gpus, err := n.discoverFromProc()
	if err == nil && len(gpus) > 0 {
		n.logger.Info("discovered GPUs via /proc filesystem", "count", len(gpus))
		return gpus, nil
	}

	n.logger.Debug("proc discovery failed or found no GPUs, trying nvidia-smi fallback", "error", err)

	// Fallback to nvidia-smi
	gpus, err = n.discoverFromNvidiaSmi()
	if err != nil {
		return nil, fmt.Errorf("GPU discovery failed: proc method failed, nvidia-smi fallback also failed: %w", err)
	}

	n.logger.Info("discovered GPUs via nvidia-smi", "count", len(gpus))
	return gpus, nil
}

// discoverFromProc discovers GPUs using /proc/driver/nvidia/gpus/
func (n *NvidiaDiscovery) discoverFromProc() ([]*GPU, error) {
	procNvidiaDir := "/proc/driver/nvidia/gpus"

	// Check if nvidia driver is loaded
	if _, err := n.platform.Stat(procNvidiaDir); err != nil {
		if n.platform.IsNotExist(err) {
			return nil, fmt.Errorf("NVIDIA driver not loaded: %s does not exist", procNvidiaDir)
		}
		return nil, fmt.Errorf("failed to access %s: %w", procNvidiaDir, err)
	}

	entries, err := n.platform.ReadDir(procNvidiaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", procNvidiaDir, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no GPUs found in %s", procNvidiaDir)
	}

	var gpus []*GPU
	for i, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		gpu, err := n.parseGPUFromProc(i, entry.Name(), procNvidiaDir)
		if err != nil {
			n.logger.Warn("failed to parse GPU from proc", "gpu", entry.Name(), "error", err)
			continue
		}

		gpus = append(gpus, gpu)
	}

	return gpus, nil
}

// parseGPUFromProc parses GPU information from /proc/driver/nvidia/gpus/{uuid}/
func (n *NvidiaDiscovery) parseGPUFromProc(index int, gpuDir, procNvidiaDir string) (*GPU, error) {
	gpuPath := filepath.Join(procNvidiaDir, gpuDir)

	gpu := &GPU{
		Index: index,
		UUID:  gpuDir, // The directory name is usually the GPU UUID
	}

	// Try to read information file
	infoPath := filepath.Join(gpuPath, "information")
	if data, err := n.platform.ReadFile(infoPath); err == nil {
		if parsedGPU := n.parseGPUInfo(string(data)); parsedGPU != nil {
			parsedGPU.Index = index
			parsedGPU.UUID = gpuDir
			return parsedGPU, nil
		}
	}

	n.logger.Debug("could not read GPU information file, using minimal info", "path", infoPath)
	return gpu, nil
}

// parseGPUInfo parses GPU information from the information file content
func (n *NvidiaDiscovery) parseGPUInfo(content string) *GPU {
	gpu := &GPU{}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse various fields
		if strings.HasPrefix(line, "Model:") {
			gpu.Name = strings.TrimSpace(strings.TrimPrefix(line, "Model:"))
		} else if strings.HasPrefix(line, "GPU UUID:") {
			uuid := strings.TrimSpace(strings.TrimPrefix(line, "GPU UUID:"))
			// Remove "GPU-" prefix if present
			gpu.UUID = strings.TrimPrefix(uuid, "GPU-")
		} else if strings.HasPrefix(line, "Total Memory:") {
			// Try to parse memory (format varies)
			memStr := strings.TrimSpace(strings.TrimPrefix(line, "Total Memory:"))
			if memory := n.parseMemoryString(memStr); memory > 0 {
				gpu.MemoryMB = memory
			}
		}
	}

	if gpu.Name == "" && gpu.UUID == "" {
		return nil
	}

	return gpu
}

// discoverFromNvidiaSmi discovers GPUs using nvidia-smi command
func (n *NvidiaDiscovery) discoverFromNvidiaSmi() ([]*GPU, error) {
	// Try to run nvidia-smi
	cmd := n.platform.CreateCommand("nvidia-smi",
		"--query-gpu=index,uuid,name,memory.total",
		"--format=csv,noheader,nounits")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi command failed: %w", err)
	}

	return n.parseNvidiaSmiOutput(string(output))
}

// parseNvidiaSmiOutput parses nvidia-smi CSV output
func (n *NvidiaDiscovery) parseNvidiaSmiOutput(output string) ([]*GPU, error) {
	var gpus []*GPU

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			n.logger.Warn("invalid nvidia-smi line format", "line", line)
			continue
		}

		// Parse fields
		indexStr := strings.TrimSpace(parts[0])
		uuid := strings.TrimSpace(parts[1])
		name := strings.TrimSpace(parts[2])
		memoryStr := strings.TrimSpace(parts[3])

		index, err := strconv.Atoi(indexStr)
		if err != nil {
			n.logger.Warn("invalid GPU index", "index", indexStr, "error", err)
			continue
		}

		memory := n.parseMemoryString(memoryStr)

		// Clean up UUID (remove GPU- prefix if present)
		uuid = strings.TrimPrefix(uuid, "GPU-")

		gpu := &GPU{
			Index:    index,
			UUID:     uuid,
			Name:     name,
			MemoryMB: memory,
			InUse:    false,
		}

		gpus = append(gpus, gpu)
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no GPUs parsed from nvidia-smi output")
	}

	return gpus, nil
}

// parseMemoryString attempts to parse memory strings in various formats
func (n *NvidiaDiscovery) parseMemoryString(memStr string) int64 {
	// Remove any whitespace
	memStr = strings.TrimSpace(memStr)

	// Try different regex patterns for memory parsing
	patterns := []string{
		`(\d+)\s*MiB`, // "11264 MiB"
		`(\d+)\s*MB`,  // "11264 MB"
		`(\d+)\s*GiB`, // "11 GiB"
		`(\d+)\s*GB`,  // "11 GB"
		`(\d+)`,       // Just a number (assume MB)
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(memStr)
		if len(matches) >= 2 {
			value, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				continue
			}

			// Convert to MB based on unit
			if strings.Contains(pattern, "GiB") || strings.Contains(pattern, "GB") {
				return value * 1024 // GB to MB
			}
			return value // Already in MB
		}
	}

	n.logger.Debug("could not parse memory string", "memoryString", memStr)
	return 0
}

// RefreshGPUs refreshes GPU information (for interface compliance)
func (n *NvidiaDiscovery) RefreshGPUs() error {
	// For now, refreshing would require re-discovery
	// In a more advanced implementation, this could check for hot-plugged GPUs
	n.logger.Debug("GPU refresh requested (no-op for current implementation)")
	return nil
}
