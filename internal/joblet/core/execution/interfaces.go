package execution

import (
	"context"
	"os"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/platform"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// JobExecutor handles the core job execution logic
//
//counterfeiter:generate . JobExecutor
type JobExecutor interface {
	StartJob(ctx context.Context, opts *StartProcessOptions) (platform.Command, error)
	StopJob(ctx context.Context, jobID string) error
}

// EnvironmentManager handles job environment setup
//
//counterfeiter:generate . EnvironmentManager
type EnvironmentManager interface {
	BuildEnvironment(job *domain.Job, phase string) []string
	PrepareWorkspace(jobID string, uploads []domain.FileUpload) (string, error)
	CleanupWorkspace(jobID string) error
	DetectCUDA() ([]string, error)
	GetCUDAEnvironment(cudaPath string) map[string]string
}

// NetworkManager handles job networking setup
//
//counterfeiter:generate . NetworkManager
type NetworkManager interface {
	SetupNetworking(ctx context.Context, jobID, networkName string) (*NetworkAllocation, error)
	ConfigureNetworkNamespace(ctx context.Context, jobID string, pid int) error
	CleanupNetworking(ctx context.Context, jobID string) error
}

// ProcessManager handles process lifecycle
//
//counterfeiter:generate . ProcessManager
type ProcessManager interface {
	LaunchProcess(ctx context.Context, config *LaunchConfig) (*ProcessResult, error)
	KillProcess(pid int) error
}

// IsolationManager handles security isolation
//
//counterfeiter:generate . IsolationManager
type IsolationManager interface {
	CreateIsolatedEnvironment(jobID string) (*IsolationContext, error)
	CreateBuilderEnvironment(jobID string) (*IsolationContext, error)
	DestroyIsolatedEnvironment(jobID string) error
	CreateGPUDeviceNodes(jobID string, gpuIndices []int) error
	MountCUDALibraries(jobID string, cudaPath string) error
}

// StartProcessOptions contains options for starting a process
type StartProcessOptions struct {
	Job               *domain.Job
	Uploads           []domain.FileUpload
	EnableStreaming   bool
	WorkspaceDir      string
	PreProcessUploads bool
}

// LaunchConfig contains process launch configuration
type LaunchConfig struct {
	InitPath    string
	Environment []string
	SysProcAttr interface{}
	Stdout      interface{}
	Stderr      interface{}
	JobID       string
	JobType     domain.JobType // Job type for isolation configuration
	Command     string
	Args        []string
	ExtraFiles  []*os.File
}

// ProcessResult contains process launch results
type ProcessResult struct {
	Command platform.Command
	PID     int
}

// NetworkAllocation represents network allocation details
type NetworkAllocation struct {
	JobID    string
	Network  string
	IP       string
	Hostname string
}

// IsolationContext contains isolation environment details
type IsolationContext struct {
	JobID        string
	Namespace    string
	CgroupPath   string
	WorkspaceDir string
	IsBuilder    bool // True for runtime build environments
}
