package job

import (
	"fmt"
	"path/filepath"
	"time"

	"joblet/internal/joblet/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Builder creates jobs with validation and defaults.
// Core job construction service that transforms requests into validated domain objects
// with proper UUIDs, resource limits, and field population.
type Builder struct {
	config      *config.Config
	logger      *logger.Logger
	idGenerator *IDGenerator
}

// NewBuilder creates a new job builder.
// Initializes builder with configuration, UUID generator, and resource validator
// for comprehensive job construction with validation.
func NewBuilder(cfg *config.Config, idGen *IDGenerator) *Builder {
	return &Builder{
		config:      cfg,
		logger:      logger.New().WithField("component", "job-builder"),
		idGenerator: idGen,
	}
}

// BuildRequest represents a request to build a job.
// Simplified from interface to concrete struct since there's only one real implementation.
type BuildRequest struct {
	Command           string
	Args              []string
	Limits            domain.ResourceLimits
	Schedule          string // Added for compatibility with scheduling
	Network           string
	Volumes           []string
	Runtime           string
	Environment       map[string]string
	SecretEnvironment map[string]string
	JobType           domain.JobType
	WorkflowUuid      string
	WorkingDirectory  string
	Uploads           []domain.FileUpload
	Dependencies      []string
	GPUCount          int32 // Number of GPUs requested
	GPUMemoryMB       int64 // GPU memory requirement in MB
}

// Build creates a new job from the request.
// Main construction method: generates UUID, populates all job fields,
// applies resource defaults, and validates configuration.
func (b *Builder) Build(req BuildRequest) (*domain.Job, error) {
	// Generate UUID
	jobUuid := b.idGenerator.Next()

	b.logger.Debug("building job", "jobUuid", jobUuid, "command", req.Command)

	// Create job - debug all field values
	volumes := b.copyStrings(req.Volumes)
	network := req.Network
	runtime := req.Runtime

	b.logger.Debug("building job with all fields",
		"jobUuid", jobUuid,
		"network", network,
		"volumes", volumes,
		"runtime", runtime,
		"hasNetwork", network != "",
		"volumeCount", len(volumes),
		"hasRuntime", runtime != "")

	job := &domain.Job{
		Uuid:              jobUuid,
		Command:           req.Command,
		Args:              b.copyStrings(req.Args),
		Type:              b.determineJobType(req), // Set job type
		Status:            domain.StatusInitializing,
		CgroupPath:        b.generateCgroupPath(jobUuid),
		StartTime:         time.Now(),
		Network:           req.Network,
		Volumes:           volumes,
		Runtime:           req.Runtime,
		Environment:       b.copyEnvironment(req.Environment),
		SecretEnvironment: b.copyEnvironment(req.SecretEnvironment),
		WorkflowUuid:      req.WorkflowUuid,
		WorkingDirectory:  req.WorkingDirectory,
		Uploads:           req.Uploads,
		Dependencies:      b.copyStrings(req.Dependencies),
		GPUCount:          req.GPUCount,           // GPU requirements
		GPUMemoryMB:       req.GPUMemoryMB,        // GPU memory requirement
		GPUIndices:        []int32{},              // Will be populated during allocation
		NodeId:            b.config.Server.NodeId, // Unique identifier of the Joblet node
	}

	// Apply resource limits with defaults
	job.Limits = b.applyResourceDefaults(req.Limits)

	// Basic resource limit validation (simplified)
	if job.Limits.CPU.Value() < 0 || job.Limits.CPU.Value() > 100 {
		return nil, fmt.Errorf("invalid CPU limit: must be between 0-100")
	}
	if job.Limits.Memory.Bytes() < 0 {
		return nil, fmt.Errorf("invalid memory limit: must be positive")
	}

	b.logger.Debug("job built successfully",
		"jobUuid", jobUuid,
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
func (b *Builder) generateCgroupPath(jobUUID string) string {
	return filepath.Join(b.config.Cgroup.BaseDir, "job-"+jobUUID)
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

// copyEnvironment creates a copy of environment map
func (b *Builder) copyEnvironment(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// determineJobType determines the job type based on the request
func (b *Builder) determineJobType(req BuildRequest) domain.JobType {
	// Use the explicit JobType from the request (set by service layer)
	jobType := req.JobType

	if jobType == domain.JobTypeRuntimeBuild {
		b.logger.Debug("using runtime build job type from service request")
	} else {
		b.logger.Debug("using standard job type from service request")
	}

	return jobType
}
