package server

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/mappers"
	"joblet/internal/joblet/state"
	"joblet/pkg/logger"
	"time"
)

type JobServiceServer struct {
	pb.UnimplementedJobletServiceServer
	auth     auth2.GrpcAuthorization
	jobStore state.Store
	joblet   interfaces.Joblet
	logger   *logger.Logger
}

func NewJobServiceServer(auth auth2.GrpcAuthorization, jobStore state.Store, joblet interfaces.Joblet) *JobServiceServer {
	return &JobServiceServer{
		auth:     auth,
		jobStore: jobStore,
		joblet:   joblet,
		logger:   logger.WithField("component", "grpc-service"),
	}
}

func (s *JobServiceServer) RunJob(ctx context.Context, req *pb.RunJobReq) (*pb.RunJobRes, error) {
	log := s.logger.WithFields(
		"operation", "RunJob",
		"command", req.Command,
		"args", req.Args,
		"maxCPU", req.MaxCPU,
		"maxMemory", req.MaxMemory,
		"maxIOBPS", req.MaxIOBPS,
		"uploadCount", len(req.Uploads),
		"schedule", req.Schedule, // New: log schedule specification
	)

	log.Debug("run job request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate upload size limits
	totalUploadSize := int64(0)
	for _, upload := range req.Uploads {
		totalUploadSize += int64(len(upload.Content))
	}

	// Set reasonable limits
	const maxTotalUploadSize = 100 * 1024 * 1024 // 100MB total
	if totalUploadSize > maxTotalUploadSize {
		log.Error("upload size exceeds limit", "totalSize", totalUploadSize, "maxSize", maxTotalUploadSize)
		return nil, status.Errorf(codes.InvalidArgument, "total upload size %.2f MB exceeds limit of %.2f MB",
			float64(totalUploadSize)/1024/1024, float64(maxTotalUploadSize)/1024/1024)
	}

	startTime := time.Now()

	domainUploads := mappers.ProtobufToFileUpload(req.Uploads)

	// StartJob now expects RFC3339 formatted schedule string (already parsed by client)
	newJob, err := s.joblet.StartJob(ctx, req.Command, req.Args, req.MaxCPU, req.MaxMemory, req.MaxIOBPS, req.CpuCores, domainUploads, req.Schedule)

	if err != nil {
		duration := time.Since(startTime)
		log.Error("job creation failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	duration := time.Since(startTime)

	// Log differently based on whether job was scheduled or executed immediately
	if req.Schedule != "" {
		log.Info("job scheduled successfully",
			"jobId", newJob.Id,
			"duration", duration,
			"uploadsProcessed", len(domainUploads),
			"scheduledTimeRFC3339", req.Schedule)
	} else {
		log.Debug("job created successfully",
			"jobId", newJob.Id,
			"duration", duration,
			"uploadsProcessed", len(domainUploads))
	}

	return mappers.DomainToRunJobResponse(newJob), nil
}

func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusReq) (*pb.GetJobStatusRes, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "jobId", req.GetId())

	log.Debug("get job status request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	job, exists := s.jobStore.GetJob(req.GetId())
	if !exists {
		log.Warn("job not found")
		return nil, status.Errorf(codes.NotFound, "job not found %v", req.GetId())
	}

	// Enhanced logging for scheduled jobs
	if job.IsScheduled() {
		log.Debug("scheduled job retrieved",
			"status", string(job.Status),
			"scheduledTime", job.ScheduledTime.Format(time.RFC3339))
	} else {
		log.Debug("job retrieved successfully", "status", string(job.Status), "duration", job.Duration())
	}

	return mappers.DomainToGetJobStatusResponse(job), nil
}

func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())

	log.Debug("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Check job status before stopping to provide better logging
	job, exists := s.jobStore.GetJob(req.GetId())
	if exists && job.IsScheduled() {
		log.Info("stopping scheduled job", "scheduledTime", job.ScheduledTime.Format(time.RFC3339))
	}

	startTime := time.Now()
	if err := s.joblet.StopJob(ctx, req.GetId()); err != nil {
		duration := time.Since(startTime)
		log.Error("job stop failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "StopJob error %v", err)
	}

	job, exists = s.jobStore.GetJob(req.GetId())
	if !exists {
		log.Warn("job not found after stop operation")
		return nil, status.Errorf(codes.NotFound, "job not found %v", req.GetId())
	}

	duration := time.Since(startTime)
	log.Debug("job stopped successfully", "finalStatus", string(job.Status), "duration", duration)

	return mappers.DomainToStopJobResponse(job), nil
}

func (s *JobServiceServer) ListJobs(ctx context.Context, _ *pb.EmptyRequest) (*pb.Jobs, error) {
	log := s.logger.WithField("operation", "ListJobs")

	log.Debug("list jobs request received")

	if err := s.auth.Authorized(ctx, auth2.ListJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	jobs := s.jobStore.ListJobs()

	rawJobs := &pb.Jobs{}
	statusCounts := make(map[string]int)

	for _, job := range jobs {
		rawJobs.Jobs = append(rawJobs.Jobs, mappers.DomainToProtobuf(job))
		statusCounts[string(job.Status)]++
	}

	duration := time.Since(startTime)
	log.Debug("jobs listed successfully",
		"totalJobs", len(jobs),
		"statusBreakdown", statusCounts,
		"duration", duration)

	return rawJobs, nil
}

func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobletService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetId())

	if err := s.auth.Authorized(stream.Context(), auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	job, exists := s.jobStore.GetJob(req.GetId())
	if !exists {
		log.Warn("logs requested for non-existent job")
		return status.Errorf(codes.NotFound, "job not found: %v", req.GetId())
	}

	if job.IsScheduled() {
		scheduledMsg := fmt.Sprintf("Job is scheduled for: %s\n", job.ScheduledTime.Format("2006-01-02 15:04:05 MST"))

		if err := stream.Send(&pb.DataChunk{Payload: []byte(scheduledMsg)}); err != nil {
			log.Error("failed to send scheduled job message", "error", err)
			return err
		}
	}

	// start streaming (handles ALL job states: SCHEDULED, RUNNING, COMPLETED)
	startTime := time.Now()
	adapter := adapters.NewGrpcStreamAdapter(stream)

	if err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), adapter); err != nil {
		duration := time.Since(startTime)

		if errors.Is(err, context.Canceled) {
			log.Debug("log stream cancelled by client", "duration", duration)
			return nil
		}

		log.Error("log stream failed", "error", err, "duration", duration)
		return status.Errorf(codes.Internal, "GetJobLogs error: %v", err)
	}

	duration := time.Since(startTime)
	log.Debug("log stream completed successfully", "duration", duration)
	return nil
}
