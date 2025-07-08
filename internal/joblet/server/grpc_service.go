package server

import (
	"context"
	"errors"
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
	)

	log.Debug("run job request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	newJob, err := s.joblet.StartJob(ctx, req.Command, req.Args, req.MaxCPU, req.MaxMemory, req.MaxIOBPS, req.CpuCores)

	if err != nil {
		duration := time.Since(startTime)
		log.Error("job creation failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	duration := time.Since(startTime)
	log.Debug("job created successfully with host networking", "jobId", newJob.Id, "duration", duration)

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

	log.Debug("job retrieved successfully", "status", string(job.Status), "duration", job.Duration())

	return mappers.DomainToGetJobStatusResponse(job), nil
}

func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())

	log.Debug("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	if err := s.joblet.StopJob(ctx, req.GetId()); err != nil {
		duration := time.Since(startTime)
		log.Error("job stop failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "StopJob error %v", err)
	}

	job, exists := s.jobStore.GetJob(req.GetId())
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

	log.Debug("job logs stream request received")

	if err := s.auth.Authorized(stream.Context(), auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	existingLogs, isRunning, err := s.jobStore.GetOutput(req.GetId())
	if err != nil {
		log.Warn("job not found for log streaming")
		return status.Errorf(codes.NotFound, "job not found")
	}

	log.Debug("streaming job logs", "jobId", req.GetId(), "existingLogSize", len(existingLogs), "isRunning", isRunning)

	// streaming the existing logs from the existingLogs
	if e := stream.Send(&pb.DataChunk{Payload: existingLogs}); e != nil {
		log.Error("failed to send existing logs", "error", e, "logSize", len(existingLogs))
		return e
	}

	log.Debug("existing logs sent", "logSize", len(existingLogs))

	// already completed, we're done
	if !isRunning {
		log.Debug("job already completed, log stream ended", "jobId", req.GetId())
		return nil
	}

	// subscribe to new updates not the existing ones
	domainStream := adapters.NewGrpcStreamAdapter(stream)
	streamStartTime := time.Now()

	e := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), domainStream)

	streamDuration := time.Since(streamStartTime)

	if e != nil && errors.Is(e, errors.New("stream cancelled by client")) {
		log.Debug("log stream cancelled by client", "duration", streamDuration)
		return status.Error(codes.Canceled, "log stream cancelled by client")
	}

	if e != nil {
		log.Error("log stream ended with error", "error", e, "duration", streamDuration)
	} else {
		log.Debug("log stream completed successfully", "duration", streamDuration)
	}

	return e
}
