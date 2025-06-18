package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"job-worker/internal/config"
	"job-worker/internal/worker/jobworker"
	"job-worker/internal/worker/server"
	"job-worker/internal/worker/store"
	"job-worker/pkg/logger"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
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

	s := store.New()
	w := jobworker.New(s)
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
		if _, err := os.Stat(config.CgroupsBaseDir); os.IsNotExist(err) {
			return fmt.Errorf("cgroups not available at %s", config.CgroupsBaseDir)
		}
		logger.Info("Linux requirements validated", "cgroupsPath", config.CgroupsBaseDir)
		return nil
	case "darwin":
		logger.Info("macOS detected - using mock implementation")
		logger.Warn("resource limits not enforced on macOS")
		return nil
	default:
		logger.Warn("unsupported platform, using mock functionality", "platform", runtime.GOOS)
		return nil
	}
}
