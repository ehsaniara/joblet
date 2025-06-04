package main

import (
	"os"

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

	initLogger.Info("job-init process starting")

	// Run the complete job initialization process
	if err := jobinit.Run(); err != nil {
		initLogger.Error("job initialization failed", "error", err)
		os.Exit(1)
	}

	// This should never be reached since Run() calls exec
	initLogger.Error("unexpected return from Run() - exec should have replaced process")
	os.Exit(1)
}
