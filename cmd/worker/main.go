package main

import (
	"context"
	"job-worker/internal/config"
	"job-worker/internal/worker"
	"job-worker/internal/worker/store"
	"job-worker/pkg/logger"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	appLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			appLogger.SetLevel(level)
			appLogger.Info("log level set from environment", "level", level)
		} else {
			appLogger.Warn("invalid log level in environment, using INFO", "logLevel", logLevel, "error", err)
		}
	}

	appLogger.Info("job-worker starting", "version", "1.0.0")

	if _, err := os.Stat(config.CgroupsBaseDir); os.IsNotExist(err) {
		appLogger.Fatal("cgroups not available", "cgroupsPath", config.CgroupsBaseDir)
	}
	appLogger.Info("cgroups available", "cgroupsPath", config.CgroupsBaseDir)

	appLogger.Debug("goroutine monitoring started")

	s := store.New()
	_ = worker.New(s)
	appLogger.Info("worker and store initialized")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	select {

	case sig := <-sigChan:
		appLogger.Info("received shutdown signal", "signal", sig)
		cancel()
		return
	case <-ctx.Done():
		cancel()
	}

	<-ctx.Done()
}
