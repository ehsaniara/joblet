package rnx

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"time"
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <job-id>",
		Short: "Stop a running job",
		Args:  cobra.ExactArgs(1),
		RunE:  runStop,
	}

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	jobClient, err := newJobClient()
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
