package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/rnx/common"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// NewStopCmd creates a new cobra command for stopping jobs.
// The command requires exactly one argument: the job UUID to stop.
// Sends a stop request to the Joblet server for the specified job.
func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <job-uuid>",
		Short: "Stop a running job",
		Long: `Stop a running or scheduled job immediately.

This command sends a termination signal to the specified job. For running jobs,
it will attempt a graceful shutdown first, then force termination if needed.
For scheduled jobs, it will cancel the scheduled execution.

Examples:
  # Stop a running job
  rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479
  
  # Cancel a scheduled job before it starts
  rnx job stop a1b2c3d4-5678-90ab-cdef-1234567890ab`,
		Args: cobra.ExactArgs(1),
		RunE: runStop,
	}

	return cmd
}

// runStop executes the job stop command.
// Takes the job ID from command arguments, connects to the server,
// and sends a stop request. Displays confirmation upon success.
func runStop(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.StopJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to stop job: %v", err)
	}

	if common.JSONOutput {
		return outputStopJobJSON(response)
	}

	fmt.Printf("Job stopped successfully:\n")
	fmt.Printf("ID: %s\n", response.Uuid)
	// Display status with color coding
	statusColor, resetColor := getStatusColor(response.Status)
	fmt.Printf("Status: %s%s%s\n", statusColor, response.Status, resetColor)

	return nil
}

// outputStopJobJSON outputs the stop job result in JSON format
func outputStopJobJSON(response *pb.StopJobRes) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
