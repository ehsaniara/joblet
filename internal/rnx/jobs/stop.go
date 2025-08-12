package jobs

import (
	"context"
	"fmt"
	"joblet/internal/rnx/common"
	"time"

	"github.com/spf13/cobra"
)

// NewStopCmd creates a new cobra command for stopping jobs.
// The command requires exactly one argument: the job ID to stop.
// Sends a stop request to the Joblet server for the specified job.
func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <job-id>",
		Short: "Stop a running job",
		Args:  cobra.ExactArgs(1),
		RunE:  runStop,
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

	fmt.Printf("Job stopped successfully:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Status: %s\n", response.Status)

	return nil
}
