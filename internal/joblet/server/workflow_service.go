package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/validation"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/mappers"
	"joblet/internal/joblet/runtime"
	"joblet/internal/joblet/workflow"
	"joblet/internal/joblet/workflow/types"
	"joblet/pkg/logger"

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
	auth              auth2.GrpcAuthorization
	jobStore          adapters.JobStoreAdapter
	joblet            interfaces.Joblet
	workflowManager   *workflow.WorkflowManager
	workflowValidator *validation.WorkflowValidator
	logger            *logger.Logger
}

// NewWorkflowServiceServer creates a new gRPC service server for workflow operations.
// This server handles workflow creation, status monitoring, and job orchestration.
// It requires authentication, job store access, joblet interface for job execution,
// a workflow manager for dependency tracking and job coordination, and managers for validation.
func NewWorkflowServiceServer(auth auth2.GrpcAuthorization, jobStore adapters.JobStoreAdapter, joblet interfaces.Joblet, workflowManager *workflow.WorkflowManager, volumeManager *volume.Manager, runtimeManager *runtime.Manager, runtimeResolver *runtime.Resolver) *WorkflowServiceServer {
	// Create workflow validator with adapters
	volumeAdapter := validation.NewVolumeManagerAdapter(volumeManager)
	runtimeAdapter := validation.NewRuntimeManagerAdapter(runtimeManager, runtimeResolver)
	workflowValidator := validation.NewWorkflowValidator(volumeAdapter, runtimeAdapter)

	return &WorkflowServiceServer{
		auth:              auth,
		jobStore:          jobStore,
		joblet:            joblet,
		workflowManager:   workflowManager,
		workflowValidator: workflowValidator,
		logger:            logger.WithField("component", "workflow-grpc"),
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
		workflowID, err := s.StartWorkflowOrchestrationWithContent(ctx, req.YamlContent, req.WorkflowFiles)
		if err != nil {
			log.Error("failed to start workflow orchestration with content", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully with uploaded content", "workflowId", workflowID)
		return &pb.RunWorkflowResponse{
			WorkflowId: int32(workflowID),
			Status:     "STARTED",
		}, nil
	}

	// Check if workflow is a YAML file path and parse it (server-side files)
	if strings.HasSuffix(req.Workflow, ".yaml") || strings.HasSuffix(req.Workflow, ".yml") {
		log.Info("detected YAML workflow, starting workflow orchestration")
		workflowID, err := s.StartWorkflowOrchestration(ctx, req.Workflow)
		if err != nil {
			log.Error("failed to start workflow orchestration", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully", "workflowId", workflowID)
		return &pb.RunWorkflowResponse{
			WorkflowId: int32(workflowID),
			Status:     "STARTED",
		}, nil
	}

	// Fallback to simple workflow creation for non-YAML workflows
	workflowID, err := s.workflowManager.CreateWorkflow(req.Workflow, make(map[string]*workflow.JobDependency), req.JobOrder)
	if err != nil {
		log.Error("failed to create workflow", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create workflow: %v", err)
	}

	log.Info("workflow created successfully", "workflowId", workflowID)
	return &pb.RunWorkflowResponse{
		WorkflowId: int32(workflowID),
		Status:     "STARTED",
	}, nil
}

// GetWorkflowStatus returns the current status of a workflow including job states.
// Provides comprehensive workflow information including completed/failed job counts,
// individual job statuses, and overall workflow progress for monitoring.
func (s *WorkflowServiceServer) GetWorkflowStatus(ctx context.Context, req *pb.GetWorkflowStatusRequest) (*pb.GetWorkflowStatusResponse, error) {
	log := s.logger.WithFields("operation", "GetWorkflowStatus", "workflowId", req.WorkflowId)
	log.Debug("get workflow status request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	workflowState, err := s.workflowManager.GetWorkflowStatus(int(req.WorkflowId))
	if err != nil {
		log.Error("failed to get workflow status", "error", err)
		return nil, status.Errorf(codes.NotFound, "workflow not found: %v", err)
	}

	workflowInfo := s.convertWorkflowStateToInfo(workflowState)
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
	log := s.logger.WithFields("operation", "GetWorkflowJobs", "workflowId", req.WorkflowId)
	log.Debug("get workflow jobs request received")

	if err := s.auth.Authorized(ctx, auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	workflowState, err := s.workflowManager.GetWorkflowStatus(int(req.WorkflowId))
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
		"operation", "RunJob",
		"command", req.Command,
		"workflowId", req.WorkflowId,
		"jobId", req.JobId,
	)
	log.Debug("run job request received for workflow")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	jobRequest, err := s.convertToWorkflowJobRequest(req)
	if err != nil {
		log.Error("failed to convert request", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	if req.WorkflowId > 0 {
		readyJobs := s.workflowManager.GetReadyJobs(int(req.WorkflowId))
		canRun := false
		for _, readyJobID := range readyJobs {
			if readyJobID == req.JobId {
				canRun = true
				break
			}
		}
		if !canRun && req.JobId != "" {
			log.Warn("job not ready to run due to dependencies", "jobId", req.JobId)
			return &pb.RunJobResponse{
				JobId:  "",
				Status: "WAITING",
			}, nil
		}
	}

	newJob, err := s.joblet.StartJob(ctx, *jobRequest)
	if err != nil {
		log.Error("job creation failed", "error", err)
		return nil, status.Errorf(codes.Internal, "job run failed: %v", err)
	}

	if req.WorkflowId > 0 {
		s.workflowManager.OnJobStateChange(newJob.Id, newJob.Status)
	}

	log.Info("workflow job started successfully", "jobId", newJob.Id, "status", newJob.Status)
	return &pb.RunJobResponse{
		JobId:  newJob.Id,
		Status: string(newJob.Status),
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
		Environment:       req.Environment,       // Regular environment variables
		SecretEnvironment: req.SecretEnvironment, // Secret environment variables
	}

	return jobRequest, nil
}

func (s *WorkflowServiceServer) convertWorkflowStateToInfo(ws *workflow.WorkflowState) *pb.WorkflowInfo {
	info := &pb.WorkflowInfo{
		Id:            int32(ws.ID),
		Workflow:      ws.Workflow,
		Status:        string(ws.Status),
		TotalJobs:     int32(ws.TotalJobs),
		CompletedJobs: int32(ws.CompletedJobs),
		FailedJobs:    int32(ws.FailedJobs),
		CreatedAt:     s.convertTimeToTimestamp(ws.CreatedAt),
	}

	if ws.StartedAt != nil {
		info.StartedAt = s.convertTimeToTimestamp(*ws.StartedAt)
	}

	if ws.CompletedAt != nil {
		info.CompletedAt = s.convertTimeToTimestamp(*ws.CompletedAt)
	}

	return info
}

func (s *WorkflowServiceServer) convertJobDependenciesToWorkflowJobs(jobs map[string]*workflow.JobDependency) []*pb.WorkflowJob {
	var workflowJobs []*pb.WorkflowJob

	for jobID, jobDep := range jobs {
		wfJob := &pb.WorkflowJob{
			JobId:  jobID,
			Status: string(jobDep.Status),
		}

		for _, req := range jobDep.Requirements {
			wfJob.Dependencies = append(wfJob.Dependencies, req.JobId)
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
func (s *WorkflowServiceServer) StartWorkflowOrchestration(ctx context.Context, yamlPath string) (int, error) {
	log := s.logger.WithField("yamlPath", yamlPath)
	log.Info("starting workflow orchestration from YAML")

	workflowYAML, err := s.parseWorkflowYAML(yamlPath)
	if err != nil {
		return 0, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate workflow before execution
	log.Info("performing server-side workflow validation")
	if err := s.workflowValidator.ValidateWorkflow(*workflowYAML); err != nil {
		log.Error("workflow validation failed", "error", err)
		return 0, fmt.Errorf("workflow validation failed: %w", err)
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
				JobId:  depJob,
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
		return 0, fmt.Errorf("failed to create workflow: %w", err)
	}

	log.Info("workflow created, starting job orchestration", "workflowId", workflowID)

	// Auto-create any missing volumes before starting orchestration
	err = s.autoCreateWorkflowVolumes(workflowYAML)
	if err != nil {
		log.Warn("failed to auto-create some volumes", "error", err)
		// Continue anyway - individual jobs will handle missing volumes
	}

	go s.orchestrateWorkflow(context.Background(), workflowID, workflowYAML, nil)

	return workflowID, nil
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
			readyJobs := s.workflowManager.GetReadyJobs(workflowID)
			if len(readyJobs) == 0 {
				workflowState, err := s.workflowManager.GetWorkflowStatus(workflowID)
				if err == nil && (workflowState.Status == workflow.WorkflowCompleted || workflowState.Status == workflow.WorkflowFailed) {
					log.Info("workflow orchestration completed", "status", workflowState.Status)
					return
				}
				continue
			}

			for _, jobName := range readyJobs {
				if jobSpec, exists := workflowYAML.Jobs[jobName]; exists {
					err := s.executeWorkflowJob(ctx, workflowID, jobName, jobSpec, workflowYAML, uploadedFiles)
					if err != nil {
						log.Error("failed to execute workflow job", "jobName", jobName, "error", err)
						s.workflowManager.OnJobStateChange(jobName, domain.StatusFailed)
					}
				}
			}
		}
	}
}

func (s *WorkflowServiceServer) executeWorkflowJob(ctx context.Context, workflowID int, jobName string, jobSpec JobSpec, workflowYAML *WorkflowYAML, uploadedFiles map[string][]byte) error {
	log := s.logger.WithFields("workflowId", workflowID, "jobName", jobName)
	log.Info("executing workflow job")

	uploads := []domain.FileUpload{}
	if jobSpec.Uploads != nil {
		for _, file := range jobSpec.Uploads.Files {
			var content []byte
			if uploadedFiles != nil {
				// Use uploaded files from client
				if fileContent, exists := uploadedFiles[file]; exists {
					content = fileContent
				} else {
					return fmt.Errorf("file %s not found in uploaded files", file)
				}
			} else {
				// For server-side workflows, we can't read files without knowing the path
				// This is a limitation - server-side workflows should use client-side upload
				return fmt.Errorf("server-side workflow file reading not supported. Use 'rnx run --workflow' with client-side file upload")
			}
			uploads = append(uploads, domain.FileUpload{
				Path:    file,
				Content: content,
				Size:    int64(len(content)),
			})
		}
	}

	// Determine network to use
	network := jobSpec.Network
	if network == "" {
		network = defaultNetworkName
	}

	// Merge environment variables: global workflow vars + job-specific vars (job overrides global)
	mergedEnvironment, mergedSecretEnvironment := s.mergeEnvironmentVariables(workflowYAML, jobSpec)

	jobRequest := interfaces.StartJobRequest{
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
		Environment:       mergedEnvironment,       // Merged global + job-specific environment variables
		SecretEnvironment: mergedSecretEnvironment, // Merged global + job-specific secret environment variables
	}

	job, err := s.joblet.StartJob(ctx, jobRequest)
	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	s.workflowManager.OnJobStateChange(jobName, job.Status)
	log.Info("workflow job started", "jobId", job.Id)

	go s.monitorWorkflowJob(ctx, jobName, job.Id)

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
			job, exists := s.jobStore.GetJob(jobID)
			if !exists {
				log.Warn("job not found in store")
				continue
			}

			s.workflowManager.OnJobStateChange(jobName, job.Status)

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
func (s *WorkflowServiceServer) StartWorkflowOrchestrationWithContent(ctx context.Context, yamlContent string, workflowFiles []*pb.FileUpload) (int, error) {
	log := s.logger.WithField("contentLength", len(yamlContent))
	log.Info("starting workflow orchestration from YAML content")

	// Parse YAML content directly
	workflowYAML, err := s.parseWorkflowYAMLContent(yamlContent)
	if err != nil {
		return 0, fmt.Errorf("failed to parse workflow YAML content: %w", err)
	}

	// Validate workflow before execution
	log.Info("performing server-side workflow validation")
	if err := s.workflowValidator.ValidateWorkflow(*workflowYAML); err != nil {
		log.Error("workflow validation failed", "error", err)
		return 0, fmt.Errorf("workflow validation failed: %w", err)
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
				JobId:  depJob,
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

	// Create workflow
	workflowID, err := s.workflowManager.CreateWorkflow(
		"client-uploaded.yaml",
		jobs,
		jobOrder,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create workflow: %w", err)
	}

	log.Info("workflow created from client content, starting job orchestration", "workflowId", workflowID)

	// Auto-create any missing volumes before starting orchestration
	err = s.autoCreateWorkflowVolumes(workflowYAML)
	if err != nil {
		log.Warn("failed to auto-create some volumes", "error", err)
		// Continue anyway - individual jobs will handle missing volumes
	}

	// Start orchestration with background context and uploaded files
	go s.orchestrateWorkflow(context.Background(), workflowID, workflowYAML, uploadedFiles)

	return workflowID, nil
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
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetId())
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
		Id:                pbJob.Id,
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
	}, nil
}

// StopJob implements the JobService interface
func (s *WorkflowServiceServer) StopJob(ctx context.Context, req *pb.StopJobReq) (*pb.StopJobRes, error) {
	log := s.logger.WithFields("operation", "StopJob", "jobId", req.GetId())
	log.Debug("stop job request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create stop request object
	stopRequest := interfaces.StopJobRequest{
		JobID: req.GetId(),
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
		Id: stopRequest.JobID,
	}, nil
}

// GetJobLogs implements the JobService interface
func (s *WorkflowServiceServer) GetJobLogs(req *pb.GetJobLogsReq, stream pb.JobService_GetJobLogsServer) error {
	log := s.logger.WithFields("operation", "GetJobLogs", "jobId", req.GetId())
	log.Debug("get job logs request received")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth2.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Create a domain streamer adapter
	streamer := &workflowGrpcToDomainStreamer{stream: stream}

	// Stream logs using the job store
	err := s.jobStore.SendUpdatesToClient(stream.Context(), req.GetId(), streamer)
	if err != nil {
		log.Error("failed to stream logs", "error", err)
		if err.Error() == "job not found" {
			return status.Errorf(codes.NotFound, "job not found: %s", req.GetId())
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

	// Copy global workflow environment variables
	if workflowYAML.Environment != nil {
		for key, value := range workflowYAML.Environment {
			// Apply basic templating to global variables
			processedValue := s.processEnvironmentTemplating(value, workflowYAML.Environment, workflowYAML.SecretEnvironment)
			mergedEnvironment[key] = processedValue
			log.Debug("inherited global environment variable", "key", key, "value", processedValue)
		}
	}

	// Copy global workflow secret environment variables
	if workflowYAML.SecretEnvironment != nil {
		for key, value := range workflowYAML.SecretEnvironment {
			// Apply basic templating to global secret variables
			processedValue := s.processEnvironmentTemplating(value, workflowYAML.Environment, workflowYAML.SecretEnvironment)
			mergedSecretEnvironment[key] = processedValue
			log.Debug("inherited global secret environment variable", "key", key)
		}
	}

	// Override with job-specific environment variables
	if jobSpec.Environment != nil {
		for key, value := range jobSpec.Environment {
			// Apply templating to job-specific variables (can reference global vars)
			processedValue := s.processEnvironmentTemplating(value, mergedEnvironment, mergedSecretEnvironment)
			mergedEnvironment[key] = processedValue
			log.Debug("job-specific environment variable", "key", key, "value", processedValue, "overrode_global", workflowYAML.Environment != nil && workflowYAML.Environment[key] != "")
		}
	}

	// Override with job-specific secret environment variables
	if jobSpec.SecretEnvironment != nil {
		for key, value := range jobSpec.SecretEnvironment {
			// Apply templating to job-specific secret variables
			processedValue := s.processEnvironmentTemplating(value, mergedEnvironment, mergedSecretEnvironment)
			mergedSecretEnvironment[key] = processedValue
			log.Debug("job-specific secret environment variable", "key", key, "overrode_global", workflowYAML.SecretEnvironment != nil && workflowYAML.SecretEnvironment[key] != "")
		}
	}

	log.Info("environment variables merged", "total_env_vars", len(mergedEnvironment), "total_secret_vars", len(mergedSecretEnvironment))
	return mergedEnvironment, mergedSecretEnvironment
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
