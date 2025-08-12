package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/joblet/workflow/types"
	"joblet/internal/rnx/common"
	pkgconfig "joblet/pkg/config"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Use shared types from workflow/types package
type WorkflowYAML = types.WorkflowYAML
type JobSpec = types.JobSpec
type JobUploads = types.JobUploads
type JobResources = types.JobResources

// NewWorkflowCmd creates the main workflow command with all subcommands.
// Provides workflow management capabilities including create, status, list, and run.
// This is the entry point for all workflow-related CLI operations.
func NewWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows",
		Long:  "Create, monitor, and manage workflows with job dependencies",
	}

	cmd.AddCommand(NewWorkflowCreateCmd())
	cmd.AddCommand(NewWorkflowStatusCmd())
	cmd.AddCommand(NewWorkflowListCmd())
	cmd.AddCommand(NewWorkflowRunCmd())

	return cmd
}

// NewWorkflowCreateCmd creates a command for creating workflows from templates.
// Takes a workflow name and template path as arguments.
// Deprecated in favor of the more flexible 'run' command with YAML support.
func NewWorkflowCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name> <template>",
		Short: "Create a new workflow",
		Long:  "Create a new workflow with specified name and template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewJobClient()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.Close()

			// Create workflow service client
			workflowClient := pb.NewJobServiceClient(client.GetConn())

			req := &pb.CreateWorkflowRequest{
				Name:      args[0],
				Template:  args[1],
				TotalJobs: 0, // Will be determined from template
				JobOrder:  []string{},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			res, err := workflowClient.CreateWorkflow(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to create workflow: %w", err)
			}

			fmt.Printf("Workflow created successfully:\n")
			fmt.Printf("  ID: %d\n", res.WorkflowId)
			fmt.Printf("  Name: %s\n", args[0])
			fmt.Printf("  Template: %s\n", args[1])

			return nil
		},
	}
}

func NewWorkflowStatusCmd() *cobra.Command {
	var statusJSON bool

	cmd := &cobra.Command{
		Use:   "status <workflow-id>",
		Short: "Get workflow status",
		Long:  "Get the current status and progress of a workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewJobClient()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.Close()

			workflowID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid workflow ID: %s", args[0])
			}

			// Create workflow service client
			workflowClient := pb.NewJobServiceClient(client.GetConn())

			req := &pb.GetWorkflowStatusRequest{
				WorkflowId: int32(workflowID),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			res, err := workflowClient.GetWorkflowStatus(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to get workflow status: %w", err)
			}

			if statusJSON {
				return outputWorkflowStatusJSON(res)
			}

			workflow := res.Workflow
			fmt.Printf("Workflow Status:\n")
			fmt.Printf("  ID: %d\n", workflow.Id)
			fmt.Printf("  Name: %s\n", workflow.Name)
			fmt.Printf("  Template: %s\n", workflow.Template)
			fmt.Printf("  Status: %s\n", workflow.Status)
			fmt.Printf("  Progress: %d/%d jobs completed\n", workflow.CompletedJobs, workflow.TotalJobs)
			fmt.Printf("  Failed: %d\n", workflow.FailedJobs)

			if len(res.Jobs) > 0 {
				fmt.Printf("\nJobs:\n")
				for _, job := range res.Jobs {
					fmt.Printf("  - %s: %s\n", job.Name, job.Status)
					if len(job.Dependencies) > 0 {
						fmt.Printf("    Dependencies: %v\n", job.Dependencies)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")

	return cmd
}

func NewWorkflowListCmd() *cobra.Command {
	var listJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long:  "List all workflows in the system",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := common.NewJobClient()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.Close()

			// Create workflow service client
			workflowClient := pb.NewJobServiceClient(client.GetConn())

			req := &pb.ListWorkflowsRequest{
				IncludeCompleted: true, // Always include all workflows
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			res, err := workflowClient.ListWorkflows(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to list workflows: %w", err)
			}

			if len(res.Workflows) == 0 {
				if listJSON {
					fmt.Println("[]")
				} else {
					fmt.Println("No workflows found")
				}
				return nil
			}

			if listJSON {
				return outputWorkflowsJSON(res.Workflows)
			}

			fmt.Printf("ID   NAME                 STATUS      PROGRESS\n")
			fmt.Printf("---- -------------------- ----------- ---------\n")
			for _, workflow := range res.Workflows {
				fmt.Printf("%-4d %-20s %-11s %d/%d\n",
					workflow.Id,
					truncateString(workflow.Name, 20),
					workflow.Status,
					workflow.CompletedJobs,
					workflow.TotalJobs)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")

	return cmd
}

// NewWorkflowRunCmd creates a command for running workflows from YAML files.
// This is the primary command for workflow execution, supporting:
// - YAML workflow definition parsing
// - File uploads and preprocessing
// - Volume mounting for data sharing between jobs
// - Automatic dependency resolution and job orchestration
func NewWorkflowRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <yaml-file>",
		Short: "Run a workflow from YAML file",
		Long:  "Parse and execute a workflow from a YAML file",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Load client configuration manually since workflow run needs config
			var err error
			common.NodeConfig, err = pkgconfig.LoadClientConfig(common.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to load client config: %w", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			yamlFile := args[0]

			fmt.Printf("Starting workflow from: %s\n", yamlFile)
			fmt.Printf("Parsing YAML and uploading files...\n")

			// Read and parse YAML file on client side
			yamlContent, err := os.ReadFile(yamlFile)
			if err != nil {
				return fmt.Errorf("failed to read YAML file %s: %w", yamlFile, err)
			}

			var workflow WorkflowYAML
			if err := yaml.Unmarshal(yamlContent, &workflow); err != nil {
				return fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Extract and upload all files referenced in jobs
			workflowFiles, err := extractWorkflowFiles(yamlFile, workflow)
			if err != nil {
				return fmt.Errorf("failed to extract workflow files: %w", err)
			}

			fmt.Printf("Found %d files to upload\n", len(workflowFiles))

			// Create client and workflow service
			client, err := common.NewJobClient()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			defer client.Close()

			workflowClient := pb.NewJobServiceClient(client.GetConn())

			// Create workflow with YAML content and files
			workflowName := fmt.Sprintf("client-workflow-%d", time.Now().Unix())
			createReq := &pb.CreateWorkflowRequest{
				Name:          workflowName,
				Template:      yamlFile, // Keep original path for reference
				YamlContent:   string(yamlContent),
				WorkflowFiles: workflowFiles,
				TotalJobs:     int32(len(workflow.Jobs)),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			createRes, err := workflowClient.CreateWorkflow(ctx, createReq)
			if err != nil {
				return fmt.Errorf("failed to create workflow: %w", err)
			}

			fmt.Printf("Workflow created with ID: %d\n", createRes.WorkflowId)
			fmt.Printf("Use 'rnx workflow status %d' to monitor progress\n", createRes.WorkflowId)

			return nil
		},
	}
}

// extractWorkflowFiles extracts and reads all files referenced in workflow jobs
func extractWorkflowFiles(yamlPath string, workflow WorkflowYAML) ([]*pb.FileUpload, error) {
	var uploads []*pb.FileUpload
	yamlDir := filepath.Dir(yamlPath)
	uploadedFiles := make(map[string]bool) // Prevent duplicate uploads

	// Collect all file uploads from all jobs
	for jobName, job := range workflow.Jobs {
		if job.Uploads != nil {
			for _, fileName := range job.Uploads.Files {
				if uploadedFiles[fileName] {
					continue // Skip duplicates
				}

				// Try relative to YAML file first, then absolute path
				filePath := filepath.Join(yamlDir, fileName)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					// Try absolute path
					filePath = fileName
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						return nil, fmt.Errorf("file %s referenced in job %s not found", fileName, jobName)
					}
				}

				// Read file content
				content, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
				}

				// Get file info
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
				}

				uploads = append(uploads, &pb.FileUpload{
					Path:        fileName, // Use relative name as specified in YAML
					Content:     content,
					Mode:        uint32(fileInfo.Mode()),
					IsDirectory: false,
				})

				uploadedFiles[fileName] = true
			}
		}
	}

	return uploads, nil
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// outputWorkflowsJSON outputs the workflows in JSON format
func outputWorkflowsJSON(workflows []*pb.WorkflowInfo) error {
	// Convert protobuf workflows to a simpler structure for JSON output
	type jsonWorkflow struct {
		ID            int32  `json:"id"`
		Name          string `json:"name"`
		Template      string `json:"template"`
		Status        string `json:"status"`
		TotalJobs     int32  `json:"total_jobs"`
		CompletedJobs int32  `json:"completed_jobs"`
		FailedJobs    int32  `json:"failed_jobs"`
		CreatedAt     string `json:"created_at,omitempty"`
		StartedAt     string `json:"started_at,omitempty"`
		CompletedAt   string `json:"completed_at,omitempty"`
	}

	var jsonWorkflows []jsonWorkflow
	for _, workflow := range workflows {
		jsonWf := jsonWorkflow{
			ID:            workflow.Id,
			Name:          workflow.Name,
			Template:      workflow.Template,
			Status:        workflow.Status,
			TotalJobs:     workflow.TotalJobs,
			CompletedJobs: workflow.CompletedJobs,
			FailedJobs:    workflow.FailedJobs,
		}

		// Convert timestamps if present
		if workflow.CreatedAt != nil {
			jsonWf.CreatedAt = time.Unix(workflow.CreatedAt.Seconds, int64(workflow.CreatedAt.Nanos)).Format(time.RFC3339)
		}
		if workflow.StartedAt != nil {
			jsonWf.StartedAt = time.Unix(workflow.StartedAt.Seconds, int64(workflow.StartedAt.Nanos)).Format(time.RFC3339)
		}
		if workflow.CompletedAt != nil {
			jsonWf.CompletedAt = time.Unix(workflow.CompletedAt.Seconds, int64(workflow.CompletedAt.Nanos)).Format(time.RFC3339)
		}

		jsonWorkflows = append(jsonWorkflows, jsonWf)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonWorkflows)
}

// outputWorkflowStatusJSON outputs the workflow status in JSON format
func outputWorkflowStatusJSON(res *pb.GetWorkflowStatusResponse) error {
	// Convert protobuf workflow status to a simpler structure for JSON output
	type jsonJob struct {
		Name         string   `json:"name"`
		Status       string   `json:"status"`
		Dependencies []string `json:"dependencies,omitempty"`
	}

	type jsonWorkflowStatus struct {
		ID            int32     `json:"id"`
		Name          string    `json:"name"`
		Template      string    `json:"template"`
		Status        string    `json:"status"`
		TotalJobs     int32     `json:"total_jobs"`
		CompletedJobs int32     `json:"completed_jobs"`
		FailedJobs    int32     `json:"failed_jobs"`
		CreatedAt     string    `json:"created_at,omitempty"`
		StartedAt     string    `json:"started_at,omitempty"`
		CompletedAt   string    `json:"completed_at,omitempty"`
		Jobs          []jsonJob `json:"jobs,omitempty"`
	}

	workflow := res.Workflow
	jsonStatus := jsonWorkflowStatus{
		ID:            workflow.Id,
		Name:          workflow.Name,
		Template:      workflow.Template,
		Status:        workflow.Status,
		TotalJobs:     workflow.TotalJobs,
		CompletedJobs: workflow.CompletedJobs,
		FailedJobs:    workflow.FailedJobs,
	}

	// Convert timestamps if present
	if workflow.CreatedAt != nil {
		jsonStatus.CreatedAt = time.Unix(workflow.CreatedAt.Seconds, int64(workflow.CreatedAt.Nanos)).Format(time.RFC3339)
	}
	if workflow.StartedAt != nil {
		jsonStatus.StartedAt = time.Unix(workflow.StartedAt.Seconds, int64(workflow.StartedAt.Nanos)).Format(time.RFC3339)
	}
	if workflow.CompletedAt != nil {
		jsonStatus.CompletedAt = time.Unix(workflow.CompletedAt.Seconds, int64(workflow.CompletedAt.Nanos)).Format(time.RFC3339)
	}

	// Convert jobs if present
	if len(res.Jobs) > 0 {
		for _, job := range res.Jobs {
			jsonJob := jsonJob{
				Name:   job.Name,
				Status: job.Status,
			}
			if len(job.Dependencies) > 0 {
				jsonJob.Dependencies = job.Dependencies
			}
			jsonStatus.Jobs = append(jsonStatus.Jobs, jsonJob)
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonStatus)
}
