package rnx

import (
	"context"
	"encoding/json"
	"fmt"
	pb "joblet/api/gen"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	listJSON bool
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all jobs",
		RunE:  runList,
	}

	cmd.Flags().BoolVar(&listJSON, "json", false, "Output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {

	jobClient, err := newJobClient()
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
		if listJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No jobs found")
		}
		return nil
	}

	if listJSON {
		return outputJobsJSON(response.Jobs)
	}

	formatJobList(response.Jobs)

	return nil
}

func formatJobList(jobs []*pb.Job) {
	maxIDWidth := len("ID")
	maxStatusWidth := len("STATUS")

	// find the maximum width needed for each column
	for _, job := range jobs {
		if len(job.Id) > maxIDWidth {
			maxIDWidth = len(job.Id)
		}
		if len(job.Status) > maxStatusWidth {
			maxStatusWidth = len(job.Status)
		}
	}

	// some padding
	maxIDWidth += 2
	maxStatusWidth += 2

	// header
	fmt.Printf("%-*s %-*s %-19s %s\n",
		maxIDWidth, "ID",
		maxStatusWidth, "STATUS",
		"START TIME",
		"COMMAND")

	// separator line
	fmt.Printf("%s %s %s %s\n",
		strings.Repeat("-", maxIDWidth),
		strings.Repeat("-", maxStatusWidth),
		strings.Repeat("-", 19), // length of "START TIME"
		strings.Repeat("-", 7))  // length of "COMMAND"

	// each job
	for _, job := range jobs {

		startTime := formatStartTime(job.StartTime)

		// truncate long commands
		command := formatCommand(job.Command, job.Args)

		fmt.Printf("%-*s %-*s %-19s %s\n",
			maxIDWidth, job.Id,
			maxStatusWidth, job.Status,
			startTime,
			command)
	}
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
	if len(args) == 0 {
		return command
	}

	fullCommand := command + " " + strings.Join(args, " ")

	// truncate very long commands
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
			ID:            job.Id,
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
