package server

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "job-worker/api/gen"
	"job-worker/internal/worker/adapters"
	auth2 "job-worker/internal/worker/auth"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/mappers"
	_errors "job-worker/pkg/errors"
	"job-worker/pkg/logger"
	"time"
)

type JobServiceServer struct {
	pb.UnimplementedJobServiceServer
	auth      auth2.GrpcAuthorization
	jobStore  interfaces.Store
	jobWorker interfaces.JobWorker
	logger    *logger.Logger
}

func NewJobServiceServer(auth auth2.GrpcAuthorization, jobStore interfaces.Store, jobWorker interfaces.JobWorker) *JobServiceServer {
	return &JobServiceServer{
		auth:      auth,
		jobStore:  jobStore,
		jobWorker: jobWorker,
		logger:    logger.WithField("component", "grpc-service"),
	}
}

func (s *JobServiceServer) CreateJob(ctx context.Context, createJobReq *pb.CreateJobReq) (*pb.CreateJobRes, error) {
	requestLogger := s.logger.WithFields(
		"operation", "CreateJob",
		"command", createJobReq.Command,
		"args", createJobReq.Args,
		"maxCPU", createJobReq.MaxCPU,
		"maxMemory", createJobReq.MaxMemory,
		"maxIOBPS", createJobReq.MaxIOBPS,
	)

	requestLogger.Info("create job request received")

	if err := s.auth.Authorized(ctx, auth2.CreateJobOp); err != nil {
		requestLogger.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	newJob, err := s.jobWorker.StartJob(ctx, createJobReq.Command, createJobReq.Args, createJobReq.MaxCPU, createJobReq.MaxMemory, createJobReq.MaxIOBPS)

	if err != nil {
		duration := time.Since(startTime)
		requestLogger.Error("job creation failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "job create failed: %v", err)
	}

	duration := time.Since(startTime)
	requestLogger.Info("job created successfully", "jobId", newJob.Id, "duration", duration)

	return mappers.DomainToCreateJobResponse(newJob), nil
}

func (s *JobServiceServer) GetJob(ctx context.Context, req *pb.GetJobReq) (*pb.GetJobRes, error) {
	requestLogger := s.logger.WithFields("operation", "GetJob", "jobId", req.GetId())

	requestLogger.Debug("get job request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		requestLogger.Warn("authorization failed", "error", err)
		return nil, err
	}

	job, exists := s.jobStore.GetJob(req.GetId())
	if !exists {
		requestLogger.Warn("job not found")
		return nil, status.Errorf(codes.NotFound, "job not found %v", req.GetId())
	}

	requestLogger.Info("job retrieved successfully", "status", string(job.Status), "duration", job.Duration())

	return mappers.DomainToGetJobResponse(job), nil
}

func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	requestLogger := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())

	requestLogger.Info("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		requestLogger.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	if err := s.jobWorker.StopJob(ctx, req.GetId()); err != nil {
		duration := time.Since(startTime)
		requestLogger.Error("job stop failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "StopJob error %v", err)
	}

	job, exists := s.jobStore.GetJob(req.GetId())
	if !exists {
		requestLogger.Warn("job not found after stop operation")
		return nil, status.Errorf(codes.NotFound, "job not found %v", req.GetId())
	}

	duration := time.Since(startTime)
	requestLogger.Info("job stopped successfully", "finalStatus", string(job.Status), "duration", duration)

	return mappers.DomainToStopJobResponse(job), nil
}

func (s *JobServiceServer) GetJobs(ctx context.Context, _ *pb.EmptyRequest) (*pb.Jobs, error) {
	requestLogger := s.logger.WithField("operation", "GetJobs")

	requestLogger.Debug("get jobs request received")

	if err := s.auth.Authorized(ctx, auth2.ListJobsOp); err != nil {
		requestLogger.Warn("authorization failed", "error", err)
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
	requestLogger.Info("jobs listed successfully",
		"totalJobs", len(jobs),
		"statusBreakdown", statusCounts,
		"duration", duration)

	return rawJobs, nil
}

func (s *JobServiceServer) GetJobsStream(req *pb.GetJobsStreamReq, stream pb.JobService_GetJobsStreamServer) error {
	requestLogger := s.logger.WithFields("operation", "GetJobsStream", "jobId", req.GetId())

	requestLogger.Info("job stream request received")

	if err := s.auth.Authorized(stream.Context(), auth2.StreamJobsOp); err != nil {
		requestLogger.Warn("authorization failed", "error", err)
		return err
	}

	existingLogs, isRunning, err := s.jobStore.GetOutput(req.GetId())
	if err != nil {
		requestLogger.Warn("job not found for streaming")
		return status.Errorf(codes.NotFound, "job not found")
	}

	requestLogger.Info("streaming job output", "jobId", req.GetId(), "existingLogSize", len(existingLogs), "isRunning", isRunning)

	// streaming the existing logs from the existingLogs
	if e := stream.Send(&pb.DataChunk{Payload: existingLogs}); e != nil {
		requestLogger.Error("failed to send existing logs", "error", e, "logSize", len(existingLogs))
		return e
	}

	requestLogger.Debug("existing logs sent", "logSize", len(existingLogs))

	// already completed, we're done
	if !isRunning {
		requestLogger.Info("job already completed, stream ended", "jobId", req.GetId())
		return nil
	}

	// subscribe to new updates not the existing ones
	domainStream := adapters.NewGrpcStreamAdapter(stream)
	streamStartTime := time.Now()

	e := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), domainStream)

	streamDuration := time.Since(streamStartTime)

	if e != nil && errors.Is(e, _errors.ErrStreamCancelled) {
		requestLogger.Info("stream cancelled by client", "duration", streamDuration)
		return status.Error(codes.Canceled, "stream cancelled by client")
	}

	if e != nil {
		requestLogger.Error("stream ended with error", "error", e, "duration", streamDuration)
	} else {
		requestLogger.Info("stream completed successfully", "duration", streamDuration)
	}

	return e
}
