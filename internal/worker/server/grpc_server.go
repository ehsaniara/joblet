package server

import (
	"fmt"
	"google.golang.org/grpc"
	pb "job-worker/api/gen"
	"job-worker/internal/config"
	"job-worker/internal/worker/interfaces"
	"job-worker/pkg/logger"
	"net"
)

const (
	serverAddress = "0.0.0.0:50051"
)

func StartGRPCServer(jobStore interfaces.Store, jobWorker interfaces.JobWorker) (*grpc.Server, error) {
	serverLogger := logger.WithField("component", "grpc-server")

	serverLogger.Info("initializing gRPC server", "address", serverAddress, "tlsEnabled", false)

	// gRPC server options
	grpcOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
		grpc.MaxHeaderListSize(uint32(config.MaxHeaderListSize)),
	}

	serverLogger.Debug("gRPC server options configured",
		"maxRecvMsgSize", config.MaxRecvMsgSize,
		"maxSendMsgSize", config.MaxSendMsgSize,
		"maxHeaderListSize", config.MaxHeaderListSize)

	grpcServer := grpc.NewServer(grpcOptions...)

	jobService := NewJobServiceServer(jobStore, jobWorker)
	pb.RegisterJobServiceServer(grpcServer, jobService)

	serverLogger.Info("job service registered successfully")

	serverLogger.Debug("creating TCP listener", "address", serverAddress)

	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		serverLogger.Error("failed to create listener", "address", serverAddress, "error", err)
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	serverLogger.Info("TCP listener created successfully", "address", serverAddress, "network", "tcp")

	go func() {
		serverLogger.Info("starting gRPC server", "address", serverAddress, "ready", true)

		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			serverLogger.Error("gRPC server stopped with error", "error", serveErr)
		} else {
			serverLogger.Info("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Info("gRPC server initialization completed", "address", serverAddress, "tlsEnabled", false, "authRequired", true)

	return grpcServer, nil
}
