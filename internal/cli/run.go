package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pb "worker/api/gen"
)

var (
	runMaxCPU    int32
	runMaxMemory int32
	runMaxIOBPS  int32
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command> [args...]",
		Short: "Run a new job",
		Long: `Run a new job with the specified command and arguments.

Examples:
  worker-cli run nginx
  worker-cli run python3 script.py
  worker-cli run bash -c "curl https://example.com"
  worker-cli --node=srv1 run ps aux

Flags:
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS`,
		Args: cobra.MinimumNArgs(1),
		RunE: runRun,
		// Remove DisableFlagParsing to allow proper flag handling
	}

	// Add local flags for resource limits
	cmd.Flags().Int32Var(&runMaxCPU, "max-cpu", 0, "Maximum CPU percentage")
	cmd.Flags().Int32Var(&runMaxMemory, "max-memory", 0, "Maximum memory in MB")
	cmd.Flags().Int32Var(&runMaxIOBPS, "max-iobps", 0, "Maximum IO bytes per second")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	// args now contains just the command and its arguments
	command := args[0]
	cmdArgs := args[1:]

	// Create job client using the global configuration
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	job := &pb.RunJobReq{
		Command:   command,
		Args:      cmdArgs,
		MaxCPU:    runMaxCPU,
		MaxMemory: runMaxMemory,
		MaxIOBPS:  runMaxIOBPS,
	}

	response, err := jobClient.RunJob(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to run job: %v", err)
	}

	fmt.Printf("Job started:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Command: %s %s\n", command, strings.Join(cmdArgs, " "))
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("StartTime: %s\n", response.StartTime)
	if runMaxCPU > 0 {
		fmt.Printf("Max CPU: %d%%\n", runMaxCPU)
	}
	if runMaxMemory > 0 {
		fmt.Printf("Max Memory: %d MB\n", runMaxMemory)
	}
	if runMaxIOBPS > 0 {
		fmt.Printf("Max IO: %d BPS\n", runMaxIOBPS)
	}

	return nil
}
