package implementation

import (
	"context"
	"fmt"

	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/mappers"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// JobletImplementation provides a concrete implementation using all new patterns
type JobletImplementation struct {
	uploadManager domain.UploadManager
	platform      platform.Platform
	logger        *logger.Logger
	config        *config.Config
	// resourceBuilder removed - using simple functions now
}

// Ensure JobletImplementation implements the new interface
var _ interfaces.Joblet = (*JobletImplementation)(nil)

// NewJobletImplementation creates a new implementation using the refactored patterns
func NewJobletImplementation(platform platform.Platform, logger *logger.Logger, config *config.Config) *JobletImplementation {
	// Create the upload manager
	uploadManager := upload.NewManager(platform, logger)

	// Resource limits now created inline using simple functions

	return &JobletImplementation{
		uploadManager: uploadManager,
		platform:      platform,
		logger:        logger.WithField("component", "joblet"),
		config:        config,
		// resourceBuilder field removed
	}
}

// StartJob implements the new interface with request objects and value objects
func (j *JobletImplementation) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	j.logger.Info("starting job with new interface",
		"command", req.Command,
		"maxCPU", req.Resources.MaxCPU,
		"maxMemory", req.Resources.MaxMemory)

	// 1. Convert primitive types to value objects for validation and rich behavior
	resourceLimits, err := j.buildResourceLimits(req.Resources)
	if err != nil {
		return nil, fmt.Errorf("invalid resource limits: %w", err)
	}

	// 2. Generate job ID
	jobID := j.generateJobID()

	// 3. Create upload session if files are provided
	var uploadSession *domain.UploadSession
	var transport domain.UploadTransport

	if len(req.Uploads) > 0 {
		// Use the new upload interface with transport abstraction
		session, e := j.uploadManager.PrepareUploadSession(jobID, req.Uploads, req.Resources.MaxMemory)
		if e != nil {
			return nil, fmt.Errorf("failed to prepare upload session: %w", e)
		}
		uploadSession = session

		// Create transport for the upload
		transport, e = j.uploadManager.CreateTransport(jobID)
		if e != nil {
			return nil, fmt.Errorf("failed to create upload transport: %w", e)
		}

		// Clean up transport on error
		defer func() {
			if e != nil && transport != nil {
				_ = j.uploadManager.CleanupTransport(transport)
			}
		}()
	}

	// 4. Create the job using value objects
	job := &domain.Job{
		Id:      jobID,
		Command: req.Command,
		Args:    req.Args,
		Limits:  *resourceLimits, // Use the built ResourceLimits directly
		Status:  domain.StatusInitializing,
		Network: req.Network,
		Volumes: req.Volumes,
		// Note: UploadSession field doesn't exist in current Job struct
	}

	// 5. Validate the complete job configuration
	if e := j.validateJobConfiguration(job, resourceLimits); e != nil {
		return nil, fmt.Errorf("job configuration validation failed: %w", e)
	}

	// 6. Start the upload process if needed
	if transport != nil {
		streamer := upload.NewStreamer(jobID, uploadSession, transport, j.platform, j.logger)

		// Start streaming in a separate goroutine
		go func() {
			if e := streamer.Start(); e != nil {
				j.logger.Error("upload streaming failed", "jobID", jobID, "error", e)
			}
		}()
	}

	// 7. Schedule or start the job execution
	if req.Schedule != "" {
		job.Status = domain.StatusScheduled
		j.logger.Info("job scheduled", "jobID", jobID, "schedule", req.Schedule)
	} else {
		job.Status = domain.StatusRunning
		// Here you would start the actual job execution
		j.logger.Info("job started", "jobID", jobID)
	}

	return job, nil
}

// StopJob implements the new interface with enhanced stop request
func (j *JobletImplementation) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	j.logger.Info("stopping job",
		"jobID", req.JobID,
		"force", req.Force,
		"reason", req.Reason)

	// Implementation would stop the actual job
	// This is a placeholder for the actual stopping logic

	if req.Force {
		j.logger.Warn("force stopping job", "jobID", req.JobID, "reason", req.Reason)
		// Force kill the job
	} else {
		j.logger.Info("gracefully stopping job", "jobID", req.JobID, "reason", req.Reason)
		// Graceful shutdown
	}

	return nil
}

// ExecuteScheduledJob implements the new interface for scheduled job execution
func (j *JobletImplementation) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	j.logger.Info("executing scheduled job", "jobID", req.Job.Id)

	// Implementation would execute the scheduled job
	// This is a placeholder for the actual execution logic

	req.Job.Status = domain.StatusRunning
	return nil
}

// buildResourceLimits converts primitive resource limits to value objects
func (j *JobletImplementation) buildResourceLimits(limits interfaces.ResourceLimits) (*domain.ResourceLimits, error) {
	resourceLimits := domain.NewResourceLimitsFromParams(
		limits.MaxCPU,
		limits.CPUCores,
		limits.MaxMemory,
		int64(limits.MaxIOBPS),
	)
	return resourceLimits, nil
}

// validateJobConfiguration performs cross-validation of the complete job configuration
func (j *JobletImplementation) validateJobConfiguration(job *domain.Job, resourceLimits *domain.ResourceLimits) error {
	// Validate CPU percentage doesn't exceed available cores
	if !resourceLimits.CPUCores.IsEmpty() {
		// Simplified validation - would need proper core counting
		j.logger.Debug("CPU cores specified", "cores", resourceLimits.CPUCores.String())
	}

	// Validate memory limits are reasonable
	if !resourceLimits.Memory.IsUnlimited() {
		maxMemoryMB := int32(32768) // 32GB
		if resourceLimits.Memory.Megabytes() > maxMemoryMB {
			return fmt.Errorf("memory limit (%dMB) exceeds maximum allowed (%dMB)",
				resourceLimits.Memory.Megabytes(),
				maxMemoryMB)
		}
	}

	// Validate network configuration
	if job.Network != "" && job.Network != "default" && job.Network != "none" && job.Network != "host" {
		// Custom network validation would go here
		j.logger.Debug("using custom network", "network", job.Network)
	}

	// Validate volume configuration
	for _, volume := range job.Volumes {
		if volume == "" {
			return fmt.Errorf("empty volume name not allowed")
		}
	}

	return nil
}

// generateJobID generates a unique job identifier
func (j *JobletImplementation) generateJobID() string {
	// Implementation would generate a proper unique ID
	// This is simplified for demonstration
	return fmt.Sprintf("job-%d", len("placeholder"))
}

// GetDefaultResourceLimits returns default resource limits
func (j *JobletImplementation) GetDefaultResourceLimits() *domain.ResourceLimits {
	return domain.NewResourceLimits()
}

// ParseResourcesFromStrings Demonstration of using value objects for parsing user input
func (j *JobletImplementation) ParseResourcesFromStrings(cpuStr, memoryStr, bandwidthStr, coresStr string) (*domain.ResourceLimits, error) {
	// Use the mapper's parsing logic
	mapper := mappers.NewJobMapper()
	return mapper.ParseUserInputToValueObjects(cpuStr, memoryStr, bandwidthStr, coresStr)
}
