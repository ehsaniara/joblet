package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	persistpb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/auth"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/security"
)

// Conversion functions between ipc (internal) and gen (external proto) types

func streamTypeIPCToGen(ipc ipcpb.StreamType) persistpb.StreamType {
	switch ipc {
	case ipcpb.StreamType_STREAM_TYPE_STDOUT:
		return persistpb.StreamType_STREAM_TYPE_STDOUT
	case ipcpb.StreamType_STREAM_TYPE_STDERR:
		return persistpb.StreamType_STREAM_TYPE_STDERR
	default:
		return persistpb.StreamType_STREAM_TYPE_UNSPECIFIED
	}
}

func streamTypeGenToIPC(gen persistpb.StreamType) ipcpb.StreamType {
	switch gen {
	case persistpb.StreamType_STREAM_TYPE_STDOUT:
		return ipcpb.StreamType_STREAM_TYPE_STDOUT
	case persistpb.StreamType_STREAM_TYPE_STDERR:
		return ipcpb.StreamType_STREAM_TYPE_STDERR
	default:
		return ipcpb.StreamType_STREAM_TYPE_UNSPECIFIED
	}
}

func logLineIPCToGen(ipc *ipcpb.LogLine) *persistpb.LogLine {
	if ipc == nil {
		return nil
	}
	return &persistpb.LogLine{
		JobId:     ipc.JobId,
		Stream:    streamTypeIPCToGen(ipc.Stream),
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Content:   ipc.Content,
	}
}

func metricIPCToGen(ipc *ipcpb.Metric) *persistpb.Metric {
	if ipc == nil {
		return nil
	}

	gen := &persistpb.Metric{
		JobId:     ipc.JobId,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
	}

	if ipc.Data != nil {
		gen.Data = &persistpb.MetricData{
			CpuUsage:    ipc.Data.CpuUsage,
			MemoryUsage: ipc.Data.MemoryUsage,
			GpuUsage:    ipc.Data.GpuUsage,
		}

		if ipc.Data.DiskIo != nil {
			gen.Data.DiskIo = &persistpb.DiskIO{
				ReadBytes:  ipc.Data.DiskIo.ReadBytes,
				WriteBytes: ipc.Data.DiskIo.WriteBytes,
				ReadOps:    ipc.Data.DiskIo.ReadOps,
				WriteOps:   ipc.Data.DiskIo.WriteOps,
			}
		}

		if ipc.Data.NetworkIo != nil {
			gen.Data.NetworkIo = &persistpb.NetworkIO{
				RxBytes:   ipc.Data.NetworkIo.RxBytes,
				TxBytes:   ipc.Data.NetworkIo.TxBytes,
				RxPackets: ipc.Data.NetworkIo.RxPackets,
				TxPackets: ipc.Data.NetworkIo.TxPackets,
			}
		}
	}

	return gen
}

// GRPCServer is the gRPC server for persist service
type GRPCServer struct {
	persistpb.UnimplementedPersistServiceServer
	auth     auth.GRPCAuthorization
	config   *config.ServerConfig
	security *config.SecurityConfig // Inherited TLS certificates
	backend  storage.Backend
	logger   *logger.Logger
	grpcSrv  *grpc.Server
	listener net.Listener
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(cfg *config.ServerConfig, backend storage.Backend, log *logger.Logger, authorization auth.GRPCAuthorization, security *config.SecurityConfig) *GRPCServer {
	return &GRPCServer{
		auth:     authorization,
		config:   cfg,
		security: security,
		backend:  backend,
		logger:   log.WithField("component", "grpc-server"),
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start(ctx context.Context) error {
	// Decide which listener to use (Unix socket or TCP)
	var listener net.Listener
	var err error
	var isUnixSocket bool

	if s.config.GRPCSocket != "" {
		// Prefer Unix socket for internal IPC
		listener, err = net.Listen("unix", s.config.GRPCSocket)
		if err != nil {
			return fmt.Errorf("failed to listen on unix socket: %w", err)
		}
		isUnixSocket = true
		s.logger.Info("gRPC server listening on Unix socket", "socket", s.config.GRPCSocket)
	} else if s.config.GRPCAddress != "" {
		// Fallback to TCP
		listener, err = net.Listen("tcp", s.config.GRPCAddress)
		if err != nil {
			return fmt.Errorf("failed to listen on TCP: %w", err)
		}
		s.logger.Info("gRPC server listening on TCP", "address", s.config.GRPCAddress)
	} else {
		return fmt.Errorf("either grpc_socket or grpc_address must be configured")
	}

	s.listener = listener

	// Create gRPC server options
	// Set large message sizes for streaming historical logs/metrics (128MB each direction)
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(s.config.MaxConnections)),
		grpc.MaxRecvMsgSize(134217728), // 128MB - handle large query requests
		grpc.MaxSendMsgSize(134217728), // 128MB - handle large historical data streams
	}

	// TLS configuration: MANDATORY for TCP, optional for Unix socket
	if !isUnixSocket {
		// TLS is MANDATORY for TCP connections
		var tlsConfig *tls.Config

		// Determine ClientAuth mode (default to "require")
		clientAuthRequired := true
		clientAuthMode := "require"
		if s.config.TLS != nil {
			if s.config.TLS.ClientAuth != "" {
				clientAuthMode = s.config.TLS.ClientAuth
			}
			clientAuthRequired = clientAuthMode == "require" || clientAuthMode == ""
		}

		// If TLS config exists and cert files are specified, use file-based loading
		if s.config.TLS != nil && s.config.TLS.CertFile != "" && s.config.TLS.KeyFile != "" {
			tlsCfg := security.TLSConfig{
				Enabled:    true,
				CertFile:   s.config.TLS.CertFile,
				KeyFile:    s.config.TLS.KeyFile,
				CAFile:     s.config.TLS.CAFile,
				ClientAuth: clientAuthRequired,
			}
			var err error
			tlsConfig, err = security.LoadServerTLSConfig(tlsCfg)
			if err != nil {
				return fmt.Errorf("failed to load TLS credentials from files: %w", err)
			}
			s.logger.Info("TLS ENABLED (from files)", "clientAuth", clientAuthMode)
		} else if s.security != nil && s.security.ServerCert != "" {
			// Use inherited embedded certificates from parent
			var err error
			tlsConfig, err = security.LoadServerTLSConfigFromPEM(
				[]byte(s.security.ServerCert),
				[]byte(s.security.ServerKey),
				[]byte(s.security.CACert),
				clientAuthRequired,
			)
			if err != nil {
				return fmt.Errorf("failed to load inherited TLS credentials: %w", err)
			}
			s.logger.Info("TLS ENABLED (inherited from parent)", "clientAuth", clientAuthMode)
		} else {
			return fmt.Errorf("TLS is mandatory for TCP but no certificates configured (neither files nor inherited)")
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
	} else {
		// Unix socket - no TLS needed (pure Linux IPC)
		s.logger.Info("Unix socket IPC - TLS disabled (native Linux IPC)")
	}

	s.grpcSrv = grpc.NewServer(opts...)
	persistpb.RegisterPersistServiceServer(s.grpcSrv, s)

	s.logger.Info("gRPC server starting", "address", s.config.GRPCAddress)

	// Start serving in goroutine
	go func() {
		if err := s.grpcSrv.Serve(listener); err != nil {
			s.logger.Error("gRPC server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() error {
	s.logger.Info("Stopping gRPC server")

	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	s.logger.Info("gRPC server stopped")
	return nil
}

// QueryLogs implements the QueryLogs RPC
func (s *GRPCServer) QueryLogs(req *persistpb.QueryLogsRequest, stream persistpb.PersistService_QueryLogsServer) error {
	// Check authorization
	if err := s.auth.Authorized(stream.Context(), auth.QueryLogsOp); err != nil {
		return err
	}

	s.logger.Info("QueryLogs request", "jobID", req.JobId, "limit", req.Limit, "offset", req.Offset, "stream", req.Stream)

	// Build query
	query := &storage.LogQuery{
		JobID:  req.JobId,
		Stream: streamTypeGenToIPC(req.Stream),
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}

	// Add time range if specified
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	// Read logs from backend
	reader, err := s.backend.ReadLogs(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read logs", "error", err, "jobID", req.JobId)
		return status.Errorf(codes.Internal, "failed to read logs: %v", err)
	}

	// Stream logs to client
	logCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryLogs cancelled by client", "jobID", req.JobId, "logCount", logCount)
			return stream.Context().Err()

		case logLine, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading logs", "error", err, "jobID", req.JobId)
						return status.Errorf(codes.Internal, "error reading logs: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryLogs completed", "jobID", req.JobId, "logCount", logCount)
				return nil
			}

			// Send log line to client (convert from ipc to gen)
			if err := stream.Send(logLineIPCToGen(logLine)); err != nil {
				s.logger.Error("Failed to send log line", "error", err, "jobID", req.JobId)
				return status.Errorf(codes.Internal, "failed to send log: %v", err)
			}
			logCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from log reader", "error", err, "jobID", req.JobId)
				return status.Errorf(codes.Internal, "error reading logs: %v", err)
			}
		}
	}
}

// QueryMetrics implements the QueryMetrics RPC
func (s *GRPCServer) QueryMetrics(req *persistpb.QueryMetricsRequest, stream persistpb.PersistService_QueryMetricsServer) error {
	// Check authorization
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryMetrics request", "jobID", req.JobId, "limit", req.Limit, "offset", req.Offset)

	// Build query
	query := &storage.MetricQuery{
		JobID:  req.JobId,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}

	// Add time range if specified
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	// Read metrics from backend
	reader, err := s.backend.ReadMetrics(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read metrics", "error", err, "jobID", req.JobId)
		return status.Errorf(codes.Internal, "failed to read metrics: %v", err)
	}

	// Stream metrics to client
	metricCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryMetrics cancelled by client", "jobID", req.JobId, "metricCount", metricCount)
			return stream.Context().Err()

		case metric, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading metrics", "error", err, "jobID", req.JobId)
						return status.Errorf(codes.Internal, "error reading metrics: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryMetrics completed", "jobID", req.JobId, "metricCount", metricCount)
				return nil
			}

			// Send metric to client (convert from ipc to gen)
			if err := stream.Send(metricIPCToGen(metric)); err != nil {
				s.logger.Error("Failed to send metric", "error", err, "jobID", req.JobId)
				return status.Errorf(codes.Internal, "failed to send metric: %v", err)
			}
			metricCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from metric reader", "error", err, "jobID", req.JobId)
				return status.Errorf(codes.Internal, "error reading metrics: %v", err)
			}
		}
	}
}

// DeleteJob implements the DeleteJob RPC
func (s *GRPCServer) DeleteJob(ctx context.Context, req *persistpb.DeleteJobRequest) (*persistpb.DeleteJobResponse, error) {
	// Check authorization
	if err := s.auth.Authorized(ctx, auth.DeleteJobOp); err != nil {
		return &persistpb.DeleteJobResponse{
			Success: false,
			Message: fmt.Sprintf("Unauthorized: %v", err),
		}, nil
	}

	s.logger.Info("DeleteJob request", "jobID", req.JobId)

	// Validate job ID
	if req.JobId == "" {
		return &persistpb.DeleteJobResponse{
			Success: false,
			Message: "Job ID cannot be empty",
		}, nil
	}

	// Delete job from backend storage
	if err := s.backend.DeleteJob(req.JobId); err != nil {
		s.logger.Error("Failed to delete job", "jobID", req.JobId, "error", err)
		return &persistpb.DeleteJobResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete job: %v", err),
		}, nil
	}

	s.logger.Info("Job deleted successfully", "jobID", req.JobId)

	return &persistpb.DeleteJobResponse{
		Success: true,
		Message: "Job deleted successfully",
	}, nil
}
