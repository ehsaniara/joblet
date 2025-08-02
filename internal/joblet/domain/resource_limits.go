package domain

import (
	"joblet/internal/joblet/domain/values"
)

// ResourceLimits represents resource constraints using proper value objects
type ResourceLimits struct {
	CPU         values.CPUPercentage
	CPUCores    values.CPUCoreSet
	Memory      values.MemorySize
	IOBandwidth values.Bandwidth
}

// NewResourceLimits creates a new ResourceLimits with defaults
func NewResourceLimits() *ResourceLimits {
	cpu, _ := values.NewCPUPercentage(0)
	cpuCores, _ := values.ParseCPUCoreSet("")
	memory, _ := values.NewMemorySize(0)
	ioBandwidth, _ := values.NewBandwidth(0)

	return &ResourceLimits{
		CPU:         cpu,
		CPUCores:    cpuCores,
		Memory:      memory,
		IOBandwidth: ioBandwidth,
	}
}

// NewResourceLimitsFromParams creates ResourceLimits from basic parameters
func NewResourceLimitsFromParams(cpuPercent int32, cpuCores string, memoryMB int32, ioBPS int64) *ResourceLimits {
	limits := NewResourceLimits()

	if cpuPercent > 0 {
		if cpu, err := values.NewCPUPercentage(cpuPercent); err == nil {
			limits.CPU = cpu
		}
	}

	if cpuCores != "" {
		if cores, err := values.ParseCPUCoreSet(cpuCores); err == nil {
			limits.CPUCores = cores
		}
	}

	if memoryMB > 0 {
		if mem, err := values.NewMemorySizeFromMB(memoryMB); err == nil {
			limits.Memory = mem
		}
	}

	if ioBPS > 0 {
		if bw, err := values.NewBandwidth(ioBPS); err == nil {
			limits.IOBandwidth = bw
		}
	}

	return limits
}

// HasCPULimit returns true if a CPU limit is set
func (r *ResourceLimits) HasCPULimit() bool {
	return !r.CPU.IsUnlimited()
}

// HasMemoryLimit returns true if a memory limit is set
func (r *ResourceLimits) HasMemoryLimit() bool {
	return !r.Memory.IsUnlimited()
}

// HasIOLimit returns true if an I/O limit is set
func (r *ResourceLimits) HasIOLimit() bool {
	return !r.IOBandwidth.IsUnlimited()
}

// HasCoreRestriction returns true if CPU cores are restricted
func (r *ResourceLimits) HasCoreRestriction() bool {
	return !r.CPUCores.IsEmpty()
}
