package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	auth2 "github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/mappers"
	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
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
	auth               auth2.GRPCAuthorization
	jobStore           adapters.JobStorer
	telemetryCollector *telemetry.Collector
	joblet             interfaces.Joblet
	persistClient      persistpb.PersistServiceClient
	logger             *logger.Logger
}

// NewJobServiceServer creates a new gRPC service server for job operations.
func NewJobServiceServer(auth auth2.GRPCAuthorization, jobStore adapters.JobStorer, telemetryCollector *telemetry.Collector, joblet interfaces.Joblet, persistClient persistpb.PersistServiceClient) *JobServiceServer {
	return &JobServiceServer{
		auth:               auth,
		jobStore:           jobStore,
		telemetryCollector: telemetryCollector,
		joblet:             joblet,
		persistClient:      persistClient,
		logger:             logger.WithField("component", "job-grpc"),
	}
}

// getJobNodeId looks up a job by UUID and returns its nodeId.
// Returns empty string if job not found (falls back to local nodeId in persist).
func (s *JobServiceServer) getJobNodeId(jobUUID string) string {
	job, exists := s.jobStore.JobByPrefix(jobUUID)
	if !exists {
		return ""
	}
	return job.NodeId
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
			"job_uuid", newJob.Uuid,
			"scheduledTime", req.Schedule)
	} else {
		log.Info("job started successfully",
			"job_uuid", newJob.Uuid,
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
			Path:        upload.Path,
			Content:     upload.Content,
			Mode:        upload.Mode,
			IsDirectory: upload.IsDirectory,
			Size:        int64(len(upload.Content)),
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
		Command: req.Command,
		Args:    req.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    req.MaxCpu,
			MaxMemory: req.MaxMemory,
			MaxIOBPS:  req.MaxIoBps,
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
func (s *JobServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "job_uuid", req.GetUuid())
	log.Debug("get job status request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	job, exists := s.jobStore.JobByPrefix(req.GetUuid())
	if !exists {
		log.Error("job not found", "job_uuid", req.GetUuid())
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetUuid())
	}

	mapper := mappers.NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

	log.Debug("job status retrieved successfully", "status", job.Status)

	maskedSecretEnv := make(map[string]string)
	for key := range pbJob.SecretEnvironment {
		maskedSecretEnv[key] = "***"
	}

	return &pb.GetJobStatusResponse{
		Uuid:              pbJob.Uuid,
		Command:           pbJob.Command,
		Args:              pbJob.Args,
		MaxCpu:            pbJob.MaxCpu,
		CpuCores:          pbJob.CpuCores,
		MaxMemory:         pbJob.MaxMemory,
		MaxIoBps:          pbJob.MaxIoBps,
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
		GpuIndices:        pbJob.GpuIndices,
		GpuCount:          pbJob.GpuCount,
		GpuMemoryMb:       pbJob.GpuMemoryMb,
		NodeId:            job.NodeId,
	}, nil
}

// StopJob stops a running job
func (s *JobServiceServer) StopJob(ctx context.Context, req *pb.StopJobRequest) (*pb.StopJobResponse, error) {
	log := s.logger.WithFields("operation", "StopJob", "job_uuid", req.GetUuid())
	log.Debug("stop job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	stopRequest := interfaces.StopJobRequest{
		JobUUID: req.GetUuid(),
	}

	log.Info("stopping job", "job_uuid", stopRequest.JobUUID)

	err := s.joblet.StopJob(ctx, stopRequest)
	if err != nil {
		log.Error("job stop failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job stop failed: %v", err)
	}

	log.Info("job stopped successfully", "job_uuid", stopRequest.JobUUID)

	return &pb.StopJobResponse{
		Uuid: stopRequest.JobUUID,
	}, nil
}

// DeleteJob deletes a job
func (s *JobServiceServer) DeleteJob(ctx context.Context, req *pb.DeleteJobRequest) (*pb.DeleteJobResponse, error) {
	log := s.logger.WithFields("operation", "DeleteJob", "job_uuid", req.GetUuid())
	log.Debug("delete job request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	deleteRequest := interfaces.DeleteJobRequest{
		JobUUID: req.GetUuid(),
		Reason:  "user_requested",
	}

	log.Debug("processing job deletion", "job_uuid", deleteRequest.JobUUID)

	err := s.joblet.DeleteJob(ctx, deleteRequest)
	if err != nil {
		log.Error("job deletion failed", "error", err)
		return &pb.DeleteJobResponse{
			Uuid:    deleteRequest.JobUUID,
			Success: false,
			Message: err.Error(),
		}, status.Errorf(codes.Internal, "job deletion failed: %v", err)
	}

	log.Info("job deletion completed successfully", "job_uuid", deleteRequest.JobUUID)
	return &pb.DeleteJobResponse{
		Uuid:    deleteRequest.JobUUID,
		Success: true,
		Message: "Job deleted successfully",
	}, nil
}

// DeleteAllJobs deletes all non-running jobs
func (s *JobServiceServer) DeleteAllJobs(ctx context.Context, req *pb.DeleteAllJobsRequest) (*pb.DeleteAllJobsResponse, error) {
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
		return &pb.DeleteAllJobsResponse{
			Success:      false,
			Message:      err.Error(),
			DeletedCount: 0,
			SkippedCount: 0,
		}, status.Errorf(codes.Internal, "bulk job deletion failed: %v", err)
	}

	log.Info("bulk job deletion completed successfully",
		"deletedCount", result.DeletedCount,
		"skippedCount", result.SkippedCount)

	return &pb.DeleteAllJobsResponse{
		Success:      true,
		Message:      fmt.Sprintf("Successfully deleted %d jobs, skipped %d running/scheduled jobs", result.DeletedCount, result.SkippedCount),
		DeletedCount: int32(result.DeletedCount),
		SkippedCount: int32(result.SkippedCount),
	}, nil
}

// GetJobLogs streams job logs to the client using the unified streaming pattern.
// For running jobs: sends historical logs from buffer/persist, then streams live.
// For completed jobs: sends all historical logs only.
// For jobs not found locally: queries persist for historical logs.
func (s *JobServiceServer) GetJobLogs(req *pb.GetJobLogsRequest, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "job_uuid", req.GetUuid())
	log.Debug("get job logs request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	jobUUID := req.GetUuid()
	if jobUUID == "" {
		return status.Errorf(codes.InvalidArgument, "job_uuid is required")
	}

	// Resolve short UUID to full UUID
	resolvedUUID, err := s.jobStore.ResolveJobUUID(jobUUID)
	if err != nil {
		log.Debug("failed to resolve UUID, using as-is", "input", jobUUID, "error", err)
		resolvedUUID = jobUUID
	}

	log = log.WithFields("resolvedUUID", resolvedUUID)

	// Determine job state
	job, exists := s.jobStore.Job(resolvedUUID)
	isCompleted := exists && job.IsCompleted()
	state := DetermineJobState(exists, isCompleted)

	log.Debug("starting log stream", "state", state)

	// Use unified streaming helper
	cfg := StreamConfig{
		JobUUID: resolvedUUID,
		Logger:  log,
		SendHistorical: func() (int, error) {
			return s.sendHistoricalLogs(stream, resolvedUUID, log)
		},
		QueryPersistOnly: func() (int, error) {
			if s.persistClient == nil {
				return 0, nil
			}
			return s.queryPersistLogs(stream, resolvedUUID, log)
		},
		StreamLive: func() error {
			return s.streamLiveLogs(stream, resolvedUUID, log)
		},
	}

	return StreamWithHistory(stream.Context(), cfg, state)
}

// sendHistoricalLogs sends buffered and persisted historical logs
func (s *JobServiceServer) sendHistoricalLogs(stream pb.JobService_GetJobLogsServer, jobUUID string, log *logger.Logger) (int, error) {
	count := 0

	// First, query persist for historical logs
	if s.persistClient != nil {
		persistReq := &persistpb.QueryLogsRequest{
			JobUuid: jobUUID,
			NodeId:  s.getJobNodeId(jobUUID), // For multi-node CloudWatch queries
			Stream:  persistpb.StreamType_STREAM_TYPE_UNSPECIFIED,
		}

		persistStream, err := s.persistClient.QueryLogs(stream.Context(), persistReq)
		if err != nil {
			log.Warn("failed to query historical logs from persist", "error", err)
		} else {
			for {
				logLine, err := persistStream.Recv()
				if err != nil {
					if err.Error() == "EOF" {
						log.Debug("historical logs from persist completed", "count", count)
						break
					}
					log.Warn("error reading historical logs", "error", err)
					break
				}

				if err := stream.Send(&pb.DataChunk{Payload: logLine.Content}); err != nil {
					log.Error("failed to send historical log to client", "error", err)
					return count, status.Errorf(codes.Internal, "failed to send historical log: %v", err)
				}
				count++
			}
		}
	}

	return count, nil
}

// queryPersistLogs queries persist for logs when job is not found locally
func (s *JobServiceServer) queryPersistLogs(stream pb.JobService_GetJobLogsServer, jobUUID string, log *logger.Logger) (int, error) {
	persistReq := &persistpb.QueryLogsRequest{
		JobUuid: jobUUID,
		NodeId:  s.getJobNodeId(jobUUID), // For multi-node CloudWatch queries
		Stream:  persistpb.StreamType_STREAM_TYPE_UNSPECIFIED,
	}

	persistStream, err := s.persistClient.QueryLogs(stream.Context(), persistReq)
	if err != nil {
		log.Warn("failed to query logs from persist", "error", err)
		return 0, status.Errorf(codes.NotFound, "job not found: %s", jobUUID)
	}

	count := 0
	for {
		logLine, err := persistStream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				log.Debug("persist logs streaming completed", "count", count)
				break
			}
			log.Warn("error reading logs from persist", "error", err)
			break
		}

		if err := stream.Send(&pb.DataChunk{Payload: logLine.Content}); err != nil {
			log.Error("failed to send log to client", "error", err)
			return count, status.Errorf(codes.Internal, "failed to send log: %v", err)
		}
		count++
	}

	return count, nil
}

// streamLiveLogs streams live logs for running jobs
func (s *JobServiceServer) streamLiveLogs(stream pb.JobService_GetJobLogsServer, jobUUID string, log *logger.Logger) error {
	log.Debug("starting live log streaming from buffer")
	streamer := &grpcToDomainStreamer{stream: stream}

	err := s.jobStore.SendUpdatesToClient(stream.Context(), jobUUID, streamer)
	if err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", jobUUID)
		}
		return status.Errorf(codes.Internal, "failed to stream logs: %v", err)
	}

	log.Debug("live log streaming completed")
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

// StreamJobMetrics streams live metrics for a running job.
// Metrics are sampled every ~5 seconds from cgroups and include CPU, memory, disk I/O, and network.
func (s *JobServiceServer) StreamJobMetrics(req *pb.StreamJobMetricsRequest, stream grpc.ServerStreamingServer[pb.JobMetricsEvent]) error {
	log := s.logger.WithFields("operation", "StreamJobMetrics", "uuid", req.JobUuid)
	log.Debug("stream job metrics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.JobUuid == "" {
		return status.Errorf(codes.InvalidArgument, "job_uuid is required")
	}

	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.JobUuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.JobUuid, "error", err)
		resolvedUUID = req.JobUuid
	}

	// Check if telemetry collector is available
	if s.telemetryCollector == nil {
		log.Warn("telemetry collector not configured")
		return status.Errorf(codes.Unavailable, "telemetry collector not configured")
	}

	// Create filter for metrics only
	filter := &telemetry.EventFilter{
		Types: []telemetry.EventType{telemetry.EventTypeMetrics},
	}

	log.Debug("starting metrics stream")

	// Determine job state
	job, exists := s.jobStore.Job(resolvedUUID)
	isCompleted := exists && job.IsCompleted()
	state := DetermineJobState(exists, isCompleted)

	// Use unified streaming helper
	cfg := StreamConfig{
		JobUUID: resolvedUUID,
		Logger:  log,
		SendHistorical: func() (int, error) {
			err := s.sendHistoricalMetrics(stream, resolvedUUID, filter, 0, 0, 0, log)
			return 0, err // sendHistoricalMetrics doesn't return count
		},
		QueryPersistOnly: func() (int, error) {
			if s.persistClient == nil {
				return 0, nil
			}
			return s.queryPersistMetrics(stream, resolvedUUID, 0, 0, 0, log)
		},
		StreamLive: func() error {
			return s.streamLiveMetrics(stream, resolvedUUID, filter, log)
		},
	}

	return StreamWithHistory(stream.Context(), cfg, state)
}

// GetJobMetrics retrieves historical metrics for a job.
func (s *JobServiceServer) GetJobMetrics(req *pb.GetJobMetricsRequest, stream grpc.ServerStreamingServer[pb.JobMetricsEvent]) error {
	log := s.logger.WithFields("operation", "GetJobMetrics", "uuid", req.JobUuid)
	log.Debug("get job metrics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.JobUuid == "" {
		return status.Errorf(codes.InvalidArgument, "job_uuid is required")
	}

	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.JobUuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.JobUuid, "error", err)
		resolvedUUID = req.JobUuid
	}

	// Create filter for metrics only
	filter := &telemetry.EventFilter{
		Types: []telemetry.EventType{telemetry.EventTypeMetrics},
	}

	return s.sendHistoricalMetrics(stream, resolvedUUID, filter, req.StartTime, req.EndTime, int(req.Limit), log)
}

// StreamJobTelematics streams live eBPF security events for a running job.
// Events include exec, connect, accept, file access, mmap, and mprotect.
func (s *JobServiceServer) StreamJobTelematics(req *pb.StreamJobTelematicsRequest, stream grpc.ServerStreamingServer[pb.TelematicsEvent]) error {
	log := s.logger.WithFields("operation", "StreamJobTelematics", "uuid", req.JobUuid)
	log.Debug("stream job telematics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.JobUuid == "" {
		return status.Errorf(codes.InvalidArgument, "job_uuid is required")
	}

	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.JobUuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.JobUuid, "error", err)
		resolvedUUID = req.JobUuid
	}

	// Check if telemetry collector is available
	if s.telemetryCollector == nil {
		log.Warn("telemetry collector not configured")
		return status.Errorf(codes.Unavailable, "telemetry collector not configured")
	}

	// Parse event type filter - exclude metrics
	filter := s.parseTelematicsFilter(req.Types)

	log.Debug("starting telematics stream", "types", req.Types)

	// Determine job state
	job, exists := s.jobStore.Job(resolvedUUID)
	isCompleted := exists && job.IsCompleted()
	state := DetermineJobState(exists, isCompleted)

	// Use unified streaming helper
	cfg := StreamConfig{
		JobUUID: resolvedUUID,
		Logger:  log,
		SendHistorical: func() (int, error) {
			err := s.sendHistoricalTelematics(stream, resolvedUUID, filter, 0, 0, 0, log)
			return 0, err // sendHistoricalTelematics doesn't return count
		},
		QueryPersistOnly: func() (int, error) {
			if s.persistClient == nil {
				return 0, nil
			}
			return s.queryPersistTelematics(stream, resolvedUUID, filter, 0, 0, 0, log)
		},
		StreamLive: func() error {
			return s.streamLiveTelematics(stream, resolvedUUID, filter, log)
		},
	}

	return StreamWithHistory(stream.Context(), cfg, state)
}

// GetJobTelematics retrieves historical eBPF security events for a job.
func (s *JobServiceServer) GetJobTelematics(req *pb.GetJobTelematicsRequest, stream grpc.ServerStreamingServer[pb.TelematicsEvent]) error {
	log := s.logger.WithFields("operation", "GetJobTelematics", "uuid", req.JobUuid)
	log.Debug("get job telematics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.JobUuid == "" {
		return status.Errorf(codes.InvalidArgument, "job_uuid is required")
	}

	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.JobUuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.JobUuid, "error", err)
		resolvedUUID = req.JobUuid
	}

	// Parse event type filter - exclude metrics
	filter := s.parseTelematicsFilter(req.Types)

	return s.sendHistoricalTelematics(stream, resolvedUUID, filter, req.StartTime, req.EndTime, int(req.Limit), log)
}

// parseTelematicsFilter creates a filter for telematics event types (excluding metrics)
func (s *JobServiceServer) parseTelematicsFilter(types []string) *telemetry.EventFilter {
	if len(types) == 0 {
		// Default to all telematics events (exclude metrics)
		return &telemetry.EventFilter{
			Types: []telemetry.EventType{
				telemetry.EventTypeExec,
				telemetry.EventTypeConnect,
				telemetry.EventTypeAccept,
				telemetry.EventTypeFile,
				telemetry.EventTypeMmap,
				telemetry.EventTypeMprotect,
				telemetry.EventTypeSocketData,
			},
		}
	}
	return &telemetry.EventFilter{
		Types: telemetry.ParseEventTypes(types),
	}
}

// sendHistoricalMetrics sends historical metrics from buffer and persist
func (s *JobServiceServer) sendHistoricalMetrics(stream grpc.ServerStreamingServer[pb.JobMetricsEvent], jobID string, filter *telemetry.EventFilter, startTime, endTime int64, limit int, log *logger.Logger) error {
	eventCount := 0

	// Parse time range
	var startT, endT time.Time
	if startTime > 0 {
		startT = time.Unix(0, startTime)
	}
	if endTime > 0 {
		endT = time.Unix(0, endTime)
	}

	// First query persist for complete historical data
	if s.persistClient != nil {
		count, err := s.queryPersistMetrics(stream, jobID, startTime, endTime, int32(limit), log)
		if err != nil {
			log.Warn("failed to query persist metrics", "error", err)
		} else {
			eventCount += count
		}
	}

	// If no persist data, try in-memory buffer (for recently completed jobs)
	if eventCount == 0 && s.telemetryCollector != nil {
		events := s.telemetryCollector.GetBufferedEvents(jobID, filter, startT, endT, limit)
		for _, event := range events {
			if event.Type == telemetry.EventTypeMetrics {
				pbEvent := s.telemetryEventToMetricsEvent(event)
				if err := stream.Send(pbEvent); err != nil {
					log.Warn("failed to send metrics event", "error", err)
					return status.Errorf(codes.Internal, "failed to send metrics event: %v", err)
				}
				eventCount++
			}
		}
	}

	log.Debug("metrics query completed", "eventCount", eventCount)
	return nil
}

// queryPersistMetrics queries metrics from persist service
func (s *JobServiceServer) queryPersistMetrics(stream grpc.ServerStreamingServer[pb.JobMetricsEvent], jobID string, startTime, endTime int64, limit int32, log *logger.Logger) (int, error) {
	eventCount := 0

	metricsReq := &persistpb.QueryMetricsRequest{
		JobUuid:   jobID,
		NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
	}
	metricsStream, err := s.persistClient.QueryMetrics(stream.Context(), metricsReq)
	if err != nil {
		log.Debug("failed to query metrics from persist", "error", err)
		return 0, nil
	}

	for {
		metric, err := metricsStream.Recv()
		if err != nil {
			break // EOF or error
		}
		pbEvent := &pb.JobMetricsEvent{
			Timestamp:      metric.Timestamp,
			JobUuid:        metric.JobUuid,
			CpuPercent:     metric.Data.CpuUsage * 100,
			MemoryBytes:    metric.Data.MemoryUsage,
			GpuPercent:     metric.Data.GpuUsage * 100,
			DiskReadBytes:  metric.Data.DiskIo.GetReadBytes(),
			DiskWriteBytes: metric.Data.DiskIo.GetWriteBytes(),
			NetRecvBytes:   metric.Data.NetworkIo.GetRxBytes(),
			NetSentBytes:   metric.Data.NetworkIo.GetTxBytes(),
		}
		if err := stream.Send(pbEvent); err != nil {
			log.Warn("failed to send metric event", "error", err)
			return eventCount, status.Errorf(codes.Internal, "failed to send metric event: %v", err)
		}
		eventCount++
	}

	return eventCount, nil
}

// streamLiveMetrics streams live metrics for a running job
func (s *JobServiceServer) streamLiveMetrics(stream grpc.ServerStreamingServer[pb.JobMetricsEvent], jobID string, filter *telemetry.EventFilter, log *logger.Logger) error {
	streamCtx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Subscribe to job events to detect completion
	updates, unsubscribe, err := s.jobStore.PubSub().Subscribe(streamCtx, "jobs")
	if err != nil {
		log.Error("failed to subscribe to job events", "error", err)
		return status.Errorf(codes.Internal, "failed to subscribe to job events: %v", err)
	}
	defer unsubscribe()

	done := make(chan error, 1)

	go func() {
		streamErr := s.telemetryCollector.Stream(streamCtx, jobID, filter, func(event *telemetry.Event) error {
			if event.Type == telemetry.EventTypeMetrics {
				pbEvent := s.telemetryEventToMetricsEvent(event)
				if err := stream.Send(pbEvent); err != nil {
					return err
				}
			}
			return nil
		})
		if streamErr != nil && streamErr != context.Canceled {
			done <- streamErr
		} else {
			done <- nil
		}
	}()

	// Monitor for job completion
	jobCompleted := false
	drainDeadline := time.Time{}

	for {
		if jobCompleted && time.Now().After(drainDeadline) {
			cancel()
			<-done
			return nil
		}

		var selectTimeout <-chan time.Time
		if jobCompleted {
			selectTimeout = time.After(50 * time.Millisecond)
		}

		select {
		case <-stream.Context().Done():
			cancel()
			<-done
			return nil
		case err := <-done:
			if err != nil {
				return status.Errorf(codes.Internal, "failed to stream metrics: %v", err)
			}
			return nil
		case <-selectTimeout:
			continue
		case msg, ok := <-updates:
			if !ok {
				cancel()
				<-done
				return nil
			}

			event := msg.Payload
			if event.JobUUID != jobID {
				continue
			}

			if event.Type == "UPDATED" {
				if event.Status == "COMPLETED" || event.Status == "FAILED" || event.Status == "STOPPED" || event.Status == "TIMEOUT" {
					if !jobCompleted {
						jobCompleted = true
						drainDeadline = time.Now().Add(500 * time.Millisecond)
					}
				}
			}
		}
	}
}

// sendHistoricalTelematics sends historical telematics events from buffer and persist
func (s *JobServiceServer) sendHistoricalTelematics(stream grpc.ServerStreamingServer[pb.TelematicsEvent], jobID string, filter *telemetry.EventFilter, startTime, endTime int64, limit int, log *logger.Logger) error {
	eventCount := 0

	// Parse time range
	var startT, endT time.Time
	if startTime > 0 {
		startT = time.Unix(0, startTime)
	}
	if endTime > 0 {
		endT = time.Unix(0, endTime)
	}

	// First query persist for complete historical data
	if s.persistClient != nil {
		count, err := s.queryPersistTelematics(stream, jobID, filter, startTime, endTime, int32(limit), log)
		if err != nil {
			log.Warn("failed to query persist telematics", "error", err)
		} else {
			eventCount += count
		}
	}

	// If no persist data, try in-memory buffer (for recently completed jobs)
	if eventCount == 0 && s.telemetryCollector != nil {
		events := s.telemetryCollector.GetBufferedEvents(jobID, filter, startT, endT, limit)
		for _, event := range events {
			if event.Type != telemetry.EventTypeMetrics {
				pbEvent := s.telemetryEventToTelematicsEvent(event)
				if pbEvent != nil {
					if err := stream.Send(pbEvent); err != nil {
						log.Warn("failed to send telematics event", "error", err)
						return status.Errorf(codes.Internal, "failed to send telematics event: %v", err)
					}
					eventCount++
				}
			}
		}
	}

	log.Debug("telematics query completed", "eventCount", eventCount)
	return nil
}

// queryPersistTelematics queries telematics events from persist service
func (s *JobServiceServer) queryPersistTelematics(stream grpc.ServerStreamingServer[pb.TelematicsEvent], jobID string, filter *telemetry.EventFilter, startTime, endTime int64, limit int32, log *logger.Logger) (int, error) {
	// Collect all events first, then sort by timestamp
	var allEvents []*pb.TelematicsEvent

	// Check which event types are requested (empty filter = all types)
	wantsExec := len(filter.Types) == 0
	wantsConnect := len(filter.Types) == 0
	wantsMmap := len(filter.Types) == 0
	wantsMprotect := len(filter.Types) == 0
	wantsFile := len(filter.Types) == 0
	wantsAccept := len(filter.Types) == 0
	wantsSocketData := len(filter.Types) == 0
	for _, t := range filter.Types {
		switch t {
		case telemetry.EventTypeExec:
			wantsExec = true
		case telemetry.EventTypeConnect:
			wantsConnect = true
		case telemetry.EventTypeMmap:
			wantsMmap = true
		case telemetry.EventTypeMprotect:
			wantsMprotect = true
		case telemetry.EventTypeFile:
			wantsFile = true
		case telemetry.EventTypeAccept:
			wantsAccept = true
		case telemetry.EventTypeSocketData:
			wantsSocketData = true
		}
	}

	// Query exec events if requested
	if wantsExec {
		execReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		execStream, err := s.persistClient.QueryExecEvents(stream.Context(), execReq)
		if err != nil {
			log.Debug("failed to query exec events from persist", "error", err)
		} else {
			for {
				execEvent, err := execStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: execEvent.Timestamp,
					JobUuid:   execEvent.JobUuid,
					Type:      "exec",
					Data: &pb.TelematicsEvent_Exec{
						Exec: &pb.TelematicsExecData{
							Pid:    execEvent.Pid,
							Ppid:   execEvent.Ppid,
							Binary: execEvent.Filename,
							Args:   execEvent.Args,
						},
					},
				})
			}
		}
	}

	// Query connect events if requested
	if wantsConnect {
		connectReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		connectStream, err := s.persistClient.QueryConnectEvents(stream.Context(), connectReq)
		if err != nil {
			log.Debug("failed to query connect events from persist", "error", err)
		} else {
			for {
				connectEvent, err := connectStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: connectEvent.Timestamp,
					JobUuid:   connectEvent.JobUuid,
					Type:      "connect",
					Data: &pb.TelematicsEvent_Connect{
						Connect: &pb.TelematicsConnectData{
							Pid:      connectEvent.Pid,
							DstAddr:  connectEvent.DstAddr,
							DstPort:  connectEvent.DstPort,
							Protocol: connectEvent.Protocol,
							SrcAddr:  connectEvent.SrcAddr,
							SrcPort:  connectEvent.SrcPort,
						},
					},
				})
			}
		}
	}

	// Query mmap events if requested
	if wantsMmap {
		mmapReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		mmapStream, err := s.persistClient.QueryMmapEvents(stream.Context(), mmapReq)
		if err != nil {
			log.Debug("failed to query mmap events from persist", "error", err)
		} else {
			for {
				mmapEvent, err := mmapStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: mmapEvent.Timestamp,
					JobUuid:   mmapEvent.JobUuid,
					Type:      "mmap",
					Data: &pb.TelematicsEvent_Mmap{
						Mmap: &pb.TelematicsMmapData{
							Pid:    mmapEvent.Pid,
							Addr:   mmapEvent.Addr,
							Length: mmapEvent.Length,
							Prot:   mmapEvent.Prot,
						},
					},
				})
			}
		}
	}

	// Query mprotect events if requested
	if wantsMprotect {
		mprotectReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		mprotectStream, err := s.persistClient.QueryMprotectEvents(stream.Context(), mprotectReq)
		if err != nil {
			log.Debug("failed to query mprotect events from persist", "error", err)
		} else {
			for {
				mprotectEvent, err := mprotectStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: mprotectEvent.Timestamp,
					JobUuid:   mprotectEvent.JobUuid,
					Type:      "mprotect",
					Data: &pb.TelematicsEvent_Mprotect{
						Mprotect: &pb.TelematicsMprotectData{
							Pid:    mprotectEvent.Pid,
							Addr:   mprotectEvent.Addr,
							Length: mprotectEvent.Length,
							Prot:   mprotectEvent.Prot,
						},
					},
				})
			}
		}
	}

	// Query file events if requested
	if wantsFile {
		fileReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		fileStream, err := s.persistClient.QueryFileEvents(stream.Context(), fileReq)
		if err != nil {
			log.Debug("failed to query file events from persist", "error", err)
		} else {
			for {
				fileEvent, err := fileStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: fileEvent.Timestamp,
					JobUuid:   fileEvent.JobUuid,
					Type:      "file",
					Data: &pb.TelematicsEvent_File{
						File: &pb.TelematicsFileData{
							Pid:       fileEvent.Pid,
							Path:      fileEvent.Path,
							Operation: fileEvent.Operation,
							Bytes:     fileEvent.Bytes,
						},
					},
				})
			}
		}
	}

	// Query accept events if requested
	if wantsAccept {
		acceptReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		acceptStream, err := s.persistClient.QueryAcceptEvents(stream.Context(), acceptReq)
		if err != nil {
			log.Debug("failed to query accept events from persist", "error", err)
		} else {
			for {
				acceptEvent, err := acceptStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: acceptEvent.Timestamp,
					JobUuid:   acceptEvent.JobUuid,
					Type:      "accept",
					Data: &pb.TelematicsEvent_Accept{
						Accept: &pb.TelematicsAcceptData{
							Pid:      acceptEvent.Pid,
							SrcAddr:  acceptEvent.SrcAddr,
							SrcPort:  acceptEvent.SrcPort,
							DstAddr:  acceptEvent.DstAddr,
							DstPort:  acceptEvent.DstPort,
							Protocol: acceptEvent.Protocol,
						},
					},
				})
			}
		}
	}

	// Query socket data events if requested
	if wantsSocketData {
		socketDataReq := &persistpb.QueryTelemetryRequest{
			JobUuid:   jobID,
			NodeId:    s.getJobNodeId(jobID), // For multi-node CloudWatch queries
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     limit,
		}
		socketDataStream, err := s.persistClient.QuerySocketDataEvents(stream.Context(), socketDataReq)
		if err != nil {
			log.Debug("failed to query socket data events from persist", "error", err)
		} else {
			for {
				socketDataEvent, err := socketDataStream.Recv()
				if err != nil {
					break // EOF or error
				}
				allEvents = append(allEvents, &pb.TelematicsEvent{
					Timestamp: socketDataEvent.Timestamp,
					JobUuid:   socketDataEvent.JobUuid,
					Type:      "socket_data",
					Data: &pb.TelematicsEvent_SocketData{
						SocketData: &pb.TelematicsSocketDataData{
							Pid:       socketDataEvent.Pid,
							Direction: socketDataEvent.Direction,
							DstAddr:   socketDataEvent.DstAddr,
							DstPort:   socketDataEvent.DstPort,
							Bytes:     socketDataEvent.Bytes,
						},
					},
				})
			}
		}
	}

	// Sort all events by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Stream sorted events
	eventCount := 0
	for _, event := range allEvents {
		if err := stream.Send(event); err != nil {
			log.Warn("failed to send telematics event", "error", err, "type", event.Type)
			return eventCount, status.Errorf(codes.Internal, "failed to send telematics event: %v", err)
		}
		eventCount++
	}

	return eventCount, nil
}

// streamLiveTelematics streams live telematics events for a running job
func (s *JobServiceServer) streamLiveTelematics(stream grpc.ServerStreamingServer[pb.TelematicsEvent], jobID string, filter *telemetry.EventFilter, log *logger.Logger) error {
	streamCtx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Subscribe to job events to detect completion
	updates, unsubscribe, err := s.jobStore.PubSub().Subscribe(streamCtx, "jobs")
	if err != nil {
		log.Error("failed to subscribe to job events", "error", err)
		return status.Errorf(codes.Internal, "failed to subscribe to job events: %v", err)
	}
	defer unsubscribe()

	done := make(chan error, 1)

	go func() {
		streamErr := s.telemetryCollector.Stream(streamCtx, jobID, filter, func(event *telemetry.Event) error {
			if event.Type != telemetry.EventTypeMetrics {
				pbEvent := s.telemetryEventToTelematicsEvent(event)
				if pbEvent != nil {
					if err := stream.Send(pbEvent); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if streamErr != nil && streamErr != context.Canceled {
			done <- streamErr
		} else {
			done <- nil
		}
	}()

	// Monitor for job completion
	jobCompleted := false
	drainDeadline := time.Time{}

	for {
		if jobCompleted && time.Now().After(drainDeadline) {
			cancel()
			<-done
			return nil
		}

		var selectTimeout <-chan time.Time
		if jobCompleted {
			selectTimeout = time.After(50 * time.Millisecond)
		}

		select {
		case <-stream.Context().Done():
			cancel()
			<-done
			return nil
		case err := <-done:
			if err != nil {
				return status.Errorf(codes.Internal, "failed to stream telematics: %v", err)
			}
			return nil
		case <-selectTimeout:
			continue
		case msg, ok := <-updates:
			if !ok {
				cancel()
				<-done
				return nil
			}

			event := msg.Payload
			if event.JobUUID != jobID {
				continue
			}

			if event.Type == "UPDATED" {
				if event.Status == "COMPLETED" || event.Status == "FAILED" || event.Status == "STOPPED" || event.Status == "TIMEOUT" {
					if !jobCompleted {
						jobCompleted = true
						drainDeadline = time.Now().Add(500 * time.Millisecond)
					}
				}
			}
		}
	}
}

// telemetryEventToMetricsEvent converts a telemetry event to a JobMetricsEvent proto
func (s *JobServiceServer) telemetryEventToMetricsEvent(event *telemetry.Event) *pb.JobMetricsEvent {
	if event.Type != telemetry.EventTypeMetrics {
		return nil
	}
	data, ok := event.Data.(*telemetry.MetricsData)
	if !ok {
		return nil
	}
	return &pb.JobMetricsEvent{
		Timestamp:      event.Timestamp.UnixNano(),
		JobUuid:        event.JobUUID,
		CpuPercent:     data.CPUPercent,
		MemoryBytes:    data.MemoryBytes,
		MemoryLimit:    data.MemoryLimit,
		DiskReadBytes:  data.DiskReadBytes,
		DiskWriteBytes: data.DiskWriteBytes,
		NetRecvBytes:   data.NetRecvBytes,
		NetSentBytes:   data.NetSentBytes,
		GpuPercent:     data.GPUPercent,
		GpuMemoryBytes: data.GPUMemoryBytes,
	}
}

// telemetryEventToTelematicsEvent converts a telemetry event to a TelematicsEvent proto
func (s *JobServiceServer) telemetryEventToTelematicsEvent(event *telemetry.Event) *pb.TelematicsEvent {
	pbEvent := &pb.TelematicsEvent{
		Timestamp: event.Timestamp.UnixNano(),
		JobUuid:   event.JobUUID,
		Type:      string(event.Type),
	}

	switch event.Type {
	case telemetry.EventTypeExec:
		data, ok := event.Data.(*telemetry.ExecData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_Exec{
			Exec: &pb.TelematicsExecData{
				Pid:      data.PID,
				Ppid:     data.PPID,
				Binary:   data.Binary,
				Args:     data.Args,
				ExitCode: data.ExitCode,
			},
		}
	case telemetry.EventTypeConnect:
		data, ok := event.Data.(*telemetry.ConnectData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_Connect{
			Connect: &pb.TelematicsConnectData{
				Pid:      data.PID,
				DstAddr:  data.Address,
				DstPort:  data.Port,
				Protocol: data.Protocol,
				SrcAddr:  data.LocalAddress,
				SrcPort:  data.LocalPort,
			},
		}
	case telemetry.EventTypeAccept:
		data, ok := event.Data.(*telemetry.AcceptData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_Accept{
			Accept: &pb.TelematicsAcceptData{
				Pid:      data.PID,
				SrcAddr:  data.RemoteAddr,
				SrcPort:  data.RemotePort,
				DstPort:  data.LocalPort,
				Protocol: data.Protocol,
			},
		}
	case telemetry.EventTypeFile:
		data, ok := event.Data.(*telemetry.FileData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_File{
			File: &pb.TelematicsFileData{
				Pid:       data.PID,
				Path:      data.Path,
				Operation: data.Operation,
				Bytes:     data.Bytes,
			},
		}
	case telemetry.EventTypeMmap:
		data, ok := event.Data.(*telemetry.MmapData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_Mmap{
			Mmap: &pb.TelematicsMmapData{
				Pid:    data.PID,
				Addr:   data.Addr,
				Length: data.Length,
				Prot:   data.Prot,
				Flags:  data.Flags,
			},
		}
	case telemetry.EventTypeMprotect:
		data, ok := event.Data.(*telemetry.MprotectData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_Mprotect{
			Mprotect: &pb.TelematicsMprotectData{
				Pid:    data.PID,
				Addr:   data.Addr,
				Length: data.Length,
				Prot:   data.Prot,
			},
		}
	case telemetry.EventTypeSocketData:
		data, ok := event.Data.(*telemetry.SocketDataData)
		if !ok {
			return nil
		}
		pbEvent.Data = &pb.TelematicsEvent_SocketData{
			SocketData: &pb.TelematicsSocketDataData{
				Pid:       data.PID,
				Direction: data.Direction,
				DstAddr:   data.Address,
				DstPort:   data.Port,
				Protocol:  data.Protocol,
				Bytes:     data.Bytes,
			},
		}
	default:
		return nil
	}

	return pbEvent
}
