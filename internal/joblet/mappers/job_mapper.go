package mappers

import (
	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
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
		Uuid:              job.Uuid,
		Command:           job.Command,
		Args:              job.Args,
		MaxCpu:            job.Limits.CPU.Value(),
		CpuCores:          job.Limits.CPUCores.String(),
		MaxMemory:         job.Limits.Memory.Megabytes(),
		MaxIoBps:          int32(job.Limits.IOBandwidth.BytesPerSecond()),
		Status:            string(job.Status),
		StartTime:         job.FormattedStartTime(), // Use job's formatting method
		ExitCode:          job.ExitCode,
		Runtime:           job.Runtime,
		Environment:       job.Environment,
		SecretEnvironment: job.SecretEnvironment,
		GpuIndices:        job.GPUIndices,         // GPU allocation info
		GpuCount:          job.GPUCount,           // GPU requirements
		GpuMemoryMb:       int32(job.GPUMemoryMB), // GPU memory requirement
		NodeId:            job.NodeId,             // Unique identifier of the Joblet node
	}

	if job.Timeout > 0 {
		pbJob.Timeout = job.Timeout.String()
	}

	pbJob.EndTime = job.FormattedEndTime()             // Use job's formatting method
	pbJob.ScheduledTime = job.FormattedScheduledTime() // Use job's formatting method

	return pbJob
}
