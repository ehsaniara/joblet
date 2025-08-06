package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "joblet/api/gen"
	"joblet/internal/joblet/adapters"
	auth2 "joblet/internal/joblet/auth"
	"joblet/pkg/logger"
)

// NetworkServiceServer implements the gRPC network service
type NetworkServiceServer struct {
	pb.UnimplementedNetworkServiceServer
	auth         auth2.GrpcAuthorization
	networkStore adapters.NetworkStoreAdapter
	logger       *logger.Logger
}

// NewNetworkServiceServer creates a new network service server
func NewNetworkServiceServer(auth auth2.GrpcAuthorization, networkStore adapters.NetworkStoreAdapter) *NetworkServiceServer {
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

	// Create network config for the adapter
	networkConfig := &adapters.NetworkConfig{
		Name:      req.Name,
		Type:      "custom",
		CIDR:      req.Cidr,
		Gateway:   "", // Will be calculated by adapter
		DNS:       []string{},
		Metadata:  make(map[string]string),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Create the network
	if err := s.networkStore.CreateNetwork(networkConfig); err != nil {
		log.Error("failed to create network", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to create network: %v", err)
	}

	// Get network info for response
	network, exists := s.networkStore.GetNetwork(req.Name)
	if !exists {
		return nil, status.Errorf(codes.Internal, "network created but not found")
	}

	log.Info("network created successfully")

	return &pb.CreateNetworkRes{
		Name:   network.Name,
		Cidr:   network.CIDR,
		Bridge: network.BridgeName,
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

	for _, network := range networks {
		// Count jobs in this network for compatibility
		jobsInNetwork := s.networkStore.ListJobsInNetwork(network.Name)
		jobCount := int32(len(jobsInNetwork))

		resp.Networks = append(resp.Networks, &pb.Network{
			Name:     network.Name,
			Cidr:     network.CIDR,
			Bridge:   network.BridgeName,
			JobCount: jobCount,
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
