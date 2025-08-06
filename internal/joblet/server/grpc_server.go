package server

import (
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/monitoring"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// StartGRPCServer initializes and starts the main Joblet gRPC server.
// Configures the server with:
//   - mTLS authentication using embedded certificates
//   - High-performance settings for production traffic (128MB messages, 1000 concurrent streams)
//   - Keepalive parameters for connection health monitoring
//   - All Joblet services: Job execution, Network management, Volume management, Monitoring
//
// Creates a non-blocking server that listens on the configured address and po
// Creates a non-blocking server that listens on the configured address and port.
// Returns the gRPC server instance for graceful shutdown control.
func StartGRPCServer(jobStore adapters.JobStoreAdapter, joblet interfaces.Joblet, cfg *config.Config, networkStore adapters.NetworkStoreAdapter, volumeManager *volume.Manager, monitoringService *monitoring.Service) (*grpc.Server, error) {
	serverLogger := logger.WithField("component", "grpc-server")
	serverAddress := cfg.GetServerAddress()

	serverLogger.Debug("initializing gRPC server with embedded certificates",
		"address", serverAddress,
		"maxRecvMsgSize", cfg.GRPC.MaxRecvMsgSize,
		"maxSendMsgSize", cfg.GRPC.MaxSendMsgSize)

	// Get TLS configuration from embedded certificates
	tlsConfig, err := cfg.GetServerTLSConfig()
	if err != nil {
		serverLogger.Error("failed to create TLS config from embedded certificates", "error", err)
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	serverLogger.Debug("TLS configuration created from embedded certificates",
		"clientAuth", "RequireAndVerifyClientCert",
		"minTLSVersion", "1.3")

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

	serverLogger.Debug("gRPC server options configured for high-performance production traffic",
		"maxRecvMsgSize", cfg.GRPC.MaxRecvMsgSize,
		"maxSendMsgSize", cfg.GRPC.MaxSendMsgSize,
		"maxHeaderListSize", cfg.GRPC.MaxHeaderListSize,
		"maxConcurrentStreams", cfg.GRPC.MaxConcurrentStreams,
		"connectionTimeout", cfg.GRPC.ConnectionTimeout,
		"keepAliveTime", cfg.GRPC.KeepAliveTime,
		"keepAliveTimeout", cfg.GRPC.KeepAliveTimeout)

	grpcServer := grpc.NewServer(grpcOptions...)

	auth := auth2.NewGrpcAuthorization()
	serverLogger.Debug("authorization module initialized")

	// Create and register job service (no longer needs network/volume dependencies)
	jobService := NewJobServiceServer(auth, jobStore, joblet)
	pb.RegisterJobletServiceServer(grpcServer, jobService)

	// Create and register network service
	networkService := NewNetworkServiceServer(auth, networkStore)
	pb.RegisterNetworkServiceServer(grpcServer, networkService)

	// Create and register volume service
	volumeService := NewVolumeServiceServer(auth, volumeManager)
	pb.RegisterVolumeServiceServer(grpcServer, volumeService)

	// Create and register monitoring service
	monitoringGrpcService := NewMonitoringServiceServer(monitoringService)
	pb.RegisterMonitoringServiceServer(grpcServer, monitoringGrpcService)

	serverLogger.Debug("all gRPC services registered successfully", "services", []string{"job", "network", "volume", "monitoring"})

	serverLogger.Debug("creating TCP listener", "address", serverAddress)

	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		serverLogger.Error("failed to create listener", "address", serverAddress, "error", err)
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	serverLogger.Debug("TCP listener created successfully", "address", serverAddress, "network", "tcp")

	go func() {
		serverLogger.Debug("starting TLS gRPC server with embedded certificates",
			"address", serverAddress, "ready", true)

		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			serverLogger.Error("gRPC server stopped with error", "error", serveErr)
		} else {
			serverLogger.Debug("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Debug("gRPC server initialization completed",
		"address", serverAddress, "tlsEnabled", true, "authRequired", true, "certType", "embedded")

	return grpcServer, nil
}
