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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Builder creates jobs with validation and defaults.
// Core job construction service that transforms requests into validated domain objects
// with proper UUIDs, resource limits, and field population.
type Builder struct {
	config       *config.Config
	logger       *logger.Logger
	idGenerator  *IDGenerator
	resValidator *validation.ResourceValidator
}

// NewBuilder creates a new job builder.
// Initializes builder with configuration, UUID generator, and resource validator
// for comprehensive job construction with validation.
func NewBuilder(cfg *config.Config, idGen *IDGenerator, resValidator *validation.ResourceValidator) *Builder {
	return &Builder{
		config:       cfg,
		logger:       logger.New().WithField("component", "job-builder"),
		idGenerator:  idGen,
		resValidator: resValidator,
	}
}

// BuildRequest represents a request to build a job.
// Interface defining all parameters needed for job construction,
// implemented by various request types (individual, workflow, scheduled).
//
//counterfeiter:generate . BuildRequest
type BuildRequest interface {
	GetCommand() string
	GetArgs() []string
	GetLimits() domain.ResourceLimits
	GetNetwork() string
	GetVolumes() []string
	GetRuntime() string
	GetEnvironment() map[string]string
	GetSecretEnvironment() map[string]string
	GetJobType() domain.JobType
	GetWorkflowUuid() string
	GetWorkingDirectory() string
	GetUploads() []domain.FileUpload
	GetDependencies() []string
}

// Build creates a new job from the request.
// Main construction method: generates UUID, populates all job fields,
// applies resource defaults, and validates configuration.
func (b *Builder) Build(req BuildRequest) (*domain.Job, error) {
	// Generate UUID
	jobUuid := b.idGenerator.Next()

	b.logger.Debug("building job", "jobUuid", jobUuid, "command", req.GetCommand())

	// Create job - debug all field values
	volumes := b.copyStrings(req.GetVolumes())
	network := req.GetNetwork()
	runtime := req.GetRuntime()

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
		Command:           req.GetCommand(),
		Args:              b.copyStrings(req.GetArgs()),
		Type:              b.determineJobType(req), // Set job type
		Status:            domain.StatusInitializing,
		CgroupPath:        b.generateCgroupPath(jobUuid),
		StartTime:         time.Now(),
		Network:           req.GetNetwork(),
		Volumes:           volumes,
		Runtime:           req.GetRuntime(),
		Environment:       b.copyEnvironment(req.GetEnvironment()),
		SecretEnvironment: b.copyEnvironment(req.GetSecretEnvironment()),
		WorkflowUuid:      req.GetWorkflowUuid(),
		WorkingDirectory:  req.GetWorkingDirectory(),
		Uploads:           req.GetUploads(),
		Dependencies:      b.copyStrings(req.GetDependencies()),
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
func (b *Builder) generateCgroupPath(jobUuid string) string {
	return filepath.Join(b.config.Cgroup.BaseDir, "job-"+jobUuid)
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

// BuildParams can be used as an alternative to the interface approach
type BuildParams struct {
	Command           string
	Network           string
	Args              []string
	Limits            domain.ResourceLimits
	Volumes           []string
	Runtime           string
	Environment       map[string]string
	SecretEnvironment map[string]string
	JobType           domain.JobType
	WorkflowUuid      string
	WorkingDirectory  string
	Uploads           []domain.FileUpload
	Dependencies      []string
}

// Implement BuildRequest interface for BuildParams
func (p BuildParams) GetCommand() string                      { return p.Command }
func (p BuildParams) GetArgs() []string                       { return p.Args }
func (p BuildParams) GetLimits() domain.ResourceLimits        { return p.Limits }
func (p BuildParams) GetNetwork() string                      { return p.Network }
func (p BuildParams) GetVolumes() []string                    { return p.Volumes }
func (p BuildParams) GetRuntime() string                      { return p.Runtime }
func (p BuildParams) GetEnvironment() map[string]string       { return p.Environment }
func (p BuildParams) GetSecretEnvironment() map[string]string { return p.SecretEnvironment }
func (p BuildParams) GetJobType() domain.JobType              { return p.JobType }
func (p BuildParams) GetWorkflowUuid() string                 { return p.WorkflowUuid }
func (p BuildParams) GetWorkingDirectory() string             { return p.WorkingDirectory }
func (p BuildParams) GetUploads() []domain.FileUpload         { return p.Uploads }
func (p BuildParams) GetDependencies() []string               { return p.Dependencies }

// BuildFromParams builds a job from BuildParams (convenience method)
func (b *Builder) BuildFromParams(params BuildParams) (*domain.Job, error) {
	return b.Build(params)
}

// determineJobType determines the job type based on the request
func (b *Builder) determineJobType(req BuildRequest) domain.JobType {
	// Use the explicit JobType from the request (set by service layer)
	jobType := req.GetJobType()

	if jobType == domain.JobTypeRuntimeBuild {
		b.logger.Debug("using runtime build job type from service request")
	} else {
		b.logger.Debug("using standard job type from service request")
	}

	return jobType
}
