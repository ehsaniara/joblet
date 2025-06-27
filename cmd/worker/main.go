package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	"worker/internal/worker"
	"worker/internal/worker/core/linux/resource"
	"worker/internal/worker/server"
	"worker/internal/worker/state"
	"worker/pkg/logger"
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

	appLogger.Info("job-worker starting", "version", "1.0.0", "platform", runtime.GOOS)

	// Platform-specific initialization
	if err := validatePlatformRequirements(appLogger); err != nil {
		appLogger.Fatal("platform requirements not met", "error", err)
	}

	appLogger.Debug("goroutine monitoring started")

	s := state.New()
	w := worker.NewWorker(s)
	appLogger.Info("worker and store initialized", "platform", runtime.GOOS)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	var grpcServer *grpc.Server
	serverStarted := make(chan error, 1)

	go func() {
		appLogger.Info("starting JobService gRPC server", "address", ":50051")
		var err error
		grpcServer, err = server.StartGRPCServer(s, w)
		serverStarted <- err
	}()

	select {
	case err := <-serverStarted:
		if err != nil {
			appLogger.Fatal("failed to start server", "error", err)
		}
		appLogger.Info("gRPC server started successfully")

	case sig := <-sigChan:
		appLogger.Info("received shutdown signal", "signal", sig)
		cancel()
		return
	}

	go func() {
		sig := <-sigChan
		appLogger.Info("received shutdown signal, initiating graceful shutdown", "signal", sig)
		cancel()
	}()

	<-ctx.Done()

	appLogger.Info("starting graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if grpcServer != nil {
		appLogger.Info("stopping gRPC server")

		serverStopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(serverStopped)
		}()

		select {
		case <-serverStopped:
			appLogger.Info("gRPC server stopped gracefully")
		case <-shutdownCtx.Done():
			appLogger.Warn("gRPC server shutdown timeout, forcing stop")
			grpcServer.Stop()
		}
	}

	appLogger.Info("server gracefully stopped")
}

// validatePlatformRequirements checks platform-specific requirements
func validatePlatformRequirements(logger *logger.Logger) error {
	switch runtime.GOOS {
	case "linux":
		// Check cgroups v2
		if _, err := os.Stat(resource.CgroupsBaseDir); os.IsNotExist(err) {
			return fmt.Errorf("cgroups not available at %s", resource.CgroupsBaseDir)
		}

		// Check cgroup namespace support
		if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
			return fmt.Errorf("cgroup namespaces not supported by kernel (required)")
		}

		// Check kernel version
		if err := validateKernelVersion(); err != nil {
			return fmt.Errorf("kernel version validation failed: %w", err)
		}

		logger.Info("Linux requirements validated",
			"cgroupsPath", resource.CgroupsBaseDir,
			"cgroupNamespace", true)
		return nil

	case "darwin":
		return fmt.Errorf("macOS not supported when cgroup namespaces are required")

	default:
		return fmt.Errorf("unsupported platform: %s (cgroup namespaces required)", runtime.GOOS)
	}
}

// Kernel version validation
func validateKernelVersion() error {
	// Read kernel version
	version, err := os.ReadFile("/proc/version")
	if err != nil {
		return fmt.Errorf("cannot read kernel version: %w", err)
	}

	// Simple check for minimum kernel version (4.6+)
	versionStr := string(version)
	if strings.Contains(versionStr, "Linux version 4.") {
		// Check if it's 4.6 or higher
		if strings.Contains(versionStr, "4.0.") ||
			strings.Contains(versionStr, "4.1.") ||
			strings.Contains(versionStr, "4.2.") ||
			strings.Contains(versionStr, "4.3.") ||
			strings.Contains(versionStr, "4.4.") ||
			strings.Contains(versionStr, "4.5.") {
			return fmt.Errorf("kernel too old for cgroup namespaces (need 4.6+)")
		}
	}

	return nil
}
