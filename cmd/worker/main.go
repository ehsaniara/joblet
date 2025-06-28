package main

import (
	"fmt"
	"os"
	"path/filepath"

	"worker/internal/modes"
)

func main() {
	// Determine execution mode based on process name or environment
	mode := determineExecutionMode()

	switch mode {
	case modes.ServerMode:
		modes.RunServer()
	case modes.InitMode:
		modes.RunJobInit()
	default:
		fmt.Fprintf(os.Stderr, "Unknown execution mode: %s\n", mode)
		os.Exit(1)
	}
}

// determineExecutionMode figures out which stage should run
func determineExecutionMode() modes.ExecutionMode {
	// Method 1: Check if we're running as job-init (symlink or renamed binary)
	execName := filepath.Base(os.Args[0])
	if execName == "job-init" {
		return modes.InitMode
	}

	// Method 2: Check environment variable (most reliable for our use case)
	if os.Getenv("WORKER_MODE") == "init" {
		return modes.InitMode
	}

	// Method 3: Check if we have job-specific environment variables
	if os.Getenv("JOB_ID") != "" && os.Getenv("JOB_COMMAND") != "" {
		return modes.InitMode
	}

	// Default to server mode
	return modes.ServerMode
}
