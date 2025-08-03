package server

import (
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "joblet/api/gen"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/monitoring"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"net"
)

func StartGRPCServer(jobStore state.Store, joblet interfaces.Joblet, cfg *config.Config, networkStore *state.NetworkStore, volumeManager *volume.Manager, monitoringService *monitoring.Service) (*grpc.Server, error) {
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
	}

	serverLogger.Debug("gRPC server options configured",
		"maxRecvMsgSize", cfg.GRPC.MaxRecvMsgSize,
		"maxSendMsgSize", cfg.GRPC.MaxSendMsgSize,
		"maxHeaderListSize", cfg.GRPC.MaxHeaderListSize)

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
