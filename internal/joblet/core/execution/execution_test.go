package execution_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/core/execution"
	"github.com/ehsaniara/joblet/internal/joblet/core/execution/executionfakes"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"
)

// Test ExecutionCoordinator with unified logging - runtime jobs use joblet binary
func TestExecutionCoordinator_RuntimeInitPathResolution(t *testing.T) {
	// Setup
	fakeEnvManager := &executionfakes.FakeEnvironmentManager{}
	fakeNetworkManager := &executionfakes.FakeNetworkManager{}
	fakeProcessManager := &executionfakes.FakeProcessManager{}
	fakeIsolationManager := &executionfakes.FakeIsolationManager{}
	fakeCommand := &platformfakes.FakeCommand{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		fakeEnvManager,
		fakeNetworkManager,
		fakeProcessManager,
		fakeIsolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	// Setup mocks for success path
	fakeIsolationManager.CreateIsolatedEnvironmentReturns(&execution.IsolationContext{
		JobID:        "test-job-123",
		WorkspaceDir: "/tmp/test-workspace",
	}, nil)
	fakeEnvManager.PrepareWorkspaceReturns("/tmp/test-workspace", nil)
	fakeEnvManager.BuildEnvironmentReturns([]string{"TEST=value"})
	// Note: GetRuntimeInitPath should NOT be called for unified logging
	fakeProcessManager.LaunchProcessReturns(&execution.ProcessResult{
		Command: fakeCommand,
		PID:     12345,
	}, nil)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "python3",
		Args:    []string{"script.py"},
		Runtime: "python-3.11-ml",
	}

	opts := &execution.StartProcessOptions{
		Job:     job,
		Uploads: []domain.FileUpload{},
	}

	// Test
	result, err := coordinator.StartJob(context.Background(), opts)

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != fakeCommand {
		t.Fatalf("Expected command result, got: %v", result)
	}

	// Verify launch config uses joblet binary for unified logging
	if fakeProcessManager.LaunchProcessCallCount() != 1 {
		t.Errorf("Expected LaunchProcess to be called once, got: %d", fakeProcessManager.LaunchProcessCallCount())
	}
	_, launchConfig := fakeProcessManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/bin/joblet" {
		t.Errorf("Expected InitPath to be '/opt/joblet/bin/joblet' for unified logging, got: %s", launchConfig.InitPath)
	}
}

func TestExecutionCoordinator_NoRuntimeFallback(t *testing.T) {
	// Setup
	fakeEnvManager := &executionfakes.FakeEnvironmentManager{}
	fakeNetworkManager := &executionfakes.FakeNetworkManager{}
	fakeProcessManager := &executionfakes.FakeProcessManager{}
	fakeIsolationManager := &executionfakes.FakeIsolationManager{}
	fakeCommand := &platformfakes.FakeCommand{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		fakeEnvManager,
		fakeNetworkManager,
		fakeProcessManager,
		fakeIsolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	// Setup mocks
	fakeIsolationManager.CreateIsolatedEnvironmentReturns(&execution.IsolationContext{}, nil)
	fakeEnvManager.PrepareWorkspaceReturns("/tmp/test-workspace", nil)
	fakeEnvManager.BuildEnvironmentReturns([]string{"TEST=value"})
	fakeProcessManager.LaunchProcessReturns(&execution.ProcessResult{Command: fakeCommand, PID: 12345}, nil)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo",
		Args:    []string{"hello"},
		Runtime: "", // No runtime - should use fallback
	}

	opts := &execution.StartProcessOptions{Job: job}

	// Test
	result, err := coordinator.StartJob(context.Background(), opts)

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != fakeCommand {
		t.Fatalf("Expected command result, got: %v", result)
	}

	// Verify launch config uses joblet binary for two-stage execution (not command path)
	_, launchConfig := fakeProcessManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/bin/joblet" {
		t.Errorf("Expected InitPath to be '/opt/joblet/bin/joblet' for two-stage execution, got: %s", launchConfig.InitPath)
	}
}

func TestExecutionCoordinator_RuntimeJobsSucceedWithoutRuntimeResolution(t *testing.T) {
	// This test verifies that runtime jobs succeed without needing runtime init path resolution
	// because we now use unified logging with joblet binary for all jobs

	// Setup
	fakeEnvManager := &executionfakes.FakeEnvironmentManager{}
	fakeNetworkManager := &executionfakes.FakeNetworkManager{}
	fakeProcessManager := &executionfakes.FakeProcessManager{}
	fakeIsolationManager := &executionfakes.FakeIsolationManager{}
	fakeCommand := &platformfakes.FakeCommand{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		fakeEnvManager,
		fakeNetworkManager,
		fakeProcessManager,
		fakeIsolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	// Setup mocks for success path (no GetRuntimeInitPath needed)
	fakeIsolationManager.CreateIsolatedEnvironmentReturns(&execution.IsolationContext{}, nil)
	fakeEnvManager.PrepareWorkspaceReturns("/tmp/test-workspace", nil)
	fakeEnvManager.BuildEnvironmentReturns([]string{"TEST=value"})
	fakeProcessManager.LaunchProcessReturns(&execution.ProcessResult{
		Command: fakeCommand,
		PID:     12345,
	}, nil)

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "python3",
		Runtime: "python-3.11-ml",
	}
	opts := &execution.StartProcessOptions{Job: job}

	// Test
	result, err := coordinator.StartJob(context.Background(), opts)

	// Verify success
	if err != nil {
		t.Fatalf("Expected no error with unified logging, got: %v", err)
	}
	if result != fakeCommand {
		t.Fatalf("Expected command result, got: %v", result)
	}

	// Verify process was launched with joblet binary
	if fakeProcessManager.LaunchProcessCallCount() != 1 {
		t.Errorf("Expected LaunchProcess to be called once, got: %d", fakeProcessManager.LaunchProcessCallCount())
	}

	_, launchConfig := fakeProcessManager.LaunchProcessArgsForCall(0)
	if launchConfig.InitPath != "/opt/joblet/bin/joblet" {
		t.Errorf("Expected InitPath to be '/opt/joblet/bin/joblet' for unified logging, got: %s", launchConfig.InitPath)
	}
}

func TestExecutionCoordinator_StopJob(t *testing.T) {
	// Setup
	fakeEnvManager := &executionfakes.FakeEnvironmentManager{}
	fakeNetworkManager := &executionfakes.FakeNetworkManager{}
	fakeProcessManager := &executionfakes.FakeProcessManager{}
	fakeIsolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		fakeEnvManager,
		fakeNetworkManager,
		fakeProcessManager,
		fakeIsolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	// Setup mocks for successful cleanup
	fakeNetworkManager.CleanupNetworkingReturns(nil)
	fakeEnvManager.CleanupWorkspaceReturns(nil)
	fakeIsolationManager.DestroyIsolatedEnvironmentReturns(nil)

	// Test
	err := coordinator.StopJob(context.Background(), "test-job-123")

	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify all cleanup operations were called in proper order (reverse order)
	if fakeNetworkManager.CleanupNetworkingCallCount() != 1 {
		t.Errorf("Expected CleanupNetworking to be called once, got: %d", fakeNetworkManager.CleanupNetworkingCallCount())
	}
	if fakeEnvManager.CleanupWorkspaceCallCount() != 1 {
		t.Errorf("Expected CleanupWorkspace to be called once, got: %d", fakeEnvManager.CleanupWorkspaceCallCount())
	}
	if fakeIsolationManager.DestroyIsolatedEnvironmentCallCount() != 1 {
		t.Errorf("Expected DestroyIsolatedEnvironment to be called once, got: %d", fakeIsolationManager.DestroyIsolatedEnvironmentCallCount())
	}

	// Verify correct job ID was passed to all cleanup methods
	_, jobID := fakeNetworkManager.CleanupNetworkingArgsForCall(0)
	if jobID != "test-job-123" {
		t.Errorf("Expected jobID 'test-job-123' for network cleanup, got: %s", jobID)
	}

	envJobID := fakeEnvManager.CleanupWorkspaceArgsForCall(0)
	if envJobID != "test-job-123" {
		t.Errorf("Expected jobID 'test-job-123' for workspace cleanup, got: %s", envJobID)
	}

	isoJobID := fakeIsolationManager.DestroyIsolatedEnvironmentArgsForCall(0)
	if isoJobID != "test-job-123" {
		t.Errorf("Expected jobID 'test-job-123' for isolation cleanup, got: %s", isoJobID)
	}
}

func TestExecutionCoordinator_StopJobWithErrors(t *testing.T) {
	// Setup
	fakeEnvManager := &executionfakes.FakeEnvironmentManager{}
	fakeNetworkManager := &executionfakes.FakeNetworkManager{}
	fakeProcessManager := &executionfakes.FakeProcessManager{}
	fakeIsolationManager := &executionfakes.FakeIsolationManager{}
	testLogger := logger.New()

	coordinator := execution.NewExecutionCoordinator(
		fakeEnvManager,
		fakeNetworkManager,
		fakeProcessManager,
		fakeIsolationManager,
		&executionfakes.FakeGPUManager{},
		&platformfakes.FakePlatform{},
		testLogger,
	)

	// Setup mocks to fail
	fakeNetworkManager.CleanupNetworkingReturns(errors.New("network cleanup failed"))
	fakeEnvManager.CleanupWorkspaceReturns(errors.New("workspace cleanup failed"))
	fakeIsolationManager.DestroyIsolatedEnvironmentReturns(errors.New("isolation cleanup failed"))

	// Test
	err := coordinator.StopJob(context.Background(), "test-job-123")

	// Verify error aggregation
	if err == nil {
		t.Fatal("Expected error when cleanup operations fail, got nil")
	}
	errorMessage := err.Error()
	if !strings.Contains(errorMessage, "cleanup errors") {
		t.Errorf("Expected aggregated cleanup errors, got: %v", err)
	}
	if !strings.Contains(errorMessage, "network cleanup failed") {
		t.Errorf("Expected network cleanup error in aggregated message, got: %v", err)
	}
	if !strings.Contains(errorMessage, "workspace cleanup failed") {
		t.Errorf("Expected workspace cleanup error in aggregated message, got: %v", err)
	}
	if !strings.Contains(errorMessage, "isolation cleanup failed") {
		t.Errorf("Expected isolation cleanup error in aggregated message, got: %v", err)
	}

	// Verify all cleanup operations were still attempted despite failures
	if fakeNetworkManager.CleanupNetworkingCallCount() != 1 {
		t.Errorf("Expected CleanupNetworking to be called once despite failures, got: %d", fakeNetworkManager.CleanupNetworkingCallCount())
	}
	if fakeEnvManager.CleanupWorkspaceCallCount() != 1 {
		t.Errorf("Expected CleanupWorkspace to be called once despite failures, got: %d", fakeEnvManager.CleanupWorkspaceCallCount())
	}
	if fakeIsolationManager.DestroyIsolatedEnvironmentCallCount() != 1 {
		t.Errorf("Expected DestroyIsolatedEnvironment to be called once despite failures, got: %d", fakeIsolationManager.DestroyIsolatedEnvironmentCallCount())
	}
}
