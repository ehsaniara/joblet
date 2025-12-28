package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/ehsaniara/joblet/internal/joblet/auth"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
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
		JobUuid:   ipc.JobUuid,
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
		JobUuid:   ipc.JobUuid,
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
		// Remove existing socket to prevent "address already in use" errors
		if err := os.Remove(s.config.GRPCSocket); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}

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

	s.logger.Info("QueryLogs request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset, "stream", req.Stream)

	// Build query
	query := &storage.LogQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Stream:  streamTypeGenToIPC(req.Stream),
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
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
		s.logger.Error("Failed to read logs", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read logs: %v", err)
	}

	// Stream logs to client
	logCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryLogs cancelled by client", "job_uuid", req.JobUuid, "logCount", logCount)
			return stream.Context().Err()

		case logLine, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading logs", "error", err, "job_uuid", req.JobUuid)
						return status.Errorf(codes.Internal, "error reading logs: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryLogs completed", "job_uuid", req.JobUuid, "logCount", logCount)
				return nil
			}

			// Send log line to client (convert from ipc to gen)
			if err := stream.Send(logLineIPCToGen(logLine)); err != nil {
				s.logger.Error("Failed to send log line", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "failed to send log: %v", err)
			}
			logCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from log reader", "error", err, "job_uuid", req.JobUuid)
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

	s.logger.Info("QueryMetrics request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	// Build query
	query := &storage.MetricQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
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
		s.logger.Error("Failed to read metrics", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read metrics: %v", err)
	}

	// Stream metrics to client
	metricCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryMetrics cancelled by client", "job_uuid", req.JobUuid, "metricCount", metricCount)
			return stream.Context().Err()

		case metric, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading metrics", "error", err, "job_uuid", req.JobUuid)
						return status.Errorf(codes.Internal, "error reading metrics: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryMetrics completed", "job_uuid", req.JobUuid, "metricCount", metricCount)
				return nil
			}

			// Send metric to client (convert from ipc to gen)
			if err := stream.Send(metricIPCToGen(metric)); err != nil {
				s.logger.Error("Failed to send metric", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "failed to send metric: %v", err)
			}
			metricCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from metric reader", "error", err, "job_uuid", req.JobUuid)
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

	s.logger.Info("DeleteJob request", "job_uuid", req.JobUuid)

	// Validate job ID
	if req.JobUuid == "" {
		return &persistpb.DeleteJobResponse{
			Success: false,
			Message: "Job ID cannot be empty",
		}, nil
	}

	// Delete job from backend storage
	if err := s.backend.DeleteJob(req.JobUuid); err != nil {
		s.logger.Error("Failed to delete job", "job_uuid", req.JobUuid, "error", err)
		return &persistpb.DeleteJobResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete job: %v", err),
		}, nil
	}

	s.logger.Info("Job deleted successfully", "job_uuid", req.JobUuid)

	return &persistpb.DeleteJobResponse{
		Success: true,
		Message: "Job deleted successfully",
	}, nil
}

// Ping implements the health check RPC
func (s *GRPCServer) Ping(ctx context.Context, req *persistpb.PingRequest) (*persistpb.PingResponse, error) {
	// No authorization check for ping - it's a health check
	return &persistpb.PingResponse{
		Healthy:   true,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

// execEventIPCToGen converts an exec event from ipc to persist proto
func execEventIPCToGen(ipc *ipcpb.ExecEvent) *persistpb.ExecEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.ExecEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Ppid:      ipc.Ppid,
		Uid:       ipc.Uid,
		Gid:       ipc.Gid,
		Comm:      ipc.Comm,
		Filename:  ipc.Filename,
		Args:      ipc.Args,
	}
}

// connectEventIPCToGen converts a connect event from ipc to persist proto
func connectEventIPCToGen(ipc *ipcpb.ConnectEvent) *persistpb.ConnectEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.ConnectEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		SrcAddr:   ipc.SrcAddr,
		SrcPort:   ipc.SrcPort,
		DstAddr:   ipc.DstAddr,
		DstPort:   ipc.DstPort,
		Protocol:  ipc.Protocol,
	}
}

// mmapEventIPCToGen converts a mmap event from ipc to persist proto
func mmapEventIPCToGen(ipc *ipcpb.MmapEvent) *persistpb.MmapEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.MmapEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		Addr:      ipc.Addr,
		Length:    ipc.Length,
		Prot:      ipc.Prot,
		Flags:     ipc.Flags,
		Filename:  ipc.Filename,
	}
}

// mprotectEventIPCToGen converts a mprotect event from ipc to persist proto
func mprotectEventIPCToGen(ipc *ipcpb.MprotectEvent) *persistpb.MprotectEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.MprotectEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		Addr:      ipc.Addr,
		Length:    ipc.Length,
		Prot:      ipc.Prot,
	}
}

// fileEventIPCToGen converts a file event from ipc to persist proto
func fileEventIPCToGen(ipc *ipcpb.FileEvent) *persistpb.FileEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.FileEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		Path:      ipc.Path,
		Operation: ipc.Operation,
		Bytes:     ipc.Bytes,
	}
}

// acceptEventIPCToGen converts an accept event from ipc to persist proto
func acceptEventIPCToGen(ipc *ipcpb.AcceptEvent) *persistpb.AcceptEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.AcceptEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		SrcAddr:   ipc.SrcAddr,
		SrcPort:   ipc.SrcPort,
		DstAddr:   ipc.DstAddr,
		DstPort:   ipc.DstPort,
		Protocol:  ipc.Protocol,
	}
}

// socketDataEventIPCToGen converts a socket data event from ipc to persist proto
func socketDataEventIPCToGen(ipc *ipcpb.SocketDataEvent) *persistpb.SocketDataEvent {
	if ipc == nil {
		return nil
	}
	return &persistpb.SocketDataEvent{
		JobUuid:   ipc.JobUuid,
		Timestamp: ipc.Timestamp,
		Sequence:  ipc.Sequence,
		Pid:       ipc.Pid,
		Comm:      ipc.Comm,
		Direction: ipc.Direction,
		DstAddr:   ipc.Addr,
		DstPort:   ipc.Port,
		Bytes:     ipc.Bytes,
		Protocol:  ipc.Protocol,
	}
}

// QueryExecEvents implements the QueryExecEvents RPC
func (s *GRPCServer) QueryExecEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryExecEventsServer) error {
	// Check authorization
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryExecEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	// Build query
	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}

	// Add time range if specified
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	// Read exec events from backend
	reader, err := s.backend.ReadExecEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read exec events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read exec events: %v", err)
	}

	// Stream events to client
	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryExecEvents cancelled by client", "job_uuid", req.JobUuid, "eventCount", eventCount)
			return stream.Context().Err()

		case event, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading exec events", "error", err, "job_uuid", req.JobUuid)
						return status.Errorf(codes.Internal, "error reading exec events: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryExecEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}

			// Send event to client (convert from ipc to gen)
			if err := stream.Send(execEventIPCToGen(event)); err != nil {
				s.logger.Error("Failed to send exec event", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "failed to send exec event: %v", err)
			}
			eventCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from exec event reader", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "error reading exec events: %v", err)
			}
		}
	}
}

// QueryConnectEvents implements the QueryConnectEvents RPC
func (s *GRPCServer) QueryConnectEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryConnectEventsServer) error {
	// Check authorization
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryConnectEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	// Build query
	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}

	// Add time range if specified
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	// Read connect events from backend
	reader, err := s.backend.ReadConnectEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read connect events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read connect events: %v", err)
	}

	// Stream events to client
	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			s.logger.Debug("QueryConnectEvents cancelled by client", "job_uuid", req.JobUuid, "eventCount", eventCount)
			return stream.Context().Err()

		case event, ok := <-reader.Channel:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-reader.Error:
					if err != nil {
						s.logger.Error("Error reading connect events", "error", err, "job_uuid", req.JobUuid)
						return status.Errorf(codes.Internal, "error reading connect events: %v", err)
					}
				default:
				}
				// Successful completion
				s.logger.Info("QueryConnectEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}

			// Send event to client (convert from ipc to gen)
			if err := stream.Send(connectEventIPCToGen(event)); err != nil {
				s.logger.Error("Failed to send connect event", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "failed to send connect event: %v", err)
			}
			eventCount++

		case err := <-reader.Error:
			if err != nil {
				s.logger.Error("Error from connect event reader", "error", err, "job_uuid", req.JobUuid)
				return status.Errorf(codes.Internal, "error reading connect events: %v", err)
			}
		}
	}
}

// QueryMmapEvents implements the QueryMmapEvents RPC
func (s *GRPCServer) QueryMmapEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryMmapEventsServer) error {
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryMmapEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	reader, err := s.backend.ReadMmapEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read mmap events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read mmap events: %v", err)
	}

	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-reader.Channel:
			if !ok {
				select {
				case err := <-reader.Error:
					if err != nil {
						return status.Errorf(codes.Internal, "error reading mmap events: %v", err)
					}
				default:
				}
				s.logger.Info("QueryMmapEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}
			if err := stream.Send(mmapEventIPCToGen(event)); err != nil {
				return status.Errorf(codes.Internal, "failed to send mmap event: %v", err)
			}
			eventCount++
		case err := <-reader.Error:
			if err != nil {
				return status.Errorf(codes.Internal, "error reading mmap events: %v", err)
			}
		}
	}
}

// QueryMprotectEvents implements the QueryMprotectEvents RPC
func (s *GRPCServer) QueryMprotectEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryMprotectEventsServer) error {
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryMprotectEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	reader, err := s.backend.ReadMprotectEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read mprotect events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read mprotect events: %v", err)
	}

	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-reader.Channel:
			if !ok {
				select {
				case err := <-reader.Error:
					if err != nil {
						return status.Errorf(codes.Internal, "error reading mprotect events: %v", err)
					}
				default:
				}
				s.logger.Info("QueryMprotectEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}
			if err := stream.Send(mprotectEventIPCToGen(event)); err != nil {
				return status.Errorf(codes.Internal, "failed to send mprotect event: %v", err)
			}
			eventCount++
		case err := <-reader.Error:
			if err != nil {
				return status.Errorf(codes.Internal, "error reading mprotect events: %v", err)
			}
		}
	}
}

// QueryFileEvents implements the QueryFileEvents RPC
func (s *GRPCServer) QueryFileEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryFileEventsServer) error {
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryFileEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	reader, err := s.backend.ReadFileEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read file events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read file events: %v", err)
	}

	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-reader.Channel:
			if !ok {
				select {
				case err := <-reader.Error:
					if err != nil {
						return status.Errorf(codes.Internal, "error reading file events: %v", err)
					}
				default:
				}
				s.logger.Info("QueryFileEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}
			if err := stream.Send(fileEventIPCToGen(event)); err != nil {
				return status.Errorf(codes.Internal, "failed to send file event: %v", err)
			}
			eventCount++
		case err := <-reader.Error:
			if err != nil {
				return status.Errorf(codes.Internal, "error reading file events: %v", err)
			}
		}
	}
}

// QueryAcceptEvents implements the QueryAcceptEvents RPC
func (s *GRPCServer) QueryAcceptEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QueryAcceptEventsServer) error {
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QueryAcceptEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	reader, err := s.backend.ReadAcceptEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read accept events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read accept events: %v", err)
	}

	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-reader.Channel:
			if !ok {
				select {
				case err := <-reader.Error:
					if err != nil {
						return status.Errorf(codes.Internal, "error reading accept events: %v", err)
					}
				default:
				}
				s.logger.Info("QueryAcceptEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}
			if err := stream.Send(acceptEventIPCToGen(event)); err != nil {
				return status.Errorf(codes.Internal, "failed to send accept event: %v", err)
			}
			eventCount++
		case err := <-reader.Error:
			if err != nil {
				return status.Errorf(codes.Internal, "error reading accept events: %v", err)
			}
		}
	}
}

// QuerySocketDataEvents implements the QuerySocketDataEvents RPC
func (s *GRPCServer) QuerySocketDataEvents(req *persistpb.QueryTelemetryRequest, stream persistpb.PersistService_QuerySocketDataEventsServer) error {
	if err := s.auth.Authorized(stream.Context(), auth.QueryMetricsOp); err != nil {
		return err
	}

	s.logger.Info("QuerySocketDataEvents request", "job_uuid", req.JobUuid, "limit", req.Limit, "offset", req.Offset)

	query := &storage.TelemetryQuery{
		JobUUID: req.JobUuid,
		NodeID:  req.NodeId, // For multi-node CloudWatch queries
		Limit:   int(req.Limit),
		Offset:  int(req.Offset),
	}
	if req.StartTime > 0 {
		query.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		query.EndTime = &req.EndTime
	}

	reader, err := s.backend.ReadSocketDataEvents(stream.Context(), query)
	if err != nil {
		s.logger.Error("Failed to read socket data events", "error", err, "job_uuid", req.JobUuid)
		return status.Errorf(codes.Internal, "failed to read socket data events: %v", err)
	}

	eventCount := 0
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-reader.Channel:
			if !ok {
				select {
				case err := <-reader.Error:
					if err != nil {
						return status.Errorf(codes.Internal, "error reading socket data events: %v", err)
					}
				default:
				}
				s.logger.Info("QuerySocketDataEvents completed", "job_uuid", req.JobUuid, "eventCount", eventCount)
				return nil
			}
			if err := stream.Send(socketDataEventIPCToGen(event)); err != nil {
				return status.Errorf(codes.Internal, "failed to send socket data event: %v", err)
			}
			eventCount++
		case err := <-reader.Error:
			if err != nil {
				return status.Errorf(codes.Internal, "error reading socket data events: %v", err)
			}
		}
	}
}
