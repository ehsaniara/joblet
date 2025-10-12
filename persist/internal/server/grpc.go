package server

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/persist/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/security"
)

// GRPCServer is the gRPC server for persist service
type GRPCServer struct {
	persistpb.UnimplementedPersistServiceServer
	config   *config.ServerConfig
	backend  storage.Backend
	logger   *logger.Logger
	grpcSrv  *grpc.Server
	listener net.Listener
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(cfg *config.ServerConfig, backend storage.Backend, log *logger.Logger) *GRPCServer {
	return &GRPCServer{
		config:  cfg,
		backend: backend,
		logger:  log.WithField("component", "grpc-server"),
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.GRPCAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener

	// Create gRPC server with TLS if enabled
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(s.config.MaxConnections)),
	}

	// Add TLS credentials if enabled
	if s.config.TLS.Enabled {
		tlsCfg := security.TLSConfig{
			Enabled:    s.config.TLS.Enabled,
			CertFile:   s.config.TLS.CertFile,
			KeyFile:    s.config.TLS.KeyFile,
			CAFile:     s.config.TLS.CAFile,
			ClientAuth: s.config.TLS.ClientAuth,
		}

		tlsConfig, err := security.LoadServerTLSConfig(tlsCfg)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		s.logger.Info("TLS enabled", "clientAuth", s.config.TLS.ClientAuth)
	} else {
		s.logger.Warn("TLS is DISABLED - server running without encryption")
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
	s.logger.Debug("QueryLogs request", "jobID", req.JobId)

	// TODO: Implement log querying
	return fmt.Errorf("QueryLogs not implemented yet")
}

// QueryMetrics implements the QueryMetrics RPC
func (s *GRPCServer) QueryMetrics(req *persistpb.QueryMetricsRequest, stream persistpb.PersistService_QueryMetricsServer) error {
	s.logger.Debug("QueryMetrics request", "jobID", req.JobId)

	// TODO: Implement metrics querying
	return fmt.Errorf("QueryMetrics not implemented yet")
}

// NOTE: The following RPC methods are not yet implemented in persist.proto
// They are commented out until the proto definitions are added

// // GetJobInfo implements the GetJobInfo RPC
// func (s *GRPCServer) GetJobInfo(ctx context.Context, req *persistpb.GetJobInfoRequest) (*persistpb.GetJobInfoResponse, error) {
// 	s.logger.Debug("GetJobInfo request", "jobID", req.JobId)
//
// 	info, err := s.backend.GetJobInfo(req.JobId)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get job info: %w", err)
// 	}
//
// 	return &persistpb.GetJobInfoResponse{
// 		JobId:       info.JobID,
// 		CreatedAt:   info.CreatedAt,
// 		LastUpdated: info.LastUpdated,
// 		LogCount:    info.LogCount,
// 		MetricCount: info.MetricCount,
// 		SizeBytes:   info.SizeBytes,
// 	}, nil
// }
//
// // ListJobs implements the ListJobs RPC
// func (s *GRPCServer) ListJobs(ctx context.Context, req *persistpb.ListJobsRequest) (*persistpb.ListJobsResponse, error) {
// 	s.logger.Debug("ListJobs request", "since", req.Since, "until", req.Until)
//
// 	filter := &storage.JobFilter{
// 		Limit:  int(req.Limit),
// 		Offset: int(req.Offset),
// 	}
//
// 	if req.Since > 0 {
// 		since := req.Since
// 		filter.Since = &since
// 	}
//
// 	if req.Until > 0 {
// 		until := req.Until
// 		filter.Until = &until
// 	}
//
// 	jobIDs, err := s.backend.ListJobs(filter)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list jobs: %w", err)
// 	}
//
// 	// Get info for each job
// 	jobs := make([]*persistpb.JobInfo, 0, len(jobIDs))
// 	for _, jobID := range jobIDs {
// 		info, err := s.backend.GetJobInfo(jobID)
// 		if err != nil {
// 			s.logger.Warn("Failed to get job info", "jobID", jobID, "error", err)
// 			continue
// 		}
//
// 		jobs = append(jobs, &persistpb.JobInfo{
// 			JobId:       info.JobID,
// 			CreatedAt:   info.CreatedAt,
// 			LastUpdated: info.LastUpdated,
// 			LogCount:    info.LogCount,
// 			MetricCount: info.MetricCount,
// 			SizeBytes:   info.SizeBytes,
// 		})
// 	}
//
// 	return &persistpb.ListJobsResponse{
// 		Jobs:       jobs,
// 		TotalCount: int32(len(jobs)),
// 	}, nil
// }
//
// // DeleteJob implements the DeleteJob RPC
// func (s *GRPCServer) DeleteJob(ctx context.Context, req *persistpb.DeleteJobRequest) (*persistpb.DeleteJobResponse, error) {
// 	s.logger.Info("DeleteJob request", "jobID", req.JobId)
//
// 	if err := s.backend.DeleteJob(req.JobId); err != nil {
// 		return &persistpb.DeleteJobResponse{
// 			Success: false,
// 			Message: fmt.Sprintf("Failed to delete job: %v", err),
// 		}, nil
// 	}
//
// 	return &persistpb.DeleteJobResponse{
// 		Success: true,
// 		Message: "Job deleted successfully",
// 	}, nil
// }
//
// // GetStats implements the GetStats RPC
// func (s *GRPCServer) GetStats(ctx context.Context, req *persistpb.GetStatsRequest) (*persistpb.GetStatsResponse, error) {
// 	s.logger.Debug("GetStats request")
//
// 	// TODO: Implement stats collection
// 	return &persistpb.GetStatsResponse{
// 		TotalJobs:        0,
// 		TotalLogs:        0,
// 		TotalMetrics:     0,
// 		TotalSizeBytes:   0,
// 		MessagesReceived: 0,
// 		MessagesWritten:  0,
// 		WriteErrors:      0,
// 	}, nil
// }
//
// // CleanupOldData implements the CleanupOldData RPC
// func (s *GRPCServer) CleanupOldData(ctx context.Context, req *persistpb.CleanupRequest) (*persistpb.CleanupResponse, error) {
// 	s.logger.Info("CleanupOldData request", "dryRun", req.DryRun)
//
// 	// TODO: Implement cleanup logic
// 	return &persistpb.CleanupResponse{
// 		JobsDeleted:   0,
// 		BytesFreed:    0,
// 		DeletedJobIds: []string{},
// 	}, nil
// }
