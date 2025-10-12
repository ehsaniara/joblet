package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/core/execution"
	"github.com/ehsaniara/joblet/internal/joblet/core/execution/executionfakes"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/domain/values"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"
)

func TestExecutionCoordinator_StartJob_DefaultRuntime(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up mocks
	envManager.PrepareWorkspaceReturns("/test/workspace", nil)
	envManager.BuildEnvironmentReturns([]string{"TEST=1"})

	mockCmd := &platformfakes.FakeCommand{}
	processManager.LaunchProcessReturns(&execution.ProcessResult{
		Command: mockCmd,
		PID:     12345,
	}, nil)

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "python3",
		Args:    []string{"test.py"},
		Limits: func() domain.ResourceLimits {
			cpu, _ := values.NewCPUPercentage(100)
			memory, _ := values.NewMemorySizeFromMB(512)
			io, _ := values.NewBandwidth(1000)
			return domain.ResourceLimits{
				CPU:         cpu,
				Memory:      memory,
				IOBandwidth: io,
			}
		}(),
	}

	opts := &execution.StartProcessOptions{
		Job:     job,
		Uploads: []domain.FileUpload{},
	}

	result, err := coordinator.StartJob(context.Background(), opts)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Verify LaunchProcess was called with joblet binary init path
	if processManager.LaunchProcessCallCount() != 1 {
		t.Errorf("Expected LaunchProcess to be called once, got %d", processManager.LaunchProcessCallCount())
	}

	_, launchConfig := processManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/joblet" {
		t.Errorf("Expected init path '/opt/joblet/joblet', got '%s'", launchConfig.InitPath)
	}
	if launchConfig.JobID != "test-job-123" {
		t.Errorf("Expected job ID 'test-job-123', got '%s'", launchConfig.JobID)
	}
}

func TestExecutionCoordinator_StartJob_RuntimeJob(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up mocks
	envManager.PrepareWorkspaceReturns("/test/workspace", nil)
	envManager.BuildEnvironmentReturns([]string{"TEST=1"})

	mockCmd := &platformfakes.FakeCommand{}
	processManager.LaunchProcessReturns(&execution.ProcessResult{
		Command: mockCmd,
		PID:     12345,
	}, nil)

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "python3",
		Args:    []string{"test.py"},
		Runtime: "python-3.11-ml", // Runtime specified
		Limits: func() domain.ResourceLimits {
			cpu, _ := values.NewCPUPercentage(100)
			memory, _ := values.NewMemorySizeFromMB(512)
			io, _ := values.NewBandwidth(1000)
			return domain.ResourceLimits{
				CPU:         cpu,
				Memory:      memory,
				IOBandwidth: io,
			}
		}(),
	}

	opts := &execution.StartProcessOptions{
		Job:     job,
		Uploads: []domain.FileUpload{},
	}

	result, err := coordinator.StartJob(context.Background(), opts)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Critical assertion: Runtime jobs should use joblet binary, not runtime-specific init
	if processManager.LaunchProcessCallCount() != 1 {
		t.Errorf("Expected LaunchProcess to be called once, got %d", processManager.LaunchProcessCallCount())
	}

	_, launchConfig := processManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/joblet" {
		t.Errorf("Expected init path '/opt/joblet/joblet' for unified logging, got '%s'", launchConfig.InitPath)
	}

	// Verify runtime information is still passed through environment
	if envManager.BuildEnvironmentCallCount() != 1 {
		t.Errorf("Expected BuildEnvironment to be called once, got %d", envManager.BuildEnvironmentCallCount())
	}

	jobArg, phaseArg := envManager.BuildEnvironmentArgsForCall(0)
	if jobArg.Runtime != "python-3.11-ml" {
		t.Errorf("Expected runtime 'python:3.11-ml', got '%s'", jobArg.Runtime)
	}
	if phaseArg != "execute" {
		t.Errorf("Expected phase 'execute', got '%s'", phaseArg)
	}

	// Critical: Should not try to resolve runtime init path
	if envManager.GetRuntimeInitPathCallCount() != 0 {
		t.Errorf("Expected GetRuntimeInitPath NOT to be called, but it was called %d times", envManager.GetRuntimeInitPathCallCount())
	}
}

func TestExecutionCoordinator_StartJob_WithNetworking(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up mocks
	envManager.PrepareWorkspaceReturns("/test/workspace", nil)
	envManager.BuildEnvironmentReturns([]string{"TEST=1"})

	networkManager.SetupNetworkingReturns(&execution.NetworkAllocation{
		JobID:   "test-job-123",
		Network: "test-network",
	}, nil)

	mockCmd := &platformfakes.FakeCommand{}
	processManager.LaunchProcessReturns(&execution.ProcessResult{
		Command: mockCmd,
		PID:     12345,
	}, nil)

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "python3",
		Args:    []string{"test.py"},
		Runtime: "python:3.11-ml",
		Network: "test-network",
		Limits: func() domain.ResourceLimits {
			cpu, _ := values.NewCPUPercentage(100)
			memory, _ := values.NewMemorySizeFromMB(512)
			io, _ := values.NewBandwidth(1000)
			return domain.ResourceLimits{
				CPU:         cpu,
				Memory:      memory,
				IOBandwidth: io,
			}
		}(),
	}

	opts := &execution.StartProcessOptions{
		Job:     job,
		Uploads: []domain.FileUpload{},
	}

	result, err := coordinator.StartJob(context.Background(), opts)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Verify networking was set up
	if networkManager.SetupNetworkingCallCount() != 1 {
		t.Errorf("Expected SetupNetworking to be called once, got %d", networkManager.SetupNetworkingCallCount())
	}

	// Still use joblet binary for unified logging
	_, launchConfig := processManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/joblet" {
		t.Errorf("Expected init path '/opt/joblet/joblet' even with networking, got '%s'", launchConfig.InitPath)
	}
}

func TestExecutionCoordinator_StartJob_IsolationCreationFails(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up failure
	isolationManager.CreateIsolatedEnvironmentReturns(nil, errors.New("isolation failed"))

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	job := &domain.Job{
		Uuid: "test-job-123",
		Limits: func() domain.ResourceLimits {
			cpu, _ := values.NewCPUPercentage(100)
			memory, _ := values.NewMemorySizeFromMB(512)
			io, _ := values.NewBandwidth(1000)
			return domain.ResourceLimits{
				CPU:         cpu,
				Memory:      memory,
				IOBandwidth: io,
			}
		}(),
	}

	opts := &execution.StartProcessOptions{Job: job, Uploads: []domain.FileUpload{}}

	_, err := coordinator.StartJob(context.Background(), opts)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create isolated environment") {
		t.Errorf("Expected error about isolated environment, got %v", err)
	}

	// Should not call cleanup since workspace wasn't created
	if envManager.CleanupWorkspaceCallCount() != 0 {
		t.Errorf("Expected CleanupWorkspace not to be called, but it was called %d times", envManager.CleanupWorkspaceCallCount())
	}
}

func TestExecutionCoordinator_StartJob_WorkspacePreparationFails(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up failure
	envManager.PrepareWorkspaceReturns("", errors.New("workspace failed"))

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	job := &domain.Job{
		Uuid: "test-job-123",
		Limits: func() domain.ResourceLimits {
			cpu, _ := values.NewCPUPercentage(100)
			memory, _ := values.NewMemorySizeFromMB(512)
			io, _ := values.NewBandwidth(1000)
			return domain.ResourceLimits{
				CPU:         cpu,
				Memory:      memory,
				IOBandwidth: io,
			}
		}(),
	}

	opts := &execution.StartProcessOptions{Job: job, Uploads: []domain.FileUpload{}}

	_, err := coordinator.StartJob(context.Background(), opts)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to prepare workspace") {
		t.Errorf("Expected error about workspace preparation, got %v", err)
	}

	// Should cleanup isolation
	if isolationManager.DestroyIsolatedEnvironmentCallCount() != 1 {
		t.Errorf("Expected DestroyIsolatedEnvironment to be called once, got %d", isolationManager.DestroyIsolatedEnvironmentCallCount())
	}

	jobID := isolationManager.DestroyIsolatedEnvironmentArgsForCall(0)
	if jobID != "test-job-123" {
		t.Errorf("Expected job ID 'test-job-123', got '%s'", jobID)
	}
}

func TestExecutionCoordinator_StopJob_Success(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	err := coordinator.StopJob(context.Background(), "test-job-123")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify all cleanup methods were called
	if networkManager.CleanupNetworkingCallCount() != 1 {
		t.Errorf("Expected CleanupNetworking to be called once, got %d", networkManager.CleanupNetworkingCallCount())
	}
	if envManager.CleanupWorkspaceCallCount() != 1 {
		t.Errorf("Expected CleanupWorkspace to be called once, got %d", envManager.CleanupWorkspaceCallCount())
	}
	if isolationManager.DestroyIsolatedEnvironmentCallCount() != 1 {
		t.Errorf("Expected DestroyIsolatedEnvironment to be called once, got %d", isolationManager.DestroyIsolatedEnvironmentCallCount())
	}
}

func TestExecutionCoordinator_StopJob_ContinuesCleanupOnFailures(t *testing.T) {
	envManager := &executionfakes.FakeEnvironmentManager{}
	networkManager := &executionfakes.FakeNetworkManager{}
	processManager := &executionfakes.FakeProcessManager{}
	isolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	// Set up failure
	networkManager.CleanupNetworkingReturns(errors.New("network cleanup failed"))

	coordinator := execution.NewExecutionCoordinator(
		envManager,
		networkManager,
		processManager,
		isolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	err := coordinator.StopJob(context.Background(), "test-job-123")

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "cleanup errors") {
		t.Errorf("Expected error about cleanup errors, got %v", err)
	}

	// Should still call other cleanup methods
	if envManager.CleanupWorkspaceCallCount() != 1 {
		t.Errorf("Expected CleanupWorkspace to be called once, got %d", envManager.CleanupWorkspaceCallCount())
	}
	if isolationManager.DestroyIsolatedEnvironmentCallCount() != 1 {
		t.Errorf("Expected DestroyIsolatedEnvironment to be called once, got %d", isolationManager.DestroyIsolatedEnvironmentCallCount())
	}
}
