package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
)

// NewListCmd creates a new cobra command for listing jobs.
// The command supports JSON output format via the --json flag.
// Lists all jobs with their basic information.
func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all jobs",
		Long: `List all jobs in the system.

Examples:
  # List all jobs
  rnx job list

  # List jobs in JSON format
  rnx job list --json

  # For workflows, use:
  rnx workflow list`,
		RunE: runList,
	}

	return cmd
}

// runList executes the job listing command.
// Connects to the Joblet server, retrieves all jobs, and displays them
// in either readable table format or JSON format based on flags.
func runList(cmd *cobra.Command, args []string) error {
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %v", err)
	}

	if len(response.Jobs) == 0 {
		if common.JSONOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No jobs found")
		}
		return nil
	}

	if common.JSONOutput {
		return outputJobsJSON(response.Jobs)
	}

	formatJobList(response.Jobs)

	return nil
}

func formatJobList(jobs []*pb.Job) {
	maxIDWidth := len("ID")
	maxNameWidth := len("NAME")
	maxNodeIDWidth := len("NODE ID")
	maxStatusWidth := len("STATUS")

	// find the maximum width needed for each column
	for _, job := range jobs {
		if len(job.Uuid) > maxIDWidth {
			maxIDWidth = len(job.Uuid)
		}
		jobName := job.Name
		if jobName == "" {
			jobName = "-"
		}
		if len(jobName) > maxNameWidth {
			maxNameWidth = len(jobName)
		}
		nodeId := job.NodeId
		if nodeId == "" {
			nodeId = "-"
		}
		if len(nodeId) > maxNodeIDWidth {
			maxNodeIDWidth = len(nodeId)
		}
		if len(job.Status) > maxStatusWidth {
			maxStatusWidth = len(job.Status)
		}
	}

	// some padding and limit max widths for readability
	// UUID width should accommodate full UUIDs (36 chars) plus padding
	maxIDWidth = min(maxIDWidth+2, 38) // Full UUID width
	maxNameWidth = min(maxNameWidth+2, 25)
	maxNodeIDWidth = min(maxNodeIDWidth+2, 38) // Node ID width (also UUID)
	maxStatusWidth += 2

	// header
	fmt.Printf("%-*s %-*s %-*s %-*s %-19s %s\n",
		maxIDWidth, "ID",
		maxNameWidth, "NAME",
		maxNodeIDWidth, "NODE ID",
		maxStatusWidth, "STATUS",
		"START TIME",
		"COMMAND")

	// separator line
	fmt.Printf("%s %s %s %s %s %s\n",
		strings.Repeat("-", maxIDWidth),
		strings.Repeat("-", maxNameWidth),
		strings.Repeat("-", maxNodeIDWidth),
		strings.Repeat("-", maxStatusWidth),
		strings.Repeat("-", 19), // length of "START TIME"
		strings.Repeat("-", 7))  // length of "COMMAND"

	// each job
	for _, job := range jobs {

		// For SCHEDULED jobs, show scheduled time; for others, show start time
		var displayTime string
		if job.Status == "SCHEDULED" && job.ScheduledTime != "" {
			displayTime = formatStartTime(job.ScheduledTime)
		} else {
			displayTime = formatStartTime(job.StartTime)
		}

		// truncate long commands
		command := formatCommand(job.Command, job.Args)

		// Format job name
		jobName := job.Name
		if jobName == "" {
			jobName = "-"
		}
		if len(jobName) > maxNameWidth-2 {
			jobName = jobName[:maxNameWidth-5] + "..."
		}

		// Format node ID
		nodeId := job.NodeId
		if nodeId == "" {
			nodeId = "-"
		}
		if len(nodeId) > maxNodeIDWidth-2 {
			nodeId = nodeId[:maxNodeIDWidth-5] + "..."
		}

		// Get status color
		statusColor, resetColor := getStatusColor(job.Status)

		fmt.Printf("%-*s %-*s %-*s %s%-*s%s %-19s %s\n",
			maxIDWidth, job.Uuid,
			maxNameWidth, jobName,
			maxNodeIDWidth, nodeId,
			statusColor, maxStatusWidth, job.Status, resetColor,
			displayTime,
			command)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatStartTime(timeStr string) string {
	if timeStr == "" {
		return "N/A"
	}

	// Parse the RFC3339 timestamp
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return timeStr
	}

	return t.Format("2006-01-02 15:04:05")
}

func formatCommand(command string, args []string) string {
	var fullCommand string
	if len(args) == 0 {
		fullCommand = command
	} else {
		fullCommand = command + " " + strings.Join(args, " ")
	}

	// Handle multiline commands - show only the first line
	if strings.Contains(fullCommand, "\n") {
		firstLine := strings.Split(fullCommand, "\n")[0]
		// Truncate first line if it's too long
		maxCommandLength := 80
		if len(firstLine) > maxCommandLength-3 {
			return firstLine[:maxCommandLength-6] + "..."
		}
		return firstLine + "..."
	}

	// Truncate very long single-line commands
	maxCommandLength := 80
	if len(fullCommand) > maxCommandLength {
		return fullCommand[:maxCommandLength-3] + "..."
	}

	return fullCommand
}

// outputJobsJSON outputs the jobs in JSON format
func outputJobsJSON(jobs []*pb.Job) error {
	// Convert protobuf jobs to a simpler structure for JSON output
	type jsonJob struct {
		ID            string   `json:"id"`
		Name          string   `json:"name,omitempty"`
		NodeID        string   `json:"node_id,omitempty"`
		Status        string   `json:"status"`
		StartTime     string   `json:"start_time"`
		EndTime       string   `json:"end_time,omitempty"`
		Command       string   `json:"command"`
		Args          []string `json:"args,omitempty"`
		ExitCode      int32    `json:"exit_code,omitempty"`
		MaxCPU        int32    `json:"max_cpu,omitempty"`
		MaxMemory     int32    `json:"max_memory,omitempty"`
		MaxIOBPS      int32    `json:"max_iobps,omitempty"`
		CPUCores      string   `json:"cpu_cores,omitempty"`
		ScheduledTime string   `json:"scheduled_time,omitempty"`
	}

	jsonJobs := make([]jsonJob, len(jobs))
	for i, job := range jobs {
		jsonJobs[i] = jsonJob{
			ID:            job.Uuid,
			Name:          job.Name,
			NodeID:        job.NodeId,
			Status:        job.Status,
			StartTime:     job.StartTime,
			EndTime:       job.EndTime,
			Command:       job.Command,
			Args:          job.Args,
			ExitCode:      job.ExitCode,
			MaxCPU:        job.MaxCPU,
			MaxMemory:     job.MaxMemory,
			MaxIOBPS:      job.MaxIOBPS,
			CPUCores:      job.CpuCores,
			ScheduledTime: job.ScheduledTime,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonJobs)
}
