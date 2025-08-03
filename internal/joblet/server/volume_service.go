package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "joblet/api/gen"
	auth2 "joblet/internal/joblet/auth"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
)

// VolumeServiceServer implements the gRPC volume service
type VolumeServiceServer struct {
	pb.UnimplementedVolumeServiceServer
	auth          auth2.GrpcAuthorization
	volumeManager *volume.Manager
	logger        *logger.Logger
}

// NewVolumeServiceServer creates a new volume service server
func NewVolumeServiceServer(auth auth2.GrpcAuthorization, volumeManager *volume.Manager) *VolumeServiceServer {
	return &VolumeServiceServer{
		auth:          auth,
		volumeManager: volumeManager,
		logger:        logger.WithField("component", "volume-grpc"),
	}
}

// CreateVolume creates a new volume
func (s *VolumeServiceServer) CreateVolume(ctx context.Context, req *pb.CreateVolumeReq) (*pb.CreateVolumeRes, error) {
	log := s.logger.WithFields(
		"operation", "CreateVolume",
		"name", req.Name,
		"size", req.Size,
		"type", req.Type)

	log.Debug("create volume request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Create the volume
	volume, err := s.volumeManager.CreateVolume(req.Name, req.Size, domain.VolumeType(req.Type))
	if err != nil {
		log.Error("failed to create volume", "error", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to create volume: %v", err)
	}

	log.Info("volume created successfully")

	return &pb.CreateVolumeRes{
		Name: volume.Name,
		Size: volume.Size,
		Type: string(volume.Type),
		Path: volume.Path,
	}, nil
}

// ListVolumes returns all available volumes
func (s *VolumeServiceServer) ListVolumes(ctx context.Context, req *pb.EmptyRequest) (*pb.Volumes, error) {
	log := s.logger.WithField("operation", "ListVolumes")

	if err := s.auth.Authorized(ctx, auth2.StreamJobsOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	volumes := s.volumeManager.ListVolumes()

	resp := &pb.Volumes{
		Volumes: make([]*pb.Volume, 0, len(volumes)),
	}

	for _, vol := range volumes {
		resp.Volumes = append(resp.Volumes, &pb.Volume{
			Name:        vol.Name,
			Size:        vol.Size,
			Type:        string(vol.Type),
			Path:        vol.Path,
			CreatedTime: vol.CreatedTime.Format("2006-01-02T15:04:05Z07:00"),
			JobCount:    vol.JobCount,
		})
	}

	return resp, nil
}

// RemoveVolume removes a volume
func (s *VolumeServiceServer) RemoveVolume(ctx context.Context, req *pb.RemoveVolumeReq) (*pb.RemoveVolumeRes, error) {
	log := s.logger.WithFields(
		"operation", "RemoveVolume",
		"name", req.Name)

	log.Debug("remove volume request received")

	if err := s.auth.Authorized(ctx, auth2.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if err := s.volumeManager.RemoveVolume(req.Name); err != nil {
		log.Error("failed to remove volume", "error", err)
		return &pb.RemoveVolumeRes{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Info("volume removed successfully")

	return &pb.RemoveVolumeRes{
		Success: true,
		Message: "Volume removed successfully",
	}, nil
}
