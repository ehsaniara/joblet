package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/ehsaniara/joblet/api/gen"
	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	auth2 "github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/core/validation"
	"github.com/ehsaniara/joblet/internal/joblet/core/volume"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/mappers"
	metricsdomain "github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/runtime"
	"github.com/ehsaniara/joblet/internal/joblet/workflow"
	"github.com/ehsaniara/joblet/internal/joblet/workflow/types"
	"github.com/ehsaniara/joblet/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

const (
	// defaultNetworkName is the default network for workflow jobs
	defaultNetworkName = "bridge"

	// workflowOrchestrationInterval is how often we check for ready jobs
	workflowOrchestrationInterval = 5 * time.Second

	// jobMonitoringInterval is how often we check job status
	jobMonitoringInterval = 2 * time.Second

	// defaultVolumeSize is the default size for auto-created volumes
	defaultVolumeSize = "100MB"
)

type WorkflowServiceServer struct {
	pb.UnimplementedJobServiceServer
	auth              auth2.GRPCAuthorization
	jobStore          adapters.JobStorer
	metricsStore      *adapters.MetricsStoreAdapter
	joblet            interfaces.Joblet
	workflowManager   *workflow.WorkflowManager
	workflowValidator *validation.WorkflowValidator
	logger            *logger.Logger

	// UUID to workflow ID mapping
	workflowUuidMap  map[string]int
	workflowMapMutex sync.RWMutex
}

// NewWorkflowServiceServer creates a new gRPC service server for workflow operations.
// This server handles workflow creation, status monitoring, and job orchestration.
// It requires authentication, job store access, joblet interface for job execution,
// a workflow manager for dependency tracking and job coordination, and managers for validation.
func NewWorkflowServiceServer(auth auth2.GRPCAuthorization, jobStore adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, joblet interfaces.Joblet, workflowManager *workflow.WorkflowManager, volumeManager *volume.Manager, runtimeResolver *runtime.Resolver) *WorkflowServiceServer {
	// Create workflow validator with concrete managers (no adapter pattern needed)
	workflowValidator := validation.NewWorkflowValidator(volumeManager, runtimeResolver)

	return &WorkflowServiceServer{
		auth:              auth,
		jobStore:          jobStore,
		metricsStore:      metricsStore,
		joblet:            joblet,
		workflowManager:   workflowManager,
		workflowValidator: workflowValidator,
		logger:            logger.WithField("component", "workflow-grpc"),
		workflowUuidMap:   make(map[string]int),
	}
}

// RunWorkflow handles gRPC requests to execute workflow-based jobs and workflows.
// Supports both server-side workflow files and client-uploaded YAML content.
// For client uploads, automatically processes uploaded files and starts orchestration.
// Returns the workflow ID and status for monitoring progress.
func (s *WorkflowServiceServer) RunWorkflow(ctx context.Context, req *pb.RunWorkflowRequest) (*pb.RunWorkflowResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunWorkflow",
		"workflow", req.Workflow,
		"totalJobs", req.TotalJobs,
	)
	log.Debug("run job workflow request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.Workflow == "" {
		return nil, status.Errorf(codes.InvalidArgument, "workflow is required")
	}

	// Check if we have YAML content (client-side upload) or just a workflow path
	if req.YamlContent != "" {
		log.Info("detected client-side YAML content, starting workflow orchestration with uploaded files")
		workflowUuid, err := s.StartWorkflowOrchestrationWithContent(ctx, req.YamlContent, req.WorkflowFiles)
		if err != nil {
			log.Error("failed to start workflow orchestration with content", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully with uploaded content", "workflowUuid", workflowUuid)
		return &pb.RunWorkflowResponse{
			WorkflowUuid: workflowUuid,
			Status:       "STARTED",
		}, nil
	}

	// Check if workflow is a YAML file path and parse it (server-side files)
	if strings.HasSuffix(req.Workflow, ".yaml") || strings.HasSuffix(req.Workflow, ".yml") {
		log.Info("detected YAML workflow, starting workflow orchestration")
		workflowUuid, err := s.StartWorkflowOrchestration(ctx, req.Workflow)
		if err != nil {
			log.Error("failed to start workflow orchestration", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully", "workflowUuid", workflowUuid)
		return &pb.RunWorkflowResponse{
			WorkflowUuid: workflowUuid,
			Status:       "STARTED",
		}, nil
	}

	// Fallback to simple workflow creation for non-YAML workflows
	// Generate UUID for simple workflow creation
	workflowUuid := s.generateWorkflowUUID()
	workflowID, err := s.workflowManager.CreateWorkflow(req.Workflow, make(map[string]*workflow.JobDependency), req.JobOrder)
	if err != nil {
		log.Error("failed to create workflow", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create workflow: %v", err)
	}

	// Store workflow UUID -> ID mapping
	s.storeWorkflowMapping(workflowUuid, workflowID)
	log.Info("workflow created successfully", "workflowId", workflowID, "workflowUuid", workflowUuid)
	return &pb.RunWorkflowResponse{
		WorkflowUuid: workflowUuid,
		Status:       "STARTED",
	}, nil
}

// GetWorkflowStatus returns the current status of a workflow including job states.
// Provides comprehensive workflow information including completed/failed job counts,
// individual job statuses, and overall workflow progress for monitoring.
func (s *WorkflowServiceServer) GetWorkflowStatus(ctx context.Context, req *pb.GetWorkflowStatusRequest) (*pb.GetWorkflowStatusResponse, error) {
	log := s.logger.WithFields("operation", "GetWorkflowStatus", "workflowUuid", req.WorkflowUuid)
	log.Debug("get workflow status request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Look up workflow ID by UUID (supports prefix matching)
	workflowID, found := s.lookupWorkflowID(req.WorkflowUuid)
	if !found {
		log.Error("workflow not found", "workflowUuid", req.WorkflowUuid)
		return nil, status.Errorf(codes.NotFound, "workflow not found: %s", req.WorkflowUuid)
	}

	// Find the full UUID for this workflow ID
	fullUuid := s.getFullUuidForWorkflowID(workflowID)

	workflowState, err := s.workflowManager.GetWorkflowStatus(workflowID)
	if err != nil {
		log.Error("failed to get workflow status", "error", err)
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	workflowInfo := s.convertWorkflowStateToInfo(workflowState)
	// Override the UUID with the actual full UUID
	workflowInfo.Uuid = fullUuid
	workflowJobs := s.convertJobDependenciesToWorkflowJobs(workflowState.Jobs)

	return &pb.GetWorkflowStatusResponse{
		Workflow: workflowInfo,
		Jobs:     workflowJobs,
	}, nil
}

// ListWorkflows returns a list of all workflows with their current status.
// Supports filtering and pagination for large workflow lists.
// Provides workflow overview information for monitoring and management interfaces.
func (s *WorkflowServiceServer) ListWorkflows(ctx context.Context, req *pb.ListWorkflowsRequest) (*pb.ListWorkflowsResponse, error) {
	log := s.logger.WithField("operation", "ListWorkflows")
	log.Debug("list workflows request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	workflows := s.workflowManager.ListWorkflows()
	var pbWorkflows []*pb.WorkflowInfo

	for _, wf := range workflows {
		if !req.IncludeCompleted && (wf.Status == workflow.WorkflowCompleted || wf.Status == workflow.WorkflowFailed) {
			continue
		}
		pbWorkflows = append(pbWorkflows, s.convertWorkflowStateToInfo(wf))
	}

	log.Debug("workflows listed", "count", len(pbWorkflows))
	return &pb.ListWorkflowsResponse{
		Workflows: pbWorkflows,
	}, nil
}

func (s *WorkflowServiceServer) GetWorkflowJobs(ctx context.Context, req *pb.GetWorkflowJobsRequest) (*pb.GetWorkflowJobsResponse, error) {
	log := s.logger.WithFields("operation", "GetWorkflowJobs", "workflowUuid", req.WorkflowUuid)
	log.Debug("get workflow jobs request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	workflowID := s.convertWorkflowUUIDToID(req.WorkflowUuid)
	workflowState, err := s.workflowManager.GetWorkflowStatus(workflowID)
	if err != nil {
		log.Error("failed to get workflow", "error", err)
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	workflowJobs := s.convertJobDependenciesToWorkflowJobs(workflowState.Jobs)
	return &pb.GetWorkflowJobsResponse{
		Jobs: workflowJobs,
	}, nil
}

func (s *WorkflowServiceServer) RunJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunJob-Unified",
		"command", req.Command,
		"workflowUuid", req.WorkflowUuid,
	)
	log.Debug("run job request received - unified approach")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// UNIFIED APPROACH: Handle individual jobs using original JobService logic
	if req.WorkflowUuid == "" {
		// This is an individual job - use original job processing (bypasses workflow validation)
		return s.runIndividualJob(ctx, req)
	}

	// This is part of existing workflow - use current logic
	return s.runExistingWorkflowJob(ctx, req)
}

// NEW: Handle individual jobs using original JobService logic (bypasses workflow validation)
func (s *WorkflowServiceServer) runIndividualJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunIndividualJob",
		"command", req.Command,
		"args", req.Args,
		"uploadCount", len(req.Uploads),
		"schedule", req.Schedule,
	)

	log.Debug("processing individual job request (original JobService logic)")

	// Convert protobuf request to domain request object (reuse JobService conversion logic)
	jobRequest, err := s.convertToIndividualJobRequest(req)
	if err != nil {
		log.Error("failed to convert request", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Log the request (excluding sensitive environment variables)
	envCount := 0
	if jobRequest.Environment != nil {
		envCount = len(jobRequest.Environment)
	}
	log.Info("starting individual job with request object",
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

	// Use joblet interface directly (bypasses workflow validation, handles volume creation on-demand)
	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("individual job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	// Log success
	if req.Schedule != "" {
		log.Info("individual job scheduled successfully",
			"jobUuid", newJob.Uuid,
			"scheduledTime", req.Schedule)
	} else {
		log.Info("individual job started successfully",
			"jobUuid", newJob.Uuid,
			"status", newJob.Status)
	}

	// Convert domain job to protobuf response
	return &pb.RunJobResponse{
		JobUuid: newJob.Uuid,
		Status:  string(newJob.Status),
	}, nil
}

// runExistingWorkflowJob handles jobs that are part of existing workflows
func (s *WorkflowServiceServer) runExistingWorkflowJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithField("workflowUuid", req.WorkflowUuid)

	jobRequest, err := s.convertToWorkflowJobRequest(req)
	if err != nil {
		log.Error("failed to convert request", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	workflowID := s.convertWorkflowUUIDToID(req.WorkflowUuid)
	readyJobs := s.workflowManager.GetReadyJobs(workflowID)
	canRun := false
	for _, readyJobID := range readyJobs {
		if readyJobID == req.JobUuid {
			canRun = true
			break
		}
	}
	if !canRun && req.JobUuid != "" {
		log.Warn("job not ready to run due to dependencies", "jobId", req.JobUuid)
		return &pb.RunJobResponse{
			JobUuid: "",
			Status:  "WAITING",
		}, nil
	}

	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	s.workflowManager.OnJobStateChange(newJob.Uuid, newJob.Status)

	log.Info("workflow job started successfully", "jobId", newJob.Uuid, "status", newJob.Status)
	return &pb.RunJobResponse{
		JobUuid: newJob.Uuid,
		Status:  string(newJob.Status),
	}, nil
}

func (s *WorkflowServiceServer) convertToWorkflowJobRequest(req *pb.RunJobRequest) (*interfaces.StartJobRequest, error) {
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

	// Determine job type from environment variables (same logic as job service)
	jobType := domain.JobTypeStandard
	if req.Environment != nil {
		if envJobType, exists := req.Environment["JOB_TYPE"]; exists && envJobType == "runtime-build" {
			jobType = domain.JobTypeRuntimeBuild
			s.logger.Info("detected runtime build job type from workflow environment", "jobType", jobType)
		}
	}

	jobRequest := &interfaces.StartJobRequest{
		Name:    req.Name, // Pass through job name from request
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
		Environment:       req.Environment,       // Regular environment variables
		SecretEnvironment: req.SecretEnvironment, // Secret environment variables
		JobType:           jobType,               // Pass job type to the core
	}

	return jobRequest, nil
}

// convertToIndividualJobRequest converts protobuf request to domain request object (for individual jobs)
func (s *WorkflowServiceServer) convertToIndividualJobRequest(req *pb.RunJobRequest) (*interfaces.StartJobRequest, error) {
	// Validate required fields
	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set default network if not specified
	network := req.Network
	if network == "" {
		network = "bridge"
	}

	// Convert file uploads
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

	// Determine job type from environment variables (same as JobService)
	jobType := domain.JobTypeStandard // Default to standard production jobs
	if req.Environment != nil {
		if envJobType, exists := req.Environment["JOB_TYPE"]; exists && envJobType == "runtime-build" {
			jobType = domain.JobTypeRuntimeBuild
			s.logger.Info("detected runtime build job from environment", "envJobType", envJobType)
		}
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
		Environment:       req.Environment,       // Regular environment variables (logged)
		SecretEnvironment: req.SecretEnvironment, // Secret environment variables (not logged)
		JobType:           jobType,               // Set job type for isolation configuration
	}

	// Validate the request (reuse validation logic from JobService)
	if err := s.validateIndividualJobRequest(jobRequest); err != nil {
		return nil, fmt.Errorf("request validation failed: %w", err)
	}

	return jobRequest, nil
}

// validateIndividualJobRequest performs validation on the individual job request object
func (s *WorkflowServiceServer) validateIndividualJobRequest(req *interfaces.StartJobRequest) error {
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

	// Validate runtime specification if provided
	if req.Runtime != "" {
		if err := s.validateRuntime(req.Runtime); err != nil {
			return fmt.Errorf("invalid runtime: %w", err)
		}
	}

	return nil
}

// validateRuntime validates the runtime specification (reused from JobService)
func (s *WorkflowServiceServer) validateRuntime(runtimeSpec string) error {
	// Basic format validation
	if runtimeSpec == "" {
		return fmt.Errorf("runtime specification cannot be empty")
	}

	// Support both formats: hyphen and colon separated
	if strings.Contains(runtimeSpec, ":") {
		// Traditional format: language:version (e.g., "python:3.11")
		parts := strings.Split(runtimeSpec, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid runtime format: expected 'language:version', got '%s'", runtimeSpec)
		}
		if parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("runtime language and version cannot be empty")
		}
	} else {
		// Runtime name format: language-version-tags (e.g., "python-3.11-ml")
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

// convertUploadsToStringArray converts FileUpload array to string array of paths
func (s *WorkflowServiceServer) convertUploadsToStringArray(uploads []domain.FileUpload) []string {
	var uploadPaths []string
	for _, upload := range uploads {
		uploadPaths = append(uploadPaths, upload.Path)
	}
	return uploadPaths
}

func (s *WorkflowServiceServer) convertWorkflowStateToInfo(ws *workflow.WorkflowState) *pb.WorkflowInfo {
	info := &pb.WorkflowInfo{
		Uuid:          s.getFullUuidForWorkflowID(ws.ID),
		Status:        string(ws.Status),
		TotalJobs:     int32(ws.TotalJobs),
		CompletedJobs: int32(ws.CompletedJobs),
		FailedJobs:    int32(ws.FailedJobs),
		CreatedAt:     s.convertTimeToTimestamp(ws.CreatedAt),
		YamlContent:   ws.YamlContent,
	}

	if ws.StartedAt != nil {
		info.StartedAt = s.convertTimeToTimestamp(*ws.StartedAt)
	}

	if ws.CompletedAt != nil {
		info.CompletedAt = s.convertTimeToTimestamp(*ws.CompletedAt)
	}

	return info
}

// convertJobDependenciesToWorkflowJobs converts internal JobDependency structures to protobuf WorkflowJob messages.
//
// RESPONSIBILITY:
// - Transforms internal workflow job data into API-compatible protobuf messages
// - Properly separates job IDs from job names for CLI display
// - Constructs dependency lists from job requirement specifications
// - Enables proper job status reporting with both IDs and readable names
//
// JOB NAMES INTEGRATION:
// - JobId field contains actual job IDs (e.g., "42", "43") for jobs that have been started
// - JobId field contains job names (e.g., "setup-data") for jobs that haven't been started yet
// - JobName field always contains readable job names from workflow YAML
// - This separation allows CLI to display both columns correctly
//
// WORKFLOW:
// 1. Iterates through internal JobDependency map
// 2. Creates WorkflowJob protobuf message for each job
// 3. Sets JobId from JobDependency.JobID (actual ID if started, job name if not)
// 4. Sets JobName from JobDependency.InternalName (always the workflow YAML name)
// 5. Maps job status to string representation
// 6. Builds dependency list from job requirements
//
// PARAMETERS:
// - jobs: Map of job identifiers to JobDependency structures from workflow manager
//
// RETURNS:
// - []*pb.WorkflowJob: Array of protobuf messages ready for gRPC response
//
// CLI DISPLAY IMPACT:
// This method directly affects how workflow status is displayed:
// - JOB ID column shows actual job IDs for started jobs
// - JOB NAME column shows readable names from YAML
// - DEPENDENCIES column shows job name dependencies for clarity
//
// USAGE:
// Called by GetWorkflowStatus to convert internal data for API responses.
func (s *WorkflowServiceServer) convertJobDependenciesToWorkflowJobs(jobs map[string]*workflow.JobDependency) []*pb.WorkflowJob {
	var workflowJobs []*pb.WorkflowJob

	for _, jobDep := range jobs {
		// For non-started jobs, JobID still contains the job name, so show "0"
		jobID := jobDep.JobID
		if jobID == jobDep.InternalName {
			// Job hasn't been started yet, show "0"
			jobID = "0"
		}

		wfJob := &pb.WorkflowJob{
			JobUuid: jobID,               // Show actual job ID for started jobs, "0" for non-started jobs
			JobName: jobDep.InternalName, // Use InternalName as the job name from workflow
			Status:  string(jobDep.Status),
		}

		for _, req := range jobDep.Requirements {
			wfJob.Dependencies = append(wfJob.Dependencies, req.JobID)
		}

		workflowJobs = append(workflowJobs, wfJob)
	}

	return workflowJobs
}

func (s *WorkflowServiceServer) convertTimeToTimestamp(t time.Time) *pb.Timestamp {
	return &pb.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

// StartWorkflowOrchestration initiates workflow execution from a YAML file path.
// Parses the workflow definition, creates jobs with dependencies, and begins orchestration.
// This method handles server-side workflow files stored on the filesystem.
// Returns the workflow ID for tracking progress and status.
func (s *WorkflowServiceServer) StartWorkflowOrchestration(ctx context.Context, yamlPath string) (string, error) {
	// Generate UUID for this workflow
	workflowUuid := s.generateWorkflowUUID()
	log := s.logger.WithFields("yamlPath", yamlPath, "workflowUuid", workflowUuid)
	log.Info("starting workflow orchestration from YAML")

	workflowYAML, err := s.parseWorkflowYAML(yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate workflow before execution
	log.Info("performing server-side workflow validation")
	if err := s.workflowValidator.ValidateWorkflow(*workflowYAML); err != nil {
		log.Error("workflow validation failed", "error", err)
		return "", fmt.Errorf("workflow validation failed: %w", err)
	}
	log.Info("workflow validation passed")

	jobs := make(map[string]*workflow.JobDependency)
	var jobOrder []string

	for jobName, jobSpec := range workflowYAML.Jobs {
		dependencies := make(map[string]string)
		if jobSpec.Requires != nil {
			for _, req := range jobSpec.Requires {
				for depJob, status := range req {
					dependencies[depJob] = status
				}
			}
		}

		var requirements []workflow.Requirement
		for depJob, status := range dependencies {
			requirements = append(requirements, workflow.Requirement{
				Type:   workflow.RequirementSimple,
				JobID:  depJob,
				Status: status,
			})
		}
		jobs[jobName] = &workflow.JobDependency{
			JobID:        jobName,
			InternalName: jobName,
			Requirements: requirements,
			Status:       domain.StatusPending,
		}
		jobOrder = append(jobOrder, jobName)
	}

	workflowID, err := s.workflowManager.CreateWorkflow(
		yamlPath,
		jobs,
		jobOrder,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create workflow: %w", err)
	}

	// Store workflow UUID -> ID mapping
	s.storeWorkflowMapping(workflowUuid, workflowID)

	log.Info("workflow created, starting job orchestration", "workflowId", workflowID)

	// Auto-create any missing volumes before starting orchestration
	err = s.autoCreateWorkflowVolumes(workflowYAML)
	if err != nil {
		log.Warn("failed to auto-create some volumes", "error", err)
		// Continue anyway - individual jobs will handle missing volumes
	}

	go s.orchestrateWorkflow(context.Background(), workflowID, workflowYAML, nil)

	return workflowUuid, nil
}

func (s *WorkflowServiceServer) orchestrateWorkflow(ctx context.Context, workflowID int, workflowYAML *WorkflowYAML, uploadedFiles map[string][]byte) {
	log := s.logger.WithField("workflowId", workflowID)
	ticker := time.NewTicker(workflowOrchestrationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("workflow orchestration context canceled")
			return
		case <-ticker.C:
			log.Debug("orchestration tick - checking for ready jobs")
			readyJobs := s.workflowManager.GetReadyJobs(workflowID)
			log.Debug("orchestration ready jobs check", "readyJobsCount", len(readyJobs), "readyJobs", readyJobs)
			if len(readyJobs) == 0 {
				workflowState, err := s.workflowManager.GetWorkflowStatus(workflowID)
				if err != nil {
					log.Warn("failed to get workflow status during orchestration", "error", err)
					continue
				}
				log.Debug("orchestration status check", "workflowStatus", workflowState.Status, "completedJobs", workflowState.CompletedJobs, "totalJobs", workflowState.TotalJobs)
				if workflowState.Status == workflow.WorkflowCompleted || workflowState.Status == workflow.WorkflowFailed {
					log.Info("workflow orchestration completed", "status", workflowState.Status)
					return
				}
				continue
			}

			log.Info("found ready jobs for orchestration", "readyJobs", readyJobs)

			for _, jobName := range readyJobs {
				if jobSpec, exists := workflowYAML.Jobs[jobName]; exists {
					err := s.executeWorkflowJob(ctx, workflowID, jobName, jobSpec, workflowYAML, uploadedFiles)
					if err != nil {
						log.Error("failed to execute workflow job", "jobName", jobName, "error", err)
						// For failed job startup, we still use jobName since no actual job ID was created
						s.workflowManager.OnJobStateChange(jobName, domain.StatusFailed)
					}
				}
			}
		}
	}
}

// executeWorkflowJob executes a single job within a workflow context.
//
// RESPONSIBILITY:
// - Creates and starts a job based on workflow job specification
// - Handles file uploads and environment variable merging for the job
// - Updates workflow manager with actual job ID after successful job creation
// - Initiates job monitoring to track status changes throughout execution
// - Integrates job names feature by mapping workflow job names to actual job IDs
//
// WORKFLOW:
// 1. Processes file uploads from workflow job specification
// 2. Validates and normalizes network configuration for job isolation
// 3. Merges global workflow and job-specific environment variables
// 4. Creates StartJobRequest with job name from workflow YAML
// 5. Starts job via joblet service and receives actual job ID
// 6. Updates workflow manager to map job name to actual job ID
// 7. Notifies workflow manager of initial job status
// 8. Launches background job monitoring goroutine
//
// PARAMETERS:
// - ctx: Context for request cancellation and timeout handling
// - workflowID: Unique identifier of the parent workflow
// - jobName: readable job name from workflow YAML (e.g., "setup-data")
// - jobSpec: Complete job specification including command, resources, dependencies
// - workflowYAML: Full workflow configuration for environment variable merging
// - uploadedFiles: Pre-uploaded files available to all jobs in the workflow
//
// RETURNS:
// - error: If job creation fails, network validation fails, or job startup errors
//
// JOB NAMES INTEGRATION:
// - Sets StartJobRequest.Name to jobName for proper job identification
// - Calls UpdateJobID to map job name to actual job ID after creation
// - Enables CLI to display both job IDs and readable names
//
// ERROR HANDLING:
// - Returns error immediately if job creation fails
// - Logs warnings for non-critical issues (e.g., job ID mapping failures)
// - Ensures job monitoring starts even if secondary operations fail
//
// CONCURRENCY:
// - Safe for concurrent execution across multiple workflow jobs
// - Job monitoring runs in separate goroutine to prevent blocking
func (s *WorkflowServiceServer) executeWorkflowJob(ctx context.Context, workflowID int, jobName string, jobSpec JobSpec, workflowYAML *WorkflowYAML, uploadedFiles map[string][]byte) error {
	log := s.logger.WithFields("workflowId", workflowID, "jobName", jobName)
	log.Info("executing workflow job")

	// RACE CONDITION FIX: Process ALL file uploads BEFORE starting the job
	uploadCount := 0
	if jobSpec.Uploads != nil {
		uploadCount = len(jobSpec.Uploads.Files)
	}
	log.Debug("processing file uploads for job", "uploadCount", uploadCount)
	uploads := []domain.FileUpload{}
	if jobSpec.Uploads != nil {
		// Validate that ALL required files are available BEFORE proceeding
		for _, file := range jobSpec.Uploads.Files {
			if uploadedFiles == nil {
				return fmt.Errorf("server-side workflow file reading not supported. Use 'rnx job run --workflow' with client-side file upload")
			}

			// Check file availability first (fail fast if missing)
			if _, exists := uploadedFiles[file]; !exists {
				log.Error("required file not found in uploaded files", "file", file, "availableFiles", getFileKeys(uploadedFiles))
				return fmt.Errorf("file %s not found in uploaded files", file)
			}
		}

		// Now process all files (we know they're all available)
		for _, file := range jobSpec.Uploads.Files {
			fileContent := uploadedFiles[file] // We already validated this exists
			uploads = append(uploads, domain.FileUpload{
				Path:    file,
				Content: fileContent,
				Size:    int64(len(fileContent)),
			})
			log.Debug("prepared file upload for job", "file", file, "size", len(fileContent))
		}
	}

	log.Info("all file uploads prepared successfully", "uploadCount", len(uploads))

	// Determine network to use
	network := jobSpec.Network
	if network == "" {
		network = defaultNetworkName
	}

	// Merge environment variables: global workflow vars + job-specific vars (job overrides global)
	mergedEnvironment, mergedSecretEnvironment := s.mergeEnvironmentVariables(workflowYAML, jobSpec)

	jobRequest := interfaces.StartJobRequest{
		Name:    jobName, // Use the workflow job name
		Command: jobSpec.Command,
		Args:    jobSpec.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    int32(jobSpec.Resources.MaxCPU),
			MaxMemory: int32(jobSpec.Resources.MaxMemory),
			MaxIOBPS:  int32(jobSpec.Resources.MaxIOBPS),
			CPUCores:  jobSpec.Resources.CPUCores,
		},
		Uploads:           uploads,
		Network:           network,
		Volumes:           jobSpec.Volumes,
		Runtime:           jobSpec.Runtime,
		Environment:       mergedEnvironment,                    // Merged global + job-specific environment variables
		SecretEnvironment: mergedSecretEnvironment,              // Merged global + job-specific secret environment variables
		GPUCount:          int32(jobSpec.Resources.GPUCount),    // GPU requirements from YAML
		GPUMemoryMB:       int64(jobSpec.Resources.GPUMemoryMB), // GPU memory requirement
	}

	job, err := s.joblet.StartJob(ctx, jobRequest)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	// Update the workflow manager with the actual job ID
	if err := s.workflowManager.UpdateJobID(jobName, job.Uuid); err != nil {
		log.Warn("failed to update job ID mapping", "jobName", jobName, "actualJobId", job.Uuid, "error", err)
	}

	s.workflowManager.OnJobStateChange(job.Uuid, job.Status)
	log.Info("workflow job started", "jobId", job.Uuid)

	go s.monitorWorkflowJob(ctx, job.Uuid, job.Uuid)

	return nil
}

// monitorWorkflowJob continuously monitors a workflow job's status and updates the workflow manager.
// Runs in a separate goroutine for each job, checking status at regular intervals.
// Handles job state changes and notifies the workflow manager for dependency processing.
// Terminates when the job reaches a terminal state (completed, failed, canceled, stopped).
func (s *WorkflowServiceServer) monitorWorkflowJob(ctx context.Context, jobName, jobID string) {
	log := s.logger.WithFields("jobName", jobName, "jobId", jobID)
	ticker := time.NewTicker(jobMonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, exists := s.jobStore.Job(jobID)
			if !exists {
				log.Warn("job not found in store")
				continue
			}

			s.workflowManager.OnJobStateChange(jobID, job.Status)

			if job.Status == domain.StatusCompleted || job.Status == domain.StatusFailed {
				log.Info("job monitoring completed", "status", job.Status)
				return
			}
		}
	}
}

// parseWorkflowYAML reads and parses a workflow YAML file from the filesystem.
// Used for server-side workflow files stored on disk.
// Returns the parsed workflow structure or an error if reading/parsing fails.
func (s *WorkflowServiceServer) parseWorkflowYAML(yamlPath string) (*WorkflowYAML, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var workflow WorkflowYAML
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &workflow, nil
}

// Use shared types from workflow/types package
type WorkflowYAML = types.WorkflowYAML
type JobSpec = types.JobSpec
type JobUploads = types.JobUploads
type JobResources = types.JobResources

// StartWorkflowOrchestrationWithContent initiates workflow execution from YAML content.
// Handles client-uploaded workflow definitions with associated files.
// Creates necessary volumes, processes file uploads, creates jobs, and starts orchestration.
// This is the primary method for client-side workflow execution via the CLI.
func (s *WorkflowServiceServer) StartWorkflowOrchestrationWithContent(ctx context.Context, yamlContent string, workflowFiles []*pb.FileUpload) (string, error) {
	// Generate UUID for this workflow
	workflowUuid := s.generateWorkflowUUID()
	log := s.logger.WithFields("contentLength", len(yamlContent), "workflowUuid", workflowUuid)
	log.Info("starting workflow orchestration from YAML content")

	// Parse YAML content directly
	workflowYAML, err := s.parseWorkflowYAMLContent(yamlContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow YAML content: %w", err)
	}

	// Validate workflow before execution
	log.Info("performing server-side workflow validation")
	if err := s.workflowValidator.ValidateWorkflow(*workflowYAML); err != nil {
		log.Error("workflow validation failed", "error", err)
		return "", fmt.Errorf("workflow validation failed: %w", err)
	}
	log.Info("workflow validation passed")

	// Store uploaded files in memory map for job execution
	uploadedFiles := make(map[string][]byte)
	for _, file := range workflowFiles {
		uploadedFiles[file.Path] = file.Content
		log.Info("stored uploaded file", "path", file.Path, "size", len(file.Content))
	}

	// Create job dependencies map (only tracks dependencies, not job specs)
	jobs := make(map[string]*workflow.JobDependency)
	var jobOrder []string

	for jobName, jobSpec := range workflowYAML.Jobs {
		dependencies := make(map[string]string)
		if jobSpec.Requires != nil {
			for _, req := range jobSpec.Requires {
				for depJob, depStatus := range req {
					dependencies[depJob] = depStatus
				}
			}
		}

		var requirements []workflow.Requirement
		for depJob, status := range dependencies {
			requirements = append(requirements, workflow.Requirement{
				Type:   workflow.RequirementSimple,
				JobID:  depJob,
				Status: status,
			})
		}

		jobs[jobName] = &workflow.JobDependency{
			JobID:        jobName,
			InternalName: jobName,
			Requirements: requirements,
			Status:       domain.StatusPending,
		}
		jobOrder = append(jobOrder, jobName)
	}

	// Generate meaningful workflow name
	workflowName := s.generateWorkflowName(workflowYAML)

	// Create workflow
	workflowID, err := s.workflowManager.CreateWorkflowWithYaml(
		workflowName,
		yamlContent,
		jobs,
		jobOrder,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create workflow: %w", err)
	}

	// Store workflow UUID -> ID mapping
	s.storeWorkflowMapping(workflowUuid, workflowID)

	log.Info("workflow created from client content, starting job orchestration", "workflowId", workflowID)

	// Auto-create any missing volumes before starting orchestration
	err = s.autoCreateWorkflowVolumes(workflowYAML)
	if err != nil {
		log.Warn("failed to auto-create some volumes", "error", err)
		// Continue anyway - individual jobs will handle missing volumes
	}

	// Start orchestration with background context and uploaded files
	go s.orchestrateWorkflow(context.Background(), workflowID, workflowYAML, uploadedFiles)

	return workflowUuid, nil
}

// parseWorkflowYAMLContent parses workflow YAML content from a string.
// Used for client-uploaded workflow definitions sent via gRPC.
// Returns the parsed workflow structure ready for job creation and orchestration.
func (s *WorkflowServiceServer) parseWorkflowYAMLContent(yamlContent string) (*WorkflowYAML, error) {
	var workflow WorkflowYAML
	if err := yaml.Unmarshal([]byte(yamlContent), &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML content: %w", err)
	}
	return &workflow, nil
}

func (s *WorkflowServiceServer) autoCreateWorkflowVolumes(workflowYAML *WorkflowYAML) error {
	log := s.logger.WithField("operation", "auto-create-volumes")

	// Collect all unique volumes from all jobs
	volumeSet := make(map[string]bool)
	for jobName, jobSpec := range workflowYAML.Jobs {
		for _, volumeName := range jobSpec.Volumes {
			if volumeName != "" {
				volumeSet[volumeName] = true
				log.Debug("found volume requirement", "job", jobName, "volume", volumeName)
			}
		}
	}

	if len(volumeSet) == 0 {
		log.Debug("no volumes required by workflow")
		return nil
	}

	// Check which volumes exist and create missing ones
	for volumeName := range volumeSet {
		volumePath := filepath.Join("/opt/joblet/volumes", volumeName, "data")
		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			log.Info("auto-creating missing volume", "volume", volumeName)

			// Create volume directory structure
			// This is a simplified approach - creates the basic directory structure
			volumeBaseDir := filepath.Join("/opt/joblet/volumes", volumeName)
			if err := os.MkdirAll(volumePath, 0755); err != nil {
				log.Error("failed to create volume directory", "volume", volumeName, "error", err)
				return fmt.Errorf("failed to create volume directory %s: %w", volumeName, err)
			}

			// Create volume metadata file (basic info)
			metadataPath := filepath.Join(volumeBaseDir, "volume-info.json")
			metadata := fmt.Sprintf(`{
  "name": "%s",
  "type": "filesystem",
  "size": "`+defaultVolumeSize+`",
  "created": "%s",
  "auto_created": true
}`, volumeName, time.Now().Format(time.RFC3339))

			if err := os.WriteFile(metadataPath, []byte(metadata), 0644); err != nil {
				log.Warn("failed to create volume metadata", "volume", volumeName, "error", err)
				// Continue anyway - the directory is what matters for job execution
			}

			log.Info("volume auto-created successfully", "volume", volumeName, "size", defaultVolumeSize, "type", "filesystem")
		} else {
			log.Debug("volume already exists", "volume", volumeName)
		}
	}

	return nil
}

// ListJobs implements the JobService interface
func (s *WorkflowServiceServer) ListJobs(ctx context.Context, req *pb.EmptyRequest) (*pb.Jobs, error) {
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
	pbJobs := make([]*pb.Job, len(jobs))
	for i, job := range jobs {
		pbJobs[i] = mapper.DomainToProtobuf(job)
	}

	return &pb.Jobs{Jobs: pbJobs}, nil
}

// GetJobStatus implements the JobService interface
func (s *WorkflowServiceServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusReq) (*pb.GetJobStatusRes, error) {
	log := s.logger.WithFields("operation", "GetJobStatus", "jobId", req.GetUuid())
	log.Debug("get job status request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Retrieve job from store (supports both full UUID and prefix)
	job, exists := s.jobStore.JobByPrefix(req.GetUuid())
	if !exists {
		log.Error("job not found", "jobId", req.GetUuid())
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetUuid())
	}

	// Convert to protobuf using mapper
	mapper := mappers.NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

	log.Debug("job status retrieved successfully", "status", job.Status)

	// Mask secret environment variables for status display
	maskedSecretEnv := make(map[string]string)
	for key := range pbJob.SecretEnvironment {
		maskedSecretEnv[key] = "***"
	}

	return &pb.GetJobStatusRes{
		Uuid:              pbJob.Uuid,
		Name:              pbJob.Name, // Include job name in response
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
		Dependencies:      job.Dependencies,
		WorkflowUuid:      job.WorkflowUuid,
		NodeId:            job.NodeId, // Unique identifier of the Joblet node
	}, nil
}

// StopJob implements the JobService interface
func (s *WorkflowServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetUuid())
	log.Debug("stop job request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create stop request object
	stopRequest := interfaces.StopJobRequest{
		JobID: req.GetUuid(),
	}

	log.Info("stopping job", "jobId", stopRequest.JobID)

	// Use the joblet interface to stop the job
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

// DeleteJob implements the JobService interface
func (s *WorkflowServiceServer) DeleteJob(ctx context.Context, req *pb.DeleteJobReq) (*pb.DeleteJobRes, error) {
	log := s.logger.WithFields("operation", "DeleteJob", "jobId", req.GetUuid())
	log.Debug("delete job request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create delete request
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

// DeleteAllJobs implements the JobService interface for bulk job deletion
func (s *WorkflowServiceServer) DeleteAllJobs(ctx context.Context, req *pb.DeleteAllJobsReq) (*pb.DeleteAllJobsRes, error) {
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

// GetJobLogs implements the JobService interface
func (s *WorkflowServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetUuid())
	log.Debug("get job logs request received")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Create a domain streamer adapter
	streamer := &workflowGrpcToDomainStreamer{stream: stream}

	// Stream logs using the job store
	err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetUuid(), streamer)
	if err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", req.GetUuid())
		}
		return status.Errorf(codes.Internal, "failed to stream logs: %v", err)
	}

	log.Debug("log streaming completed successfully")
	return nil
}

// workflowGrpcToDomainStreamer adapts gRPC stream to domain streamer interface
type workflowGrpcToDomainStreamer struct {
	stream pb.JobService_GetJobLogsServer
}

func (g *workflowGrpcToDomainStreamer) SendData(data []byte) error {
	return g.stream.Send(&pb.DataChunk{
		Payload: data,
	})
}

func (g *workflowGrpcToDomainStreamer) SendKeepalive() error {
	// Send an empty chunk as keepalive
	return g.stream.Send(&pb.DataChunk{
		Payload: []byte{},
	})
}

func (g *workflowGrpcToDomainStreamer) Context() context.Context {
	return g.stream.Context()
}

// mergeEnvironmentVariables combines global workflow environment variables with job-specific ones.
// Job-specific variables take precedence over global workflow variables.
// Supports basic templating for referencing workflow-level variables.
func (s *WorkflowServiceServer) mergeEnvironmentVariables(workflowYAML *WorkflowYAML, jobSpec JobSpec) (map[string]string, map[string]string) {
	log := s.logger.WithField("operation", "merge-environment-variables")

	// Start with global workflow environment variables
	mergedEnvironment := make(map[string]string)
	mergedSecretEnvironment := make(map[string]string)

	// Process job-specific environment variables
	if jobSpec.Environment != nil {
		for key, value := range jobSpec.Environment {
			// Separate secrets from regular environment variables based on naming convention
			if isSecretKey(key) {
				// Apply templating to secret variables
				processedValue := s.processEnvironmentTemplating(value, mergedEnvironment, mergedSecretEnvironment)
				mergedSecretEnvironment[key] = processedValue
				log.Debug("job secret environment variable", "key", key)
			} else {
				// Apply templating to regular variables
				processedValue := s.processEnvironmentTemplating(value, mergedEnvironment, mergedSecretEnvironment)
				mergedEnvironment[key] = processedValue
				log.Debug("job environment variable", "key", key, "value", processedValue)
			}
		}
	}

	log.Info("environment variables merged", "total_env_vars", len(mergedEnvironment), "total_secret_vars", len(mergedSecretEnvironment))
	return mergedEnvironment, mergedSecretEnvironment
}

// isSecretKey determines if an environment variable key represents a secret based on naming conventions.
// Keys starting with "SECRET_" or ending with "_TOKEN", "_KEY", "_PASSWORD", "_SECRET" are considered secrets.
func isSecretKey(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "SECRET_") ||
		strings.HasSuffix(key, "_TOKEN") ||
		strings.HasSuffix(key, "_KEY") ||
		strings.HasSuffix(key, "_PASSWORD") ||
		strings.HasSuffix(key, "_SECRET")
}

// processEnvironmentTemplating processes basic environment variable templating.
// Supports ${VAR_NAME} syntax for referencing other environment variables.
// This provides a simple templating system for workflow environment variable inheritance.
func (s *WorkflowServiceServer) processEnvironmentTemplating(value string, envVars map[string]string, secretEnvVars map[string]string) string {
	// Simple templating: replace ${VAR_NAME} with the value of VAR_NAME
	// This is a basic implementation - could be enhanced with more sophisticated templating later

	processedValue := value

	// Process regular environment variable references
	for refKey, refValue := range envVars {
		placeholder := fmt.Sprintf("${%s}", refKey)
		if strings.Contains(processedValue, placeholder) {
			processedValue = strings.ReplaceAll(processedValue, placeholder, refValue)
			s.logger.Debug("templating: replaced environment variable reference", "placeholder", placeholder, "value", refValue)
		}
	}

	// Process secret environment variable references
	for refKey, refValue := range secretEnvVars {
		placeholder := fmt.Sprintf("${%s}", refKey)
		if strings.Contains(processedValue, placeholder) {
			processedValue = strings.ReplaceAll(processedValue, placeholder, refValue)
			s.logger.Debug("templating: replaced secret environment variable reference", "placeholder", placeholder)
		}
	}

	return processedValue
}

// generateWorkflowUUID generates a UUID for workflow identification
func (s *WorkflowServiceServer) generateWorkflowUUID() string {
	// Read UUID from kernel
	if data, err := os.ReadFile("/proc/sys/kernel/random/uuid"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Fallback: generate a simple UUID-like string
	return fmt.Sprintf("workflow-%d-%d", time.Now().Unix(), time.Now().Nanosecond())
}

// storeWorkflowMapping stores the UUID to workflow ID mapping
func (s *WorkflowServiceServer) storeWorkflowMapping(uuid string, workflowID int) {
	s.workflowMapMutex.Lock()
	defer s.workflowMapMutex.Unlock()
	s.workflowUuidMap[uuid] = workflowID
	s.logger.Debug("stored workflow UUID mapping", "uuid", uuid, "workflowID", workflowID)
}

// lookupWorkflowID looks up workflow ID by UUID (supports prefix matching)
func (s *WorkflowServiceServer) lookupWorkflowID(uuid string) (int, bool) {
	s.workflowMapMutex.RLock()
	defer s.workflowMapMutex.RUnlock()

	// First try exact match
	if id, exists := s.workflowUuidMap[uuid]; exists {
		return id, true
	}

	// If exact match fails and it's a prefix (less than 36 chars), try prefix matching
	if len(uuid) < 36 {
		var matches []int
		var matchedUuids []string
		for storedUuid, id := range s.workflowUuidMap {
			if strings.HasPrefix(storedUuid, uuid) {
				matches = append(matches, id)
				matchedUuids = append(matchedUuids, storedUuid)
			}
		}

		if len(matches) == 1 {
			s.logger.Debug("found unique workflow by prefix", "prefix", uuid, "fullUuid", matchedUuids[0], "workflowID", matches[0])
			return matches[0], true
		} else if len(matches) > 1 {
			s.logger.Warn("workflow prefix matches multiple workflows", "prefix", uuid, "matches", matchedUuids)
			return 0, false
		}
	}

	s.logger.Debug("workflow not found by UUID", "uuid", uuid)
	return 0, false
}

// getFullUuidForWorkflowID gets the full UUID for a given workflow ID
func (s *WorkflowServiceServer) getFullUuidForWorkflowID(workflowID int) string {
	s.workflowMapMutex.RLock()
	defer s.workflowMapMutex.RUnlock()

	for uuid, id := range s.workflowUuidMap {
		if id == workflowID {
			return uuid
		}
	}

	// Fallback if not found (shouldn't happen with our implementation)
	s.logger.Warn("full UUID not found for workflow ID", "workflowID", workflowID)
	return fmt.Sprintf("unknown-%d", workflowID)
}

// convertWorkflowUUIDToID converts workflow UUID to internal integer ID
func (s *WorkflowServiceServer) convertWorkflowUUIDToID(uuid string) int {
	return 1
}

// generateWorkflowName generates a meaningful workflow name with fallback strategy
func (s *WorkflowServiceServer) generateWorkflowName(workflowYAML *WorkflowYAML) string {
	// Priority 1: Explicit name from YAML
	if workflowYAML.Name != "" && strings.TrimSpace(workflowYAML.Name) != "" {
		return strings.TrimSpace(workflowYAML.Name)
	}

	// Priority 2: Generate from job names (first few jobs)
	if len(workflowYAML.Jobs) > 0 {
		var jobNames []string
		for jobName := range workflowYAML.Jobs {
			jobNames = append(jobNames, jobName)
		}

		if len(jobNames) == 1 {
			return fmt.Sprintf("workflow-%s", jobNames[0])
		} else if len(jobNames) <= 3 {
			return fmt.Sprintf("workflow-%s", strings.Join(jobNames, "-"))
		} else {
			return fmt.Sprintf("workflow-%s-and-%d-more", strings.Join(jobNames[:2], "-"), len(jobNames)-2)
		}
	}

	// Priority 3: Default fallback
	return "client-uploaded.yaml"
}

// getFileKeys returns a list of available file keys for debugging
func getFileKeys(files map[string][]byte) []string {
	if files == nil {
		return []string{}
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	return keys
}

// StreamJobMetrics streams real-time metrics for a running job
func (s *WorkflowServiceServer) StreamJobMetrics(req *pb.JobMetricsRequest, stream pb.JobService_StreamJobMetricsServer) error {
	log := s.logger.WithFields("operation", "StreamJobMetrics", "uuid", req.Uuid)
	log.Debug("stream job metrics request received")

	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.Uuid == "" {
		return status.Errorf(codes.InvalidArgument, "uuid is required")
	}

	// Resolve short UUID to full UUID (supports both short and full UUIDs)
	resolvedUUID, err := s.jobStore.ResolveJobUUID(req.Uuid)
	if err != nil {
		log.Warn("failed to resolve UUID", "input", req.Uuid, "error", err)
		// If resolution fails, try using the UUID as-is (might be full UUID of completed job)
		resolvedUUID = req.Uuid
	}

	// Stream metrics using the metrics store
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

// NOTE: GetJobMetricsHistory has been removed - historical metrics are now handled
// by joblet-persist service. Use the persist QueryMetrics RPC instead.

// GetJobMetricsSummary returns aggregated metrics summary for a job
func (s *WorkflowServiceServer) GetJobMetricsSummary(ctx context.Context, req *pb.JobMetricsSummaryRequest) (*pb.JobMetricsSummaryResponse, error) {
	log := s.logger.WithFields("operation", "GetJobMetricsSummary", "uuid", req.Uuid)
	log.Debug("get job metrics summary request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.Uuid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "uuid is required")
	}

	// Check if job exists
	jobID := req.Uuid
	_, exists := s.jobStore.Job(jobID)
	if !exists {
		log.Warn("job not found")
		return nil, status.Errorf(codes.NotFound, "job not found")
	}

	if s.metricsStore == nil {
		log.Warn("metrics store not available")
		return nil, status.Errorf(codes.Unimplemented, "metrics collection not enabled")
	}

	// Calculate time range for metrics
	var from time.Time
	if req.PeriodSeconds > 0 {
		from = time.Now().Add(-time.Duration(req.PeriodSeconds) * time.Second)
	}

	// Get all metrics samples for the job
	samples, err := s.metricsStore.GetHistoricalMetrics(jobID, from, time.Time{})
	if err != nil {
		log.Error("failed to read job metrics", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to read metrics: %v", err)
	}

	if len(samples) == 0 {
		log.Warn("no metrics samples found for job")
		return nil, status.Errorf(codes.NotFound, "no metrics samples found for job")
	}

	log.Debug("aggregating metrics", "sampleCount", len(samples))

	// Aggregate metrics
	response := &pb.JobMetricsSummaryResponse{
		Cpu:     s.aggregateCPUMetrics(samples),
		Memory:  s.aggregateMemoryMetrics(samples),
		Io:      s.aggregateIOMetrics(samples),
		Network: s.aggregateNetworkMetrics(samples),
	}

	log.Info("metrics summary calculated", "samples", len(samples))
	return response, nil
}
