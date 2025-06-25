package cli

import (
	"context"
	"fmt"
	"strings"
	"time"
	"worker/pkg/client"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all jobs",
		RunE:  runList,
	}

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	jobClient, err := client.NewJobClient(cfg.ServerAddr)
	if err != nil {
		return err
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := jobClient.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %v", err)
	}

	if len(response.Jobs) == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	for _, job := range response.Jobs {
		fmt.Printf("%s %s StartTime: %s Command: %s %s\n",
			job.Id, job.Status, job.StartTime, job.Command, strings.Join(job.Args, " "))
	}

	return nil
}
