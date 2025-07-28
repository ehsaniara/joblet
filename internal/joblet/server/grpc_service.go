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
	auth         auth2.GrpcAuthorization
	jobStore     state.Store
	joblet       interfaces.Joblet
	logger       *logger.Logger
	networkStore *state.NetworkStore
}

func NewJobServiceServer(auth auth2.GrpcAuthorization, jobStore state.Store, joblet interfaces.Joblet, networkStore *state.NetworkStore) *JobServiceServer {
	return &JobServiceServer{
		auth:         auth,
		jobStore:     jobStore,
		joblet:       joblet,
		networkStore: networkStore,
		logger:       logger.WithField("component", "grpc-service"),
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
		"schedule", req.Schedule,
	)

	log.Debug("run job request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Set default network if not specified
	if req.Network == "" {
		req.Network = "bridge"
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

	domainUploads := mappers.ProtobufToFileUpload(req.Uploads)

	// StartJob now expects RFC3339 formatted schedule string (already parsed by client)
	newJob, err := s.joblet.StartJob(ctx, req.Command, req.Args, req.MaxCPU, req.MaxMemory, req.MaxIOBPS, req.CpuCores, domainUploads, req.Schedule, req.Network)

	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	// Log differently based on whether job was scheduled or executed immediately
	if req.Schedule != "" {
		log.Info("job scheduled successfully",
			"jobId", newJob.Id,
			"uploadsProcessed", len(domainUploads),
			"scheduledTimeRFC3339", req.Schedule)
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

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Check job status before stopping to provide better logging
	job, exists := s.jobStore.GetJob(req.GetId())
	if exists && job.IsScheduled() {
		log.Info("stopping scheduled job", "scheduledTime", job.ScheduledTime.Format(time.RFC3339))
	}

	if err := s.joblet.StopJob(ctx, req.GetId()); err != nil {
		log.Error("job stop failed", "error", err)
		return nil, status.Errorf(codes.Internal, "StopJob error %v", err)
	}

	job, exists = s.jobStore.GetJob(req.GetId())
	if !exists {
		log.Warn("job not found after stop operation")
		return nil, status.Errorf(codes.NotFound, "job not found %v", req.GetId())
	}

	return mappers.DomainToStopJobResponse(job), nil
}

func (s *JobServiceServer) ListJobs(ctx context.Context, _ *pb.EmptyRequest) (*pb.Jobs, error) {
	log := s.logger.WithField("operation", "ListJobs")

	if err := s.auth.Authorized(ctx, auth2.ListJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	jobs := s.jobStore.ListJobs()

	rawJobs := &pb.Jobs{}
	statusCounts := make(map[string]int)

	for _, job := range jobs {
		rawJobs.Jobs = append(rawJobs.Jobs, mappers.DomainToProtobuf(job))
		statusCounts[string(job.Status)]++
	}

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
	adapter := adapters.NewGrpcStreamAdapter(stream)

	if err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), adapter); err != nil {

		if errors.Is(err, context.Canceled) {
			log.Debug("log stream cancelled by client")
			return nil
		}

		log.Error("log stream failed", "error", err)
		return status.Errorf(codes.Internal, "GetJobLogs error: %v", err)
	}

	log.Debug("log stream completed successfully")
	return nil
}

func (s *JobServiceServer) CreateNetwork(ctx context.Context, req *pb.CreateNetworkReq) (*pb.CreateNetworkRes, error) {
	log := s.logger.WithFields(
		"operation", "CreateNetwork",
		"name", req.Name,
		"cidr", req.Cidr)

	log.Debug("create network request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create the network
	if err := s.networkStore.CreateNetwork(req.Name, req.Cidr); err != nil {
		log.Error("failed to create network", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to create network: %v", err)
	}

	// Get network info for response
	networks := s.networkStore.ListNetworks()
	netInfo, exists := networks[req.Name]
	if !exists {
		return nil, status.Errorf(codes.Internal, "network created but not found")
	}

	log.Info("network created successfully")

	return &pb.CreateNetworkRes{
		Name:   netInfo.Name,
		Cidr:   netInfo.CIDR,
		Bridge: netInfo.Bridge,
	}, nil
}

func (s *JobServiceServer) ListNetworks(ctx context.Context, req *pb.EmptyRequest) (*pb.Networks, error) {
	log := s.logger.WithField("operation", "ListNetworks")

	if err := s.auth.Authorized(ctx, auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	networks := s.networkStore.ListNetworks()

	resp := &pb.Networks{
		Networks: make([]*pb.Network, 0, len(networks)),
	}

	for _, net := range networks {
		resp.Networks = append(resp.Networks, &pb.Network{
			Name:     net.Name,
			Cidr:     net.CIDR,
			Bridge:   net.Bridge,
			JobCount: int32(net.JobCount),
		})
	}

	return resp, nil
}

func (s *JobServiceServer) RemoveNetwork(ctx context.Context, req *pb.RemoveNetworkReq) (*pb.RemoveNetworkRes, error) {
	log := s.logger.WithFields(
		"operation", "RemoveNetwork",
		"name", req.Name)

	log.Debug("remove network request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if err := s.networkStore.RemoveNetwork(req.Name); err != nil {
		log.Error("failed to remove network", "error", err)
		return &pb.RemoveNetworkRes{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Info("network removed successfully")

	return &pb.RemoveNetworkRes{
		Success: true,
		Message: "Network removed successfully",
	}, nil
}
