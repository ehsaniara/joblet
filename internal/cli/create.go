package cli

import (
	"context"
	"fmt"
	"job-worker/pkg/client"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	pb "job-worker/api/gen"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <command> [args...]",
		Short: "Create a new job",
		Long: `Create a new job with the specified command and arguments.

All jobs run with host networking (no isolation).

Examples:
  cli create nginx
  cli create mysql
  cli create python3 script.py
  cli create bash -c "curl http://example.com"

Flags:
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS

All jobs share the host network interface and can communicate
with each other and external services directly.`,
		Args:               cobra.MinimumNArgs(1),
		RunE:               runCreate,
		DisableFlagParsing: true,
	}

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	var (
		maxCPU    int32
		maxMemory int32
		maxIOBPS  int32
	)

	commandStartIndex := 0
	for i, arg := range args {
		if strings.HasPrefix(arg, "--max-cpu=") {
			if val, err := parseIntFlag(arg, "--max-cpu="); err == nil {
				maxCPU = int32(val)
			}
		} else if strings.HasPrefix(arg, "--max-memory=") {
			if val, err := parseIntFlag(arg, "--max-memory="); err == nil {
				maxMemory = int32(val)
			}
		} else if strings.HasPrefix(arg, "--max-iobps=") {
			if val, err := parseIntFlag(arg, "--max-iobps="); err == nil {
				maxIOBPS = int32(val)
			}
		} else if !strings.HasPrefix(arg, "--") {
			commandStartIndex = i
			break
		} else {
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}

	if commandStartIndex >= len(args) {
		return fmt.Errorf("must specify a command")
	}

	commandArgs := args[commandStartIndex:]
	command := commandArgs[0]
	cmdArgs := commandArgs[1:]

	jobClient, err := client.NewJobClient(cfg.ServerAddr)
	if err != nil {
		return err
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	job := &pb.CreateJobReq{
		Command:   command,
		Args:      cmdArgs,
		MaxCPU:    maxCPU,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
	}

	response, err := jobClient.CreateJob(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to create job: %v", err)
	}

	fmt.Printf("Job created:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Command: %s\n", strings.Join(commandArgs, " "))
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("StartTime: %s\n", response.StartTime)
	fmt.Printf("Network: host (shared with system)\n")

	return nil
}

func parseIntFlag(arg, prefix string) (int64, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.ParseInt(valueStr, 10, 32)
}
