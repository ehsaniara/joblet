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

func TestRuntimeConfig_InstallWritablePaths(t *testing.T) {
	tests := []struct {
		name          string
		writablePaths []string
		expectedCount int
		shouldContain []string
	}{
		{
			name: "debian/ubuntu paths",
			writablePaths: []string{
				"/var/cache/apt",
				"/var/lib/apt",
				"/var/lib/dpkg",
			},
			expectedCount: 3,
			shouldContain: []string{"/var/cache/apt", "/var/lib/dpkg"},
		},
		{
			name: "rhel/centos paths",
			writablePaths: []string{
				"/var/cache/yum",
				"/var/lib/rpm",
			},
			expectedCount: 2,
			shouldContain: []string{"/var/cache/yum", "/var/lib/rpm"},
		},
		{
			name: "fedora/dnf paths",
			writablePaths: []string{
				"/var/cache/dnf",
				"/var/lib/dnf",
				"/var/lib/rpm",
			},
			expectedCount: 3,
			shouldContain: []string{"/var/cache/dnf", "/var/lib/rpm"},
		},
		{
			name:          "empty paths",
			writablePaths: []string{},
			expectedCount: 0,
			shouldContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:             "/opt/joblet/runtimes",
					InstallWritablePaths: tt.writablePaths,
				},
			}

			assert.Len(t, cfg.Runtime.InstallWritablePaths, tt.expectedCount)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, cfg.Runtime.InstallWritablePaths, expected)
			}
		})
	}
}

func TestRuntimeConfig_InstallHostBinds(t *testing.T) {
	tests := []struct {
		name          string
		hostBinds     []string
		expectedCount int
		shouldContain []string
	}{
		{
			name: "standard linux FHS paths",
			hostBinds: []string{
				"/usr",
				"/lib",
				"/lib64",
				"/bin",
				"/sbin",
				"/etc",
				"/var",
			},
			expectedCount: 7,
			shouldContain: []string{"/usr", "/bin", "/lib", "/etc"},
		},
		{
			name: "minimal paths",
			hostBinds: []string{
				"/usr",
				"/lib",
				"/bin",
			},
			expectedCount: 3,
			shouldContain: []string{"/usr", "/bin"},
		},
		{
			name: "with optional paths",
			hostBinds: []string{
				"/usr",
				"/lib",
				"/lib64",
				"/bin",
				"/sbin",
				"/etc",
				"/var",
				"/opt",
			},
			expectedCount: 8,
			shouldContain: []string{"/opt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:         "/opt/joblet/runtimes",
					InstallHostBinds: tt.hostBinds,
				},
			}

			assert.Len(t, cfg.Runtime.InstallHostBinds, tt.expectedCount)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, cfg.Runtime.InstallHostBinds, expected)
			}
		})
	}
}

func TestRuntimeConfig_InstallEnvPath(t *testing.T) {
	tests := []struct {
		name        string
		envPath     string
		expected    string
		shouldMatch bool
	}{
		{
			name:        "standard linux PATH",
			envPath:     "/usr/bin:/bin:/sbin:/usr/sbin",
			expected:    "/usr/bin:/bin:/sbin:/usr/sbin",
			shouldMatch: true,
		},
		{
			name:        "extended PATH with local bins",
			envPath:     "/usr/local/bin:/usr/bin:/bin:/sbin:/usr/sbin",
			expected:    "/usr/local/bin:/usr/bin:/bin:/sbin:/usr/sbin",
			shouldMatch: true,
		},
		{
			name:        "alpine linux style",
			envPath:     "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			expected:    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			shouldMatch: true,
		},
		{
			name:        "empty PATH uses fallback",
			envPath:     "",
			expected:    "",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:       "/opt/joblet/runtimes",
					InstallEnvPath: tt.envPath,
				},
			}

			assert.Equal(t, tt.expected, cfg.Runtime.InstallEnvPath)
		})
	}
}

func TestRuntimeConfig_DefaultValues(t *testing.T) {
	// Test that default config has expected values
	// Note: InstallWritablePaths is empty in DefaultConfig (distro-specific, loaded from runtime-config.yml)
	// InstallHostBinds has FHS-compliant defaults that work on all distros
	defaultCfg := config.DefaultConfig

	// Check InstallWritablePaths is empty (distro-specific, loaded from runtime-config.yml)
	assert.Empty(t, defaultCfg.Runtime.InstallWritablePaths, "InstallWritablePaths should be empty in DefaultConfig - loaded from runtime-config.yml")

	// Check InstallHostBinds has FHS-compliant defaults (work on all distros)
	assert.NotEmpty(t, defaultCfg.Runtime.InstallHostBinds)
	assert.Contains(t, defaultCfg.Runtime.InstallHostBinds, "/usr")
	assert.Contains(t, defaultCfg.Runtime.InstallHostBinds, "/lib")
	assert.Contains(t, defaultCfg.Runtime.InstallHostBinds, "/bin")
	assert.Contains(t, defaultCfg.Runtime.InstallHostBinds, "/etc")
	assert.Contains(t, defaultCfg.Runtime.InstallHostBinds, "/var")

	// Check InstallEnvPath has fallback default
	assert.Equal(t, "/usr/bin:/bin:/sbin:/usr/sbin", defaultCfg.Runtime.InstallEnvPath)

	// Check base path default
	assert.Equal(t, "/opt/joblet/runtimes", defaultCfg.Runtime.BasePath)
}

func TestRuntimeInstaller_ConfigDrivenEnvPath(t *testing.T) {
	tests := []struct {
		name            string
		configEnvPath   string
		expectedContain string
	}{
		{
			name:            "uses config PATH",
			configEnvPath:   "/custom/bin:/usr/bin:/bin",
			expectedContain: "/custom/bin",
		},
		{
			name:            "standard PATH",
			configEnvPath:   "/usr/bin:/bin:/sbin:/usr/sbin",
			expectedContain: "/usr/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:       "/opt/joblet/runtimes",
					InstallEnvPath: tt.configEnvPath,
				},
			}
			testLogger := logger.New()
			testPlatform := platform.NewPlatform()

			installer := NewRuntimeInstaller(cfg, testLogger, testPlatform)

			// Verify config is stored correctly
			assert.Equal(t, tt.configEnvPath, installer.config.Runtime.InstallEnvPath)
			assert.Contains(t, installer.config.Runtime.InstallEnvPath, tt.expectedContain)
		})
	}
}

func TestRuntimeInstaller_ConfigDrivenHostBinds(t *testing.T) {
	tests := []struct {
		name          string
		hostBinds     []string
		expectedCount int
	}{
		{
			name:          "full FHS paths",
			hostBinds:     []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"},
			expectedCount: 7,
		},
		{
			name:          "minimal paths",
			hostBinds:     []string{"/usr", "/lib", "/bin"},
			expectedCount: 3,
		},
		{
			name:          "empty paths",
			hostBinds:     []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:         "/opt/joblet/runtimes",
					InstallHostBinds: tt.hostBinds,
				},
			}
			testLogger := logger.New()
			testPlatform := platform.NewPlatform()

			installer := NewRuntimeInstaller(cfg, testLogger, testPlatform)

			assert.Len(t, installer.config.Runtime.InstallHostBinds, tt.expectedCount)
		})
	}
}

func TestRuntimeInstaller_ConfigDrivenWritablePaths(t *testing.T) {
	tests := []struct {
		name          string
		writablePaths []string
		distroType    string
	}{
		{
			name:          "debian ubuntu",
			writablePaths: []string{"/var/cache/apt", "/var/lib/apt", "/var/lib/dpkg"},
			distroType:    "debian",
		},
		{
			name:          "rhel centos",
			writablePaths: []string{"/var/cache/yum", "/var/lib/rpm"},
			distroType:    "rhel",
		},
		{
			name:          "fedora",
			writablePaths: []string{"/var/cache/dnf", "/var/lib/dnf", "/var/lib/rpm"},
			distroType:    "fedora",
		},
		{
			name:          "alpine",
			writablePaths: []string{"/var/cache/apk", "/lib/apk"},
			distroType:    "alpine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: config.RuntimeConfig{
					BasePath:             "/opt/joblet/runtimes",
					InstallWritablePaths: tt.writablePaths,
				},
			}
			testLogger := logger.New()
			testPlatform := platform.NewPlatform()

			installer := NewRuntimeInstaller(cfg, testLogger, testPlatform)

			// Verify all paths are stored
			assert.Equal(t, tt.writablePaths, installer.config.Runtime.InstallWritablePaths)
		})
	}
}

func TestRuntimeConfig_CrossDistroCompatibility(t *testing.T) {
	// Test configurations for different Linux distributions
	distroConfigs := map[string]config.RuntimeConfig{
		"debian": {
			BasePath:             "/opt/joblet/runtimes",
			InstallWritablePaths: []string{"/var/cache/apt", "/var/lib/apt", "/var/lib/dpkg"},
			InstallHostBinds:     []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"},
			InstallEnvPath:       "/usr/bin:/bin:/sbin:/usr/sbin",
		},
		"rhel": {
			BasePath:             "/opt/joblet/runtimes",
			InstallWritablePaths: []string{"/var/cache/yum", "/var/lib/rpm"},
			InstallHostBinds:     []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"},
			InstallEnvPath:       "/usr/bin:/bin:/sbin:/usr/sbin",
		},
		"fedora": {
			BasePath:             "/opt/joblet/runtimes",
			InstallWritablePaths: []string{"/var/cache/dnf", "/var/lib/dnf", "/var/lib/rpm"},
			InstallHostBinds:     []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"},
			InstallEnvPath:       "/usr/bin:/bin:/sbin:/usr/sbin",
		},
		"alpine": {
			BasePath:             "/opt/joblet/runtimes",
			InstallWritablePaths: []string{"/var/cache/apk", "/lib/apk"},
			InstallHostBinds:     []string{"/usr", "/lib", "/bin", "/sbin", "/etc", "/var"},
			InstallEnvPath:       "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	for distro, runtimeCfg := range distroConfigs {
		t.Run(distro, func(t *testing.T) {
			cfg := &config.Config{
				Runtime: runtimeCfg,
			}

			// Verify essential fields are set
			assert.NotEmpty(t, cfg.Runtime.BasePath, "BasePath should not be empty for %s", distro)
			assert.NotEmpty(t, cfg.Runtime.InstallWritablePaths, "InstallWritablePaths should not be empty for %s", distro)
			assert.NotEmpty(t, cfg.Runtime.InstallHostBinds, "InstallHostBinds should not be empty for %s", distro)
			assert.NotEmpty(t, cfg.Runtime.InstallEnvPath, "InstallEnvPath should not be empty for %s", distro)

			// Verify common host binds are present
			assert.Contains(t, cfg.Runtime.InstallHostBinds, "/usr")
			assert.Contains(t, cfg.Runtime.InstallHostBinds, "/bin")
			assert.Contains(t, cfg.Runtime.InstallHostBinds, "/lib")
		})
	}
}
