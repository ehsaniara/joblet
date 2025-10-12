package execution_test

import (
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/core/execution"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"
)

func TestStartProcessOptions(t *testing.T) {
	job := &domain.Job{
		Uuid:    "test-job",
		Command: "python3",
		Args:    []string{"script.py"},
		Runtime: "python-3.11-ml",
	}

	uploads := []domain.FileUpload{
		{
			Path:    "script.py",
			Content: []byte("print('hello')"),
		},
	}

	opts := &execution.StartProcessOptions{
		Job:               job,
		Uploads:           uploads,
		EnableStreaming:   true,
		WorkspaceDir:      "/tmp/workspace",
		PreProcessUploads: false,
	}

	// Verify struct fields are accessible
	if opts.Job != job {
		t.Errorf("Expected job to match, got different job")
	}
	if len(opts.Uploads) != 1 {
		t.Errorf("Expected 1 upload, got %d", len(opts.Uploads))
	}
	if !opts.EnableStreaming {
		t.Errorf("Expected streaming to be enabled")
	}
	if opts.WorkspaceDir != "/tmp/workspace" {
		t.Errorf("Expected workspace dir '/tmp/workspace', got: %s", opts.WorkspaceDir)
	}
	if opts.PreProcessUploads {
		t.Errorf("Expected PreProcessUploads to be false")
	}
}

func TestLaunchConfig(t *testing.T) {
	config := &execution.LaunchConfig{
		InitPath:    "/usr/bin/python3",
		Environment: []string{"PATH=/usr/bin", "HOME=/root"},
		JobID:       "test-job-123",
		Command:     "python3",
		Args:        []string{"script.py", "--verbose"},
	}

	// Verify struct fields
	if config.InitPath != "/usr/bin/python3" {
		t.Errorf("Expected InitPath '/usr/bin/python3', got: %s", config.InitPath)
	}
	if config.JobID != "test-job-123" {
		t.Errorf("Expected JobID 'test-job-123', got: %s", config.JobID)
	}
	if config.Command != "python3" {
		t.Errorf("Expected Command 'python3', got: %s", config.Command)
	}
	if len(config.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(config.Args))
	}
	if len(config.Environment) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(config.Environment))
	}
}

func TestProcessResult(t *testing.T) {
	fakeCommand := &platformfakes.FakeCommand{}

	result := &execution.ProcessResult{
		Command: fakeCommand,
		PID:     12345,
	}

	// Verify struct fields
	if result.Command != fakeCommand {
		t.Errorf("Expected command to match fake command")
	}
	if result.PID != 12345 {
		t.Errorf("Expected PID 12345, got: %d", result.PID)
	}
}

func TestNetworkAllocation(t *testing.T) {
	allocation := &execution.NetworkAllocation{
		JobID:    "job-123",
		Network:  "test-network",
		IP:       "192.168.1.100",
		Hostname: "job-123.test-network",
	}

	// Verify struct fields
	if allocation.JobID != "job-123" {
		t.Errorf("Expected JobID 'job-123', got: %s", allocation.JobID)
	}
	if allocation.Network != "test-network" {
		t.Errorf("Expected Network 'test-network', got: %s", allocation.Network)
	}
	if allocation.IP != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100', got: %s", allocation.IP)
	}
	if allocation.Hostname != "job-123.test-network" {
		t.Errorf("Expected Hostname 'job-123.test-network', got: %s", allocation.Hostname)
	}
}

func TestIsolationContext(t *testing.T) {
	context := &execution.IsolationContext{
		JobID:        "job-123",
		Namespace:    "job-ns-123",
		CgroupPath:   "/sys/fs/cgroup/joblet/job-123",
		WorkspaceDir: "/tmp/jobs/job-123/work",
	}

	// Verify struct fields
	if context.JobID != "job-123" {
		t.Errorf("Expected JobID 'job-123', got: %s", context.JobID)
	}
	if context.Namespace != "job-ns-123" {
		t.Errorf("Expected Namespace 'job-ns-123', got: %s", context.Namespace)
	}
	if context.CgroupPath != "/sys/fs/cgroup/joblet/job-123" {
		t.Errorf("Expected CgroupPath '/sys/fs/cgroup/joblet/job-123', got: %s", context.CgroupPath)
	}
	if context.WorkspaceDir != "/tmp/jobs/job-123/work" {
		t.Errorf("Expected WorkspaceDir '/tmp/jobs/job-123/work', got: %s", context.WorkspaceDir)
	}
}
