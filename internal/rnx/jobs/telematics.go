package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
)

// ANSI color codes for telematics output
const (
	colorCyan    = "\033[36m"
	colorYellow  = "\033[33m"
	colorGreen   = "\033[32m"
	colorMagenta = "\033[35m"
	colorRed     = "\033[31m"
	colorBlue    = "\033[34m"
	colorReset   = "\033[0m"
)

func NewTelematicsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telematics <job-uuid>",
		Short: "View eBPF security telematics events for a job",
		Long: `View eBPF security telematics events for a running or completed job.

This command shows security-relevant events captured by eBPF tracing:
  - EXEC: Process executions (fork/exec syscalls)
  - CONNECT: Outgoing network connections
  - ACCEPT: Incoming network connections
  - FILE: File operations (open, read, write)
  - MMAP: Memory mappings with executable permissions
  - MPROTECT: Memory protection changes
  - SOCKET_DATA: Socket data transfers

For COMPLETED jobs: Shows all events from start to finish, then exits
For RUNNING jobs: Shows all events from start to current, then continues
                  streaming live updates until job completes

Short-form UUIDs are supported - you can use just the first 8 characters
if they uniquely identify a job.

Examples:
  # View all telematics events
  rnx job telematics f47ac10b

  # Filter specific event types
  rnx job telematics f47ac10b --types exec,connect

  # Output as JSON (one event per line)
  rnx --json job telematics f47ac10b

  # Filter with grep
  rnx job telematics f47ac10b | grep EXEC
  rnx job telematics f47ac10b | grep CONNECT`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTelematics(cmd, args)
		},
	}

	cmd.Flags().StringSlice("types", nil, "Filter event types (exec,connect,accept,file,mmap,mprotect,socket_data)")

	return cmd
}

func runTelematics(cmd *cobra.Command, args []string) error {
	jobID := args[0]
	types, _ := cmd.Flags().GetStringSlice("types")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nStopping telematics stream...")
		cancel()
	}()

	eventCount := 0

	// Connect to joblet server
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	// Use StreamJobTelematics for live telematics events
	stream, err := jobClient.StreamJobTelematics(ctx, jobID, types)
	if err != nil {
		return fmt.Errorf("couldn't start reading telematics events: %v", err)
	}

	if !common.JSONOutput {
		if len(types) > 0 {
			fmt.Fprintf(os.Stderr, "Streaming telematics events [%s] (Ctrl+C to stop)...\n\n", strings.Join(types, ","))
		} else {
			fmt.Fprintf(os.Stderr, "Streaming telematics events (Ctrl+C to stop)...\n\n")
		}
	}

	for {
		event, e := stream.Recv()
		if e == io.EOF {
			if eventCount == 0 {
				return fmt.Errorf("no telematics events for job %s (eBPF tracing may not be enabled)", jobID)
			}
			return nil
		}
		if e != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			if s, ok := status.FromError(e); ok {
				return fmt.Errorf("problem reading telematics events: %v", s.Message())
			}

			return fmt.Errorf("error receiving telematics stream: %v", e)
		}

		eventCount++

		if common.JSONOutput {
			if err := outputTelematicsJSON(event); err != nil {
				return fmt.Errorf("couldn't format output as JSON: %v", err)
			}
		} else {
			outputTelematicsEventHuman(event)
		}
	}
}

// outputTelematicsJSON outputs a telematics event as JSON
func outputTelematicsJSON(event *pb.TelematicsEvent) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(event)
}

// outputTelematicsEventHuman outputs a telematics event in human-readable format
func outputTelematicsEventHuman(event *pb.TelematicsEvent) {
	timestamp := time.Unix(0, event.Timestamp).Format("15:04:05.000")

	switch event.Type {
	case "exec":
		if exec := event.GetExec(); exec != nil {
			argsStr := ""
			if len(exec.Args) > 0 {
				argsStr = " " + strings.Join(exec.Args, " ")
			}
			fmt.Printf("[%s] %sEXEC%s     pid=%d ppid=%d %s%s\n",
				timestamp,
				colorCyan,
				colorReset,
				exec.Pid,
				exec.Ppid,
				exec.Binary,
				argsStr,
			)
		}

	case "connect":
		if conn := event.GetConnect(); conn != nil {
			fmt.Printf("[%s] %sCONNECT%s  pid=%d %s:%d -> %s:%d (%s)\n",
				timestamp,
				colorYellow,
				colorReset,
				conn.Pid,
				conn.SrcAddr,
				conn.SrcPort,
				conn.DstAddr,
				conn.DstPort,
				conn.Protocol,
			)
		}

	case "accept":
		if acc := event.GetAccept(); acc != nil {
			fmt.Printf("[%s] %sACCEPT%s   pid=%d %s:%d <- %s:%d (%s)\n",
				timestamp,
				colorGreen,
				colorReset,
				acc.Pid,
				acc.DstAddr,
				acc.DstPort,
				acc.SrcAddr,
				acc.SrcPort,
				acc.Protocol,
			)
		}

	case "file":
		if file := event.GetFile(); file != nil {
			fmt.Printf("[%s] %sFILE%s     pid=%d %s %s (%d bytes)\n",
				timestamp,
				colorBlue,
				colorReset,
				file.Pid,
				file.Operation,
				file.Path,
				file.Bytes,
			)
		}

	case "mmap":
		if mmap := event.GetMmap(); mmap != nil {
			fmt.Printf("[%s] %sMMAP%s     pid=%d addr=0x%x len=%d prot=%d\n",
				timestamp,
				colorMagenta,
				colorReset,
				mmap.Pid,
				mmap.Addr,
				mmap.Length,
				mmap.Prot,
			)
		}

	case "mprotect":
		if mprot := event.GetMprotect(); mprot != nil {
			fmt.Printf("[%s] %sMPROTECT%s pid=%d addr=0x%x len=%d prot=%d\n",
				timestamp,
				colorRed,
				colorReset,
				mprot.Pid,
				mprot.Addr,
				mprot.Length,
				mprot.Prot,
			)
		}

	case "socket_data":
		if sock := event.GetSocketData(); sock != nil {
			direction := "SEND"
			if sock.Direction == "recv" {
				direction = "RECV"
			}
			fmt.Printf("[%s] %s%s%s     pid=%d %s:%d (%d bytes)\n",
				timestamp,
				colorYellow,
				direction,
				colorReset,
				sock.Pid,
				sock.DstAddr,
				sock.DstPort,
				sock.Bytes,
			)
		}
	}
}
