package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"os"
	pb "worker/api/gen"
	auth2 "worker/internal/worker/auth"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/state"
	"worker/pkg/config"
	"worker/pkg/logger"
)

func StartGRPCServer(jobStore state.Store, jobWorker interfaces.Worker, cfg *config.Config) (*grpc.Server, error) {
	serverLogger := logger.WithField("component", "grpc-server")
	serverAddress := cfg.GetServerAddress()

	serverLogger.Debug("initializing gRPC server",
		"address", serverAddress,
		"maxRecvMsgSize", cfg.GRPC.MaxRecvMsgSize,
		"maxSendMsgSize", cfg.GRPC.MaxSendMsgSize)

	serverCert, err := tls.LoadX509KeyPair(cfg.Security.ServerCertPath, cfg.Security.ServerKeyPath)
	if err != nil {
		serverLogger.Error("failed to load server cert/key", "certPath", cfg.Security.ServerCertPath, "keyPath", cfg.Security.ServerKeyPath, "error", err)
		return nil, fmt.Errorf("failed to load server cert/key: %w", err)
	}

	serverLogger.Debug("server certificate loaded successfully")

	serverLogger.Debug("loading CA certificate", "caPath", cfg.Security.CACertPath)

	caCert, err := os.ReadFile(cfg.Security.CACertPath)
	if err != nil {
		serverLogger.Error("failed to read CA cert", "caPath", cfg.Security.CACertPath, "error", err)
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caCert); !ok {
		serverLogger.Error("failed to add CA cert to pool")
		return nil, fmt.Errorf("failed to add CA cert to pool")
	}

	serverLogger.Debug("CA certificate loaded successfully")

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	serverLogger.Debug("TLS configuration completed",
		"clientAuth", "RequireAndVerifyClientCert",
		"minTLSVersion", "1.3")

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

	jobService := NewJobServiceServer(auth, jobStore, jobWorker)
	pb.RegisterJobServiceServer(grpcServer, jobService)

	serverLogger.Debug("job service registered successfully")

	serverLogger.Debug("creating TCP listener", "address", serverAddress)

	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		serverLogger.Error("failed to create listener", "address", serverAddress, "error", err)
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	serverLogger.Debug("TCP listener created successfully", "address", serverAddress, "network", "tcp")

	go func() {
		serverLogger.Debug("starting TLS gRPC server", "address", serverAddress, "ready", true)

		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			serverLogger.Error("gRPC server stopped with error", "error", serveErr)
		} else {
			serverLogger.Debug("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Debug("gRPC server initialization completed", "address", serverAddress, "tlsEnabled", true, "authRequired", true)

	return grpcServer, nil
}
