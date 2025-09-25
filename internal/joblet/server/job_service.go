package server

import (
	"context"
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/mappers"
	"joblet/pkg/errors"
	"joblet/pkg/logger"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JobServiceServer is LEGACY - kept only for test compatibility
// Production uses WorkflowServiceServer which implements the unified architecture
// where all jobs (individual and workflow) are handled through a single service
// DO NOT USE FOR NEW CODE - will be removed in future versions
//
// Deprecated: Use WorkflowServiceServer for all job operations
type JobServiceServer struct {
	pb.UnimplementedJobServiceServer
	auth     auth2.GRPCAuthorization
	jobStore adapters.JobStorer // Uses the new adapter interface
	joblet   interfaces.Joblet  // Uses the new interface
	logger   *logger.Logger
}

// NewJobServiceServer creates a new job service that uses request objects
func NewJobServiceServer(auth auth2.GRPCAuthorization, jobStore adapters.JobStorer, joblet interfaces.Joblet) *JobServiceServer {
	return &JobServiceServer{
		auth:     auth,
		jobStore: jobStore,
		joblet:   joblet,
		logger:   logger.WithField("component", "job-grpc"),
	}
}

// convertErrorToGRPCStatus converts structured errors to appropriate gRPC status codes
func (s *JobServiceServer) convertErrorToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	// Handle context errors
	if errors.IsContextError(err) {
		return status.Errorf(codes.DeadlineExceeded, "operation timeout: %v", err)
	}

	// Handle specific error types
	switch {
	case errors.IsNotFoundError(err):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.IsPermissionError(err):
		return status.Errorf(codes.PermissionDenied, "%v", err)
	case errors.IsResourceError(err):
		return status.Errorf(codes.ResourceExhausted, "%v", err)
	case errors.IsConfigError(err):
		return status.Errorf(codes.InvalidArgument, "configuration error: %v", err)
	case errors.IsTimeoutError(err):
		return status.Errorf(codes.DeadlineExceeded, "%v", err)
	default:
		// Get severity to determine appropriate code
		severity := errors.GetSeverity(err)
		switch severity {
		case errors.SeverityCritical:
			return status.Errorf(codes.Internal, "critical system error: %v", err)
		case errors.SeverityHigh:
			return status.Errorf(codes.Internal, "internal error: %v", err)
		default:
			return status.Errorf(codes.Internal, "%v", err)
		}
	}
}

// RunJob implements the gRPC service using the new request object pattern
func (s *JobServiceServer) RunJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunJob",
		"command", req.Command,
		"args", req.Args,
		"uploadCount", len(req.Uploads),
		"schedule", req.Schedule,
	)

	log.Debug("run job request received (using new interface)")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Convert protobuf request to domain request object
	jobRequest, err := s.convertToJobRequest(req)
	if err != nil {
		// Wrap as config error since it's a request validation issue
		wrappedErr := errors.WrapConfigError("job-request", "conversion", err)
		log.Error("failed to convert request", "error", wrappedErr)
		return nil, s.convertErrorToGRPCStatus(wrappedErr)
	}

	// Log the cleaned request structure (excluding sensitive environment variables)
	envCount := 0
	if jobRequest.Environment != nil {
		envCount = len(jobRequest.Environment)
	}
	log.Info("starting job with request object",
		"command", jobRequest.Command,
		"resourceLimits", fmt.Sprintf("CPU=%d%%, Memory=%dMB, IO=%d BPS, Cores=%s",
			jobRequest.Resources.MaxCPU,
			jobRequest.Resources.MaxMemory,
			jobRequest.Resources.MaxIOBPS,
			jobRequest.Resources.CPUCores),
		"network", jobRequest.Network,
		"volumes", jobRequest.Volumes,
		"runtime", jobRequest.Runtime,
		"uploadCount", len(jobRequest.Uploads),
		"envVarsCount", envCount,
		"secretEnvVarsCount", len(jobRequest.SecretEnvironment)) // Only log counts, not actual values

	// Use the new interface with request object
	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, s.convertErrorToGRPCStatus(err)
	}

	// Log success
	if req.Schedule != "" {
		log.Info("job scheduled successfully",
			"jobUuid", newJob.Uuid,
			"scheduledTime", req.Schedule)
	} else {
		log.Info("job started successfully",
			"jobUuid", newJob.Uuid,
			"status", newJob.Status)
	}

	// Create mapper and convert
	mapper := mappers.NewJobMapper()
	return mapper.DomainToRunJobResponse(newJob), nil
}

// StopJob implements the gRPC service using the new request object pattern
func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobUuid", req.GetUuid())
	log.Debug("stop job request received (using new interface)")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create stop request object
	stopRequest := interfaces.StopJobRequest{
		JobID: req.GetUuid(),
		// Force and Reason fields would need to be added to protobuf if needed
		// Force:  false,
		// Reason: "",
	}

	log.Info("stopping job", "jobUuid", stopRequest.JobID)

	// Use the new interface with request object
	err := s.joblet.StopJob(ctx, stopRequest)
	if err != nil {
		log.Error("job stop failed", "error", err)
		return nil, s.convertErrorToGRPCStatus(err)
	}

	log.Info("job stopped successfully", "jobUuid", stopRequest.JobID)

	return &pb.StopJobRes{
		// Success and Message fields would need to be added to protobuf if needed
		Uuid: stopRequest.JobID,
	}, nil
}

// DeleteJob implements the gRPC service for complete job deletion
func (s *JobServiceServer) DeleteJob(ctx context.Context, req *pb.DeleteJobReq) (*pb.DeleteJobRes, error) {
	log := s.logger.WithFields("operation", "DeleteJob", "jobUuid", req.GetUuid())
	log.Debug("delete job request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	deleteRequest := interfaces.DeleteJobRequest{
		JobID:  req.GetUuid(),
		Reason: "user_requested",
	}

	log.Debug("processing job deletion", "jobId", deleteRequest.JobID)

	// Call core joblet to delete the job
	err := s.joblet.DeleteJob(ctx, deleteRequest)
	if err != nil {
		log.Error("job deletion failed", "error", err)
		return &pb.DeleteJobRes{
			Uuid:    deleteRequest.JobID,
			Success: false,
			Message: err.Error(),
		}, status.Errorf(codes.Internal, "job deletion failed: %v", err)
	}

	log.Info("job deletion completed successfully", "jobId", deleteRequest.JobID)
	return &pb.DeleteJobRes{
		Uuid:    deleteRequest.JobID,
		Success: true,
		Message: "Job deleted successfully",
	}, nil
}

// DeleteAllJobs implements the gRPC service for deleting all non-running jobs
func (s *JobServiceServer) DeleteAllJobs(ctx context.Context, req *pb.DeleteAllJobsReq) (*pb.DeleteAllJobsRes, error) {
	log := s.logger.WithField("operation", "DeleteAllJobs")
	log.Debug("delete all jobs request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	deleteRequest := interfaces.DeleteAllJobsRequest{
		Reason: "user_requested",
	}

	log.Info("processing bulk job deletion")

	// Call core joblet to delete all non-running jobs
	result, err := s.joblet.DeleteAllJobs(ctx, deleteRequest)
	if err != nil {
		log.Error("bulk job deletion failed", "error", err)
		return &pb.DeleteAllJobsRes{
			Success:      false,
			Message:      err.Error(),
			DeletedCount: 0,
			SkippedCount: 0,
		}, status.Errorf(codes.Internal, "bulk job deletion failed: %v", err)
	}

	log.Info("bulk job deletion completed successfully",
		"deletedCount", result.DeletedCount,
		"skippedCount", result.SkippedCount)

	return &pb.DeleteAllJobsRes{
		Success:      true,
		Message:      fmt.Sprintf("Successfully deleted %d jobs, skipped %d running/scheduled jobs", result.DeletedCount, result.SkippedCount),
		DeletedCount: int32(result.DeletedCount),
		SkippedCount: int32(result.SkippedCount),
	}, nil
}

// GetJobStatus remains the same as it doesn't need request objects
func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusReq) (*pb.GetJobStatusRes, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "jobUuid", req.GetUuid())
	log.Debug("get job status request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Retrieve job from store (supports both full UUID and prefix)
	job, exists := s.jobStore.JobByPrefix(req.GetUuid())
	if !exists {
		log.Error("job not found", "jobUuid", req.GetUuid())
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.GetUuid())
	}

	log.Debug("job status retrieved", "jobUuid", job.Uuid, "status", job.Status)

	// Mask secret environment variables
	maskedSecretEnv := make(map[string]string)
	for key := range job.SecretEnvironment {
		maskedSecretEnv[key] = "***"
	}

	// Convert time fields to strings
	startTimeStr := job.StartTime.Format("2006-01-02T15:04:05Z07:00")
	var endTimeStr string
	if job.EndTime != nil {
		endTimeStr = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}
	var scheduledTimeStr string
	if job.ScheduledTime != nil {
		scheduledTimeStr = job.ScheduledTime.Format("2006-01-02T15:04:05Z07:00")
	}

	var uploadStrings []string
	var dependencies []string
	workDir := ""
	workflowUuid := ""

	// Debug logging to identify empty fields
	s.logger.Debug("Job retrieved from store",
		"uuid", job.Uuid,
		"network", job.Network,
		"volumes", job.Volumes,
		"runtime", job.Runtime,
		"hasNetwork", job.Network != "",
		"volumeCount", len(job.Volumes),
		"hasRuntime", job.Runtime != "",
		"environment", len(job.Environment),
		"secretEnv", len(job.SecretEnvironment))

	return &pb.GetJobStatusRes{
		Uuid:              job.Uuid,
		Name:              job.Name,
		Status:            string(job.Status),
		Command:           job.Command,
		Args:              job.Args,
		MaxCPU:            job.Limits.CPU.Value(),
		CpuCores:          job.Limits.CPUCores.String(),
		MaxMemory:         job.Limits.Memory.Megabytes(),
		MaxIOBPS:          int32(job.Limits.IOBandwidth.BytesPerSecond()),
		StartTime:         startTimeStr,
		EndTime:           endTimeStr,
		ExitCode:          job.ExitCode,
		ScheduledTime:     scheduledTimeStr,
		Environment:       job.Environment,
		SecretEnvironment: maskedSecretEnv,
		Network:           job.Network,
		Volumes:           job.Volumes,
		Runtime:           job.Runtime,
		WorkDir:           workDir,
		Uploads:           uploadStrings,
		Dependencies:      dependencies,
		WorkflowUuid:      workflowUuid,
	}, nil
}

// convertToJobRequest converts protobuf request to domain request object
func (s *JobServiceServer) convertToJobRequest(req *pb.RunJobRequest) (*interfaces.StartJobRequest, error) {
	// Validate required fields
	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set default network if not specified
	network := req.Network
	if network == "" {
		network = "bridge"
	}

	// Convert file uploads - simplified conversion
	var domainUploads []domain.FileUpload
	for _, upload := range req.Uploads {
		domainUploads = append(domainUploads, domain.FileUpload{
			Path:    upload.Path,
			Content: upload.Content,
			Size:    int64(len(upload.Content)),
		})
	}

	// Log upload processing (no size limits)
	if len(domainUploads) > 0 {
		totalSize := int64(0)
		for _, upload := range domainUploads {
			totalSize += int64(len(upload.Content))
		}
		s.logger.Info("processing file uploads",
			"fileCount", len(domainUploads),
			"totalSize", totalSize)
	}

	// Determine job type from environment variables
	jobType := domain.JobTypeStandard // Default to standard production jobs
	s.logger.Info("CHECKING ENVIRONMENT FOR JOB TYPE", "environment", req.Environment)
	if req.Environment != nil {
		if envJobType, exists := req.Environment["JOB_TYPE"]; exists && envJobType == "runtime-build" {
			jobType = domain.JobTypeRuntimeBuild
			s.logger.Info("DETECTED RUNTIME BUILD JOB FROM ENVIRONMENT", "envJobType", envJobType)
		} else {
			s.logger.Info("NO RUNTIME BUILD DETECTED, USING STANDARD", "JOB_TYPE_exists", req.Environment["JOB_TYPE"])
		}
	} else {
		s.logger.Info("NO ENVIRONMENT PROVIDED, USING STANDARD JOB TYPE")
	}

	// Create the request object with validation
	jobRequest := &interfaces.StartJobRequest{
		Command: req.Command,
		Args:    req.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    req.MaxCpu,
			MaxMemory: req.MaxMemory,
			MaxIOBPS:  req.MaxIobps,
			CPUCores:  req.CpuCores,
		},
		Uploads:           domainUploads,
		Schedule:          req.Schedule,
		Network:           network,
		Volumes:           req.Volumes,
		Runtime:           req.Runtime,
		Environment:       req.Environment,        // Regular environment variables (logged)
		SecretEnvironment: req.SecretEnvironment,  // Secret environment variables (not logged)
		JobType:           jobType,                // Set job type for isolation configuration
		GPUCount:          req.GpuCount,           // Number of GPUs requested
		GPUMemoryMB:       int64(req.GpuMemoryMb), // GPU memory requirement in MB
	}

	// Validate the request
	if err := s.validateJobRequest(jobRequest); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return jobRequest, nil
}

// validateJobRequest performs validation on the job request object
func (s *JobServiceServer) validateJobRequest(req *interfaces.StartJobRequest) error {
	// Validate resource limits
	if req.Resources.MaxCPU < 0 {
		return fmt.Errorf("maxCPU cannot be negative")
	}
	if req.Resources.MaxMemory < 0 {
		return fmt.Errorf("maxMemory cannot be negative")
	}
	if req.Resources.MaxIOBPS < 0 {
		return fmt.Errorf("maxIOBPS cannot be negative")
	}

	// Validate GPU requirements
	if req.GPUCount < 0 {
		return fmt.Errorf("GPUCount cannot be negative")
	}
	if req.GPUMemoryMB < 0 {
		return fmt.Errorf("GPUMemoryMB cannot be negative")
	}
	// Note: GPUCount = 0 is valid (means no GPU required)

	// Validate network
	validNetworks := map[string]bool{
		"bridge": true,
		"host":   true,
		"none":   true,
	}
	if req.Network != "" && !validNetworks[req.Network] {
		// Custom network - would need additional validation
		s.logger.Debug("using custom network", "network", req.Network)
	}

	// Validate volumes
	for _, volume := range req.Volumes {
		if volume == "" {
			return fmt.Errorf("empty volume name not allowed")
		}
	}

	// Validate runtime specification if provided
	if req.Runtime != "" {
		if err := s.validateRuntime(req.Runtime); err != nil {
			return fmt.Errorf("invalid runtime: %w", err)
		}
	}

	return nil
}

// validateRuntime validates the runtime specification
func (s *JobServiceServer) validateRuntime(runtimeSpec string) error {
	// Basic format validation
	if runtimeSpec == "" {
		return fmt.Errorf("runtime specification cannot be empty")
	}

	// Support both formats:
	// 1. Traditional format: language:version or language:version+tags (e.g., "python-3.11-ml")
	// 2. Runtime name format: language-version-tags (e.g., "python-3.11-ml")

	if strings.Contains(runtimeSpec, ":") {
		// Traditional format: language:version+tags
		return s.validateTraditionalRuntimeFormat(runtimeSpec)
	} else {
		// Runtime name format: language-version-tags
		return s.validateRuntimeNameFormat(runtimeSpec)
	}
}

// validateTraditionalRuntimeFormat validates language:version+tags format
func (s *JobServiceServer) validateTraditionalRuntimeFormat(runtimeSpec string) error {
	parts := strings.Split(runtimeSpec, ":")
	if len(parts) != 2 {
		return fmt.Errorf("runtime specification must be in format 'language:version' or 'language:version+tag'")
	}

	language := parts[0]
	versionPart := parts[1]

	// Validate language
	validLanguages := []string{"python", "java", "node", "go"}
	validLanguage := false
	for _, validLang := range validLanguages {
		if language == validLang {
			validLanguage = true
			break
		}
	}
	if !validLanguage {
		return fmt.Errorf("unsupported runtime language '%s', supported: %s", language, strings.Join(validLanguages, ", "))
	}

	// Validate version format (simple validation)
	versionAndTags := strings.Split(versionPart, "+")
	version := versionAndTags[0]
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Basic version format validation
	if len(version) > 20 {
		return fmt.Errorf("version too long")
	}

	s.logger.Debug("runtime validation passed (traditional format)", "runtime", runtimeSpec)
	return nil
}

// validateRuntimeNameFormat validates language-version-tags format
func (s *JobServiceServer) validateRuntimeNameFormat(runtimeSpec string) error {
	// Basic sanity checks for runtime name format
	if len(runtimeSpec) == 0 {
		return fmt.Errorf("runtime name cannot be empty")
	}

	if len(runtimeSpec) > 50 {
		return fmt.Errorf("runtime name too long (max 50 characters)")
	}

	// Check for valid characters (letters, numbers, hyphens, dots)
	for _, char := range runtimeSpec {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '.') {
			return fmt.Errorf("runtime name contains invalid character: '%c'", char)
		}
	}

	// Must start with a letter
	if len(runtimeSpec) > 0 {
		first := runtimeSpec[0]
		if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
			return fmt.Errorf("runtime name must start with a letter")
		}
	}

	s.logger.Debug("runtime validation passed (name format)", "runtime", runtimeSpec)
	return nil
}

// ListJobs returns all jobs
func (s *JobServiceServer) ListJobs(ctx context.Context, req *pb.EmptyRequest) (*pb.Jobs, error) {
	log := s.logger.WithField("operation", "ListJobs")
	log.Debug("list jobs request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Get all jobs from store
	jobs := s.jobStore.ListJobs()

	// Convert to protobuf
	mapper := mappers.NewJobMapper()
	pbJobs := make([]*pb.Job, 0, len(jobs))
	for _, job := range jobs {
		pbJobs = append(pbJobs, mapper.DomainToProtobuf(job))
	}

	log.Debug("jobs listed", "count", len(pbJobs))

	return &pb.Jobs{
		Jobs: pbJobs,
	}, nil
}

// GetJobLogs streams job logs to the client
func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobUuid", req.GetUuid())
	log.Debug("get job logs request received")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Create a domain streamer adapter
	domainStream := &grpcToDomainStreamer{stream: stream}

	// Use the store's SendUpdatesToClient method
	if err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetUuid(), domainStream); err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", req.GetUuid())
		}
		return status.Errorf(codes.Internal, "failed to stream logs: %v", err)
	}

	log.Debug("log streaming completed", "jobUuid", req.GetUuid())
	return nil
}

// grpcToDomainStreamer adapts gRPC stream to domain streamer interface
type grpcToDomainStreamer struct {
	stream pb.JobService_GetJobLogsServer
}

func (g *grpcToDomainStreamer) SendData(data []byte) error {
	return g.stream.Send(&pb.DataChunk{
		Payload: data,
	})
}

func (g *grpcToDomainStreamer) SendKeepalive() error {
	// Send empty chunk as keepalive
	return g.stream.Send(&pb.DataChunk{
		Payload: []byte{},
	})
}

func (g *grpcToDomainStreamer) Context() context.Context {
	return g.stream.Context()
}

// ExecuteScheduledJob can be added if needed for scheduled job execution
func (s *JobServiceServer) ExecuteScheduledJob(ctx context.Context, jobID string) error {
	log := s.logger.WithFields("operation", "ExecuteScheduledJob", "jobId", jobID)
	log.Debug("executing scheduled job")

	// Retrieve the job
	job, exists := s.jobStore.Job(jobID)
	if !exists {
		return fmt.Errorf("scheduled job not found: %s", jobID)
	}

	// Create execution request
	execRequest := interfaces.ExecuteScheduledJobRequest{
		Job: job,
	}

	// Use the new interface - ExecuteScheduledJob returns error, not bool
	if err := s.joblet.ExecuteScheduledJob(ctx, execRequest); err != nil {
		log.Error("scheduled job execution failed", "error", err)
		return fmt.Errorf("scheduled job execution failed: %w", err)
	}

	log.Info("scheduled job executed successfully", "jobId", jobID)
	return nil
}
