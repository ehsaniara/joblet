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
	"worker/pkg/logger"
)

const (
	serverCertPath = "./certs/server-cert.pem"
	serverKeyPath  = "./certs/server-key.pem"
	caCertPath     = "./certs/ca-cert.pem"

	MaxRecvMsgSize    = 512 * 1024      // 512KB
	MaxSendMsgSize    = 4 * 1024 * 1024 // 4MB
	MaxHeaderListSize = 1 * 1024 * 1024 // 1MB

	serverAddress = "0.0.0.0:50051"
)

func StartGRPCServer(jobStore state.Store, jobWorker interfaces.Worker) (*grpc.Server, error) {
	serverLogger := logger.WithField("component", "grpc-server")

	serverLogger.Info("initializing gRPC server", "address", serverAddress, "tlsEnabled", true)

	serverLogger.Debug("loading server certificates", "certPath", serverCertPath, "keyPath", serverKeyPath)

	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		serverLogger.Error("failed to load server cert/key", "certPath", serverCertPath, "keyPath", serverKeyPath, "error", err)
		return nil, fmt.Errorf("failed to load server cert/key: %w", err)
	}

	serverLogger.Debug("server certificate loaded successfully")

	serverLogger.Debug("loading CA certificate", "caPath", caCertPath)

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		serverLogger.Error("failed to read CA cert", "caPath", caCertPath, "error", err)
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

	serverLogger.Info("TLS configuration completed",
		"clientAuth", "RequireAndVerifyClientCert",
		"minTLSVersion", "1.3")

	grpcOptions := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(MaxRecvMsgSize),
		grpc.MaxSendMsgSize(MaxSendMsgSize),
		grpc.MaxHeaderListSize(uint32(MaxHeaderListSize)),
	}

	serverLogger.Debug("gRPC server options configured",
		"maxRecvMsgSize", MaxRecvMsgSize,
		"maxSendMsgSize", MaxSendMsgSize,
		"maxHeaderListSize", MaxHeaderListSize)

	grpcServer := grpc.NewServer(grpcOptions...)

	auth := auth2.NewGrpcAuthorization()
	serverLogger.Debug("authorization module initialized")

	jobService := NewJobServiceServer(auth, jobStore, jobWorker)
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
		serverLogger.Info("starting TLS gRPC server", "address", serverAddress, "ready", true)

		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			serverLogger.Error("gRPC server stopped with error", "error", serveErr)
		} else {
			serverLogger.Info("gRPC server stopped gracefully")
		}
	}()

	serverLogger.Info("gRPC server initialization completed", "address", serverAddress, "tlsEnabled", true, "authRequired", true)

	return grpcServer, nil
}
