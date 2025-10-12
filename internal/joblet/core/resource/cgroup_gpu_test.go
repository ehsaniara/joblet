package resource

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetGPUDevices_CgroupsV2(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cgroup-gpu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroups v2 structure (cgroup.controllers file indicates v2)
	cgroupDir := filepath.Join(tmpDir, "test-cgroup")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create cgroup.controllers file to simulate cgroups v2
	controllersFile := filepath.Join(tmpDir, "cgroup.controllers")
	err = os.WriteFile(controllersFile, []byte("cpuset cpu io memory pids"), 0644)
	require.NoError(t, err)

	// Create cgroup resource manager
	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := &cgroup{
		logger: logger.New().WithField("component", "test"),
		config: cfg,
	}

	// Test GPU device configuration
	gpuIndices := []int{0, 1}
	err = cg.SetGPUDevices(cgroupDir, gpuIndices)

	// Should succeed for cgroups v2 (doesn't try to write to devices.allow)
	assert.NoError(t, err)

	// Verify cgroup version detection
	version := cg.detectCgroupVersion()
	assert.Equal(t, 2, version, "Should detect cgroups v2")
}

func TestSetGPUDevices_CgroupsV1(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cgroup-gpu-v1-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cgroups v1 structure (devices.allow file indicates v1)
	cgroupDir := filepath.Join(tmpDir, "test-cgroup")
	err = os.MkdirAll(cgroupDir, 0755)
	require.NoError(t, err)

	// Create devices.allow file to simulate cgroups v1
	devicesAllowFile := filepath.Join(cgroupDir, "devices.allow")
	err = os.WriteFile(devicesAllowFile, []byte(""), 0644)
	require.NoError(t, err)

	// Create devices.allow file in base dir to trigger v1 detection
	baseDevicesAllow := filepath.Join(tmpDir, "devices.allow")
	err = os.WriteFile(baseDevicesAllow, []byte(""), 0644)
	require.NoError(t, err)

	// Create cgroup resource manager
	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := &cgroup{
		logger: logger.New().WithField("component", "test"),
		config: cfg,
	}

	// Test GPU device configuration
	gpuIndices := []int{0, 1}
	err = cg.SetGPUDevices(cgroupDir, gpuIndices)

	// Should succeed for cgroups v1 (writes to devices.allow)
	assert.NoError(t, err)

	// Verify cgroup version detection
	version := cg.detectCgroupVersion()
	assert.Equal(t, 1, version, "Should detect cgroups v1")

	// Verify that the devices.allow file exists and was written to
	// (In real cgroups v1, each write grants a device permission in the kernel)
	// The file content shows only the last write, but all permissions are granted
	info, err := os.Stat(devicesAllowFile)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "devices.allow file should have been written to")

	// Read the final content (should be the last device rule that was written)
	content, err := os.ReadFile(devicesAllowFile)
	require.NoError(t, err)
	contentStr := string(content)

	// The file should contain the last rule that was written (nvidia1)
	assert.Contains(t, contentStr, "c 195:1 rwm", "Should contain the last device rule written")
	assert.Greater(t, len(contentStr), 0, "devices.allow should not be empty")
}

func TestDetectCgroupVersion(t *testing.T) {
	tests := []struct {
		name            string
		setupFiles      func(tmpDir string) error
		expectedVersion int
	}{
		{
			name: "cgroups v2 detection",
			setupFiles: func(tmpDir string) error {
				controllersFile := filepath.Join(tmpDir, "cgroup.controllers")
				return os.WriteFile(controllersFile, []byte("cpuset cpu io memory"), 0644)
			},
			expectedVersion: 2,
		},
		{
			name: "cgroups v1 detection",
			setupFiles: func(tmpDir string) error {
				devicesAllowFile := filepath.Join(tmpDir, "devices.allow")
				return os.WriteFile(devicesAllowFile, []byte(""), 0644)
			},
			expectedVersion: 1,
		},
		{
			name: "no cgroup files - defaults to v2",
			setupFiles: func(tmpDir string) error {
				return nil // Don't create any files
			},
			expectedVersion: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "cgroup-version-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			err = tt.setupFiles(tmpDir)
			require.NoError(t, err)

			cfg := config.CgroupConfig{
				BaseDir:           tmpDir,
				EnableControllers: []string{"cpu", "memory"},
				CleanupTimeout:    30 * time.Second,
			}

			cg := &cgroup{
				logger: logger.New().WithField("component", "test"),
				config: cfg,
			}

			version := cg.detectCgroupVersion()
			assert.Equal(t, tt.expectedVersion, version)
		})
	}
}

func TestSetGPUDevicesV2_NonexistentCgroup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cgroup-nonexistent-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cfg := config.CgroupConfig{
		BaseDir:           tmpDir,
		EnableControllers: []string{"cpu", "memory"},
		CleanupTimeout:    30 * time.Second,
	}

	cg := &cgroup{
		logger: logger.New().WithField("component", "test"),
		config: cfg,
	}

	// Test with non-existent cgroup directory
	nonexistentPath := filepath.Join(tmpDir, "nonexistent-cgroup")
	gpuIndices := []int{0}

	err = cg.setGPUDevicesV2(nonexistentPath, gpuIndices, cg.logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cgroup path does not exist")
}
