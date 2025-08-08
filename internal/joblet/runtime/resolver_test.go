package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"joblet/pkg/platform/platformfakes"

	"gopkg.in/yaml.v3"
)

func TestParseRuntimeSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		expected RuntimeSpec
	}{
		{
			name: "simple language and version",
			spec: "python:3.11",
			expected: RuntimeSpec{
				Language: "python",
				Version:  "3.11",
				Tags:     []string{},
			},
		},
		{
			name: "with single tag",
			spec: "python:3.11+ml",
			expected: RuntimeSpec{
				Language: "python",
				Version:  "3.11",
				Tags:     []string{"ml"},
			},
		},
		{
			name: "with multiple tags",
			spec: "python:3.11+ml+gpu",
			expected: RuntimeSpec{
				Language: "python",
				Version:  "3.11",
				Tags:     []string{"ml", "gpu"},
			},
		},
		{
			name: "java version",
			spec: "java:17",
			expected: RuntimeSpec{
				Language: "java",
				Version:  "17",
				Tags:     []string{},
			},
		},
		{
			name: "no version specified",
			spec: "python",
			expected: RuntimeSpec{
				Language: "python",
				Version:  "latest",
				Tags:     []string{},
			},
		},
	}

	platform := &platformfakes.FakePlatform{}
	resolver := NewResolver("/opt/joblet/runtimes", platform)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.parseRuntimeSpec(tt.spec)

			if result.Language != tt.expected.Language {
				t.Errorf("Language: got %s, want %s", result.Language, tt.expected.Language)
			}
			if result.Version != tt.expected.Version {
				t.Errorf("Version: got %s, want %s", result.Version, tt.expected.Version)
			}
			if len(result.Tags) != len(tt.expected.Tags) {
				t.Errorf("Tags length: got %d, want %d", len(result.Tags), len(tt.expected.Tags))
			}
			for i, tag := range result.Tags {
				if i < len(tt.expected.Tags) && tag != tt.expected.Tags[i] {
					t.Errorf("Tag[%d]: got %s, want %s", i, tag, tt.expected.Tags[i])
				}
			}
		})
	}
}

func TestResolveRuntime(t *testing.T) {
	// Create a fake platform
	platform := &platformfakes.FakePlatform{}

	// Setup test runtime directory structure
	runtimesPath := "/opt/joblet/runtimes"
	pythonPath := filepath.Join(runtimesPath, "python", "python-3.11")
	_ = filepath.Join(pythonPath, "runtime.yml")

	// Mock runtime config
	runtimeConfig := RuntimeConfig{
		Name:        "python-3.11",
		Version:     "3.11.5",
		Description: "Python 3.11 runtime",
		Mounts: []MountSpec{
			{
				Source:   "bin",
				Target:   "/usr/local/bin",
				ReadOnly: true,
			},
		},
		Environment: map[string]string{
			"PYTHON_HOME":  "/usr/local",
			"PATH_PREPEND": "/usr/local/bin",
		},
		Requirements: RuntimeRequirements{
			Architectures: []string{"x86_64", "amd64"},
		},
	}

	// Marshal config to YAML
	configData, _ := yaml.Marshal(runtimeConfig)

	// Setup platform mocks
	platform.DirExistsReturns(true)
	platform.FileExistsReturns(true)
	platform.ReadFileReturns(configData, nil)

	resolver := NewResolver(runtimesPath, platform)

	// Test resolving runtime
	config, err := resolver.ResolveRuntime("python:3.11")
	if err != nil {
		t.Fatalf("Failed to resolve runtime: %v", err)
	}

	if config.Name != "python-3.11" {
		t.Errorf("Name: got %s, want python-3.11", config.Name)
	}
	if config.Version != "3.11.5" {
		t.Errorf("Version: got %s, want 3.11.5", config.Version)
	}
	if len(config.Mounts) != 1 {
		t.Errorf("Mounts: got %d, want 1", len(config.Mounts))
	}

	// Verify the absolute path was set for mount source
	expectedSource := filepath.Join(pythonPath, "bin")
	if config.Mounts[0].Source != expectedSource {
		t.Errorf("Mount source: got %s, want %s", config.Mounts[0].Source, expectedSource)
	}
}

func TestValidateRuntime(t *testing.T) {
	platform := &platformfakes.FakePlatform{}
	resolver := NewResolver("/opt/joblet/runtimes", platform)

	// Test architecture validation
	t.Run("valid architecture", func(t *testing.T) {
		config := &RuntimeConfig{
			Requirements: RuntimeRequirements{
				Architectures: []string{"amd64", "x86_64"},
			},
			Mounts: []MountSpec{},
		}

		platform.DirExistsReturns(true)

		err := resolver.validateRuntime(config)
		if err != nil {
			t.Errorf("Expected no error for valid architecture, got: %v", err)
		}
	})

	// Test mount validation
	t.Run("missing mount source", func(t *testing.T) {
		config := &RuntimeConfig{
			Requirements: RuntimeRequirements{
				Architectures: []string{"amd64"},
			},
			Mounts: []MountSpec{
				{
					Source: "/nonexistent/path",
					Target: "/usr/local/bin",
				},
			},
		}

		platform.DirExistsReturns(false)
		platform.FileExistsReturns(false)

		err := resolver.validateRuntime(config)
		if err == nil {
			t.Error("Expected error for missing mount source")
		}
	})

	// Test GPU requirement warning (should not fail)
	t.Run("gpu requirement", func(t *testing.T) {
		config := &RuntimeConfig{
			Requirements: RuntimeRequirements{
				GPU:           true,
				Architectures: []string{"amd64"},
			},
			Mounts: []MountSpec{},
		}

		// Mock no GPU available
		_, err := os.Stat("/dev/nvidia0")
		gpuAvailable := err == nil

		err = resolver.validateRuntime(config)
		if err != nil && gpuAvailable {
			t.Errorf("GPU validation should not fail, got: %v", err)
		}
	})
}
