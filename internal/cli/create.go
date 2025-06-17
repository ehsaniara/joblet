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

Examples:
  cli create python3 script.py
  cli create bash -c "for i in {1..10}; do echo $i; done"
  cli create ping -c 5 google.com
  cli create ls -la /tmp
  cli create echo "hello world"
  
  # With resource limits
  cli create --max-cpu=50 python3 heavy_script.py
  cli create --max-memory=1024 java -jar app.jar
  cli create --max-cpu=25 --max-memory=512 bash -c "intensive command"
  
  # With network groups (jobs in same group share network namespace)
  cli create --network-group=web-services nginx -g "daemon off;"
  cli create --network-group=web-services curl http://localhost
  cli create --network-group=database redis-server
  
  # Isolated jobs (default - each gets own network namespace)
  cli create ip addr show
  cli create ip addr show

Resource limit flags (must appear before command):
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB
  --max-iobps=N       Max IO BPS
  --network-group=ID  Network group ID (jobs in same group share network namespace)

Note: Resource flags must come before the command. Everything after the first 
non-flag argument is treated as the command and its arguments.`,
		Args:               cobra.MinimumNArgs(1),
		RunE:               runCreate,
		DisableFlagParsing: true,
	}

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	var (
		maxCPU         int32
		maxMemory      int32
		maxIOBPS       int32
		networkGroupID string
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
		} else if strings.HasPrefix(arg, "--network-group=") {
			networkGroupID = strings.TrimPrefix(arg, "--network-group=")
			if networkGroupID == "" {
				return fmt.Errorf("network group ID cannot be empty")
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
	if len(commandArgs) == 0 {
		return fmt.Errorf("must specify a command")
	}

	command := commandArgs[0]
	cmdArgs := commandArgs[1:]
	displayCommand := strings.Join(commandArgs, " ")

	jobClient, err := client.NewJobClient(cfg.ServerAddr)
	if err != nil {
		return err
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	job := &pb.CreateJobReq{
		Command:        command,
		Args:           cmdArgs,
		MaxCPU:         maxCPU,
		MaxMemory:      maxMemory,
		MaxIOBPS:       maxIOBPS,
		NetworkGroupID: networkGroupID,
	}

	response, err := jobClient.CreateJob(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to create job: %v", err)
	}

	fmt.Printf("Job created:\n")
	fmt.Printf("ID: %s\n", response.Id)
	fmt.Printf("Command: %s\n", displayCommand)
	fmt.Printf("Status: %s\n", response.Status)
	fmt.Printf("StartTime: %s\n", response.StartTime)

	// Show network group info if specified
	if networkGroupID != "" {
		fmt.Printf("NetworkGroup: %s\n", networkGroupID)
	} else {
		fmt.Printf("NetworkGroup: isolated (default)\n")
	}

	return nil
}

func parseIntFlag(arg, prefix string) (int64, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.ParseInt(valueStr, 10, 32)
}
