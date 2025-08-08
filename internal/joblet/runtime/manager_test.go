package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"joblet/pkg/platform/platformfakes"

	"gopkg.in/yaml.v3"
)

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return m.sys }

func TestSetupRuntime(t *testing.T) {
	// Create fake platform
	platform := &platformfakes.FakePlatform{}

	// Setup test environment
	runtimesPath := "/opt/joblet/runtimes"
	jobRootDir := "/opt/joblet/jobs/test-job/chroot"

	// Create runtime config
	runtimeConfig := RuntimeConfig{
		Name:        "python-3.11",
		Version:     "3.11.5",
		Description: "Python 3.11 runtime",
		Mounts: []MountSpec{
			{
				Source:   filepath.Join(runtimesPath, "python", "python-3.11", "bin"),
				Target:   "/usr/local/bin",
				ReadOnly: true,
			},
			{
				Source:   filepath.Join(runtimesPath, "python", "python-3.11", "lib"),
				Target:   "/usr/local/lib",
				ReadOnly: true,
			},
		},
		Environment: map[string]string{
			"PYTHON_HOME":  "/usr/local",
			"PATH_PREPEND": "/usr/local/bin",
		},
		PackageManager: &PackageManagerConfig{
			Type:        "pip",
			CacheVolume: "pip-cache",
		},
	}

	// Marshal config to YAML
	configData, _ := yaml.Marshal(runtimeConfig)

	// Setup platform mocks
	platform.DirExistsReturns(true)
	platform.FileExistsReturns(true)
	platform.ReadFileReturns(configData, nil)
	platform.MkdirAllReturns(nil)
	platform.MountReturns(nil) // Mock the mount operation

	// Mock Stat calls for mount sources - return a valid os.FileInfo
	mockFileInfo := &mockFileInfo{isDir: true}
	platform.StatReturns(mockFileInfo, nil)

	// Mock OpenFile for file creation (we just need it to not error)
	// Create a temp file that we can return and close immediately
	tempFile, _ := os.CreateTemp("", "test")
	platform.OpenFileReturns(tempFile, nil)

	manager := NewManager(runtimesPath, platform)

	// Test runtime setup
	config, err := manager.SetupRuntime(jobRootDir, "python:3.11", []string{"pip-cache"})
	if err != nil {
		t.Fatalf("Failed to setup runtime: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.Name != "python-3.11" {
		t.Errorf("Config name: got %s, want python-3.11", config.Name)
	}

	// Verify MkdirAll was called for mount points
	if platform.MkdirAllCallCount() == 0 {
		t.Error("Expected MkdirAll to be called for mount points")
	}
}

func TestSetupRuntimeNoRuntime(t *testing.T) {
	platform := &platformfakes.FakePlatform{}
	manager := NewManager("/opt/joblet/runtimes", platform)

	// Test with empty runtime spec
	config, err := manager.SetupRuntime("/opt/joblet/jobs/test-job", "", nil)
	if err != nil {
		t.Errorf("Expected no error for empty runtime spec, got: %v", err)
	}
	if config != nil {
		t.Error("Expected nil config for empty runtime spec")
	}
}

func TestGetEnvironmentVariables(t *testing.T) {
	platform := &platformfakes.FakePlatform{}
	manager := NewManager("/opt/joblet/runtimes", platform)

	tests := []struct {
		name     string
		config   *RuntimeConfig
		expected map[string]string
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: nil,
		},
		{
			name: "basic environment",
			config: &RuntimeConfig{
				Environment: map[string]string{
					"JAVA_HOME":  "/usr/lib/jvm/java-17",
					"MAVEN_HOME": "/usr/local/maven",
				},
			},
			expected: map[string]string{
				"JAVA_HOME":  "/usr/lib/jvm/java-17",
				"MAVEN_HOME": "/usr/local/maven",
			},
		},
		{
			name: "with PATH_PREPEND",
			config: &RuntimeConfig{
				Environment: map[string]string{
					"PYTHON_HOME":  "/usr/local",
					"PATH_PREPEND": "/usr/local/bin",
				},
			},
			expected: map[string]string{
				"PYTHON_HOME":          "/usr/local",
				"RUNTIME_PATH_PREPEND": "/usr/local/bin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.GetEnvironmentVariables(tt.config)

			if tt.expected == nil {
				if result != nil {
					t.Error("Expected nil result")
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Environment size: got %d, want %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Missing key: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Key %s: got %s, want %s", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestMountPath(t *testing.T) {
	// This test would require actual filesystem operations
	// In a real implementation, we'd use a temporary directory
	// and test actual mount operations

	t.Run("directory mount", func(t *testing.T) {
		// Create temporary directories for testing
		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "source")
		targetDir := filepath.Join(tmpDir, "target")

		platform := &platformfakes.FakePlatform{}
		platform.MkdirAllReturns(nil)
		platform.MountReturns(nil)

		// Mock Stat to return directory info
		mockFileInfo := &mockFileInfo{isDir: true}
		platform.StatReturns(mockFileInfo, nil)

		// Mock OpenFile for file creation
		tempFile, _ := os.CreateTemp("", "test")
		platform.OpenFileReturns(tempFile, nil)

		manager := NewManager("/opt/joblet/runtimes", platform)

		// Note: Actual mount would require root privileges
		// This test validates the setup logic
		err := manager.mountPath(sourceDir, targetDir, true)
		if err != nil {
			t.Fatalf("mountPath failed: %v", err)
		}

		// Verify MkdirAll was called
		if platform.MkdirAllCallCount() > 0 {
			_, perms := platform.MkdirAllArgsForCall(0)
			if perms != 0755 {
				t.Errorf("Expected permissions 0755, got %o", perms)
			}
		}
	})

	t.Run("file mount", func(t *testing.T) {
		// Create temporary files for testing
		tmpDir := t.TempDir()
		sourceFile := filepath.Join(tmpDir, "source.txt")
		targetFile := filepath.Join(tmpDir, "target.txt")

		platform := &platformfakes.FakePlatform{}
		platform.MkdirAllReturns(nil)
		platform.MountReturns(nil)

		// Mock Stat to return file info (not a directory)
		mockFileInfo := &mockFileInfo{isDir: false}
		platform.StatReturns(mockFileInfo, nil)

		// Mock OpenFile for file creation
		tempFile, _ := os.CreateTemp("", "test")
		platform.OpenFileReturns(tempFile, nil)

		manager := NewManager("/opt/joblet/runtimes", platform)

		// Note: Actual mount would require root privileges
		err := manager.mountPath(sourceFile, targetFile, false)
		if err != nil {
			t.Fatalf("mountPath failed: %v", err)
		}

		// Verify parent directory creation was attempted
		if platform.MkdirAllCallCount() > 0 {
			path, _ := platform.MkdirAllArgsForCall(0)
			expectedParent := filepath.Dir(targetFile)
			if path != expectedParent {
				t.Errorf("Expected parent dir %s, got %s", expectedParent, path)
			}
		}
	})
}
