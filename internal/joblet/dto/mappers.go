package dto

import (
	"fmt"
	"net"

	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/network"
)

// JobMapper handles conversions between domain.Job and JobDTO
type JobMapper struct{}

// ToDTO converts domain.Job to JobDTO
func (m *JobMapper) ToDTO(job *domain.Job) *JobDTO {
	if job == nil {
		return nil
	}

	maxCPU, maxMemory, maxIOBPS, cpuCores := job.ResourceLimitsToDTO()

	return &JobDTO{
		Uuid:              job.Uuid,
		Name:              job.Name,
		Command:           job.Command,
		Args:              job.Args,
		Status:            string(job.Status),
		Pid:               job.Pid,
		StartTime:         job.StartTime,
		EndTime:           job.EndTime,
		ExitCode:          job.ExitCode,
		ScheduledTime:     job.ScheduledTime,
		Network:           job.Network,
		Volumes:           job.Volumes,
		Runtime:           job.Runtime,
		Environment:       job.Environment,
		SecretEnvironment: job.MaskedSecretEnvironment(), // Use job's method instead of mapper doing the work
		ResourceLimits: ResourceLimitsDTO{
			MaxCPU:    maxCPU,
			MaxMemory: maxMemory,
			MaxIOBPS:  maxIOBPS,
			CPUCores:  cpuCores,
		},
	}
}

// ToDomain converts JobDTO to domain.Job
func (m *JobMapper) ToDomain(dto *JobDTO) (*domain.Job, error) {
	if dto == nil {
		return nil, nil
	}

	// Create resource limits
	limits := domain.NewResourceLimitsFromParams(
		dto.ResourceLimits.MaxCPU,
		dto.ResourceLimits.CPUCores,
		dto.ResourceLimits.MaxMemory,
		dto.ResourceLimits.MaxIOBPS,
	)

	return &domain.Job{
		Uuid:              dto.Uuid,
		Name:              dto.Name,
		Command:           dto.Command,
		Args:              dto.Args,
		Limits:            *limits,
		Status:            domain.JobStatus(dto.Status),
		Pid:               dto.Pid,
		StartTime:         dto.StartTime,
		EndTime:           dto.EndTime,
		ExitCode:          dto.ExitCode,
		ScheduledTime:     dto.ScheduledTime,
		Network:           dto.Network,
		Volumes:           dto.Volumes,
		Runtime:           dto.Runtime,
		Environment:       dto.Environment,
		SecretEnvironment: dto.SecretEnvironment, // Note: This would be empty from transport
	}, nil
}

// ToStatusDTO converts domain.Job to JobStatusDTO
func (m *JobMapper) ToStatusDTO(job *domain.Job) *JobStatusDTO {
	if job == nil {
		return nil
	}

	return &JobStatusDTO{
		Uuid:      job.Uuid,
		Status:    string(job.Status),
		StartTime: job.FormattedStartTime(),
		EndTime:   job.FormattedEndTime(),
		Duration:  job.FormattedDuration(),
		ExitCode:  job.ExitCode,
	}
}

// ToListItemDTO converts domain.Job to JobListItemDTO
func (m *JobMapper) ToListItemDTO(job *domain.Job) *JobListItemDTO {
	if job == nil {
		return nil
	}

	return &JobListItemDTO{
		Uuid:      job.Uuid,
		Name:      job.Name,
		Command:   job.Command,
		Status:    string(job.Status),
		StartTime: job.FormattedStartTime(),
		Duration:  job.FormattedDuration(),
		Network:   job.Network,
		Runtime:   job.Runtime,
	}
}

// RequestMapper handles conversions for request DTOs
type RequestMapper struct{}

// ToStartJobRequest converts StartJobRequestDTO to interfaces.StartJobRequest
func (m *RequestMapper) ToStartJobRequest(dto *StartJobRequestDTO) (*interfaces.StartJobRequest, error) {
	if dto == nil {
		return nil, fmt.Errorf("request DTO cannot be nil")
	}

	// Convert file uploads
	var uploads []domain.FileUpload
	for _, uploadDTO := range dto.Uploads {
		uploads = append(uploads, domain.FileUpload{
			Path:        uploadDTO.Path,
			Content:     uploadDTO.Content,
			Size:        uploadDTO.Size,
			Mode:        uploadDTO.Mode,
			IsDirectory: uploadDTO.IsDirectory,
		})
	}

	return &interfaces.StartJobRequest{
		Name:    dto.Name,
		Command: dto.Command,
		Args:    dto.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    dto.Resources.MaxCPU,
			MaxMemory: dto.Resources.MaxMemory,
			MaxIOBPS:  int32(dto.Resources.MaxIOBPS),
			CPUCores:  dto.Resources.CPUCores,
		},
		Uploads:           uploads,
		Schedule:          dto.Schedule,
		Network:           dto.Network,
		Volumes:           dto.Volumes,
		Runtime:           dto.Runtime,
		Environment:       dto.Environment,
		SecretEnvironment: dto.SecretEnvironment,
	}, nil
}

// ToStartJobRequestDTO converts interfaces.StartJobRequest to StartJobRequestDTO
func (m *RequestMapper) ToStartJobRequestDTO(req *interfaces.StartJobRequest) *StartJobRequestDTO {
	if req == nil {
		return nil
	}

	// Convert file uploads
	var uploads []FileUploadDTO
	for _, upload := range req.Uploads {
		uploads = append(uploads, FileUploadDTO{
			Path:        upload.Path,
			Content:     upload.Content,
			Size:        upload.Size,
			Mode:        upload.Mode,
			IsDirectory: upload.IsDirectory,
		})
	}

	return &StartJobRequestDTO{
		Name:    req.Name,
		Command: req.Command,
		Args:    req.Args,
		Resources: ResourceLimitsDTO{
			MaxCPU:    req.Resources.MaxCPU,
			MaxMemory: req.Resources.MaxMemory,
			MaxIOBPS:  int64(req.Resources.MaxIOBPS),
			CPUCores:  req.Resources.CPUCores,
		},
		Uploads:           uploads,
		Schedule:          req.Schedule,
		Network:           req.Network,
		Volumes:           req.Volumes,
		Runtime:           req.Runtime,
		Environment:       req.Environment,
		SecretEnvironment: req.SecretEnvironment,
	}
}

// ToStopJobRequest converts StopJobRequestDTO to interfaces.StopJobRequest
func (m *RequestMapper) ToStopJobRequest(dto *StopJobRequestDTO) *interfaces.StopJobRequest {
	if dto == nil {
		return nil
	}

	return &interfaces.StopJobRequest{
		JobID:  dto.JobID,
		Force:  dto.Force,
		Reason: dto.Reason,
	}
}

// ToDeleteJobRequest converts DeleteJobRequestDTO to interfaces.DeleteJobRequest
func (m *RequestMapper) ToDeleteJobRequest(dto *DeleteJobRequestDTO) *interfaces.DeleteJobRequest {
	if dto == nil {
		return nil
	}

	return &interfaces.DeleteJobRequest{
		JobID:  dto.JobID,
		Reason: dto.Reason,
	}
}

// VolumeMapper handles conversions between domain.Volume and VolumeDTO
type VolumeMapper struct{}

// ToDTO converts domain.Volume to VolumeDTO
func (m *VolumeMapper) ToDTO(volume *domain.Volume) *VolumeDTO {
	if volume == nil {
		return nil
	}

	return &VolumeDTO{
		Name:        volume.Name,
		Type:        string(volume.Type),
		Size:        volume.Size,
		SizeBytes:   volume.SizeBytes,
		Path:        volume.Path,
		CreatedTime: volume.CreatedTime,
		JobCount:    volume.JobCount,
		MountPath:   volume.MountPath(), // Use volume's own method
		InUse:       volume.IsInUse(),   // Use volume's own method
	}
}

// ToDomain converts VolumeDTO to domain.Volume
func (m *VolumeMapper) ToDomain(dto *VolumeDTO) (*domain.Volume, error) {
	if dto == nil {
		return nil, nil
	}

	volume, err := domain.NewVolume(dto.Name, dto.Size, domain.VolumeType(dto.Type))
	if err != nil {
		return nil, fmt.Errorf("failed to create volume from DTO: %w", err)
	}

	volume.Path = dto.Path
	volume.CreatedTime = dto.CreatedTime
	volume.JobCount = dto.JobCount

	return volume, nil
}

// ToListItemDTO converts domain.Volume to VolumeListItemDTO
func (m *VolumeMapper) ToListItemDTO(volume *domain.Volume) *VolumeListItemDTO {
	if volume == nil {
		return nil
	}

	return &VolumeListItemDTO{
		Name:        volume.Name,
		Type:        string(volume.Type),
		Size:        volume.Size,
		JobCount:    volume.JobCount,
		CreatedTime: volume.FormattedCreatedTime(), // Use volume's own formatting method
		InUse:       volume.IsInUse(),              // Use volume's own method
	}
}

// NetworkMapper handles conversions between network types and NetworkDTOs
type NetworkMapper struct{}

// ConfigToDTO converts network.NetworkConfig to NetworkConfigDTO
func (m *NetworkMapper) ConfigToDTO(config *network.NetworkConfig) *NetworkConfigDTO {
	if config == nil {
		return nil
	}

	return &NetworkConfigDTO{
		CIDR:   config.CIDR,
		Bridge: config.Bridge,
	}
}

// ConfigToDomain converts NetworkConfigDTO to network.NetworkConfig
func (m *NetworkMapper) ConfigToDomain(dto *NetworkConfigDTO) *network.NetworkConfig {
	if dto == nil {
		return nil
	}

	return &network.NetworkConfig{
		CIDR:   dto.CIDR,
		Bridge: dto.Bridge,
	}
}

// InfoToDTO converts network.NetworkInfo to NetworkInfoDTO
func (m *NetworkMapper) InfoToDTO(info *network.NetworkInfo) *NetworkInfoDTO {
	if info == nil {
		return nil
	}

	return &NetworkInfoDTO{
		Name:     info.Name,
		CIDR:     info.CIDR,
		Bridge:   info.Bridge,
		JobCount: info.JobCount,
		Status:   "active", // Could be determined from job count or other factors
	}
}

// InfoToDomain converts NetworkInfoDTO to network.NetworkInfo
func (m *NetworkMapper) InfoToDomain(dto *NetworkInfoDTO) *network.NetworkInfo {
	if dto == nil {
		return nil
	}

	return &network.NetworkInfo{
		Name:     dto.Name,
		CIDR:     dto.CIDR,
		Bridge:   dto.Bridge,
		JobCount: dto.JobCount,
	}
}

// AllocationToDTO converts network.JobAllocation to JobAllocationDTO
func (m *NetworkMapper) AllocationToDTO(allocation *network.JobAllocation) *JobAllocationDTO {
	if allocation == nil {
		return nil
	}

	var ipStr string
	if allocation.IP != nil {
		ipStr = allocation.IP.String()
	}

	return &JobAllocationDTO{
		JobID:    allocation.JobID,
		Network:  allocation.Network,
		IP:       ipStr,
		Hostname: allocation.Hostname,
		VethHost: allocation.VethHost,
		VethPeer: allocation.VethPeer,
	}
}

// AllocationToDomain converts JobAllocationDTO to network.JobAllocation
func (m *NetworkMapper) AllocationToDomain(dto *JobAllocationDTO) *network.JobAllocation {
	if dto == nil {
		return nil
	}

	var ip net.IP
	if dto.IP != "" {
		ip = net.ParseIP(dto.IP)
	}

	return &network.JobAllocation{
		JobID:    dto.JobID,
		Network:  dto.Network,
		IP:       ip,
		Hostname: dto.Hostname,
		VethHost: dto.VethHost,
		VethPeer: dto.VethPeer,
	}
}

// BandwidthStatsToDTO converts network.BandwidthStats to BandwidthStatsDTO
func (m *NetworkMapper) BandwidthStatsToDTO(stats *network.BandwidthStats) *BandwidthStatsDTO {
	if stats == nil {
		return nil
	}

	return &BandwidthStatsDTO{
		Interface:       stats.Interface,
		BytesSent:       stats.BytesSent,
		BytesReceived:   stats.BytesReceived,
		PacketsSent:     stats.PacketsSent,
		PacketsReceived: stats.PacketsReceived,
	}
}
