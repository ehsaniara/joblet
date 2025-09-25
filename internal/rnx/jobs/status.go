package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/rnx/common"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var workflowFlag bool
var detailFlag bool

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <uuid>",
		Short: "Get comprehensive status and details of a job or workflow by UUID",
		Long: `Get comprehensive status and details of a job or workflow by UUID.

The status command shows complete job information including:
• Job identification (UUID, name, command, arguments)
• Execution status and timing (created, started, ended, duration)
• Resource limits (CPU, memory, I/O, core binding, GPU allocation)
• Runtime environment (Python, Java, Node.js runtimes)
• Network configuration (bridge, isolated, custom networks)
• Volume storage (mounted persistent and memory volumes)
• Environment variables (regular and secret/masked)
• File uploads and working directory
• Workflow information (UUID, dependencies for workflow jobs)
• Process results (exit code, completion status)
• Contextual next actions (view logs, stop job, etc.)

Both jobs and workflows use UUIDs (36-character identifiers).
Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job or workflow.
Use --workflow flag to explicitly request workflow status.
Use --detail flag with workflow status to show the original YAML content.

Job Status Examples:
  # Get comprehensive job status (using full UUID)
  rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479
  
  # Get job status (using short-form UUID)
  rnx job status f47ac10b
  
  # Get job status in JSON format (all fields)
  rnx job status --json f47ac10b

Workflow Status Examples:
  # Get workflow status (using full UUID)
  rnx job status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef
  
  # Get workflow status (using short-form UUID)
  rnx job status --workflow a1b2c3d4
  
  # Get workflow status with original YAML content
  rnx job status --workflow --detail a1b2c3d4
  
  # Get workflow status in JSON format
  rnx job status --workflow --json a1b2c3d4
  rnx job status --workflow --json --detail a1b2c3d4  # JSON with YAML content

Job Status Information Displayed:
  • Basic Info: Job UUID, name, command with arguments, current status
  • Timing: Creation time, start time, end time, execution duration
  • Resource Limits: CPU percentage, memory MB, I/O bandwidth, CPU cores, GPUs
  • Runtime Environment: Python, Java, Node.js runtime specifications
  • Network: Network configuration (bridge, isolated, custom networks)
  • Storage: Mounted volumes (filesystem and memory-based)
  • Working Directory: Job execution directory path
  • Uploaded Files: List of files uploaded for job execution
  • Environment: Regular environment variables (visible in logs)
  • Secrets: Secret environment variables (masked as ***)
  • Workflow Context: Workflow UUID and job dependencies (if applicable)
  • Results: Exit code and completion status
  • Actions: Contextual next steps (view logs, stop job, etc.)

Output Formats:
  • Default: Human-readable formatted output with sections
  • --json: Machine-readable JSON with all available fields`,
		Args: cobra.ExactArgs(1),
		RunE: runStatus,
	}

	cmd.Flags().BoolVarP(&workflowFlag, "workflow", "w", false, "Get workflow status instead of job status")
	cmd.Flags().BoolVarP(&detailFlag, "detail", "d", false, "Show YAML content when displaying workflow status")

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Validate flag combinations
	if detailFlag && !workflowFlag {
		return fmt.Errorf("the --detail option only works with --workflow")
	}

	// If workflow flag is set, try workflow status directly
	if workflowFlag {
		// Workflow UUID can be passed directly
		return getWorkflowStatus(id)
	}

	// Try job ID first (for backward compatibility)
	jobErr := getJobStatus(id)
	if jobErr == nil {
		return nil
	}

	// If job lookup fails, try as workflow UUID
	workflowErr := getWorkflowStatus(id)
	if workflowErr == nil {
		return nil
	}

	// If both fail, show a helpful error message
	if strings.Contains(jobErr.Error(), "not found") && strings.Contains(workflowErr.Error(), "not found") {
		return fmt.Errorf("couldn't find a job or workflow with ID '%s'\n\nTip: Try 'rnx job list' to see all jobs, or 'rnx job list --workflow' to see workflows", id)
	}

	// If workflow exists but job also exists with same ID, suggest using --workflow flag
	if !strings.Contains(workflowErr.Error(), "not found") {
		fmt.Fprintf(os.Stderr, "\nNote: Both job and workflow exist with ID '%s'. Showing job status.\nUse 'rnx job status --workflow %s' to see workflow status.\n\n", id, id)
		return nil
	}

	return workflowErr
}

func getJobStatus(jobID string) error {
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.GetJobStatus(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't get job status: %v", err)
	}

	if common.JSONOutput {
		return outputJobStatusJSON(response)
	}

	// Display basic job information
	fmt.Printf("Job ID: %s\n", response.Uuid)
	if response.Name != "" {
		fmt.Printf("Job Name: %s\n", response.Name)
	}
	fmt.Printf("Command: %s %s\n", response.Command, strings.Join(response.Args, " "))

	// Display status with color coding (if terminal supports it)
	statusColor, resetColor := getStatusColor(response.Status)
	fmt.Printf("Status: %s%s%s\n", statusColor, response.Status, resetColor)

	// Display scheduling information if available
	if response.ScheduledTime != "" {
		fmt.Printf("\nScheduling Information:\n")

		scheduledTime, err := time.Parse("2006-01-02T15:04:05Z07:00", response.ScheduledTime)
		if err == nil {
			fmt.Printf("  Scheduled Time: %s\n", scheduledTime.Format("2006-01-02 15:04:05 MST"))

			// Show time until execution for scheduled jobs
			if response.Status == "SCHEDULED" {
				now := time.Now()
				if scheduledTime.After(now) {
					duration := scheduledTime.Sub(now)
					fmt.Printf("  Time Until Execution: %s\n", formatDuration(duration))
				} else {
					fmt.Printf("  Time Until Execution: Due now (waiting for execution)\n")
				}
			}
		} else {
			fmt.Printf("  Scheduled Time: %s\n", response.ScheduledTime)
		}
	}

	// Display timing information
	fmt.Printf("\nTiming:\n")
	fmt.Printf("  Created: %s\n", formatTimestamp(response.StartTime))

	if response.EndTime != "" {
		fmt.Printf("  Ended: %s\n", formatTimestamp(response.EndTime))

		// Calculate duration for completed jobs
		startTime, startErr := time.Parse("2006-01-02T15:04:05Z07:00", response.StartTime)
		endTime, endErr := time.Parse("2006-01-02T15:04:05Z07:00", response.EndTime)
		if startErr == nil && endErr == nil {
			duration := endTime.Sub(startTime)
			fmt.Printf("  Duration: %s\n", formatDuration(duration))
		}
	} else if response.Status == "RUNNING" {
		startTime, err := time.Parse("2006-01-02T15:04:05Z07:00", response.StartTime)
		if err == nil {
			duration := time.Since(startTime)
			fmt.Printf("  Running For: %s\n", formatDuration(duration))
		}
	}

	// Display resource limits (only show non-default/requested limits)
	hasResourceLimits := false
	resourceLimits := []string{}

	if response.MaxCPU > 0 {
		resourceLimits = append(resourceLimits, fmt.Sprintf("  Max CPU: %d%%", response.MaxCPU))
		hasResourceLimits = true
	}
	if response.MaxMemory > 0 {
		resourceLimits = append(resourceLimits, fmt.Sprintf("  Max Memory: %d MB", response.MaxMemory))
		hasResourceLimits = true
	}
	if response.MaxIOBPS > 0 {
		resourceLimits = append(resourceLimits, fmt.Sprintf("  Max IO BPS: %d", response.MaxIOBPS))
		hasResourceLimits = true
	}
	if response.CpuCores != "" {
		resourceLimits = append(resourceLimits, fmt.Sprintf("  CPU Cores: %s", response.CpuCores))
		hasResourceLimits = true
	}

	// Add GPU resource information if available
	if response.GpuCount > 0 {
		if len(response.GpuIndices) > 0 {
			// Show allocated GPUs
			gpuIndicesStr := make([]string, len(response.GpuIndices))
			for i, idx := range response.GpuIndices {
				gpuIndicesStr[i] = fmt.Sprintf("%d", idx)
			}
			resourceLimits = append(resourceLimits, fmt.Sprintf("  GPUs: %d allocated (indices: %s)",
				response.GpuCount, strings.Join(gpuIndicesStr, ", ")))
		} else {
			// Show requested but not yet allocated
			resourceLimits = append(resourceLimits, fmt.Sprintf("  GPUs: %d requested", response.GpuCount))
		}
		if response.GpuMemoryMb > 0 {
			resourceLimits = append(resourceLimits, fmt.Sprintf("  GPU Memory: %d MB", response.GpuMemoryMb))
		}
		hasResourceLimits = true
	}

	if hasResourceLimits {
		fmt.Printf("\nResource Limits:\n")
		for _, limit := range resourceLimits {
			fmt.Printf("%s\n", limit)
		}
	}

	// Display runtime information
	if response.Runtime != "" {
		fmt.Printf("\nRuntime Environment:\n")
		fmt.Printf("  Runtime: %s\n", response.Runtime)
	}

	// Display network configuration
	if response.Network != "" {
		fmt.Printf("\nNetwork Configuration:\n")
		fmt.Printf("  Network: %s\n", response.Network)
	}

	// Display volume information
	if len(response.Volumes) > 0 {
		fmt.Printf("\nVolumes:\n")
		for _, volume := range response.Volumes {
			fmt.Printf("  - %s\n", volume)
		}
	}

	// Display working directory
	if response.WorkDir != "" {
		fmt.Printf("\nWorking Directory:\n")
		fmt.Printf("  Work Dir: %s\n", response.WorkDir)
	}

	// Display uploaded files
	if len(response.Uploads) > 0 {
		fmt.Printf("\nUploaded Files:\n")
		for _, upload := range response.Uploads {
			fmt.Printf("  - %s\n", upload)
		}
	}

	// Display workflow information
	if response.WorkflowUuid != "" {
		fmt.Printf("\nWorkflow Information:\n")
		fmt.Printf("  Workflow UUID: %s\n", response.WorkflowUuid)

		// Display dependencies if available
		if len(response.Dependencies) > 0 {
			fmt.Printf("  Dependencies:\n")
			for _, dep := range response.Dependencies {
				fmt.Printf("    - %s\n", dep)
			}
		}
	}

	// Display environment variables (if any)
	hasEnvVars := len(response.Environment) > 0 || len(response.SecretEnvironment) > 0
	if hasEnvVars {
		fmt.Printf("\nEnvironment Variables:\n")

		// Display regular environment variables
		for key, value := range response.Environment {
			fmt.Printf("  %s=%s\n", key, value)
		}

		// Display secret environment variables (masked)
		for key, maskedValue := range response.SecretEnvironment {
			fmt.Printf("  %s=%s (secret)\n", key, maskedValue)
		}
	}

	// Display exit code for completed jobs
	if response.Status != "RUNNING" && response.Status != "SCHEDULED" && response.Status != "INITIALIZING" {
		fmt.Printf("\nResult:\n")
		fmt.Printf("  Exit Code: %d\n", response.ExitCode)
	}

	// Provide helpful next steps based on job status
	fmt.Printf("\nAvailable Actions:\n")
	switch response.Status {
	case "SCHEDULED":
		fmt.Printf("  • rnx job stop %s     # Cancel scheduled job\n", response.Uuid)
		fmt.Printf("  • rnx job status %s   # Check status again\n", response.Uuid)
	case "RUNNING":
		fmt.Printf("  • rnx job log %s      # Stream live logs\n", response.Uuid)
		fmt.Printf("  • rnx job stop %s     # Stop running job\n", response.Uuid)
	case "COMPLETED", "FAILED", "STOPPED":
		fmt.Printf("  • rnx job log %s      # View job logs\n", response.Uuid)
	default:
		fmt.Printf("  • rnx job log %s      # View job logs\n", response.Uuid)
		fmt.Printf("  • rnx job stop %s     # Stop job if running\n", response.Uuid)
	}

	return nil
}

// formatTimestamp formats a timestamp string for display
func formatTimestamp(timestamp string) string {
	if timestamp == "" {
		return "Not available"
	}

	t, err := time.Parse("2006-01-02T15:04:05Z07:00", timestamp)
	if err != nil {
		return timestamp // Return as-is if parsing fails
	}

	return t.Format("2006-01-02 15:04:05 MST")
}

// formatDuration formats a duration for human-readable display

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

// outputJobStatusJSON outputs the job status in JSON format
func outputJobStatusJSON(response *pb.GetJobStatusRes) error {
	// Create a structured output that includes all fields, even when empty
	output := map[string]interface{}{
		"uuid":              response.Uuid,
		"name":              response.Name,
		"command":           response.Command,
		"args":              response.Args,
		"status":            response.Status,
		"startTime":         response.StartTime,
		"endTime":           response.EndTime,
		"exitCode":          response.ExitCode,
		"scheduledTime":     response.ScheduledTime,
		"environment":       response.Environment,
		"secretEnvironment": response.SecretEnvironment,
		"network":           response.Network,
		"volumes":           response.Volumes,
		"runtime":           response.Runtime,
		"workDir":           response.WorkDir,
		"uploads":           response.Uploads,
		"dependencies":      response.Dependencies,
		"workflowUuid":      response.WorkflowUuid,
	}

	// Include resource limits if set
	if response.MaxCPU > 0 {
		output["maxCPU"] = response.MaxCPU
	}
	if response.MaxMemory > 0 {
		output["maxMemory"] = response.MaxMemory
	}
	if response.MaxIOBPS > 0 {
		output["maxIOBPS"] = response.MaxIOBPS
	}
	if response.CpuCores != "" {
		output["cpuCores"] = response.CpuCores
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// getWorkflowStatus retrieves and displays comprehensive workflow status with job names.
//
// RESPONSIBILITY:
// - Fetches detailed workflow status from the joblet server
// - Formats and displays workflow information with job names and dependencies
// - Provides both tabular and JSON output formats for different use cases
// - Integrates job names feature to show human-readable job identifiers
//
// JOB NAMES DISPLAY:
// - JOB ID column: Shows actual job IDs (e.g., "42", "43") for started jobs
// - JOB NAME column: Shows human-readable names from workflow YAML (e.g., "setup-data")
// - DEPENDENCIES column: Lists job name dependencies for clarity
// - Properly handles jobs that haven't been started (show job names in ID column)
//
// OUTPUT FORMATS:
// 1. Tabular format (default):
//   - Workflow summary (ID, status, progress, timing)
//   - Job table with columns: JOB ID, JOB NAME, STATUS, EXIT CODE, DEPENDENCIES
//   - Color-coded status indicators
//   - Helpful action suggestions based on workflow state
//
// 2. JSON format (--json flag):
//   - Complete workflow metadata
//   - Detailed job information including dependencies and timing
//   - Machine-readable format for scripting and automation
//
// WORKFLOW:
// 1. Creates gRPC client connection to joblet server
// 2. Sends GetWorkflowStatus request with workflow ID
// 3. Processes response based on output format preference
// 4. Formats and displays comprehensive workflow status
// 5. Provides contextual next-step suggestions
//
// PARAMETERS:
// - workflowID: Unique numeric identifier for the workflow
//
// RETURNS:
// - error: If client creation fails, request fails, or formatting errors occur
func getWorkflowStatus(workflowID string) error {
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer client.Close()

	// Create workflow service client
	workflowClient := pb.NewJobServiceClient(client.GetConn())

	req := &pb.GetWorkflowStatusRequest{
		WorkflowUuid: workflowID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := workflowClient.GetWorkflowStatus(ctx, req)
	if err != nil {
		return fmt.Errorf("couldn't get workflow status: %w", err)
	}

	if common.JSONOutput {
		return outputWorkflowStatusJSON(res)
	}

	workflow := res.Workflow

	// Display workflow summary
	fmt.Printf("Workflow UUID: %s\n", workflow.Uuid)
	fmt.Printf("Workflow: %s\n", workflow.Workflow)
	fmt.Printf("\n")

	// Display YAML content if detail flag is set
	if detailFlag {
		if workflow.YamlContent != "" {
			displayWorkflowYAMLContent(workflow.YamlContent)
		} else {
			fmt.Printf("Warning: YAML content not available from server\n\n")
		}
	}

	// Display status with color coding (if terminal supports it)
	statusColor, resetColor := getStatusColor(workflow.Status)
	fmt.Printf("Status: %s%s%s\n", statusColor, workflow.Status, resetColor)
	fmt.Printf("Progress: %d/%d jobs completed", workflow.CompletedJobs, workflow.TotalJobs)
	if workflow.FailedJobs > 0 {
		fmt.Printf(" (%d failed)", workflow.FailedJobs)
	}
	fmt.Printf("\n\n")

	// Display timing information
	fmt.Printf("Timing:\n")
	if workflow.CreatedAt != nil && workflow.CreatedAt.Seconds > 0 {
		createdTime := time.Unix(workflow.CreatedAt.Seconds, 0)
		fmt.Printf("  Created:   %s\n", createdTime.Format("2006-01-02 15:04:05 MST"))
	}
	if workflow.StartedAt != nil && workflow.StartedAt.Seconds > 0 {
		startedTime := time.Unix(workflow.StartedAt.Seconds, 0)
		fmt.Printf("  Started:   %s\n", startedTime.Format("2006-01-02 15:04:05 MST"))
	}
	if workflow.CompletedAt != nil && workflow.CompletedAt.Seconds > 0 {
		completedTime := time.Unix(workflow.CompletedAt.Seconds, 0)
		fmt.Printf("  Completed: %s\n", completedTime.Format("2006-01-02 15:04:05 MST"))
		// Calculate duration
		if workflow.StartedAt != nil && workflow.StartedAt.Seconds > 0 {
			startTime := time.Unix(workflow.StartedAt.Seconds, 0)
			duration := completedTime.Sub(startTime)
			fmt.Printf("  Duration:  %s\n", formatDuration(duration))
		}
	}
	fmt.Printf("\n")

	// Display jobs with detailed information
	if len(res.Jobs) > 0 {
		fmt.Printf("Jobs in Workflow:\n")
		fmt.Printf("-----------------------------------------------------------------------------------------------------------------------------\n")
		fmt.Printf("%-38s %-20s %-12s %-10s %-20s\n", "JOB ID", "JOB NAME", "STATUS", "EXIT CODE", "DEPENDENCIES")
		fmt.Printf("-----------------------------------------------------------------------------------------------------------------------------\n")

		for _, job := range res.Jobs {
			// Format status with color
			jobStatusColor, _ := getStatusColor(job.Status)

			exitCodeStr := "-"
			if job.ExitCode > 0 || job.Status == "COMPLETED" {
				exitCodeStr = fmt.Sprintf("%d", job.ExitCode)
			}

			deps := "-"
			if len(job.Dependencies) > 0 {
				deps = strings.Join(job.Dependencies, ", ")
				if len(deps) > 20 {
					deps = deps[:17] + "..."
				}
			}

			// Truncate job name if too long for display
			jobName := job.JobName
			if jobName == "" {
				jobName = "-"
			} else if len(jobName) > 20 {
				jobName = jobName[:17] + "..."
			}

			// Use full job UUID (no truncation needed with wider format)
			jobID := job.JobUuid

			fmt.Printf("%-38s %-20s %s%-12s%s %-10s %-20s\n",
				jobID,
				jobName,
				jobStatusColor, job.Status, resetColor,
				exitCodeStr,
				deps)

			// Show timing for completed/running jobs
			if job.StartTime != nil && job.StartTime.Seconds > 0 {
				startTime := time.Unix(job.StartTime.Seconds, 0)
				fmt.Printf("                                        Started: %s", startTime.Format("15:04:05"))
				if job.EndTime != nil && job.EndTime.Seconds > 0 {
					endTime := time.Unix(job.EndTime.Seconds, 0)
					duration := endTime.Sub(startTime)
					fmt.Printf("  Duration: %s", formatDuration(duration))
				}
				fmt.Printf("\n")
			}
		}
		fmt.Printf("\n")
	}

	// Show available actions
	fmt.Printf("\nAvailable Actions:\n")
	fmt.Printf("  • rnx job list --workflow    # List all workflows\n")
	if workflow.Status == "RUNNING" {
		fmt.Printf("  • rnx job status %s          # Refresh workflow status\n", workflow.Uuid)
	}
	for _, job := range res.Jobs {
		if job.Status == "COMPLETED" || job.Status == "FAILED" {
			fmt.Printf("  • rnx job log %s             # View logs for job %s\n", job.JobUuid, job.JobUuid)
			break
		}
	}

	return nil
}

// outputWorkflowStatusJSON outputs workflow status in JSON format
func outputWorkflowStatusJSON(res *pb.GetWorkflowStatusResponse) error {
	// Convert protobuf workflow status to JSON structure
	statusData := map[string]interface{}{
		"uuid":           res.Workflow.Uuid,
		"workflow":       res.Workflow.Workflow,
		"status":         res.Workflow.Status,
		"total_jobs":     res.Workflow.TotalJobs,
		"completed_jobs": res.Workflow.CompletedJobs,
		"failed_jobs":    res.Workflow.FailedJobs,
		"created_at":     res.Workflow.CreatedAt,
		"started_at":     res.Workflow.StartedAt,
		"completed_at":   res.Workflow.CompletedAt,
		"jobs":           make([]map[string]interface{}, 0, len(res.Jobs)),
	}

	// Include YAML content if detail flag is set and content is available
	if detailFlag && res.Workflow.YamlContent != "" {
		statusData["yaml_content"] = res.Workflow.YamlContent
	}

	// Add job details
	for _, job := range res.Jobs {
		jobData := map[string]interface{}{
			"id":     job.JobUuid,
			"name":   job.JobName,
			"status": job.Status,
		}
		if job.ExitCode != 0 {
			jobData["exit_code"] = job.ExitCode
		}
		if len(job.Dependencies) > 0 {
			jobData["dependencies"] = job.Dependencies
		}
		if job.StartTime != nil && job.StartTime.Seconds > 0 {
			jobData["start_time"] = job.StartTime
		}
		if job.EndTime != nil && job.EndTime.Seconds > 0 {
			jobData["end_time"] = job.EndTime
		}
		statusData["jobs"] = append(statusData["jobs"].([]map[string]interface{}), jobData)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(statusData)
}

// displayWorkflowYAMLContent displays YAML content directly from server
func displayWorkflowYAMLContent(yamlContent string) {
	fmt.Printf("YAML Content:\n")
	fmt.Printf("=============\n")
	fmt.Printf("%s\n", yamlContent)
}
