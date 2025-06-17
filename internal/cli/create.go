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
  # Isolated jobs (each gets own network namespace)
  cli create nginx
  cli create mysql
  cli create python3 script.py
  
  # Network group (automatic unique IP assignment by server)
  cli create --network-group=web nginx
  cli create --network-group=web mysql  
  cli create --network-group=web redis
  
  # Jobs in same group can communicate via auto-assigned IPs
  cli create --network-group=web bash -c "curl http://nginx-service"
  
  # Different groups are completely isolated
  cli create --network-group=backend postgres
  cli create --network-group=cache redis

Flags:
  --max-cpu=N         Max CPU percentage
  --max-memory=N      Max Memory in MB  
  --max-iobps=N       Max IO BPS
  --network-group=ID  Network group (server auto-assigns unique IP)

The server automatically handles:
- Unique IP assignment within network groups
- Subnet allocation per group  
- Interface creation and configuration
- Network isolation between groups`,
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

	// Simple request - only basic info, server handles all IP logic
	job := &pb.CreateJobReq{
		Command:        command,
		Args:           cmdArgs,
		MaxCPU:         maxCPU,
		MaxMemory:      maxMemory,
		MaxIOBPS:       maxIOBPS,
		NetworkGroupID: networkGroupID, // Only network group, nothing else
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

	// Show what the server decided (if response includes it)
	if networkGroupID != "" {
		fmt.Printf("NetworkGroup: %s\n", networkGroupID)

		// If server returns assigned IP info, show it
		if response.AssignedIP != "" {
			fmt.Printf("AssignedIP: %s (auto-assigned by server)\n", response.AssignedIP)
		} else {
			fmt.Printf("IP: will be auto-assigned by server\n")
		}
	} else {
		fmt.Printf("Network: isolated (own namespace)\n")
	}

	return nil
}

func parseIntFlag(arg, prefix string) (int64, error) {
	valueStr := strings.TrimPrefix(arg, prefix)
	return strconv.ParseInt(valueStr, 10, 32)
}
