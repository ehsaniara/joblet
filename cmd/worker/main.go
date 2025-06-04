package main

import (
	"context"
	"google.golang.org/grpc"
	"job-worker/internal/config"
	"job-worker/internal/worker"
	"job-worker/internal/worker/server"
	"job-worker/internal/worker/store"
	"job-worker/pkg/logger"
	"os"
	"os/signal"
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

	appLogger.Info("job-worker starting", "version", "1.0.0")

	if _, err := os.Stat(config.CgroupsBaseDir); os.IsNotExist(err) {
		appLogger.Fatal("cgroups not available", "cgroupsPath", config.CgroupsBaseDir)
	}
	appLogger.Info("cgroups available", "cgroupsPath", config.CgroupsBaseDir)

	appLogger.Debug("goroutine monitoring started")

	s := store.New()
	w := worker.New(s)
	appLogger.Info("worker and store initialized")

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
