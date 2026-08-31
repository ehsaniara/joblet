//go:build linux

package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/core/process"
	"github.com/ehsaniara/joblet/internal/joblet/core/resource/resourcefakes"
	"github.com/ehsaniara/joblet/internal/joblet/gpu/gpufakes"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getCleanupCalls extracts job IDs from CleanupCgroup calls
func getCleanupCalls(fake *resourcefakes.FakeResource) []string {
	count := fake.CleanupCgroupCallCount()
	calls := make([]string, count)
	for i := 0; i < count; i++ {
		calls[i] = fake.CleanupCgroupArgsForCall(i)
	}
	return calls
}

func TestNewCoordinator(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: "/tmp/joblet",
			TmpDir:  "/tmp/joblet/{JOB_ID}/tmp",
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	assert.NotNil(t, coordinator)
	assert.NotNil(t, coordinator.processManager)
	assert.NotNil(t, coordinator.cgroup)
	assert.NotNil(t, coordinator.platform)
	assert.NotNil(t, coordinator.config)
	assert.NotNil(t, coordinator.logger)
}

func TestCleanupJob_Success(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cleanup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create job directory structure
	jobID := "test-job-123"
	jobDir := filepath.Join(tmpDir, jobID)
	err = os.MkdirAll(jobDir, 0755)
	require.NoError(t, err)

	// Create pipes and workspace directories
	pipesDir := filepath.Join(jobDir, "pipes")
	workDir := filepath.Join(jobDir, "work")
	err = os.MkdirAll(pipesDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(workDir, 0755)
	require.NoError(t, err)

	// Setup
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.RemoveAllStub = os.RemoveAll

	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Execute cleanup
	err = coordinator.CleanupJob(jobID)

	// Verify
	assert.NoError(t, err)
	assert.Contains(t, getCleanupCalls(mockRes), jobID)

	// Verify directory was removed
	_, err = os.Stat(jobDir)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupJob_AlreadyInProgress(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: "/tmp/joblet",
			TmpDir:  "/tmp/joblet/{JOB_ID}/tmp",
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Pre-populate the activeCleanups map
	jobID := "test-job-123"
	coordinator.activeCleanups.Store(jobID, &CleanupStatus{
		JobUUID:   jobID,
		StartTime: time.Now(),
	})

	// Try to run cleanup again
	err := coordinator.CleanupJob(jobID)

	// Verify error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cleanup already in progress")
}

func TestCleanupJobSystemResourcesOnly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-system-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create job directory
	jobID := "runtime-build-job"
	jobDir := filepath.Join(tmpDir, jobID)
	err = os.MkdirAll(jobDir, 0755)
	require.NoError(t, err)

	// Setup
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Execute system-only cleanup
	err = coordinator.CleanupJobSystemResourcesOnly(jobID)

	// Verify
	assert.NoError(t, err)
	assert.Contains(t, getCleanupCalls(mockRes), jobID)

	// Verify directory was NOT removed (preserved for runtime builds)
	_, err = os.Stat(jobDir)
	assert.NoError(t, err, "Job directory should be preserved for runtime builds")
}

func TestGetCleanupStatus(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: "/tmp/joblet",
			TmpDir:  "/tmp/joblet/{JOB_ID}/tmp",
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Test non-existent status
	status, exists := coordinator.GetCleanupStatus("non-existent")
	assert.False(t, exists)
	assert.Nil(t, status)

	// Add a status
	jobID := "test-job"
	expectedStatus := &CleanupStatus{
		JobUUID:   jobID,
		StartTime: time.Now(),
	}
	coordinator.activeCleanups.Store(jobID, expectedStatus)

	// Test existing status
	status, exists = coordinator.GetCleanupStatus(jobID)
	assert.True(t, exists)
	assert.Equal(t, jobID, status.JobUUID)
}

func TestCleanupOrphanedResources(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "orphan-cleanup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create orphaned job directory
	orphanedJobUUID := "orphaned-job"
	orphanedDir := filepath.Join(tmpDir, orphanedJobUUID)
	err = os.MkdirAll(orphanedDir, 0755)
	require.NoError(t, err)

	// Create active job directory
	activeJobUUID := "active-job"
	activeDir := filepath.Join(tmpDir, activeJobUUID)
	err = os.MkdirAll(activeDir, 0755)
	require.NoError(t, err)

	// Setup
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.ReadDirStub = os.ReadDir
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.RemoveAllStub = os.RemoveAll

	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Define active jobs (only activeJobUUID is active)
	activeJobs := map[string]bool{
		activeJobUUID: true,
	}

	// Execute orphaned cleanup
	err = coordinator.CleanupOrphanedResources(activeJobs)

	// Verify
	assert.NoError(t, err)

	// Orphaned directory should be removed
	_, err = os.Stat(orphanedDir)
	assert.True(t, os.IsNotExist(err), "Orphaned directory should be removed")

	// Active directory should still exist
	_, err = os.Stat(activeDir)
	assert.NoError(t, err, "Active directory should not be removed")
}

func TestSchedulePeriodicCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "periodic-cleanup-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.ReadDirStub = os.ReadDir
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.RemoveAllStub = os.RemoveAll

	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Track cleanup calls
	var callCount int
	var mu sync.Mutex
	getActiveJobs := func() map[string]bool {
		mu.Lock()
		callCount++
		mu.Unlock()
		return map[string]bool{}
	}

	// Start periodic cleanup with short interval
	go coordinator.SchedulePeriodicCleanup(ctx, 50*time.Millisecond, getActiveJobs)

	// Wait for a few cleanup cycles
	time.Sleep(150 * time.Millisecond)

	// Cancel context to stop
	cancel()

	// Give time for goroutine to exit
	time.Sleep(20 * time.Millisecond)

	// Verify cleanup was called multiple times
	mu.Lock()
	count := callCount
	mu.Unlock()
	assert.GreaterOrEqual(t, count, 2, "Periodic cleanup should have been called at least twice")
}

func TestCleanupStatus_Fields(t *testing.T) {
	status := &CleanupStatus{
		JobUUID:       "test-job",
		StartTime:     time.Now(),
		ProcessKilled: true,
		CgroupCleaned: true,
		FilesCleaned:  true,
		Errors:        []error{},
		Completed:     true,
	}

	assert.Equal(t, "test-job", status.JobUUID)
	assert.True(t, status.ProcessKilled)
	assert.True(t, status.CgroupCleaned)
	assert.True(t, status.FilesCleaned)
	assert.True(t, status.Completed)
	assert.Empty(t, status.Errors)
}

func TestCleanupJob_FilesystemError(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cleanup-error-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	jobID := "test-job-error"

	// Setup with failing RemoveAll
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = func(name string) (os.FileInfo, error) {
		return os.Stat(name)
	}
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.RemoveAllStub = func(path string) error {
		return os.ErrPermission
	}

	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}

	// Create job directory so Stat succeeds
	jobDir := filepath.Join(tmpDir, jobID)
	err = os.MkdirAll(jobDir, 0755)
	require.NoError(t, err)

	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Execute cleanup - should fail due to permission error
	err = coordinator.CleanupJob(jobID)

	// Verify error occurred
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "errors")
}

func TestCleanupJob_NonExistentDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cleanup-nonexist-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	jobID := "non-existent-job"

	// Setup
	fakePlatform := &platformfakes.FakePlatform{}
	fakePlatform.StatStub = os.Stat
	fakePlatform.IsNotExistStub = os.IsNotExist
	fakePlatform.RemoveAllStub = os.RemoveAll

	cfg := &config.Config{
		Cgroup: config.CgroupConfig{
			CleanupTimeout: 5 * time.Second,
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: tmpDir,
			TmpDir:  filepath.Join(tmpDir, "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	mockRes := &resourcefakes.FakeResource{}
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	coordinator := NewCoordinator(procMgr, mockRes, fakePlatform, cfg, log, nil)

	// Execute cleanup on non-existent directory - should succeed
	err = coordinator.CleanupJob(jobID)

	// Should succeed (nothing to clean up is not an error)
	assert.NoError(t, err)
}

func TestCleanup_ReleasesGPUs(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	cfg := &config.Config{
		Cgroup: config.CgroupConfig{CleanupTimeout: 5 * time.Second},
		Filesystem: config.FilesystemConfig{
			BaseDir: t.TempDir(),
			TmpDir:  filepath.Join(t.TempDir(), "{JOB_ID}", "tmp"),
		},
	}
	log := logger.New().WithField("component", "test")
	procMgr := process.NewProcessManager(fakePlatform, cfg)

	// Without a GPU manager cleanup must still succeed
	coordinator := NewCoordinator(procMgr, &resourcefakes.FakeResource{}, fakePlatform, cfg, log, nil)
	require.NoError(t, coordinator.CleanupJob("job-no-gpu-manager"))

	fakeGPU := &gpufakes.FakeGPUManagerInterface{}
	coordinator = NewCoordinator(procMgr, &resourcefakes.FakeResource{}, fakePlatform, cfg, log, nil)
	coordinator.SetGPUManager(fakeGPU)

	require.NoError(t, coordinator.CleanupJob("job-full"))
	require.NoError(t, coordinator.CleanupJobSystemResourcesOnly("job-system-only"))

	require.Equal(t, 2, fakeGPU.ReleaseGPUsCallCount())
	assert.Equal(t, "job-full", fakeGPU.ReleaseGPUsArgsForCall(0))
	assert.Equal(t, "job-system-only", fakeGPU.ReleaseGPUsArgsForCall(1))
}
