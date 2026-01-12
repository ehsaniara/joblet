package gpu

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/ehsaniara/joblet/pkg/logger"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// CUDAVerifierInterface defines the interface for CUDA verification
//
//counterfeiter:generate . CUDAVerifierInterface
type CUDAVerifierInterface interface {
	IsAvailable() bool
	Verify(ctx context.Context) (*CUDAVerificationResult, error)
	CheckGPUsUsable(ctx context.Context, gpuIndices []int) error
}

// CUDAVerificationResult contains the results of CUDA runtime verification
type CUDAVerificationResult struct {
	Success           bool
	Error             string
	DriverVersion     string
	CUDAVersion       string
	DeviceCount       int
	UsableDeviceCount int
	Devices           []CUDADeviceInfo
}

// CUDADeviceInfo contains information about a single CUDA device
type CUDADeviceInfo struct {
	Index             int
	Name              string
	UUID              string
	ComputeCapability string
	TotalMemoryMB     int64
	FreeMemoryMB      int64
	Usable            bool
	UsableError       string
}

// CUDAVerifier handles CUDA runtime verification using nvidia-smi
type CUDAVerifier struct {
	logger *logger.Logger
	mu     sync.Mutex
}

// NewCUDAVerifier creates a new CUDA verifier
func NewCUDAVerifier() *CUDAVerifier {
	return &CUDAVerifier{
		logger: logger.New().WithField("component", "cuda-verifier"),
	}
}

// IsAvailable checks if nvidia-smi is available on this system
func (v *CUDAVerifier) IsAvailable() bool {
	_, err := exec.LookPath("nvidia-smi")
	return err == nil
}

// Verify performs CUDA runtime verification using nvidia-smi
func (v *CUDAVerifier) Verify(ctx context.Context) (*CUDAVerificationResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.IsAvailable() {
		return &CUDAVerificationResult{
			Success: false,
			Error:   "nvidia-smi not found",
		}, nil
	}

	result := &CUDAVerificationResult{Success: true}

	// Get driver and CUDA version
	if err := v.getVersionInfo(ctx, result); err != nil {
		return &CUDAVerificationResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get version info: %v", err),
		}, nil
	}

	// Get device information
	if err := v.getDeviceInfo(ctx, result); err != nil {
		return &CUDAVerificationResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get device info: %v", err),
		}, nil
	}

	return result, nil
}

func (v *CUDAVerifier) getVersionInfo(ctx context.Context, result *CUDAVerificationResult) error {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=driver_version",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		result.DriverVersion = strings.TrimSpace(lines[0])
	}

	// Get CUDA version from nvidia-smi header
	cmd = exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	output, _ = cmd.Output()

	// Try to get CUDA version from nvidia-smi output
	cmd = exec.CommandContext(ctx, "nvidia-smi")
	output, err = cmd.Output()
	if err == nil {
		// Parse CUDA version from nvidia-smi output header
		// Format: "CUDA Version: 12.2"
		outputStr := string(output)
		if idx := strings.Index(outputStr, "CUDA Version:"); idx != -1 {
			rest := outputStr[idx+len("CUDA Version:"):]
			if endIdx := strings.IndexAny(rest, " |\n"); endIdx != -1 {
				result.CUDAVersion = strings.TrimSpace(rest[:endIdx])
			} else {
				result.CUDAVersion = strings.TrimSpace(rest)
			}
		}
	}

	return nil
}

func (v *CUDAVerifier) getDeviceInfo(ctx context.Context, result *CUDAVerificationResult) error {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,uuid,compute_cap,memory.total,memory.free",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		device, err := v.parseDeviceLine(line)
		if err != nil {
			v.logger.Warn("failed to parse device line", "line", line, "error", err)
			continue
		}

		result.Devices = append(result.Devices, device)
		if device.Usable {
			result.UsableDeviceCount++
		}
	}

	result.DeviceCount = len(result.Devices)
	return nil
}

func (v *CUDAVerifier) parseDeviceLine(line string) (CUDADeviceInfo, error) {
	parts := strings.Split(line, ", ")
	if len(parts) < 6 {
		return CUDADeviceInfo{}, fmt.Errorf("unexpected format: %s", line)
	}

	info := CUDADeviceInfo{Usable: true}

	// Index
	idx, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return CUDADeviceInfo{}, fmt.Errorf("invalid index: %s", parts[0])
	}
	info.Index = idx

	// Name
	info.Name = strings.TrimSpace(parts[1])

	// UUID
	info.UUID = strings.TrimSpace(parts[2])

	// Compute capability
	info.ComputeCapability = strings.TrimSpace(parts[3])

	// Total memory (MB)
	totalMem, err := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
	if err != nil {
		return CUDADeviceInfo{}, fmt.Errorf("invalid total memory: %s", parts[4])
	}
	info.TotalMemoryMB = totalMem

	// Free memory (MB)
	freeMem, err := strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	if err != nil {
		return CUDADeviceInfo{}, fmt.Errorf("invalid free memory: %s", parts[5])
	}
	info.FreeMemoryMB = freeMem

	return info, nil
}

// CheckGPUsUsable verifies that specific GPUs are usable
func (v *CUDAVerifier) CheckGPUsUsable(ctx context.Context, gpuIndices []int) error {
	result, err := v.Verify(ctx)
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("GPU verification failed: %s", result.Error)
	}

	deviceMap := make(map[int]*CUDADeviceInfo)
	for i := range result.Devices {
		deviceMap[result.Devices[i].Index] = &result.Devices[i]
	}

	for _, idx := range gpuIndices {
		dev, ok := deviceMap[idx]
		if !ok {
			return fmt.Errorf("GPU %d not found", idx)
		}
		if !dev.Usable {
			return fmt.Errorf("GPU %d not usable: %s", idx, dev.UsableError)
		}
	}

	return nil
}

// Compute capability helpers

// IsComputeCapabilityCompatible checks if actual >= required (e.g., "8.6" >= "7.0")
func IsComputeCapabilityCompatible(actual, required string) bool {
	actMajor, actMinor := ParseComputeCapability(actual)
	reqMajor, reqMinor := ParseComputeCapability(required)

	if actMajor > reqMajor {
		return true
	}
	return actMajor == reqMajor && actMinor >= reqMinor
}

// ParseComputeCapability parses "X.Y" format
func ParseComputeCapability(cap string) (major, minor int) {
	parts := strings.Split(cap, ".")
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return
}
