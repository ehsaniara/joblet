package mappers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "joblet/api/gen"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
)

// JobMapper handles mapping between domain and protobuf with value object support
type JobMapper struct{}

// NewJobMapper creates a new mapper instance
func NewJobMapper() *JobMapper {
	return &JobMapper{}
}

// DomainToProtobuf converts domain Job to protobuf Job
func (m *JobMapper) DomainToProtobuf(job *domain.Job) *pb.Job {
	pbJob := &pb.Job{
		Id:        job.Id,
		Command:   job.Command,
		Args:      job.Args,
		MaxCPU:    job.Limits.CPU.Value(),
		CpuCores:  job.Limits.CPUCores.String(),
		MaxMemory: job.Limits.Memory.Megabytes(),
		MaxIOBPS:  int32(job.Limits.IOBandwidth.BytesPerSecond()),
		Status:    string(job.Status),
		StartTime: job.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExitCode:  job.ExitCode,
	}

	if job.EndTime != nil {
		pbJob.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	if job.ScheduledTime != nil {
		pbJob.ScheduledTime = job.ScheduledTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return pbJob
}

// ProtobufToDomain converts protobuf Job to domain Job
func (m *JobMapper) ProtobufToDomain(pbJob *pb.Job) (*domain.Job, error) {
	// Create resource limits with simple approach
	limits := domain.NewResourceLimitsFromParams(
		pbJob.MaxCPU,
		pbJob.CpuCores,
		pbJob.MaxMemory,
		int64(pbJob.MaxIOBPS),
	)

	job := &domain.Job{
		Id:         pbJob.Id,
		Command:    pbJob.Command,
		Args:       pbJob.Args,
		Limits:     *limits,
		Status:     domain.JobStatus(pbJob.Status),
		ExitCode:   pbJob.ExitCode,
		CgroupPath: "", // Not in protobuf
		Pid:        0,  // Not in protobuf
	}

	// Parse times
	if pbJob.StartTime != "" {
		startTime, err := parseTime(pbJob.StartTime)
		if err == nil {
			job.StartTime = startTime
		}
	}

	if pbJob.EndTime != "" {
		endTime, err := parseTime(pbJob.EndTime)
		if err == nil {
			job.EndTime = &endTime
		}
	}

	if pbJob.ScheduledTime != "" {
		scheduledTime, err := parseTime(pbJob.ScheduledTime)
		if err == nil {
			job.ScheduledTime = &scheduledTime
		}
	}

	return job, nil
}

// DomainToRunJobResponse converts domain Job to RunJobRes
func (m *JobMapper) DomainToRunJobResponse(job *domain.Job) *pb.RunJobRes {
	response := &pb.RunJobRes{
		Id:        job.Id,
		Command:   job.Command,
		Args:      job.Args,
		MaxCPU:    job.Limits.CPU.Value(),
		CpuCores:  job.Limits.CPUCores.String(),
		MaxMemory: job.Limits.Memory.Megabytes(),
		MaxIOBPS:  int32(job.Limits.IOBandwidth.BytesPerSecond()),
		Status:    string(job.Status),
		StartTime: job.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		ExitCode:  job.ExitCode,
	}

	if job.EndTime != nil {
		response.EndTime = job.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}

	if job.ScheduledTime != nil {
		response.ScheduledTime = job.ScheduledTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}

// RequestToResourceLimits converts request parameters to ResourceLimits
func (m *JobMapper) RequestToResourceLimits(maxCPU, maxMemory, maxIOBPS int32, cpuCores string) (*domain.ResourceLimits, error) {
	limits := domain.NewResourceLimitsFromParams(maxCPU, cpuCores, maxMemory, int64(maxIOBPS))
	return limits, nil
}

// ParseResourceString parses string representations of resources
func (m *JobMapper) ParseResourceString(cpu, memory, bandwidth string) (*domain.ResourceLimits, error) {
	var cpuVal int32

	// Parse CPU if provided (e.g., "50%", "200%")
	if cpu != "" {
		cpu = strings.TrimSuffix(cpu, "%")
		if val, err := strconv.Atoi(cpu); err == nil {
			cpuVal = int32(val)
		}
	}

	limits := domain.NewResourceLimitsFromParams(cpuVal, "", 0, 0)
	return limits, nil
}

// RequestObjectToResourceLimits converts request object to value objects
func (m *JobMapper) RequestObjectToResourceLimits(req interfaces.ResourceLimits) (*domain.ResourceLimits, error) {
	limits := domain.NewResourceLimitsFromParams(req.MaxCPU, req.CPUCores, req.MaxMemory, int64(req.MaxIOBPS))
	return limits, nil
}

// ResourceLimitsToRequestObject converts value objects to request object
func (m *JobMapper) ResourceLimitsToRequestObject(limits *domain.ResourceLimits) interfaces.ResourceLimits {
	if limits == nil {
		return interfaces.ResourceLimits{}
	}

	return interfaces.ResourceLimits{
		MaxCPU:    limits.CPU.Value(),
		MaxMemory: limits.Memory.Megabytes(),
		MaxIOBPS:  int32(limits.IOBandwidth.BytesPerSecond()),
		CPUCores:  limits.CPUCores.String(),
	}
}

// ParseUserInputToValueObjects parses user string input to value objects
func (m *JobMapper) ParseUserInputToValueObjects(cpuStr, memoryStr, bandwidthStr, coresStr string) (*domain.ResourceLimits, error) {
	var cpuVal int32

	// Parse CPU percentage from string
	if cpuStr != "" {
		cpuStr = strings.TrimSuffix(cpuStr, "%")
		if val, err := strconv.Atoi(cpuStr); err == nil {
			cpuVal = int32(val)
		} else {
			return nil, fmt.Errorf("invalid CPU percentage: %s", cpuStr)
		}
	}

	limits := domain.NewResourceLimitsFromParams(cpuVal, coresStr, 0, 0)
	return limits, nil
}

// ValueObjectsToDisplayStrings converts value objects to human-readable strings
func (m *JobMapper) ValueObjectsToDisplayStrings(limits *domain.ResourceLimits) map[string]string {
	if limits == nil {
		return map[string]string{}
	}

	return map[string]string{
		"cpu":       limits.CPU.String(),
		"memory":    limits.Memory.String(),
		"bandwidth": limits.IOBandwidth.String(),
		"cores":     limits.CPUCores.String(),
	}
}

// ProtobufToStartJobRequest converts protobuf request to domain request object
func (m *JobMapper) ProtobufToStartJobRequest(req *pb.RunJobReq) (*interfaces.StartJobRequest, error) {
	// Convert resource limits using value objects
	resourceLimits, err := m.RequestToResourceLimits(req.MaxCPU, req.MaxMemory, req.MaxIOBPS, req.CpuCores)
	if err != nil {
		return nil, err
	}

	// Convert uploads - simplified for now
	var domainUploads []domain.FileUpload
	for _, upload := range req.Uploads {
		domainUploads = append(domainUploads, domain.FileUpload{
			Path:    upload.Path,
			Content: upload.Content,
			Size:    int64(len(upload.Content)),
		})
	}

	// Set default network
	network := req.Network
	if network == "" {
		network = "bridge"
	}

	return &interfaces.StartJobRequest{
		Command: req.Command,
		Args:    req.Args,
		Resources: interfaces.ResourceLimits{
			MaxCPU:    resourceLimits.CPU.Value(),
			MaxMemory: resourceLimits.Memory.Megabytes(),
			MaxIOBPS:  int32(resourceLimits.IOBandwidth.BytesPerSecond()),
			CPUCores:  resourceLimits.CPUCores.String(),
		},
		Uploads:  domainUploads,
		Schedule: req.Schedule,
		Network:  network,
		Volumes:  req.Volumes,
	}, nil
}

// StartJobRequestToProtobuf converts domain request object to protobuf
func (m *JobMapper) StartJobRequestToProtobuf(req *interfaces.StartJobRequest) *pb.RunJobReq {
	// Convert uploads to protobuf format
	var pbUploads []*pb.FileUpload
	for _, upload := range req.Uploads {
		pbUploads = append(pbUploads, &pb.FileUpload{
			Path:        upload.Path,
			Content:     upload.Content,
			Mode:        upload.Mode,
			IsDirectory: upload.IsDirectory,
		})
	}

	return &pb.RunJobReq{
		Command:   req.Command,
		Args:      req.Args,
		MaxCPU:    req.Resources.MaxCPU,
		MaxMemory: req.Resources.MaxMemory,
		MaxIOBPS:  req.Resources.MaxIOBPS,
		CpuCores:  req.Resources.CPUCores,
		Uploads:   pbUploads,
		Schedule:  req.Schedule,
		Network:   req.Network,
		Volumes:   req.Volumes,
	}
}

// Helper to parse time strings
func parseTime(timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05Z07:00", timeStr)
}
