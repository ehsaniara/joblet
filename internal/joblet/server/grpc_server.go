package server

import (
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/monitoring"
	"joblet/internal/joblet/runtime"
	"joblet/internal/joblet/workflow"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// StartGRPCServerWithRegistry initializes and starts the main Joblet gRPC server using service registry pattern.
func StartGRPCServerWithRegistry(serviceComponents *core.ServiceComponents, cfg *config.Config, platform platform.Platform) (*grpc.Server, error) {
	return StartGRPCServer(
		serviceComponents.JobStore,
		serviceComponents.Joblet,
		cfg,
		serviceComponents.NetworkStore,
		serviceComponents.VolumeManager,
		serviceComponents.MonitoringService,
		platform,
	)
}

// StartGRPCServer initializes and starts the main Joblet gRPC server.
// DEPRECATED: Use StartGRPCServerWithRegistry for new implementations
func StartGRPCServer(jobStore adapters.JobStorer, joblet interfaces.Joblet, cfg *config.Config, networkStore adapters.NetworkStorer, volumeManager *volume.Manager, monitoringService *monitoring.Service, platform platform.Platform) (*grpc.Server, error) {
	serverLogger := logger.WithField("component", "grpc-server")
	serverAddress := cfg.GetServerAddress()

	// Get TLS configuration from embedded certificates
	tlsConfig, err := cfg.GetServerTLSConfig()
	if err != nil {
		serverLogger.Error("failed to create TLS config from embedded certificates", "error", err)
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	creds := credentials.NewTLS(tlsConfig)

	grpcOptions := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(int(cfg.GRPC.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(cfg.GRPC.MaxSendMsgSize)),
		grpc.MaxHeaderListSize(uint32(cfg.GRPC.MaxHeaderListSize)),
		grpc.MaxConcurrentStreams(cfg.GRPC.MaxConcurrentStreams),
		grpc.ConnectionTimeout(cfg.GRPC.ConnectionTimeout),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    cfg.GRPC.KeepAliveTime,
			Timeout: cfg.GRPC.KeepAliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             cfg.GRPC.KeepAliveTime / 2, // Allow keepalive pings every KeepAliveTime/2
			PermitWithoutStream: true,                       // Allow keepalive pings even when no streams are active
		}),
	}

	grpcServer := grpc.NewServer(grpcOptions...)

	auth := auth2.NewGRPCAuthorization()

	// Create runtime resolver for workflow validation
	// Runtime support is always enabled
	serverLogger.Info("initializing runtime resolver for workflow validation", "basePath", cfg.Runtime.BasePath)
	runtimeResolver := runtime.NewResolver(cfg.Runtime.BasePath, platform)

	// Create workflow manager and unified job service with validation
	workflowManager := workflow.NewWorkflowManager()
	jobService := NewWorkflowServiceServer(auth, jobStore, joblet, workflowManager, volumeManager, runtimeResolver)
	pb.RegisterJobServiceServer(grpcServer, jobService)

	// Create and register network service
	networkService := NewNetworkServiceServer(auth, networkStore)
	pb.RegisterNetworkServiceServer(grpcServer, networkService)

	// Create and register volume service
	volumeService := NewVolumeServiceServer(auth, volumeManager)
	pb.RegisterVolumeServiceServer(grpcServer, volumeService)

	// Create and register monitoring service
	monitoringGrpcService := NewMonitoringServiceServer(monitoringService, cfg)
	pb.RegisterMonitoringServiceServer(grpcServer, monitoringGrpcService)

	// Create and register runtime service with direct installation capabilities (no job system)
	runtimeService := NewRuntimeServiceServer(auth, cfg.Runtime.BasePath, platform, cfg)
	pb.RegisterRuntimeServiceServer(grpcServer, runtimeService)

	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		serverLogger.Error("failed to create listener", "address", serverAddress, "error", err)
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		serverLogger.Info("starting gRPC server", "address", serverAddress)

		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			serverLogger.Error("gRPC server stopped with error", "error", serveErr)
		} else {
			serverLogger.Info("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Info("gRPC server initialized", "address", serverAddress)

	return grpcServer, nil
}
