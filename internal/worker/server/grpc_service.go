package server

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	pb "worker/api/gen"
	"worker/internal/worker/adapters"
	auth2 "worker/internal/worker/auth"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/mappers"
	"worker/internal/worker/state"
	_errors "worker/pkg/errors"
	"worker/pkg/logger"
)

type JobServiceServer struct {
	pb.UnimplementedJobServiceServer
	auth      auth2.GrpcAuthorization
	jobStore  state.Store
	jobWorker interfaces.Worker
	logger    *logger.Logger
}

func NewJobServiceServer(auth auth2.GrpcAuthorization, jobStore state.Store, jobWorker interfaces.Worker) *JobServiceServer {
	return &JobServiceServer{
		auth:      auth,
		jobStore:  jobStore,
		jobWorker: jobWorker,
		logger:    logger.WithField("component", "grpc-service"),
	}
}

func (s *JobServiceServer) RunJob(ctx context.Context, runJobReq *pb.RunJobReq) (*pb.RunJobRes, error) {
	log := s.logger.WithFields(
		"operation", "RunJob",
		"command", runJobReq.Command,
		"args", runJobReq.Args,
		"maxCPU", runJobReq.MaxCPU,
		"maxMemory", runJobReq.MaxMemory,
		"maxIOBPS", runJobReq.MaxIOBPS,
	)

	log.Info("run job request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	newJob, err := s.jobWorker.StartJob(ctx, runJobReq.Command, runJobReq.Args, runJobReq.MaxCPU, runJobReq.MaxMemory, runJobReq.MaxIOBPS)

	if err != nil {
		duration := time.Since(startTime)
		log.Error("job creation failed", "error", err, "duration", duration)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	duration := time.Since(startTime)
	log.Info("job created successfully with host networking", "jobId", newJob.Id, "duration", duration)

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

	log.Info("job retrieved successfully", "status", string(job.Status), "duration", job.Duration())

	return mappers.DomainToGetJobStatusResponse(job), nil
}

func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())

	log.Info("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	startTime := time.Now()
	if err := s.jobWorker.StopJob(ctx, req.GetId()); err != nil {
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
	log.Info("job stopped successfully", "finalStatus", string(job.Status), "duration", duration)

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
	log.Info("jobs listed successfully",
		"totalJobs", len(jobs),
		"statusBreakdown", statusCounts,
		"duration", duration)

	return rawJobs, nil
}

func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetId())

	log.Info("job logs stream request received")

	if err := s.auth.Authorized(stream.Context(), auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	existingLogs, isRunning, err := s.jobStore.GetOutput(req.GetId())
	if err != nil {
		log.Warn("job not found for log streaming")
		return status.Errorf(codes.NotFound, "job not found")
	}

	log.Info("streaming job logs", "jobId", req.GetId(), "existingLogSize", len(existingLogs), "isRunning", isRunning)

	// streaming the existing logs from the existingLogs
	if e := stream.Send(&pb.DataChunk{Payload: existingLogs}); e != nil {
		log.Error("failed to send existing logs", "error", e, "logSize", len(existingLogs))
		return e
	}

	log.Debug("existing logs sent", "logSize", len(existingLogs))

	// already completed, we're done
	if !isRunning {
		log.Info("job already completed, log stream ended", "jobId", req.GetId())
		return nil
	}

	// subscribe to new updates not the existing ones
	domainStream := adapters.NewGrpcStreamAdapter(stream)
	streamStartTime := time.Now()

	e := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), domainStream)

	streamDuration := time.Since(streamStartTime)

	if e != nil && errors.Is(e, _errors.ErrStreamCancelled) {
		log.Info("log stream cancelled by client", "duration", streamDuration)
		return status.Error(codes.Canceled, "log stream cancelled by client")
	}

	if e != nil {
		log.Error("log stream ended with error", "error", e, "duration", streamDuration)
	} else {
		log.Info("log stream completed successfully", "duration", streamDuration)
	}

	return e
}
