package validation

import (
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"runtime"
	"strconv"
	"strings"
)

// ResourceValidator validates resource limits and specifications
type ResourceValidator struct {
	// System limits
	maxCPUPercent  int32
	maxMemoryMB    int32
	maxIOBPS       int32
	availableCores int

	// Minimum limits
	minCPUPercent int32
	minMemoryMB   int32
	minIOBPS      int32
}

// NewResourceValidator creates a new resource validator
func NewResourceValidator() *ResourceValidator {
	return &ResourceValidator{
		// Maximum limits
		maxCPUPercent: 1000,    // 10 cores worth
		maxMemoryMB:   32768,   // 32GB
		maxIOBPS:      1000000, // 1GB/s

		// Minimum limits
		minCPUPercent: 10,   // 0.1 core
		minMemoryMB:   64,   // 64MB minimum
		minIOBPS:      1000, // 1KB/s minimum

		// System info
		availableCores: runtime.NumCPU(),
	}
}

// Validate validates resource limits
func (rv *ResourceValidator) Validate(limits domain.ResourceLimits) error {
	// Validate CPU
	if err := rv.validateCPU(limits.MaxCPU, limits.CPUCores); err != nil {
		return err
	}

	// Validate Memory
	if err := rv.validateMemory(limits.MaxMemory); err != nil {
		return err
	}

	// Validate IO
	if err := rv.validateIO(limits.MaxIOBPS); err != nil {
		return err
	}

	// Validate CPU cores specification if present
	if limits.CPUCores != "" {
		if err := rv.validateCPUCores(limits.CPUCores); err != nil {
			return err
		}
	}

	// Cross-validate (e.g., CPU cores vs CPU percentage)
	if err := rv.crossValidate(limits); err != nil {
		return err
	}

	return nil
}

// validateCPU validates CPU limits
func (rv *ResourceValidator) validateCPU(maxCPU int32, cpuCores string) error {
	if maxCPU < 0 {
		return fmt.Errorf("CPU limit cannot be negative")
	}

	if maxCPU > 0 {
		if maxCPU < rv.minCPUPercent {
			return fmt.Errorf("CPU limit too low (minimum %d%%)", rv.minCPUPercent)
		}

		if maxCPU > rv.maxCPUPercent {
			return fmt.Errorf("CPU limit too high (maximum %d%%)", rv.maxCPUPercent)
		}
	}

	return nil
}

// validateMemory validates memory limits
func (rv *ResourceValidator) validateMemory(maxMemory int32) error {
	if maxMemory < 0 {
		return fmt.Errorf("memory limit cannot be negative")
	}

	if maxMemory > 0 {
		if maxMemory < rv.minMemoryMB {
			return fmt.Errorf("memory limit too low (minimum %dMB)", rv.minMemoryMB)
		}

		if maxMemory > rv.maxMemoryMB {
			return fmt.Errorf("memory limit too high (maximum %dMB)", rv.maxMemoryMB)
		}
	}

	return nil
}

// validateIO validates IO limits
func (rv *ResourceValidator) validateIO(maxIOBPS int32) error {
	if maxIOBPS < 0 {
		return fmt.Errorf("IO limit cannot be negative")
	}

	if maxIOBPS > 0 {
		if maxIOBPS < rv.minIOBPS {
			return fmt.Errorf("IO limit too low (minimum %d BPS)", rv.minIOBPS)
		}

		if maxIOBPS > rv.maxIOBPS {
			return fmt.Errorf("IO limit too high (maximum %d BPS)", rv.maxIOBPS)
		}
	}

	return nil
}

// validateCPUCores validates CPU core specification
func (rv *ResourceValidator) validateCPUCores(cores string) error {
	if cores == "" {
		return nil
	}

	// Parse the specification
	coreList, err := rv.parseCoreSpecification(cores)
	if err != nil {
		return fmt.Errorf("invalid CPU core specification: %v", err)
	}

	// Check each core
	for _, coreNum := range coreList {
		if coreNum < 0 {
			return fmt.Errorf("invalid core number: %d", coreNum)
		}

		if coreNum >= rv.availableCores {
			return fmt.Errorf("core %d not available (system has %d cores)", coreNum, rv.availableCores)
		}
	}

	// Check for duplicates
	seen := make(map[int]bool)
	for _, core := range coreList {
		if seen[core] {
			return fmt.Errorf("duplicate core specification: %d", core)
		}
		seen[core] = true
	}

	return nil
}

// parseCoreSpecification parses CPU core specification
func (rv *ResourceValidator) parseCoreSpecification(cores string) ([]int, error) {
	var result []int

	// Handle range format: "0-3"
	if strings.Contains(cores, "-") {
		parts := strings.Split(cores, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}

		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid range numbers")
		}

		if start > end {
			return nil, fmt.Errorf("invalid range: start > end")
		}

		for i := start; i <= end; i++ {
			result = append(result, i)
		}
		return result, nil
	}

	// Handle list format: "0,2,4,6"
	for _, coreStr := range strings.Split(cores, ",") {
		core, err := strconv.Atoi(strings.TrimSpace(coreStr))
		if err != nil {
			return nil, fmt.Errorf("invalid core number: %s", coreStr)
		}
		result = append(result, core)
	}

	return result, nil
}

// crossValidate performs cross-validation of resource limits
func (rv *ResourceValidator) crossValidate(limits domain.ResourceLimits) error {
	// If CPU cores are specified, validate against CPU percentage
	if limits.CPUCores != "" {
		coreList, _ := rv.parseCoreSpecification(limits.CPUCores)
		maxAllowedCPU := int32(len(coreList) * 100)

		if limits.MaxCPU > 0 && limits.MaxCPU > maxAllowedCPU {
			return fmt.Errorf("CPU limit (%d%%) exceeds allocated cores (%d cores = max %d%%)",
				limits.MaxCPU, len(coreList), maxAllowedCPU)
		}
	}

	// Validate memory vs CPU ratio (optional)
	if limits.MaxCPU > 0 && limits.MaxMemory > 0 {
		// Rough guideline: 1 CPU core (100%) should have at least 1GB RAM
		minMemoryPerCore := int32(1024) // 1GB in MB
		requiredMemory := (limits.MaxCPU / 100) * minMemoryPerCore

		if limits.MaxMemory < requiredMemory {
			// This is a warning, not an error
			// Could log or handle differently
			logger.Warn("limits.MaxMemory < requiredMemory")
		}
	}

	return nil
}

// CalculateEffectiveLimits calculates effective limits based on specifications
func (rv *ResourceValidator) CalculateEffectiveLimits(limits *domain.ResourceLimits) {
	// If CPU cores are specified but MaxCPU is not, calculate it
	if limits.CPUCores != "" && limits.MaxCPU == 0 {
		coreList, err := rv.parseCoreSpecification(limits.CPUCores)
		if err == nil {
			limits.MaxCPU = int32(len(coreList) * 100)
		}
	}

	// Apply other calculations as needed
}

// GetSystemLimits returns the current system limits
func (rv *ResourceValidator) GetSystemLimits() map[string]interface{} {
	return map[string]interface{}{
		"max_cpu_percent": rv.maxCPUPercent,
		"max_memory_mb":   rv.maxMemoryMB,
		"max_io_bps":      rv.maxIOBPS,
		"available_cores": rv.availableCores,
		"min_cpu_percent": rv.minCPUPercent,
		"min_memory_mb":   rv.minMemoryMB,
		"min_io_bps":      rv.minIOBPS,
	}
}

// SetSystemLimits updates the system limits (useful for testing or configuration)
func (rv *ResourceValidator) SetSystemLimits(maxCPU, maxMemory, maxIO int32) {
	rv.maxCPUPercent = maxCPU
	rv.maxMemoryMB = maxMemory
	rv.maxIOBPS = maxIO
}
