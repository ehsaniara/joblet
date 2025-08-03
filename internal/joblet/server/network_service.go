package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "joblet/api/gen"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/state"
	"joblet/pkg/logger"
)

// NetworkServiceServer implements the gRPC network service
type NetworkServiceServer struct {
	pb.UnimplementedNetworkServiceServer
	auth         auth2.GrpcAuthorization
	networkStore *state.NetworkStore
	logger       *logger.Logger
}

// NewNetworkServiceServer creates a new network service server
func NewNetworkServiceServer(auth auth2.GrpcAuthorization, networkStore *state.NetworkStore) *NetworkServiceServer {
	return &NetworkServiceServer{
		auth:         auth,
		networkStore: networkStore,
		logger:       logger.WithField("component", "network-grpc"),
	}
}

// CreateNetwork creates a new custom network
func (s *NetworkServiceServer) CreateNetwork(ctx context.Context, req *pb.CreateNetworkReq) (*pb.CreateNetworkRes, error) {
	log := s.logger.WithFields(
		"operation", "CreateNetwork",
		"name", req.Name,
		"cidr", req.Cidr)

	log.Debug("create network request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create the network
	if err := s.networkStore.CreateNetwork(req.Name, req.Cidr); err != nil {
		log.Error("failed to create network", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to create network: %v", err)
	}

	// Get network info for response
	networks := s.networkStore.ListNetworks()
	netInfo, exists := networks[req.Name]
	if !exists {
		return nil, status.Errorf(codes.Internal, "network created but not found")
	}

	log.Info("network created successfully")

	return &pb.CreateNetworkRes{
		Name:   netInfo.Name,
		Cidr:   netInfo.CIDR,
		Bridge: netInfo.Bridge,
	}, nil
}

// ListNetworks returns all available networks
func (s *NetworkServiceServer) ListNetworks(ctx context.Context, req *pb.EmptyRequest) (*pb.Networks, error) {
	log := s.logger.WithField("operation", "ListNetworks")

	if err := s.auth.Authorized(ctx, auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	networks := s.networkStore.ListNetworks()

	resp := &pb.Networks{
		Networks: make([]*pb.Network, 0, len(networks)),
	}

	for _, net := range networks {
		resp.Networks = append(resp.Networks, &pb.Network{
			Name:     net.Name,
			Cidr:     net.CIDR,
			Bridge:   net.Bridge,
			JobCount: int32(net.JobCount),
		})
	}

	return resp, nil
}

// RemoveNetwork removes a custom network
func (s *NetworkServiceServer) RemoveNetwork(ctx context.Context, req *pb.RemoveNetworkReq) (*pb.RemoveNetworkRes, error) {
	log := s.logger.WithFields(
		"operation", "RemoveNetwork",
		"name", req.Name)

	log.Debug("remove network request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if err := s.networkStore.RemoveNetwork(req.Name); err != nil {
		log.Error("failed to remove network", "error", err)
		return &pb.RemoveNetworkRes{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Info("network removed successfully")

	return &pb.RemoveNetworkRes{
		Success: true,
		Message: "Network removed successfully",
	}, nil
}
