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
	"joblet/internal/joblet/domain"
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
	auth            auth2.GrpcAuthorization
	jobStore        adapters.JobStoreAdapter
	joblet          interfaces.Joblet
	workflowManager *workflow.WorkflowManager
	logger          *logger.Logger
}

// NewWorkflowServiceServer creates a new gRPC service server for workflow operations.
// This server handles workflow creation, status monitoring, and job orchestration.
// It requires authentication, job store access, joblet interface for job execution,
// and a workflow manager for dependency tracking and job coordination.
func NewWorkflowServiceServer(auth auth2.GrpcAuthorization, jobStore adapters.JobStoreAdapter, joblet interfaces.Joblet, workflowManager *workflow.WorkflowManager) *WorkflowServiceServer {
	return &WorkflowServiceServer{
		auth:            auth,
		jobStore:        jobStore,
		joblet:          joblet,
		workflowManager: workflowManager,
		logger:          logger.WithField("component", "workflow-grpc"),
	}
}

// CreateWorkflow handles gRPC requests to create and execute workflows.
// Supports both template-based workflows and client-uploaded YAML content.
// For client uploads, automatically processes uploaded files and starts orchestration.
// Returns the workflow ID for monitoring progress and status.
func (s *WorkflowServiceServer) CreateWorkflow(ctx context.Context, req *pb.CreateWorkflowRequest) (*pb.CreateWorkflowResponse, error) {
	log := s.logger.WithFields(
		"operation", "CreateWorkflow",
		"name", req.Name,
		"template", req.Template,
		"totalJobs", req.TotalJobs,
	)
	log.Debug("create workflow request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.Name == "" || req.Template == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name and template are required")
	}

	// Check if we have YAML content (client-side upload) or just a template path
	if req.YamlContent != "" {
		log.Info("detected client-side YAML content, starting workflow orchestration with uploaded files")
		workflowID, err := s.StartWorkflowOrchestrationWithContent(ctx, req.YamlContent, req.WorkflowFiles)
		if err != nil {
			log.Error("failed to start workflow orchestration with content", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully with uploaded content", "workflowId", workflowID)
		return &pb.CreateWorkflowResponse{
			WorkflowId: int32(workflowID),
		}, nil
	}

	// Check if template is a YAML file path and parse it (server-side files)
	if strings.HasSuffix(req.Template, ".yaml") || strings.HasSuffix(req.Template, ".yml") {
		log.Info("detected YAML template, starting workflow orchestration")
		workflowID, err := s.StartWorkflowOrchestration(ctx, req.Template)
		if err != nil {
			log.Error("failed to start workflow orchestration", "error", err)
			return nil, status.Errorf(codes.Internal, "failed to start workflow orchestration: %v", err)
		}

		log.Info("workflow orchestration started successfully", "workflowId", workflowID)
		return &pb.CreateWorkflowResponse{
			WorkflowId: int32(workflowID),
		}, nil
	}

	// Fallback to simple workflow creation for non-YAML templates
	workflowID, err := s.workflowManager.CreateWorkflow(req.Name, req.Template, make(map[string]*workflow.JobDependency), req.JobOrder)
	if err != nil {
		log.Error("failed to create workflow", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create workflow: %v", err)
	}

	log.Info("workflow created successfully", "workflowId", workflowID)
	return &pb.CreateWorkflowResponse{
		WorkflowId: int32(workflowID),
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

func (s *WorkflowServiceServer) CancelWorkflow(ctx context.Context, req *pb.CancelWorkflowRequest) (*pb.CancelWorkflowResponse, error) {
	log := s.logger.WithFields("operation", "CancelWorkflow", "workflowId", req.WorkflowId)
	log.Debug("cancel workflow request received")

	if err := s.auth.Authorized(ctx, auth2.StopJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	err := s.workflowManager.CancelWorkflow(int(req.WorkflowId))
	if err != nil {
		log.Error("failed to cancel workflow", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to cancel workflow: %v", err)
	}

	log.Info("workflow canceled successfully", "workflowId", req.WorkflowId)
	return &pb.CancelWorkflowResponse{
		Success: true,
		Message: "Workflow canceled successfully",
	}, nil
}

func (s *WorkflowServiceServer) RunJob(ctx context.Context, req *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	log := s.logger.WithFields(
		"operation", "RunJob",
		"command", req.Command,
		"workflowId", req.WorkflowId,
		"jobName", req.JobName,
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
			if readyJobID == req.JobName {
				canRun = true
				break
			}
		}
		if !canRun && req.JobName != "" {
			log.Warn("job not ready to run due to dependencies", "jobName", req.JobName)
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
		Uploads:  domainUploads,
		Schedule: req.Schedule,
		Network:  network,
		Volumes:  req.Volumes,
		Runtime:  req.Runtime,
	}

	return jobRequest, nil
}

func (s *WorkflowServiceServer) convertWorkflowStateToInfo(ws *workflow.WorkflowState) *pb.WorkflowInfo {
	info := &pb.WorkflowInfo{
		Id:            int32(ws.ID),
		Name:          ws.Name,
		Template:      ws.Template,
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
			Name:   jobID,
			Status: string(jobDep.Status),
		}

		for _, req := range jobDep.Requirements {
			wfJob.Dependencies = append(wfJob.Dependencies, req.JobName)
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
// This method handles server-side workflow templates stored on the filesystem.
// Returns the workflow ID for tracking progress and status.
func (s *WorkflowServiceServer) StartWorkflowOrchestration(ctx context.Context, yamlPath string) (int, error) {
	log := s.logger.WithField("yamlPath", yamlPath)
	log.Info("starting workflow orchestration from YAML")

	workflowYAML, err := s.parseWorkflowYAML(yamlPath)
	if err != nil {
		return 0, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

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
				Type:    workflow.RequirementSimple,
				JobName: depJob,
				Status:  status,
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
		fmt.Sprintf("workflow-%d", time.Now().Unix()),
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
					err := s.executeWorkflowJob(ctx, workflowID, jobName, jobSpec, uploadedFiles)
					if err != nil {
						log.Error("failed to execute workflow job", "jobName", jobName, "error", err)
						s.workflowManager.OnJobStateChange(jobName, domain.StatusFailed)
					}
				}
			}
		}
	}
}

func (s *WorkflowServiceServer) executeWorkflowJob(ctx context.Context, workflowID int, jobName string, jobSpec JobSpec, uploadedFiles map[string][]byte) error {
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
				return fmt.Errorf("server-side workflow file reading not supported. Use 'rnx workflow run' with client-side file upload")
			}
			uploads = append(uploads, domain.FileUpload{
				Path:    file,
				Content: content,
				Size:    int64(len(content)),
			})
		}
	}

	jobRequest := interfaces.StartJobRequest{
		Command: jobSpec.Command,
		Args:    jobSpec.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    int32(jobSpec.Resources.MaxCPU),
			MaxMemory: int32(jobSpec.Resources.MaxMemory),
			MaxIOBPS:  int32(jobSpec.Resources.MaxIOBPS),
			CPUCores:  jobSpec.Resources.CPUCores,
		},
		Uploads: uploads,
		Network: defaultNetworkName,
		Volumes: jobSpec.Volumes,
		Runtime: jobSpec.Runtime,
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
// Used for server-side workflow templates stored on disk.
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
				Type:    workflow.RequirementSimple,
				JobName: depJob,
				Status:  status,
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
		fmt.Sprintf("client-workflow-%d", time.Now().Unix()),
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
