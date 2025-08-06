package server

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/mappers"
	"joblet/pkg/logger"
)

// JobServiceServer uses the new request object pattern and improved interfaces
type JobServiceServer struct {
	pb.UnimplementedJobletServiceServer
	auth     auth2.GrpcAuthorization
	jobStore adapters.JobStoreAdapter // Uses the new adapter interface
	joblet   interfaces.Joblet        // Uses the new interface
	logger   *logger.Logger
}

// NewJobServiceServer creates a new job service that uses request objects
func NewJobServiceServer(auth auth2.GrpcAuthorization, jobStore adapters.JobStoreAdapter, joblet interfaces.Joblet) *JobServiceServer {
	return &JobServiceServer{
		auth:     auth,
		jobStore: jobStore,
		joblet:   joblet,
		logger:   logger.WithField("component", "job-grpc"),
	}
}

// RunJob implements the gRPC service using the new request object pattern
func (s *JobServiceServer) RunJob(ctx context.Context, req *pb.RunJobReq) (*pb.RunJobRes, error) {
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
		log.Error("failed to convert request", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Log the cleaned request structure
	log.Info("starting job with request object",
		"command", jobRequest.Command,
		"resourceLimits", fmt.Sprintf("CPU=%d%%, Memory=%dMB, IO=%d BPS, Cores=%s",
			jobRequest.Resources.MaxCPU,
			jobRequest.Resources.MaxMemory,
			jobRequest.Resources.MaxIOBPS,
			jobRequest.Resources.CPUCores),
		"network", jobRequest.Network,
		"volumes", jobRequest.Volumes,
		"uploadCount", len(jobRequest.Uploads))

	// Use the new interface with request object
	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	// Log success
	if req.Schedule != "" {
		log.Info("job scheduled successfully",
			"jobId", newJob.Id,
			"scheduledTime", req.Schedule)
	} else {
		log.Info("job started successfully",
			"jobId", newJob.Id,
			"status", newJob.Status)
	}

	// Create mapper and convert
	mapper := mappers.NewJobMapper()
	return mapper.DomainToRunJobResponse(newJob), nil
}

// StopJob implements the gRPC service using the new request object pattern
func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())
	log.Debug("stop job request received (using new interface)")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create stop request object
	stopRequest := interfaces.StopJobRequest{
		JobID: req.GetId(),
		// Force and Reason fields would need to be added to protobuf if needed
		// Force:  false,
		// Reason: "",
	}

	log.Info("stopping job", "jobId", stopRequest.JobID)

	// Use the new interface with request object
	err := s.joblet.StopJob(ctx, stopRequest)
	if err != nil {
		log.Error("job stop failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job stop failed: %v", err)
	}

	log.Info("job stopped successfully", "jobId", stopRequest.JobID)

	return &pb.StopJobRes{
		// Success and Message fields would need to be added to protobuf if needed
		Id: stopRequest.JobID,
	}, nil
}

// GetJobStatus remains the same as it doesn't need request objects
func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusReq) (*pb.GetJobStatusRes, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "jobId", req.GetId())
	log.Debug("get job status request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Retrieve job from store
	job, exists := s.jobStore.GetJob(req.GetId())
	if !exists {
		log.Error("job not found", "jobId", req.GetId())
		return nil, status.Errorf(codes.NotFound, "job not found: %s", req.GetId())
	}

	log.Debug("job status retrieved", "jobId", job.Id, "status", job.Status)

	return &pb.GetJobStatusRes{
		Id:     job.Id,
		Status: string(job.Status),
	}, nil
}

// convertToJobRequest converts protobuf request to domain request object
func (s *JobServiceServer) convertToJobRequest(req *pb.RunJobReq) (*interfaces.StartJobRequest, error) {
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

	// Create the request object with validation
	jobRequest := &interfaces.StartJobRequest{
		Command: req.Command,
		Args:    req.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    req.MaxCPU,
			MaxMemory: req.MaxMemory,
			MaxIOBPS:  req.MaxIOBPS,
			CPUCores:  req.CpuCores,
		},
		Uploads:  domainUploads,
		Schedule: req.Schedule,
		Network:  network,
		Volumes:  req.Volumes,
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
func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobletService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetId())
	log.Debug("get job logs request received")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Create a domain streamer adapter
	domainStream := &grpcToDomainStreamer{stream: stream}

	// Use the store's SendUpdatesToClient method
	if err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), domainStream); err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", req.GetId())
		}
		return status.Errorf(codes.Internal, "failed to stream logs: %v", err)
	}

	log.Debug("log streaming completed", "jobId", req.GetId())
	return nil
}

// grpcToDomainStreamer adapts gRPC stream to domain streamer interface
type grpcToDomainStreamer struct {
	stream pb.JobletService_GetJobLogsServer
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
	job, exists := s.jobStore.GetJob(jobID)
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
