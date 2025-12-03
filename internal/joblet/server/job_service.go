package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	auth2 "github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/mappers"
	metricsdomain "github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// defaultNetworkName is the default network for jobs
	defaultNetworkName = "bridge"
)

// JobServiceServer handles job execution operations via gRPC.
// This is a lean job service focused only on individual job execution,
// without workflow orchestration (which is handled by a separate project).
type JobServiceServer struct {
	pb.UnimplementedJobServiceServer
	auth          auth2.GRPCAuthorization
	jobStore      adapters.JobStorer
	metricsStore  *adapters.MetricsStoreAdapter
	joblet        interfaces.Joblet
	persistClient persistpb.PersistServiceClient
	logger        *logger.Logger
}

// NewJobServiceServer creates a new gRPC service server for job operations.
func NewJobServiceServer(auth auth2.GRPCAuthorization, jobStore adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, joblet interfaces.Joblet, persistClient persistpb.PersistServiceClient) *JobServiceServer {
	return &JobServiceServer{
		auth:          auth,
		jobStore:      jobStore,
		metricsStore:  metricsStore,
		joblet:        joblet,
		persistClient: persistClient,
		logger:        logger.WithField("component", "job-grpc"),
	}
}

// RunJob handles gRPC requests to execute individual jobs.
func (s *JobServiceServer) RunJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunJob",
		"command", req.Command,
		"args", req.Args,
		"uploadCount", len(req.Uploads),
		"schedule", req.Schedule,
	)

	log.Debug("run job request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Verify persist is healthy before accepting jobs
	if s.persistClient != nil {
		if err := s.checkPersistHealth(ctx); err != nil {
			log.Error("persist service unavailable, cannot execute job", "error", err)
			return nil, status.Errorf(codes.Unavailable, "persist service unavailable: %v - cannot execute job to prevent data loss", err)
		}
	}

	// Convert protobuf request to domain request object
	jobRequest, err := s.convertToJobRequest(req)
	if err != nil {
		log.Error("failed to convert request", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Log the request (excluding sensitive environment variables)
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
		"secretEnvVarsCount", len(jobRequest.SecretEnvironment))

	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	if req.Schedule != "" {
		log.Info("job scheduled successfully",
			"jobUuid", newJob.Uuid,
			"scheduledTime", req.Schedule)
	} else {
		log.Info("job started successfully",
			"jobUuid", newJob.Uuid,
			"status", newJob.Status)
	}

	return &pb.RunJobResponse{
		JobUuid: newJob.Uuid,
		Status:  string(newJob.Status),
	}, nil
}

// convertToJobRequest converts protobuf request to domain request object
func (s *JobServiceServer) convertToJobRequest(req *pb.RunJobRequest) (*interfaces.StartJobRequest, error) {
	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	network := req.Network
	if network == "" {
		network = defaultNetworkName
	}

	var domainUploads []domain.FileUpload
	for _, upload := range req.Uploads {
		domainUploads = append(domainUploads, domain.FileUpload{
			Path:    upload.Path,
			Content: upload.Content,
			Size:    int64(len(upload.Content)),
		})
	}

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
	jobType := domain.JobTypeStandard
	if req.Environment != nil {
		if envJobType, exists := req.Environment["JOB_TYPE"]; exists && envJobType == "runtime-build" {
			jobType = domain.JobTypeRuntimeBuild
			s.logger.Info("detected runtime build job from environment", "envJobType", envJobType)
		}
	}

	jobRequest := &interfaces.StartJobRequest{
		Name:    req.Name,
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
		Environment:       req.Environment,
		SecretEnvironment: req.SecretEnvironment,
		JobType:           jobType,
	}

	if err := s.validateJobRequest(jobRequest); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return jobRequest, nil
}

// validateJobRequest validates the job request
func (s *JobServiceServer) validateJobRequest(req *interfaces.StartJobRequest) error {
	if req.Resources.MaxCPU < 0 {
		return fmt.Errorf("maxCPU cannot be negative")
	}
	if req.Resources.MaxMemory < 0 {
		return fmt.Errorf("maxMemory cannot be negative")
	}
	if req.Resources.MaxIOBPS < 0 {
		return fmt.Errorf("maxIOBPS cannot be negative")
	}

	validNetworks := map[string]bool{
		"bridge": true,
		"host":   true,
		"none":   true,
	}
	if req.Network != "" && !validNetworks[req.Network] {
		s.logger.Debug("using custom network", "network", req.Network)
	}

	for _, volume := range req.Volumes {
		if volume == "" {
			return fmt.Errorf("empty volume name not allowed")
		}
	}

	if req.Runtime != "" {
		if err := s.validateRuntime(req.Runtime); err != nil {
			return fmt.Errorf("invalid runtime: %w", err)
		}
	}

	return nil
}

// validateRuntime validates the runtime specification
func (s *JobServiceServer) validateRuntime(runtimeSpec string) error {
	if runtimeSpec == "" {
		return fmt.Errorf("runtime specification cannot be empty")
	}

	if strings.Contains(runtimeSpec, ":") {
		parts := strings.Split(runtimeSpec, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid runtime format: expected 'language:version', got '%s'", runtimeSpec)
		}
		if parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("runtime language and version cannot be empty")
		}
	} else {
		parts := strings.Split(runtimeSpec, "-")
		if len(parts) < 2 {
			return fmt.Errorf("invalid runtime format: expected 'language-version[-tags]', got '%s'", runtimeSpec)
		}
		if parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("runtime language and version cannot be empty")
		}
	}

	return nil
}

// ListJobs returns a list of all jobs
func (s *JobServiceServer) ListJobs(ctx context.Context, req *pb.EmptyRequest) (*pb.Jobs, error) {
	log := s.logger.WithField("operation", "ListJobs")
	log.Debug("list jobs request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	jobs := s.jobStore.ListJobs()

	mapper := mappers.NewJobMapper()
	pbJobs := make([]*pb.Job, len(jobs))
	for i, job := range jobs {
		pbJobs[i] = mapper.DomainToProtobuf(job)
	}

	return &pb.Jobs{Jobs: pbJobs}, nil
}

// GetJobStatus returns the status of a specific job
func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusReq) (*pb.GetJobStatusRes, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "jobId", req.GetUuid())
	log.Debug("get job status request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	job, exists := s.jobStore.JobByPrefix(req.GetUuid())
	if !exists {
		log.Error("job not found", "jobId", req.GetUuid())
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetUuid())
	}

	mapper := mappers.NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

	log.Debug("job status retrieved successfully", "status", job.Status)

	maskedSecretEnv := make(map[string]string)
	for key := range pbJob.SecretEnvironment {
		maskedSecretEnv[key] = "***"
	}

	return &pb.GetJobStatusRes{
		Uuid:              pbJob.Uuid,
		Name:              pbJob.Name,
		Command:           pbJob.Command,
		Args:              pbJob.Args,
		MaxCPU:            pbJob.MaxCPU,
		CpuCores:          pbJob.CpuCores,
		MaxMemory:         pbJob.MaxMemory,
		MaxIOBPS:          pbJob.MaxIOBPS,
		Status:            pbJob.Status,
		StartTime:         pbJob.StartTime,
		EndTime:           pbJob.EndTime,
		ExitCode:          pbJob.ExitCode,
		ScheduledTime:     pbJob.ScheduledTime,
		Environment:       pbJob.Environment,
		SecretEnvironment: maskedSecretEnv,
		Network:           job.Network,
		Volumes:           job.Volumes,
		Runtime:           job.Runtime,
		WorkDir:           job.WorkingDirectory,
		Uploads:           s.convertUploadsToStringArray(job.Uploads),
		NodeId:            job.NodeId,
	}, nil
}

// StopJob stops a running job
func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetUuid())
	log.Debug("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	stopRequest := interfaces.StopJobRequest{
		JobID: req.GetUuid(),
	}

	log.Info("stopping job", "jobId", stopRequest.JobID)

	err := s.joblet.StopJob(ctx, stopRequest)
	if err != nil {
		log.Error("job stop failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job stop failed: %v", err)
	}

	log.Info("job stopped successfully", "jobId", stopRequest.JobID)

	return &pb.StopJobRes{
		Uuid: stopRequest.JobID,
	}, nil
}

// DeleteJob deletes a job
func (s *JobServiceServer) DeleteJob(ctx context.Context, req *pb.DeleteJobReq) (*pb.DeleteJobRes, error) {
	log := s.logger.WithFields("operation", "DeleteJob", "jobId", req.GetUuid())
	log.Debug("delete job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	deleteRequest := interfaces.DeleteJobRequest{
		JobID:  req.GetUuid(),
		Reason: "user_requested",
	}

	log.Debug("processing job deletion", "jobId", deleteRequest.JobID)

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

// DeleteAllJobs deletes all non-running jobs
func (s *JobServiceServer) DeleteAllJobs(ctx context.Context, req *pb.DeleteAllJobsReq) (*pb.DeleteAllJobsRes, error) {
	log := s.logger.WithField("operation", "DeleteAllJobs")
	log.Debug("delete all jobs request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	deleteRequest := interfaces.DeleteAllJobsRequest{
		Reason: "user_requested",
	}

	log.Info("processing bulk job deletion")

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

// GetJobLogs streams job logs to the client
func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetUuid())
	log.Debug("get job logs request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Fetch historical logs from persist if available
	historicalCount := 0
	if s.persistClient != nil {
		log.Debug("fetching historical logs from persist")

		persistReq := &persistpb.QueryLogsRequest{
			JobId:  req.GetUuid(),
			Stream: persistpb.StreamType_STREAM_TYPE_UNSPECIFIED,
		}

		persistStream, err := s.persistClient.QueryLogs(stream.Context(), persistReq)
		if err != nil {
			log.Warn("failed to query historical logs from persist", "error", err)
		} else {
			for {
				logLine, err := persistStream.Recv()
				if err != nil {
					if err.Error() == "EOF" {
						log.Debug("historical logs streaming completed", "count", historicalCount)
						break
					}
					log.Warn("error reading historical logs", "error", err)
					break
				}

				if err := stream.Send(&pb.DataChunk{Payload: logLine.Content}); err != nil {
					log.Error("failed to send historical log to client", "error", err)
					return status.Errorf(codes.Internal, "failed to send historical log: %v", err)
				}
				historicalCount++
			}
		}
	}

	// For completed jobs with persist data, skip buffer to avoid duplicates
	if historicalCount > 0 {
		job, exists := s.jobStore.Job(req.GetUuid())
		if exists && job.IsCompleted() {
			log.Debug("job completed with persist data, skipping buffer", "historicalCount", historicalCount)
			return nil
		}
	}

	// Stream live logs
	log.Debug("starting live log streaming from buffer")
	streamer := &grpcToDomainStreamer{stream: stream}

	err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetUuid(), streamer)
	if err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", req.GetUuid())
		}
		return status.Errorf(codes.Internal, "failed to stream logs: %v", err)
	}

	log.Debug("log streaming completed successfully", "totalFromPersist", historicalCount)
	return nil
}

// grpcToDomainStreamer adapts gRPC stream to domain streamer interface
type grpcToDomainStreamer struct {
	stream pb.JobService_GetJobLogsServer
}

func (g *grpcToDomainStreamer) SendData(data []byte) error {
	return g.stream.Send(&pb.DataChunk{Payload: data})
}

func (g *grpcToDomainStreamer) SendKeepalive() error {
	return g.stream.Send(&pb.DataChunk{Payload: []byte{}})
}

func (g *grpcToDomainStreamer) Context() context.Context {
	return g.stream.Context()
}

// GetJobMetrics streams job metrics to the client
func (s *JobServiceServer) GetJobMetrics(req *pb.JobMetricsRequest, stream grpc.ServerStreamingServer[pb.JobMetricsSample]) error {
	log := s.logger.WithFields("operation", "GetJobMetrics", "uuid", req.Uuid)
	log.Debug("get job metrics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.Uuid == "" {
		return status.Errorf(codes.InvalidArgument, "uuid is required")
	}

	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.Uuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.Uuid, "error", err)
		resolvedUUID = req.Uuid
	}

	// Fetch historical metrics from persist
	historicalCount := 0
	if s.persistClient != nil {
		log.Debug("fetching historical metrics from persist")

		persistReq := &persistpb.QueryMetricsRequest{
			JobId: resolvedUUID,
		}

		persistStream, err := s.persistClient.QueryMetrics(stream.Context(), persistReq)
		if err != nil {
			log.Warn("failed to query historical metrics from persist", "error", err)
		} else {
			for {
				metric, err := persistStream.Recv()
				if err != nil {
					if err.Error() == "EOF" {
						log.Debug("historical metrics streaming completed", "count", historicalCount)
						break
					}
					log.Warn("error reading historical metrics", "error", err)
					break
				}

				pbSample := convertPersistMetricToProto(metric)
				if err := stream.Send(pbSample); err != nil {
					log.Error("failed to send historical metric to client", "error", err)
					return status.Errorf(codes.Internal, "failed to send historical metric: %v", err)
				}
				historicalCount++
			}
		}
	}

	// For completed jobs with persist data, skip buffer
	if historicalCount > 0 {
		job, exists := s.jobStore.Job(resolvedUUID)
		if exists && job.IsCompleted() {
			log.Debug("job completed with persist data, skipping metrics buffer", "historicalCount", historicalCount)
			return nil
		}
	}

	// Stream live metrics
	log.Debug("starting live metrics streaming from buffer")
	err = s.metricsStore.StreamMetrics(stream.Context(), resolvedUUID, func(sample *metricsdomain.JobMetricsSample) error {
		pbSample := convertMetricsSampleToProto(sample)
		if err := stream.Send(pbSample); err != nil {
			log.Warn("failed to send metrics sample", "error", err)
			return err
		}
		return nil
	})

	if err != nil {
		log.Error("metrics streaming failed", "error", err)
		return status.Errorf(codes.Internal, "failed to stream metrics: %v", err)
	}

	log.Debug("metrics streaming completed")
	return nil
}

// checkPersistHealth verifies that the persist service is healthy
func (s *JobServiceServer) checkPersistHealth(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := s.persistClient.Ping(checkCtx, &persistpb.PingRequest{})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	if !resp.Healthy {
		return fmt.Errorf("persist reported unhealthy status")
	}

	return nil
}

// convertUploadsToStringArray converts FileUpload array to string array of paths
func (s *JobServiceServer) convertUploadsToStringArray(uploads []domain.FileUpload) []string {
	var uploadPaths []string
	for _, upload := range uploads {
		uploadPaths = append(uploadPaths, upload.Path)
	}
	return uploadPaths
}
