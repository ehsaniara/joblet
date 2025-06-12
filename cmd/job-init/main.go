package main

import (
	"os"
	"runtime"

	"job-worker/internal/jobinit"
	"job-worker/pkg/logger"
)

func main() {
	initLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			initLogger.SetLevel(level)
		}
	}

	jobID := os.Getenv("JOB_ID")
	if jobID != "" {
		initLogger = initLogger.WithField("jobId", jobID)
	}

	initLogger.Info("job-init process starting", "platform", runtime.GOOS)

	// Run the complete job initialization process
	// This will use the platform-specific implementation
	if err := jobinit.Run(); err != nil {
		initLogger.Error("job initialization failed", "error", err)
		os.Exit(1)
	}

	// Platform-specific completion handling
	switch runtime.GOOS {
	case "linux":
		// On Linux: This should never be reached since Run() calls syscall.Exec
		initLogger.Error("unexpected return from Run() - exec should have replaced process")
		os.Exit(1)
	case "darwin":
		// On macOS: This is expected since we use exec.Command.Run() instead of syscall.Exec
		initLogger.Info("job-init process completed successfully on macOS")
	default:
		initLogger.Info("job-init process completed", "platform", runtime.GOOS)
	}
}
