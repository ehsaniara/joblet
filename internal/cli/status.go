package cli

import (
	"context"
	"fmt"
	"job-worker/pkg/client"
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

	jobClient, err := client.NewJobClient(cfg.ServerAddr)
	if err != nil {
		return err
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.GetJobStatus(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job status: %v", err)
	}

	fmt.Printf("Id: %s\n", response.Id)
	fmt.Printf("Command: %s %s\n", response.Command, strings.Join(response.Args, " "))
	if response.Status != "RUNNING" {
		fmt.Printf("ExitCode: %d\n", response.ExitCode)
	}
	fmt.Printf("Started At: %s\n", response.StartTime)
	fmt.Printf("Ended At: %s\n", response.EndTime)
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("MaxCPU: %d\n", response.MaxCPU)
	fmt.Printf("MaxMemory: %d\n", response.MaxMemory)
	fmt.Printf("MaxIOBPS: %d\n", response.MaxIOBPS)

	return nil
}
