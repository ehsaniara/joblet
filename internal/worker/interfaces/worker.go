package interfaces

import (
	"context"
	"job-worker/internal/worker/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Store
type Store interface {
	CreateNewJob(job *domain.Job)
	UpdateJob(job *domain.Job)
	GetJob(id string) (*domain.Job, bool)
	ListJobs() []*domain.Job
	WriteToBuffer(jobId string, chunk []byte)
	GetOutput(id string) ([]byte, bool, error)
	SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error
}

//counterfeiter:generate . Resource
type Resource interface {
	Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error
	SetIOLimit(cgroupPath string, ioBPS int) error
	SetCPULimit(cgroupPath string, cpuLimit int) error
	SetMemoryLimit(cgroupPath string, memoryLimitMB int) error
	CleanupCgroup(jobID string)
}

//counterfeiter:generate . JobWorker
type JobWorker interface {
	StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error)
	StopJob(ctx context.Context, jobId string) error
}

//counterfeiter:generate . DomainStreamer
type DomainStreamer interface {
	SendData(data []byte) error
	SendKeepalive() error
	Context() context.Context
}
