package resource

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := config.CgroupConfig{
		BaseDir:           "/sys/fs/cgroup/joblet.slice",
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	r := New(cfg)
	assert.NotNil(t, r, "New() should return a non-nil Resource")

	// Verify it's a cgroup type
	cg, ok := r.(*cgroup)
	assert.True(t, ok, "New() should return a *cgroup")
	assert.Equal(t, cfg.BaseDir, cg.config.BaseDir)
	assert.Equal(t, cfg.EnableControllers, cg.config.EnableControllers)
	assert.False(t, cg.initialized, "cgroup should not be initialized initially")
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "item exists",
			slice: []string{"a", "b", "c"},
			item:  "b",
			want:  true,
		},
		{
			name:  "item does not exist",
			slice: []string{"a", "b", "c"},
			item:  "d",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "a",
			want:  false,
		},
		{
			name:  "nil slice",
			slice: nil,
			item:  "a",
			want:  false,
		},
		{
			name:  "empty item",
			slice: []string{"a", "b", ""},
			item:  "",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCgroup_Create_SecurityViolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-security-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Try to create a cgroup outside the base directory
	err = cg.Create("/tmp/malicious-cgroup", 100, 256, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "security violation")
}

func TestCgroup_SetIOLimit_ValidPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-io-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroup structure
	cgroupDir := filepath.Join(tmpDir, "test-job")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create io.max file
	ioMaxFile := filepath.Join(cgroupDir, "io.max")
	err = os.WriteFile(ioMaxFile, []byte(""), 0644)
	require.NoError(t, err)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"io"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Test with 0 (unlimited)
	err = cg.SetIOLimit(cgroupDir, 0)
	assert.NoError(t, err)

	// Test with positive value
	err = cg.SetIOLimit(cgroupDir, 1024*1024)
	assert.NoError(t, err)

	// Verify the file was written with rbps format
	content, err := os.ReadFile(ioMaxFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "rbps=")
}

func TestCgroup_SetCPULimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-cpu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroup structure
	cgroupDir := filepath.Join(tmpDir, "test-job")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create cpu.max file
	cpuMaxFile := filepath.Join(cgroupDir, "cpu.max")
	err = os.WriteFile(cpuMaxFile, []byte("max 100000"), 0644)
	require.NoError(t, err)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Test with 0 (unlimited - writes 0 100000)
	err = cg.SetCPULimit(cgroupDir, 0)
	assert.NoError(t, err)

	// Verify 0 is written for unlimited
	content, err := os.ReadFile(cpuMaxFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "0 100000")

	// Test with positive value (50% of one core)
	err = cg.SetCPULimit(cgroupDir, 50)
	assert.NoError(t, err)

	content, err = os.ReadFile(cpuMaxFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "50000 100000") // 50% of 100000
}

func TestCgroup_SetCPUCores(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-cpuset-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroup structure
	cgroupDir := filepath.Join(tmpDir, "test-job")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create cpuset.cpus file
	cpusetFile := filepath.Join(cgroupDir, "cpuset.cpus")
	err = os.WriteFile(cpusetFile, []byte("0-3"), 0644)
	require.NoError(t, err)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpuset"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Test with empty string (no restriction)
	err = cg.SetCPUCores(cgroupDir, "")
	assert.NoError(t, err)

	// Test with specific cores
	err = cg.SetCPUCores(cgroupDir, "0-1")
	assert.NoError(t, err)

	content, err := os.ReadFile(cpusetFile)
	require.NoError(t, err)
	assert.Equal(t, "0-1", string(content))
}

func TestCgroup_SetMemoryLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-memory-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroup structure
	cgroupDir := filepath.Join(tmpDir, "test-job")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create memory.max file
	memMaxFile := filepath.Join(cgroupDir, "memory.max")
	err = os.WriteFile(memMaxFile, []byte("max"), 0644)
	require.NoError(t, err)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Test with 0 (unlimited - writes 0)
	err = cg.SetMemoryLimit(cgroupDir, 0)
	assert.NoError(t, err)

	content, err := os.ReadFile(memMaxFile)
	require.NoError(t, err)
	assert.Equal(t, "0", string(content))

	// Test with positive value (256 MB)
	err = cg.SetMemoryLimit(cgroupDir, 256)
	assert.NoError(t, err)

	content, err = os.ReadFile(memMaxFile)
	require.NoError(t, err)
	assert.Equal(t, "268435456", string(content)) // 256 * 1024 * 1024
}

func TestCgroup_EnsureControllers_AlreadyInitialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-init-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)
	cg.initialized = true

	// Should return immediately without error
	err = cg.EnsureControllers()
	assert.NoError(t, err)
}

func TestCgroup_EnableSubtreeControl_NoControllers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-subtree-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroup structure
	cgroupDir := filepath.Join(tmpDir, "test-job")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{}, // No controllers
		CleanupTimeout:    30 * time.Second,
	}

	cg := New(cfg).(*cgroup)

	// Should return without error when no controllers configured
	err = cg.enableSubtreeControl(cgroupDir)
	assert.NoError(t, err)
}
