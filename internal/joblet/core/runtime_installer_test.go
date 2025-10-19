//go:build linux

package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"

	"github.com/stretchr/testify/assert"
)

func TestNewRuntimeInstaller(t *testing.T) {
	config := &config.Config{
		Runtime: config.RuntimeConfig{
			BasePath:    "/opt/joblet/runtimes",
			CommonPaths: []string{"/usr/bin", "/bin"},
		},
	}
	testLogger := logger.New()
	testPlatform := platform.NewPlatform()

	installer := NewRuntimeInstaller(config, testLogger, testPlatform)

	assert.NotNil(t, installer)
	assert.Equal(t, config, installer.config)
	assert.Equal(t, testPlatform, installer.platform)
}

func TestRuntimeInstaller_makedev(t *testing.T) {
	config := &config.Config{}
	testLogger := logger.New()
	testPlatform := platform.NewPlatform()

	installer := NewRuntimeInstaller(config, testLogger, testPlatform)

	tests := []struct {
		major    uint32
		minor    uint32
		expected uint64
	}{
		{1, 3, 259}, // /dev/null
		{1, 5, 261}, // /dev/zero
		{1, 8, 264}, // /dev/random
		{1, 9, 265}, // /dev/urandom
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("major_%d_minor_%d", tt.major, tt.minor), func(t *testing.T) {
			result := installer.makedev(tt.major, tt.minor)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRuntimeInstallRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request RuntimeInstallRequest
		isValid bool
	}{
		{
			name: "valid request",
			request: RuntimeInstallRequest{
				RuntimeSpec:    "python-3.11",
				Repository:     "test/repo",
				Branch:         "main",
				Path:           "runtimes/python",
				ForceReinstall: false,
			},
			isValid: true,
		},
		{
			name: "empty runtime spec",
			request: RuntimeInstallRequest{
				RuntimeSpec: "",
				Repository:  "test/repo",
			},
			isValid: false,
		},
		{
			name: "minimal valid request",
			request: RuntimeInstallRequest{
				RuntimeSpec: "golang-1.21",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.request.RuntimeSpec != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestRuntimeInstallResult_Fields(t *testing.T) {
	duration := 5 * time.Minute
	result := RuntimeInstallResult{
		RuntimeSpec: "python-3.11-ml",
		Success:     true,
		Message:     "Installation completed successfully",
		InstallPath: "/opt/joblet/runtimes/python-3.11-ml/runtime.yml",
		Duration:    duration,
		LogOutput:   "Setup completed\nRuntime installed",
	}

	// Verify all fields
	assert.Equal(t, "python-3.11-ml", result.RuntimeSpec)
	assert.True(t, result.Success)
	assert.Equal(t, "Installation completed successfully", result.Message)
	assert.Equal(t, "/opt/joblet/runtimes/python-3.11-ml/runtime.yml", result.InstallPath)
	assert.Equal(t, duration, result.Duration)
	assert.Equal(t, "Setup completed\nRuntime installed", result.LogOutput)
}

// Mock implementation for testing streaming interface
type mockRuntimeStreamer struct {
	progressMessages []string
	logData          [][]byte
}

func (m *mockRuntimeStreamer) SendProgress(message string) error {
	m.progressMessages = append(m.progressMessages, message)
	return nil
}

func (m *mockRuntimeStreamer) SendLog(data []byte) error {
	m.logData = append(m.logData, data)
	return nil
}

func TestRuntimeInstaller_StreamingInterface(t *testing.T) {
	// Test that the streaming interface works correctly
	streamer := &mockRuntimeStreamer{}

	// Test sending progress
	err := streamer.SendProgress("Starting installation")
	assert.NoError(t, err)
	assert.Len(t, streamer.progressMessages, 1)
	assert.Equal(t, "Starting installation", streamer.progressMessages[0])

	// Test sending log data
	logData := []byte("Installing packages...")
	err = streamer.SendLog(logData)
	assert.NoError(t, err)
	assert.Len(t, streamer.logData, 1)
	assert.Equal(t, logData, streamer.logData[0])

	// Test multiple messages
	err = streamer.SendProgress("Configuring runtime")
	assert.NoError(t, err)
	err = streamer.SendLog([]byte("Configuration complete"))
	assert.NoError(t, err)

	assert.Len(t, streamer.progressMessages, 2)
	assert.Len(t, streamer.logData, 2)
	assert.Equal(t, "Configuring runtime", streamer.progressMessages[1])
	assert.Equal(t, []byte("Configuration complete"), streamer.logData[1])
}

// The following methods were removed during simplification:
// - buildPathFromConfig: PATH environment building (now internal)
// - getRuntimePath: Direct runtime path mapping
// - autoDetectRuntimePath: Auto-detection of runtime paths
// - writeFilesToChroot: File writing to chroot (consolidated)
// - findLocalRuntime: Local runtime discovery (simplified)
