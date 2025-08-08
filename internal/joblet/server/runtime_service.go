package server

import (
	"context"

	pb "joblet/api/gen"
	"joblet/internal/joblet/auth"
	"joblet/internal/joblet/runtime"
	"joblet/pkg/logger"
	"joblet/pkg/platform"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RuntimeServiceServer implements the RuntimeService gRPC interface
type RuntimeServiceServer struct {
	pb.UnimplementedRuntimeServiceServer
	auth     auth.GrpcAuthorization
	resolver *runtime.Resolver
	logger   *logger.Logger
}

// Ensure we implement the interface
var _ pb.RuntimeServiceServer = (*RuntimeServiceServer)(nil)

// NewRuntimeServiceServer creates a new runtime service server
func NewRuntimeServiceServer(auth auth.GrpcAuthorization, runtimesBasePath string, platform platform.Platform) *RuntimeServiceServer {
	return &RuntimeServiceServer{
		auth:     auth,
		resolver: runtime.NewResolver(runtimesBasePath, platform),
		logger:   logger.New().WithField("component", "runtime-grpc"),
	}
}

// ListRuntimes returns all available runtimes
func (s *RuntimeServiceServer) ListRuntimes(ctx context.Context, req *pb.EmptyRequest) (*pb.RuntimesRes, error) {
	log := s.logger.WithField("operation", "ListRuntimes")
	log.Debug("list runtimes request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Get runtimes from resolver
	runtimeInfos, err := s.resolver.ListRuntimes()
	if err != nil {
		log.Error("failed to list runtimes", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list runtimes: %v", err)
	}

	// Convert to protobuf format
	pbRuntimes := make([]*pb.RuntimeInfo, 0, len(runtimeInfos))
	for _, info := range runtimeInfos {
		pbRuntime := &pb.RuntimeInfo{
			Name:        info.Name,
			Language:    info.Language,
			Version:     info.Version,
			Description: info.Description,
			SizeBytes:   info.Size,
			Packages:    []string{}, // Will be filled from runtime config if available
			Available:   info.Available,
			Requirements: &pb.RuntimeRequirements{
				Architectures: []string{"x86_64", "amd64"},
				Gpu:           false,
			},
		}

		pbRuntimes = append(pbRuntimes, pbRuntime)
	}

	log.Debug("runtimes listed successfully", "count", len(pbRuntimes))

	return &pb.RuntimesRes{
		Runtimes: pbRuntimes,
	}, nil
}

// GetRuntimeInfo returns detailed information about a specific runtime
func (s *RuntimeServiceServer) GetRuntimeInfo(ctx context.Context, req *pb.RuntimeInfoReq) (*pb.RuntimeInfoRes, error) {
	log := s.logger.WithFields("operation", "GetRuntimeInfo", "runtime", req.Runtime)
	log.Debug("get runtime info request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate request
	if req.Runtime == "" {
		return nil, status.Errorf(codes.InvalidArgument, "runtime name is required")
	}

	// Resolve runtime
	config, err := s.resolver.ResolveRuntime(req.Runtime)
	if err != nil {
		log.Debug("runtime not found", "error", err)
		return &pb.RuntimeInfoRes{
			Found: false,
		}, nil
	}

	// Convert to protobuf format
	pbRuntime := &pb.RuntimeInfo{
		Name:        config.Name,
		Language:    extractLanguageFromName(config.Name),
		Version:     config.Version,
		Description: config.Description,
		SizeBytes:   0, // Would need to calculate
		Packages:    config.Packages,
		Available:   true,
		Requirements: &pb.RuntimeRequirements{
			Architectures: config.Requirements.Architectures,
			Gpu:           config.Requirements.GPU,
		},
	}

	log.Debug("runtime info retrieved successfully")

	return &pb.RuntimeInfoRes{
		Runtime: pbRuntime,
		Found:   true,
	}, nil
}

// TestRuntime tests if a runtime is working correctly
func (s *RuntimeServiceServer) TestRuntime(ctx context.Context, req *pb.RuntimeTestReq) (*pb.RuntimeTestRes, error) {
	log := s.logger.WithFields("operation", "TestRuntime", "runtime", req.Runtime)
	log.Debug("test runtime request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate request
	if req.Runtime == "" {
		return nil, status.Errorf(codes.InvalidArgument, "runtime name is required")
	}

	// Try to resolve runtime
	_, err := s.resolver.ResolveRuntime(req.Runtime)
	if err != nil {
		log.Debug("runtime test failed - resolution error", "error", err)
		return &pb.RuntimeTestRes{
			Success:  false,
			Output:   "",
			Error:    err.Error(),
			ExitCode: 1,
		}, nil
	}

	// Basic test passed
	log.Debug("runtime test successful")
	return &pb.RuntimeTestRes{
		Success:  true,
		Output:   "Runtime resolution successful",
		Error:    "",
		ExitCode: 0,
	}, nil
}

// extractLanguageFromName extracts language from runtime name (e.g., "python-3.11-ml" -> "python")
func extractLanguageFromName(name string) string {
	// Simple extraction - take first part before hyphen
	if len(name) == 0 {
		return ""
	}

	for i, char := range name {
		if char == '-' {
			return name[:i]
		}
	}

	return name // No hyphen found, return whole name
}
