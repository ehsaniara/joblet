package job

import (
	"fmt"
	"path/filepath"
	"time"

	"joblet/internal/joblet/core/validation"
	"joblet/internal/joblet/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Builder creates jobs with validation and defaults
type Builder struct {
	config       *config.Config
	logger       *logger.Logger
	idGenerator  *IDGenerator
	resValidator *validation.ResourceValidator
}

// NewBuilder creates a new job builder
func NewBuilder(cfg *config.Config, idGen *IDGenerator, resValidator *validation.ResourceValidator) *Builder {
	return &Builder{
		config:       cfg,
		logger:       logger.New().WithField("component", "job-builder"),
		idGenerator:  idGen,
		resValidator: resValidator,
	}
}

// BuildRequest represents a request to build a job
type BuildRequest interface {
	GetCommand() string
	GetArgs() []string
	GetLimits() domain.ResourceLimits
	GetNetwork() string
	GetVolumes() []string
}

// Build creates a new job from the request
func (b *Builder) Build(req BuildRequest) (*domain.Job, error) {
	// Generate ID
	jobID := b.idGenerator.Next()

	b.logger.Debug("building job", "jobID", jobID, "command", req.GetCommand())

	// Create job
	job := &domain.Job{
		Id:         jobID,
		Command:    req.GetCommand(),
		Args:       b.copyStrings(req.GetArgs()),
		Status:     domain.StatusInitializing,
		CgroupPath: b.generateCgroupPath(jobID),
		StartTime:  time.Now(),
		Network:    req.GetNetwork(),
		Volumes:    b.copyStrings(req.GetVolumes()),
	}

	// Apply resource limits with defaults
	job.Limits = b.applyResourceDefaults(req.GetLimits())

	// Calculate effective CPU if cores are specified
	if !job.Limits.CPUCores.IsEmpty() {
		b.resValidator.CalculateEffectiveLimits(&job.Limits)
	}

	// Final validation
	if err := b.resValidator.Validate(job.Limits); err != nil {
		return nil, fmt.Errorf("resource validation failed: %w", err)
	}

	b.logger.Debug("job built successfully",
		"jobID", jobID,
		"cpu", job.Limits.CPU.Value(),
		"memory", job.Limits.Memory.Megabytes(),
		"io", job.Limits.IOBandwidth.BytesPerSecond())

	return job, nil
}

// applyResourceDefaults applies default resource limits
func (b *Builder) applyResourceDefaults(limits domain.ResourceLimits) domain.ResourceLimits {
	// Use existing values or defaults
	cpuValue := limits.CPU.Value()
	if cpuValue <= 0 {
		cpuValue = b.config.Joblet.DefaultCPULimit
	}

	memoryValue := limits.Memory.Megabytes()
	if memoryValue <= 0 {
		memoryValue = b.config.Joblet.DefaultMemoryLimit
	}

	ioValue := limits.IOBandwidth.BytesPerSecond()
	if ioValue <= 0 {
		ioValue = int64(b.config.Joblet.DefaultIOLimit)
	}

	// Use CPU cores from existing limits or empty string
	cpuCores := ""
	if !limits.CPUCores.IsEmpty() {
		cpuCores = limits.CPUCores.String()
	}

	// Create new limits with defaults applied
	result := domain.NewResourceLimitsFromParams(cpuValue, cpuCores, memoryValue, ioValue)
	return *result
}

// generateCgroupPath generates the cgroup path for a job
func (b *Builder) generateCgroupPath(jobID string) string {
	return filepath.Join(b.config.Cgroup.BaseDir, "job-"+jobID)
}

// copyStrings creates a copy of string slice
func (b *Builder) copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// BuildParams can be used as an alternative to the interface approach
type BuildParams struct {
	Command string
	Network string
	Args    []string
	Limits  domain.ResourceLimits
	Volumes []string
}

// Implement BuildRequest interface for BuildParams
func (p BuildParams) GetCommand() string               { return p.Command }
func (p BuildParams) GetArgs() []string                { return p.Args }
func (p BuildParams) GetLimits() domain.ResourceLimits { return p.Limits }
func (p BuildParams) GetNetwork() string               { return p.Network }
func (p BuildParams) GetVolumes() []string             { return p.Volumes }

// BuildFromParams builds a job from BuildParams (convenience method)
func (b *Builder) BuildFromParams(params BuildParams) (*domain.Job, error) {
	return b.Build(params)
}
