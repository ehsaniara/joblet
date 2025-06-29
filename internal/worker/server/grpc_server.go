package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
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
		"tlsEnabled", cfg.Security.TLSEnabled,
		"maxRecvMsgSize", cfg.GRPC.MaxRecvMsgSize,
		"maxSendMsgSize", cfg.GRPC.MaxSendMsgSize,
		"requireClientCert", cfg.Security.RequireClientCert)

	var grpcOptions []grpc.ServerOption

	// Configure TLS based on configuration
	if cfg.Security.TLSEnabled {
		creds, err := setupTLSCredentials(cfg.Security, serverLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to setup TLS: %w", err)
		}
		grpcOptions = append(grpcOptions, grpc.Creds(creds))
		serverLogger.Debug("TLS enabled with configuration")
	} else {
		serverLogger.Warn("TLS disabled - server will accept unencrypted connections")
	}

	// Configure message sizes from configuration
	grpcOptions = append(grpcOptions,
		grpc.MaxRecvMsgSize(int(cfg.GRPC.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(cfg.GRPC.MaxSendMsgSize)),
		grpc.MaxHeaderListSize(uint32(cfg.GRPC.MaxHeaderListSize)),
	)

	// Configure keep-alive settings from configuration
	if cfg.GRPC.KeepAliveTime > 0 {
		kaParams := keepalive.ServerParameters{
			Time:    cfg.GRPC.KeepAliveTime,
			Timeout: cfg.GRPC.KeepAliveTimeout,
		}
		grpcOptions = append(grpcOptions, grpc.KeepaliveParams(kaParams))

		serverLogger.Debug("keep-alive configured",
			"time", cfg.GRPC.KeepAliveTime,
			"timeout", cfg.GRPC.KeepAliveTimeout)
	}

	// Create server with configured options
	grpcServer := grpc.NewServer(grpcOptions...)

	// Initialize services
	auth := auth2.NewGrpcAuthorization()
	jobService := NewJobServiceServer(auth, jobStore, jobWorker)
	pb.RegisterJobServiceServer(grpcServer, jobService)

	// Create listener
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", serverAddress, err)
	}

	// Start server in goroutine
	go func() {
		serverLogger.Debug("gRPC server starting",
			"address", serverAddress,
			"tlsEnabled", cfg.Security.TLSEnabled)

		if err := grpcServer.Serve(lis); err != nil {
			serverLogger.Error("gRPC server error", "error", err)
		} else {
			serverLogger.Debug("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Debug("gRPC server ready", "address", serverAddress)
	return grpcServer, nil
}

func setupTLSCredentials(secCfg config.SecurityConfig, logger *logger.Logger) (credentials.TransportCredentials, error) {
	logger.Debug("loading TLS certificates from configuration",
		"serverCert", secCfg.ServerCertPath,
		"serverKey", secCfg.ServerKeyPath,
		"caCert", secCfg.CACertPath,
		"requireClientCert", secCfg.RequireClientCert)

	// Load server certificate
	serverCert, err := tls.LoadX509KeyPair(secCfg.ServerCertPath, secCfg.ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert/key from config: %w", err)
	}

	// Setup client CA if required
	var clientCAs *x509.CertPool
	var clientAuth tls.ClientAuthType = tls.NoClientCert

	if secCfg.RequireClientCert {
		caCert, err := os.ReadFile(secCfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert from config: %w", err)
		}

		clientCAs = x509.NewCertPool()
		if ok := clientCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to add CA cert to pool")
		}

		clientAuth = tls.RequireAndVerifyClientCert
		logger.Debug("client certificate verification enabled")
	}

	// Configure TLS version from config
	var minVersion uint16
	switch secCfg.MinTLSVersion {
	case "1.2":
		minVersion = tls.VersionTLS12
	case "1.3":
		minVersion = tls.VersionTLS13
	default:
		minVersion = tls.VersionTLS13
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   clientAuth,
		ClientCAs:    clientCAs,
		MinVersion:   minVersion,
	}

	logger.Debug("TLS configuration completed",
		"clientAuth", clientAuth,
		"minTLSVersion", secCfg.MinTLSVersion)

	return credentials.NewTLS(tlsConfig), nil
}
