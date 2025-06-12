//go:build linux

package worker

import (
	"context"
	"errors"
	"job-worker/internal/config"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/interfaces/interfacesfakes"
	"job-worker/pkg/logger"
	"job-worker/pkg/os/osfakes"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	w := New(fakeStore)

	assert.NotNil(t, w)
	_, ok := w.(interfaces.JobWorker)
	assert.True(t, ok)
}

func TestStartJob_Success_InitProcess(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCmdFactory := &osfakes.FakeCommandFactory{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeCmd := &osfakes.FakeCommand{}
	fakeProcess := &osfakes.FakeProcess{}
	fakeCgroup := &interfacesfakes.FakeResource{}
	fakeOs := &osfakes.FakeOsInterface{}

	fakeCmdFactory.CreateCommandReturns(fakeCmd)
	fakeCmd.ProcessReturns(fakeProcess)
	fakeProcess.PidReturns(1234)

	fakeOs.EnvironReturns([]string{"PATH=/usr/bin"})
	fakeOs.ExecutableReturns("/usr/bin/job-worker", nil)
	fakeOs.StatReturns(nil, nil) // job-init exists

	fakeSyscall.CreateProcessGroupReturns(&syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	})
	fakeCgroup.CreateReturns(nil)

	w := &linuxWorker{
		store:      fakeStore,
		cgroup:     fakeCgroup,
		cmdFactory: fakeCmdFactory,
		syscall:    fakeSyscall,
		os:         fakeOs,
		logger:     logger.WithField("component", "worker-test"), // Add logger initialization
	}

	ctx := context.Background()

	job, err := w.StartJob(ctx, "echo", []string{"hello", "world"}, 50, 100, 1000)

	require.NoError(t, err)
	assert.NotNil(t, job)

	// should contain "echo" somewhere in the path
	assert.Contains(t, job.Command, "echo")
	assert.Equal(t, []string{"hello", "world"}, job.Args)
	assert.Equal(t, int32(50), job.Limits.MaxCPU)
	assert.Equal(t, int32(100), job.Limits.MaxMemory)
	assert.Equal(t, int32(1000), job.Limits.MaxIOBPS)
	assert.Equal(t, domain.StatusRunning, job.Status)
	assert.Equal(t, int32(1234), job.Pid)

	// verify init process was started not user command directly
	assert.Equal(t, 1, fakeCmdFactory.CreateCommandCallCount())
	command, args := fakeCmdFactory.CreateCommandArgsForCall(0)

	// should be job-init path, not "echo"
	assert.Contains(t, command, "job-init")
	// no args to job-init (config via env vars)
	assert.Empty(t, args)

	// verify environment was set with job config
	assert.Equal(t, 1, fakeCmd.SetEnvCallCount())
	env := fakeCmd.SetEnvArgsForCall(0)

	// check for required environment variables
	hasJobID := false
	hasJobCommand := false
	hasJobCgroupPath := false
	hasJobArgsCount := false

	for _, envVar := range env {
		if strings.HasPrefix(envVar, "JOB_ID=") {
			hasJobID = true
		}
		// The command will be resolved, so check if it contains "echo"
		if strings.HasPrefix(envVar, "JOB_COMMAND=") && strings.Contains(envVar, "echo") {
			hasJobCommand = true
		}
		if strings.HasPrefix(envVar, "JOB_CGROUP_PATH=") {
			hasJobCgroupPath = true
		}
		if strings.HasPrefix(envVar, "JOB_ARGS_COUNT=2") {
			hasJobArgsCount = true
		}
	}

	assert.True(t, hasJobID, "Should set JOB_ID environment variable")
	assert.True(t, hasJobCommand, "Should set JOB_COMMAND environment variable with resolved echo path")
	assert.True(t, hasJobCgroupPath, "Should set JOB_CGROUP_PATH environment variable")
	assert.True(t, hasJobArgsCount, "Should set JOB_ARGS_COUNT environment variable")

	// verify other interactions
	assert.Equal(t, 1, fakeCmd.StartCallCount())
	assert.Equal(t, 1, fakeCmd.SetSysProcAttrCallCount())
	assert.Equal(t, 1, fakeStore.CreateNewJobCallCount())
	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
}

func TestStartJob_JobInitNotFound(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCgroup := &interfacesfakes.FakeResource{}
	fakeOs := &osfakes.FakeOsInterface{}

	fakeOs.ExecutableReturns("/usr/bin/job-w", nil)
	// job-init doesn't exist
	fakeOs.StatReturns(nil, errors.New("file not found"))
	fakeCgroup.CreateReturns(nil)

	w := &linuxWorker{
		store:  fakeStore,
		cgroup: fakeCgroup,
		os:     fakeOs,
		logger: logger.WithField("component", "w-test"), // add logger initialization
	}

	ctx := context.Background()
	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 50, 100, 1000)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "job-init binary not found")

	// should clean up cgroup
	assert.Equal(t, 1, fakeCgroup.CleanupCgroupCallCount())
}

func TestStartJob_ContextCancelled(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	w := &linuxWorker{
		store:  fakeStore,
		logger: logger.WithField("component", "worker-test"),
	}

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 50, 100, 1000)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Equal(t, context.Canceled, err)
}

func TestStartJob_CgroupCreationFails(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCgroup := &interfacesfakes.FakeResource{}

	fakeCgroup.CreateReturns(errors.New("cgroup creation failed"))

	w := &linuxWorker{
		store:  fakeStore,
		cgroup: fakeCgroup,
		logger: logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 50, 100, 1000)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "failed to create cgroup for job")
}

func TestStartJob_InitProcessStartFails(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCmdFactory := &osfakes.FakeCommandFactory{}
	fakeCmd := &osfakes.FakeCommand{}
	fakeCgroup := &interfacesfakes.FakeResource{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeOs := &osfakes.FakeOsInterface{}

	fakeCmdFactory.CreateCommandReturns(fakeCmd)
	fakeCmd.StartReturns(errors.New("init process start failed"))
	fakeCgroup.CreateReturns(nil)
	fakeOs.ExecutableReturns("/usr/bin/job-worker", nil)
	// job-init exists
	fakeOs.StatReturns(nil, nil)
	fakeOs.EnvironReturns([]string{})
	fakeSyscall.CreateProcessGroupReturns(&syscall.SysProcAttr{})

	w := &linuxWorker{
		store:      fakeStore,
		cgroup:     fakeCgroup,
		cmdFactory: fakeCmdFactory,
		syscall:    fakeSyscall,
		os:         fakeOs,
		logger:     logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	job, err := w.StartJob(ctx, "echo", []string{}, 50, 100, 1000)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "start init process")

	// should have cleaned up cgroup
	assert.Equal(t, 1, fakeCgroup.CleanupCgroupCallCount())

	// should have updated job to failed status
	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
	updatedJob := fakeStore.UpdateJobArgsForCall(0)
	assert.Equal(t, domain.StatusFailed, updatedJob.Status)
}

func TestStartJob_ProcessNil(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCmdFactory := &osfakes.FakeCommandFactory{}
	fakeCmd := &osfakes.FakeCommand{}
	fakeCgroup := &interfacesfakes.FakeResource{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeOs := &osfakes.FakeOsInterface{}

	fakeCmdFactory.CreateCommandReturns(fakeCmd)
	fakeCmd.ProcessReturns(nil) // process is nil
	fakeCgroup.CreateReturns(nil)
	fakeOs.ExecutableReturns("/usr/bin/job-worker", nil)
	fakeOs.StatReturns(nil, nil)
	fakeOs.EnvironReturns([]string{})
	fakeSyscall.CreateProcessGroupReturns(&syscall.SysProcAttr{})

	w := &linuxWorker{
		store:      fakeStore,
		cgroup:     fakeCgroup,
		cmdFactory: fakeCmdFactory,
		syscall:    fakeSyscall,
		os:         fakeOs,
		logger:     logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 50, 100, 1000)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "process is nil after start")
}

func TestStartJob_DefaultValues(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeCmdFactory := &osfakes.FakeCommandFactory{}
	fakeCmd := &osfakes.FakeCommand{}
	fakeProcess := &osfakes.FakeProcess{}
	fakeCgroup := &interfacesfakes.FakeResource{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeOs := &osfakes.FakeOsInterface{}

	fakeCmdFactory.CreateCommandReturns(fakeCmd)
	fakeCmd.ProcessReturns(fakeProcess)
	fakeProcess.PidReturns(1234)
	fakeCgroup.CreateReturns(nil)
	fakeOs.ExecutableReturns("/usr/bin/job-worker", nil)
	fakeOs.StatReturns(nil, nil)
	fakeOs.EnvironReturns([]string{})
	fakeSyscall.CreateProcessGroupReturns(&syscall.SysProcAttr{})

	w := &linuxWorker{
		store:      fakeStore,
		cgroup:     fakeCgroup,
		cmdFactory: fakeCmdFactory,
		syscall:    fakeSyscall,
		os:         fakeOs,
		logger:     logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	// passing 0 values to test defaults
	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 0, 0, 0)

	require.NoError(t, err)
	assert.Equal(t, config.DefaultCPULimitPercent, job.Limits.MaxCPU)
	assert.Equal(t, config.DefaultMemoryLimitMB, job.Limits.MaxMemory)
	assert.Equal(t, config.DefaultIOBPS, job.Limits.MaxIOBPS)
}

func TestStopJob_Success(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeCgroup := &interfacesfakes.FakeResource{}

	existingJob := &domain.Job{
		Id:         "1111",
		Status:     domain.StatusRunning,
		Pid:        1234,
		CgroupPath: "/sys/fs/cgroup/job-test-job",
	}
	fakeStore.GetJobReturns(existingJob, true)

	// make process not exist after SIGTERM (graceful shutdown)
	fakeSyscall.KillReturnsOnCall(0, nil)           // SIGTERM succeeds
	fakeSyscall.KillReturnsOnCall(1, syscall.ESRCH) // process doesn't exist (graceful shutdown)

	w := &linuxWorker{
		store:   fakeStore,
		syscall: fakeSyscall,
		cgroup:  fakeCgroup,
		logger:  logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	err := w.StopJob(ctx, "1111")

	assert.NoError(t, err)

	// should have tried SIGTERM first
	assert.Equal(t, 2, fakeSyscall.KillCallCount())
	pid1, sig1 := fakeSyscall.KillArgsForCall(0)
	assert.Equal(t, -1234, pid1) // negative for process group
	assert.Equal(t, syscall.SIGTERM, sig1)

	// should have checked if process exists
	pid2, sig2 := fakeSyscall.KillArgsForCall(1)
	assert.Equal(t, 1234, pid2) // positive for existence check
	assert.Equal(t, syscall.Signal(0), sig2)

	// should have updated job to STOPPED with exit code 0 (graceful)
	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
	updatedJob := fakeStore.UpdateJobArgsForCall(0)
	assert.Equal(t, domain.StatusStopped, updatedJob.Status)
	assert.Equal(t, int32(0), updatedJob.ExitCode)

	// should have cleaned up cgroup
	assert.Equal(t, 1, fakeCgroup.CleanupCgroupCallCount())
}

func TestStopJob_JobNotFound(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	// job doesn't exist
	fakeStore.GetJobReturns(nil, false)

	w := &linuxWorker{
		store:  fakeStore,
		logger: logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	err := w.StopJob(ctx, "nonexistent-job")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found: nonexistent-job")
}

func TestStopJob_JobNotRunning(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	// job exists but is not running
	existingJob := &domain.Job{
		Id:     "1111",
		Status: domain.StatusCompleted,
		Pid:    1234,
	}
	fakeStore.GetJobReturns(existingJob, true)

	w := &linuxWorker{
		store:  fakeStore,
		logger: logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	err := w.StopJob(ctx, "1111")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job is not running")
}

func TestStopJob_ContextCancelled(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	w := &linuxWorker{
		store:  fakeStore,
		logger: logger.WithField("component", "worker-test"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := w.StopJob(ctx, "1111")

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStopJob_ForcedKill(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}
	fakeSyscall := &osfakes.FakeSyscallInterface{}
	fakeCgroup := &interfacesfakes.FakeResource{}

	existingJob := &domain.Job{
		Id:         "1111",
		Status:     domain.StatusRunning,
		Pid:        1234,
		CgroupPath: "/sys/fs/cgroup/job-test-job",
	}
	fakeStore.GetJobReturns(existingJob, true)

	// process survives SIGTERM but dies to SIGKILL
	fakeSyscall.KillReturnsOnCall(0, nil) // SIGTERM succeeds
	fakeSyscall.KillReturnsOnCall(1, nil) // Process still exists check
	fakeSyscall.KillReturnsOnCall(2, nil) // SIGKILL succeeds

	w := &linuxWorker{
		store:   fakeStore,
		syscall: fakeSyscall,
		cgroup:  fakeCgroup,
		logger:  logger.WithField("component", "worker-test"),
	}

	ctx := context.Background()
	err := w.StopJob(ctx, "1111")

	assert.NoError(t, err)

	// should have tried SIGTERM, checked existence, then SIGKILL
	assert.Equal(t, 3, fakeSyscall.KillCallCount())

	// first call: SIGTERM to process group
	pid1, sig1 := fakeSyscall.KillArgsForCall(0)
	assert.Equal(t, -1234, pid1)
	assert.Equal(t, syscall.SIGTERM, sig1)

	// second call: Check if process exists
	pid2, sig2 := fakeSyscall.KillArgsForCall(1)
	assert.Equal(t, 1234, pid2)
	assert.Equal(t, syscall.Signal(0), sig2)

	// third call: SIGKILL to process group
	pid3, sig3 := fakeSyscall.KillArgsForCall(2)
	assert.Equal(t, -1234, pid3)
	assert.Equal(t, syscall.SIGKILL, sig3)

	// should have updated job to STOPPED (forced kill has no specific exit code in domain)
	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
	updatedJob := fakeStore.UpdateJobArgsForCall(0)
	assert.Equal(t, domain.StatusStopped, updatedJob.Status)

	// should have cleaned up cgroup
	assert.Equal(t, 1, fakeCgroup.CleanupCgroupCallCount())
}

func TestProcessExists(t *testing.T) {
	tests := []struct {
		name     string
		killErr  error
		expected bool
	}{
		{"Process exists", nil, true},
		{"Process doesn't exist", syscall.ESRCH, false},
		{"No permission but exists", syscall.EPERM, true},
		{"Other error", errors.New("other"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeSyscall := &osfakes.FakeSyscallInterface{}
			fakeSyscall.KillReturns(tt.killErr)

			w := &linuxWorker{
				syscall: fakeSyscall,
				logger:  logger.WithField("component", "worker-test"),
			}

			result := w.processExists(1234)

			assert.Equal(t, tt.expected, result)

			// should have called Kill with signal 0
			assert.Equal(t, 1, fakeSyscall.KillCallCount())
			pid, sig := fakeSyscall.KillArgsForCall(0)
			assert.Equal(t, 1234, pid)
			assert.Equal(t, syscall.Signal(0), sig)
		})
	}
}
