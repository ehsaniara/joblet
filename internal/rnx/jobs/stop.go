package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
)

// NewStopCmd creates a new cobra command for stopping jobs.
// The command requires exactly one argument: the job UUID to stop.
// Sends a stop request to the Joblet server for the specified job.
func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <job-uuid>",
		Short: "Stop a running job",
		Long: `Stop a job that's currently running.

This will gracefully stop running jobs by sending SIGTERM, then SIGKILL if needed.
The job will be marked as STOPPED and you can safely delete it afterward.

For scheduled jobs that haven't started, use 'rnx job cancel' instead.

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

Examples:
  # Stop a running job (full UUID)
  rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479

  # Stop using short UUID
  rnx job stop f47ac10b

  # For scheduled jobs, use cancel instead
  rnx job cancel f47ac10b`,
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
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.StopJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't stop the job: %v", err)
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
