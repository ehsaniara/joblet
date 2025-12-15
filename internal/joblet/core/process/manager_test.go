//go:build linux

package process

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Runtime: config.RuntimeConfig{
			CommonPaths: []string{"/usr/bin", "/usr/local/bin"},
		},
	}
}

func TestNewProcessManager(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()

	mgr := NewProcessManager(fakePlatform, cfg)

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.platform)
	assert.NotNil(t, mgr.config)
	assert.NotNil(t, mgr.logger)
	assert.NotNil(t, mgr.uploadManager)
}

func TestValidateCommand(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		command   string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid command",
			command:   "/usr/bin/echo",
			expectErr: false,
		},
		{
			name:      "empty command",
			command:   "",
			expectErr: true,
			errMsg:    "command cannot be empty",
		},
		{
			name:      "command with semicolon",
			command:   "echo; rm -rf /",
			expectErr: true,
			errMsg:    "dangerous characters",
		},
		{
			name:      "command with pipe",
			command:   "cat | grep",
			expectErr: true,
			errMsg:    "dangerous characters",
		},
		{
			name:      "command with backtick",
			command:   "echo `whoami`",
			expectErr: true,
			errMsg:    "dangerous characters",
		},
		{
			name:      "command with dollar",
			command:   "echo $HOME",
			expectErr: true,
			errMsg:    "dangerous characters",
		},
		{
			name:      "command too long",
			command:   string(make([]byte, 2000)),
			expectErr: true,
			errMsg:    "command too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ValidateCommand(tt.command)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateArguments(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "valid args",
			args:      []string{"arg1", "arg2", "--flag"},
			expectErr: false,
		},
		{
			name:      "empty args",
			args:      []string{},
			expectErr: false,
		},
		{
			name:      "args with null byte",
			args:      []string{"arg1", "arg\x00injection"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ValidateArguments(tt.args)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveCommand(t *testing.T) {
	// Create a temporary executable for testing
	tmpDir, err := os.MkdirTemp("", "resolve-cmd-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a fake executable
	execPath := filepath.Join(tmpDir, "testcmd")
	err = os.WriteFile(execPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.LookPathStub = func(file string) (string, error) {
		if file == "testcmd" {
			return execPath, nil
		}
		return "", os.ErrNotExist
	}

	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			CommonPaths: []string{tmpDir, "/usr/bin"},
		},
	}
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		command   string
		expectErr bool
		expected  string
	}{
		{
			name:      "absolute path",
			command:   execPath,
			expectErr: false,
			expected:  execPath,
		},
		{
			name:      "resolve via PATH",
			command:   "testcmd",
			expectErr: false,
			expected:  execPath,
		},
		{
			name:      "empty command",
			command:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := mgr.ResolveCommand(tt.command)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, resolved)
			}
		})
	}
}

func TestCreateSysProcAttr(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.CreateProcessGroupStub = func() *syscall.SysProcAttr {
		return &syscall.SysProcAttr{
			Setpgid: true,
		}
	}

	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("with network namespace", func(t *testing.T) {
		attr := mgr.CreateSysProcAttr(true)
		assert.NotNil(t, attr)
		// Should have network namespace flag
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWNET != 0)
	})

	t.Run("without network namespace", func(t *testing.T) {
		attr := mgr.CreateSysProcAttr(false)
		assert.NotNil(t, attr)
		// Should not have network namespace flag
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWNET == 0)
	})

	t.Run("always has required namespaces", func(t *testing.T) {
		attr := mgr.CreateSysProcAttr(false)
		// Should always have PID, Mount, IPC, UTS, Cgroup namespaces
		// Note: We don't use CLONE_NEWUSER because it breaks mounts.
		// Privilege dropping happens via setuid/setgid before exec.
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWPID != 0)
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWNS != 0)
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWIPC != 0)
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWUTS != 0)
		assert.True(t, attr.Cloneflags&syscall.CLONE_NEWCGROUP != 0)
	})
}

func TestBuildJobEnvironment(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.EnvironStub = func() []string {
		return []string{"PATH=/usr/bin", "HOME=/root"}
	}

	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	// Create resource limits using the domain helper
	limits := domain.NewResourceLimitsFromParams(50, "", 512, 1000000)

	job := &domain.Job{
		Uuid:       "test-job-123",
		Command:    "/usr/bin/echo",
		Args:       []string{"hello", "world"},
		CgroupPath: "/sys/fs/cgroup/joblet/test-job-123",
		Limits:     *limits,
	}

	env := mgr.BuildJobEnvironment(job, "/opt/joblet/bin/joblet")

	// Check base environment is included
	assert.Contains(t, env, "PATH=/usr/bin")
	assert.Contains(t, env, "HOME=/root")

	// Check job-specific environment
	var hasJobID, hasJobCommand, hasJobletMode bool
	for _, e := range env {
		if e == "JOB_ID=test-job-123" {
			hasJobID = true
		}
		if e == "JOB_COMMAND=/usr/bin/echo" {
			hasJobCommand = true
		}
		if e == "JOBLET_MODE=init" {
			hasJobletMode = true
		}
	}
	assert.True(t, hasJobID, "Should have JOB_ID")
	assert.True(t, hasJobCommand, "Should have JOB_COMMAND")
	assert.True(t, hasJobletMode, "Should have JOBLET_MODE")
}

func TestPrepareEnvironment(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.EnvironStub = func() []string {
		return []string{"PATH=/usr/bin"}
	}

	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("with base env", func(t *testing.T) {
		baseEnv := []string{"KEY1=VALUE1"}
		jobEnv := []string{"KEY2=VALUE2"}
		result := mgr.PrepareEnvironment(baseEnv, jobEnv)
		assert.Contains(t, result, "KEY1=VALUE1")
		assert.Contains(t, result, "KEY2=VALUE2")
	})

	t.Run("with nil base env", func(t *testing.T) {
		jobEnv := []string{"KEY2=VALUE2"}
		result := mgr.PrepareEnvironment(nil, jobEnv)
		assert.Contains(t, result, "PATH=/usr/bin")
		assert.Contains(t, result, "KEY2=VALUE2")
	})
}

func TestIsProcessAlive(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("invalid PID", func(t *testing.T) {
		assert.False(t, mgr.IsProcessAlive(0))
		assert.False(t, mgr.IsProcessAlive(-1))
	})

	t.Run("process alive", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return nil
		}
		assert.True(t, mgr.IsProcessAlive(1234))
	})

	t.Run("process not found", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return syscall.ESRCH
		}
		assert.False(t, mgr.IsProcessAlive(1234))
	})

	t.Run("permission denied means alive", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return syscall.EPERM
		}
		assert.True(t, mgr.IsProcessAlive(1234))
	})
}

func TestKillProcess(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("successful kill", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return nil
		}
		err := mgr.KillProcess(1234, syscall.SIGTERM)
		assert.NoError(t, err)
	})

	t.Run("invalid PID", func(t *testing.T) {
		err := mgr.KillProcess(0, syscall.SIGTERM)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PID")
	})

	t.Run("kill error", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return syscall.EPERM
		}
		err := mgr.KillProcess(1234, syscall.SIGTERM)
		assert.Error(t, err)
	})
}

func TestKillProcessGroup(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("successful kill group", func(t *testing.T) {
		var killedPID int
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			killedPID = pid
			return nil
		}
		err := mgr.KillProcessGroup(1234, syscall.SIGTERM)
		assert.NoError(t, err)
		assert.Equal(t, -1234, killedPID, "Should use negative PID for process group")
	})

	t.Run("invalid PID", func(t *testing.T) {
		err := mgr.KillProcessGroup(-1, syscall.SIGTERM)
		assert.Error(t, err)
	})
}

func TestCleanupProcess(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	t.Run("nil request", func(t *testing.T) {
		_, err := mgr.CleanupProcess(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("invalid request - empty job ID", func(t *testing.T) {
		req := &CleanupRequest{
			JobID: "",
			PID:   1234,
		}
		_, err := mgr.CleanupProcess(context.Background(), req)
		assert.Error(t, err)
	})

	t.Run("process already dead", func(t *testing.T) {
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			return syscall.ESRCH
		}
		req := &CleanupRequest{
			JobID:           "test-job",
			PID:             1234,
			GracefulTimeout: 100 * time.Millisecond,
		}
		result, err := mgr.CleanupProcess(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, "already_dead", result.Method)
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		callCount := 0
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			callCount++
			if callCount == 1 {
				// First call is to check if alive (signal 0)
				return nil
			}
			if callCount == 2 {
				// Second call is SIGTERM to process group
				return nil
			}
			// After graceful timeout, process is dead
			return syscall.ESRCH
		}
		req := &CleanupRequest{
			JobID:           "test-job",
			PID:             1234,
			GracefulTimeout: 10 * time.Millisecond,
		}
		result, err := mgr.CleanupProcess(context.Background(), req)
		assert.NoError(t, err)
		assert.True(t, result.ProcessKilled)
		assert.Equal(t, "graceful", result.Method)
	})

	t.Run("force kill", func(t *testing.T) {
		callCount := 0
		fakePlatform.KillStub = func(pid int, sig syscall.Signal) error {
			callCount++
			if callCount <= 2 {
				// First calls: alive and SIGTERM
				return nil
			}
			// After SIGKILL, process is dead
			return syscall.ESRCH
		}
		req := &CleanupRequest{
			JobID:     "test-job",
			PID:       1234,
			ForceKill: true,
		}
		result, err := mgr.CleanupProcess(context.Background(), req)
		assert.NoError(t, err)
		assert.True(t, result.ProcessKilled)
		assert.Equal(t, "forced", result.Method)
	})

	t.Run("no PID to cleanup", func(t *testing.T) {
		req := &CleanupRequest{
			JobID: "test-job",
			PID:   0, // No PID
		}
		result, err := mgr.CleanupProcess(context.Background(), req)
		assert.NoError(t, err)
		assert.False(t, result.ProcessKilled)
	})
}

func TestValidationError(t *testing.T) {
	err := ValidationError{
		Field:   "testField",
		Value:   "testValue",
		Message: "test message",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "testField")
	assert.Contains(t, errStr, "testValue")
	assert.Contains(t, errStr, "test message")
}

func TestValidateLaunchConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "launch-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test executable
	initPath := filepath.Join(tmpDir, "init")
	err = os.WriteFile(initPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist

	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		config    *LaunchConfig
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			config: &LaunchConfig{
				InitPath:    initPath,
				JobID:       "test-job",
				Command:     "echo",
				Environment: []string{"KEY=VALUE"},
			},
			expectErr: false,
		},
		{
			name: "empty init path",
			config: &LaunchConfig{
				InitPath: "",
				JobID:    "test-job",
			},
			expectErr: true,
			errMsg:    "init path cannot be empty",
		},
		{
			name: "empty job ID",
			config: &LaunchConfig{
				InitPath: initPath,
				JobID:    "",
			},
			expectErr: true,
			errMsg:    "job ID cannot be empty",
		},
		{
			name: "invalid environment",
			config: &LaunchConfig{
				InitPath:    initPath,
				JobID:       "test-job",
				Environment: []string{"INVALID"}, // Missing =
			},
			expectErr: true,
			errMsg:    "invalid environment",
		},
		{
			name: "non-absolute init path",
			config: &LaunchConfig{
				InitPath: "relative/path",
				JobID:    "test-job",
			},
			expectErr: true,
			errMsg:    "must be absolute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validateLaunchConfig(tt.config)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCleanupRequest(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		req       *CleanupRequest
		expectErr bool
	}{
		{
			name: "valid request",
			req: &CleanupRequest{
				JobID:           "test-job",
				PID:             1234,
				GracefulTimeout: 5 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "empty job ID",
			req: &CleanupRequest{
				JobID: "",
			},
			expectErr: true,
		},
		{
			name: "negative timeout",
			req: &CleanupRequest{
				JobID:           "test-job",
				GracefulTimeout: -1 * time.Second,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validateCleanupRequest(tt.req)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePID(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	tests := []struct {
		name      string
		pid       int32
		expectErr bool
	}{
		{
			name:      "valid PID",
			pid:       1234,
			expectErr: false,
		},
		{
			name:      "zero PID",
			pid:       0,
			expectErr: true,
		},
		{
			name:      "negative PID",
			pid:       -1,
			expectErr: true,
		},
		{
			name:      "PID too large",
			pid:       5000000,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validatePID(tt.pid)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLaunchProcess_NilConfig(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	_, err := mgr.LaunchProcess(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestLaunchProcess_InvalidConfig(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	launchCfg := &LaunchConfig{
		InitPath: "", // Invalid
		JobID:    "test-job",
	}

	_, err := mgr.LaunchProcess(context.Background(), launchCfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid launch config")
}

func TestLaunchProcess_Timeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "launch-timeout-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	initPath := filepath.Join(tmpDir, "init")
	err = os.WriteFile(initPath, []byte("#!/bin/bash\necho test"), 0755)
	require.NoError(t, err)

	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist

	cfg := createTestConfig()
	mgr := NewProcessManager(fakePlatform, cfg)

	// Test that context cancellation works
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	launchCfg := &LaunchConfig{
		InitPath: initPath,
		JobID:    "test-job",
		Command:  "echo",
	}

	// Give time for context to expire
	time.Sleep(5 * time.Millisecond)

	_, err = mgr.LaunchProcess(ctx, launchCfg)
	// Should error due to context timeout
	assert.Error(t, err)
}
