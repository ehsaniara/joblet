package rnx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Get the status of a job by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.GetJobStatus(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job status: %v", err)
	}

	// Display basic job information
	fmt.Printf("Job ID: %s\n", response.Id)
	fmt.Printf("Command: %s %s\n", response.Command, strings.Join(response.Args, " "))
	fmt.Printf("Status: %s\n", response.Status)

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

	// Display resource limits
	fmt.Printf("\nResource Limits:\n")
	fmt.Printf("  Max CPU: %d%%\n", response.MaxCPU)
	fmt.Printf("  Max Memory: %d MB\n", response.MaxMemory)
	fmt.Printf("  Max IO BPS: %d\n", response.MaxIOBPS)
	if response.CpuCores != "" {
		fmt.Printf("  CPU Cores: %s\n", response.CpuCores)
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
		fmt.Printf("  • rnx stop %s     # Cancel scheduled job\n", response.Id)
		fmt.Printf("  • rnx status %s   # Check status again\n", response.Id)
	case "RUNNING":
		fmt.Printf("  • rnx log %s      # Stream live logs\n", response.Id)
		fmt.Printf("  • rnx stop %s     # Stop running job\n", response.Id)
	case "COMPLETED", "FAILED", "STOPPED":
		fmt.Printf("  • rnx log %s      # View job logs\n", response.Id)
	default:
		fmt.Printf("  • rnx log %s      # View job logs\n", response.Id)
		fmt.Printf("  • rnx stop %s     # Stop job if running\n", response.Id)
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
