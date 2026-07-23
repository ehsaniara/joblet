package runtime

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/ehsaniara/joblet/pkg/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResolver(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test/runtimes", testPlatform)

	assert.NotNil(t, resolver)
	assert.Equal(t, "/test/runtimes", resolver.runtimesPath)
	assert.Equal(t, testPlatform, resolver.platform)
}

func TestResolver_ListRuntimes_EmptyDirectory(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	// Test with non-existent directory
	runtimes, err := resolver.ListRuntimes()
	require.NoError(t, err)
	assert.Empty(t, runtimes)

	// Create empty directory
	err = os.MkdirAll(runtimesPath, 0755)
	require.NoError(t, err)

	runtimes, err = resolver.ListRuntimes()
	require.NoError(t, err)
	assert.Empty(t, runtimes)
}

func TestResolver_ListRuntimes_WithValidRuntimes(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create test runtime directories with valid runtime.yml files
	pythonRuntime := filepath.Join(runtimesPath, "python-3.11-ml")
	javaRuntime := filepath.Join(runtimesPath, "openjdk-21")

	// Create directories
	err := os.MkdirAll(pythonRuntime, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(javaRuntime, 0755)
	require.NoError(t, err)

	// Create python runtime.yml
	pythonConfig := `name: python-3.11-ml
language: python
version: "3.11.9"
description: "Python 3.11 with ML packages"
mounts:
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
environment:
  PATH: "/opt/venv/bin:/usr/bin:/bin"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
`
	err = os.WriteFile(filepath.Join(pythonRuntime, "runtime.yml"), []byte(pythonConfig), 0644)
	require.NoError(t, err)

	// Create java runtime.yml
	javaConfig := `name: openjdk-21
language: java
version: "21.0.4"
description: "OpenJDK 21"
mounts:
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
environment:
  JAVA_HOME: "/opt/java/jdk-21"
  PATH: "/opt/java/jdk-21/bin:/usr/bin:/bin"
`
	err = os.WriteFile(filepath.Join(javaRuntime, "runtime.yml"), []byte(javaConfig), 0644)
	require.NoError(t, err)

	// Create some test files to verify size calculation
	testFile := filepath.Join(pythonRuntime, "test_file.txt")
	err = os.WriteFile(testFile, []byte("test data"), 0644)
	require.NoError(t, err)

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	runtimes, err := resolver.ListRuntimes()
	require.NoError(t, err)
	require.Len(t, runtimes, 2)

	// Verify python runtime
	var pythonInfo *RuntimeInfo
	for _, runtime := range runtimes {
		if runtime.Name == "python-3.11-ml" {
			pythonInfo = runtime
			break
		}
	}
	require.NotNil(t, pythonInfo)
	assert.Equal(t, "python-3.11-ml", pythonInfo.Name)
	assert.Equal(t, "python", pythonInfo.Language)
	assert.Equal(t, "3.11.9", pythonInfo.Version)
	assert.Equal(t, "Python 3.11 with ML packages", pythonInfo.Description)
	assert.True(t, pythonInfo.Available)
	assert.Greater(t, pythonInfo.Size, int64(0)) // Should have calculated size

	// Verify java runtime
	var javaInfo *RuntimeInfo
	for _, runtime := range runtimes {
		if runtime.Name == "openjdk-21" {
			javaInfo = runtime
			break
		}
	}
	require.NotNil(t, javaInfo)
	assert.Equal(t, "openjdk-21", javaInfo.Name)
	assert.Equal(t, "java", javaInfo.Language)
	assert.Equal(t, "21.0.4", javaInfo.Version)
	assert.Equal(t, "OpenJDK 21", javaInfo.Description)
	assert.True(t, javaInfo.Available)
}

func TestResolver_ListRuntimes_SkipsInvalidRuntimes(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create directory with invalid runtime.yml
	invalidRuntime := filepath.Join(runtimesPath, "invalid-runtime")
	err := os.MkdirAll(invalidRuntime, 0755)
	require.NoError(t, err)

	// Create invalid YAML
	invalidConfig := `invalid yaml content [[[`
	err = os.WriteFile(filepath.Join(invalidRuntime, "runtime.yml"), []byte(invalidConfig), 0644)
	require.NoError(t, err)

	// Create directory without runtime.yml
	noConfigRuntime := filepath.Join(runtimesPath, "no-config")
	err = os.MkdirAll(noConfigRuntime, 0755)
	require.NoError(t, err)

	// Create valid runtime
	validRuntime := filepath.Join(runtimesPath, "valid-runtime")
	err = os.MkdirAll(validRuntime, 0755)
	require.NoError(t, err)
	validConfig := `name: valid-runtime
language: test
version: "1.0"
description: "Test runtime"
mounts: []
`
	err = os.WriteFile(filepath.Join(validRuntime, "runtime.yml"), []byte(validConfig), 0644)
	require.NoError(t, err)

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	runtimes, err := resolver.ListRuntimes()
	require.NoError(t, err)
	require.Len(t, runtimes, 1) // Only the valid runtime should be returned
	assert.Equal(t, "valid-runtime", runtimes[0].Name)
}

func TestResolver_ResolveRuntime(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create test runtime
	runtimeDir := filepath.Join(runtimesPath, "python-3.11")
	err := os.MkdirAll(runtimeDir, 0755)
	require.NoError(t, err)

	// Use the current system architecture to ensure the test passes
	currentArch := goruntime.GOARCH
	config := `name: python-3.11
language: python
version: "3.11.9"
description: "Python 3.11 runtime"
mounts:
  - source: "isolated/bin"
    target: "/bin"
requirements:
  architectures: ["` + currentArch + `"]
packages:
  - numpy
  - pandas
`
	err = os.WriteFile(filepath.Join(runtimeDir, "runtime.yml"), []byte(config), 0644)
	require.NoError(t, err)

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	// Test resolving existing runtime
	resolvedConfig, err := resolver.ResolveRuntime("python-3.11")
	require.NoError(t, err)
	require.NotNil(t, resolvedConfig)
	assert.Equal(t, "python-3.11", resolvedConfig.Name)
	assert.Equal(t, "python", resolvedConfig.Language)
	assert.Equal(t, "3.11.9", resolvedConfig.Version)
	assert.Equal(t, []string{currentArch}, resolvedConfig.Requirements.Architectures)
	assert.Equal(t, []string{"numpy", "pandas"}, resolvedConfig.Packages)

	// Test resolving non-existent runtime
	_, err = resolver.ResolveRuntime("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runtime not found")

	// Test empty spec
	resolvedConfig, err = resolver.ResolveRuntime("")
	require.NoError(t, err)
	assert.Nil(t, resolvedConfig)
}

func TestResolver_extractTypeFromName(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test", testPlatform)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "python", "python"},
		{"with version", "python-3.11", "python"},
		{"with multiple parts", "python-3.11-ml", "python"},
		{"java runtime", "openjdk-21", "openjdk"},
		{"single character", "r-4.0", "r"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.extractTypeFromName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolver_parseRuntimeSpec(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test", testPlatform)

	tests := []struct {
		name         string
		spec         string
		expectedLang string
		expectedVer  string
		expectedTags []string
		shouldMatch  bool
	}{
		{
			name:         "simple spec",
			spec:         "python-3.11",
			expectedLang: "python",
			expectedVer:  "3.11",
			expectedTags: nil,
			shouldMatch:  true,
		},
		{
			name:         "spec with tags",
			spec:         "python-3.11-ml+gpu",
			expectedLang: "python",
			expectedVer:  "3.11",
			expectedTags: []string{"ml", "gpu"},
			shouldMatch:  true,
		},
		{
			name:         "runtime name only",
			spec:         "python-3.11-ml",
			expectedLang: "python",
			expectedVer:  "unknown",
			expectedTags: []string{},
			shouldMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.parseRuntimeSpec(tt.spec)
			assert.Equal(t, tt.expectedLang, result.Language)
			assert.Equal(t, tt.expectedVer, result.Version)
			assert.Equal(t, tt.expectedTags, result.Tags)
		})
	}
}

func TestResolver_runtimeMatches(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test", testPlatform)

	config := &RuntimeConfig{
		Name:     "python-3.11-ml",
		Language: "python",
		Version:  "3.11.9",
	}

	tests := []struct {
		name     string
		spec     *RuntimeSpec
		expected bool
	}{
		{
			name: "exact match",
			spec: &RuntimeSpec{
				Language: "python",
				Version:  "3.11.9",
				Tags:     nil,
			},
			expected: true,
		},
		{
			name: "language match version unknown",
			spec: &RuntimeSpec{
				Language: "python",
				Version:  "unknown",
				Tags:     nil,
			},
			expected: true,
		},
		{
			name: "language mismatch",
			spec: &RuntimeSpec{
				Language: "java",
				Version:  "3.11.9",
				Tags:     nil,
			},
			expected: false,
		},
		{
			name: "version mismatch",
			spec: &RuntimeSpec{
				Language: "python",
				Version:  "3.12",
				Tags:     nil,
			},
			expected: false,
		},
		{
			name: "empty language in spec",
			spec: &RuntimeSpec{
				Language: "",
				Version:  "3.11.9",
				Tags:     nil,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.runtimeMatches(config, tt.spec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolver_validateRuntime(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test", testPlatform)

	// Get current architecture to build dynamic test cases
	currentArch := goruntime.GOARCH

	// Create an incompatible architecture that differs from current
	incompatibleArch := "mips"
	if currentArch == "mips" {
		incompatibleArch = "sparc"
	}

	tests := []struct {
		name        string
		config      *RuntimeConfig
		expectError bool
		errorText   string
	}{
		{
			name: "valid config no requirements",
			config: &RuntimeConfig{
				Name:         "test-runtime",
				Language:     "python",
				Version:      "3.11",
				Requirements: RuntimeRequirements{},
			},
			expectError: false,
		},
		{
			name: "valid config with compatible arch",
			config: &RuntimeConfig{
				Name:     "test-runtime",
				Language: "python",
				Version:  "3.11",
				Requirements: RuntimeRequirements{
					Architectures: []string{currentArch},
				},
			},
			expectError: false,
		},
		{
			name: "incompatible architecture",
			config: &RuntimeConfig{
				Name:     "test-runtime",
				Language: "python",
				Version:  "3.11",
				Requirements: RuntimeRequirements{
					Architectures: []string{incompatibleArch},
				},
			},
			expectError: true,
			errorText:   "runtime requires architecture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.validateRuntime(tt.config)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorText != "" {
					assert.Contains(t, err.Error(), tt.errorText)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolver_findRuntimeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create test runtime
	runtimeDir := filepath.Join(runtimesPath, "python-3.11-ml")
	err := os.MkdirAll(runtimeDir, 0755)
	require.NoError(t, err)

	config := `name: python-3.11-ml
language: python
version: "3.11.9"
description: "Python 3.11 with ML"
`
	err = os.WriteFile(filepath.Join(runtimeDir, "runtime.yml"), []byte(config), 0644)
	require.NoError(t, err)

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	// Test finding existing runtime
	foundPath, err := resolver.FindRuntimeDirectory("python-3.11-ml")
	require.NoError(t, err)
	assert.Equal(t, runtimeDir, foundPath)

	// Test runtime not found
	_, err = resolver.FindRuntimeDirectory("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runtime not found")
}

func TestResolver_FindRuntimeDirectory_DefaultToLatest(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create nested version structure with multiple versions
	// /runtimes/python-3.11-ml/1.0.0/
	// /runtimes/python-3.11-ml/1.2.0/
	// /runtimes/python-3.11-ml/1.1.0/
	runtimeName := "python-3.11-ml"
	runtimeBaseDir := filepath.Join(runtimesPath, runtimeName)

	versions := []string{"1.0.0", "1.2.0", "1.1.0"}
	for _, version := range versions {
		versionDir := filepath.Join(runtimeBaseDir, version)
		err := os.MkdirAll(versionDir, 0755)
		require.NoError(t, err)

		config := `name: python-3.11-ml
language: python
version: "` + version + `"
description: "Python 3.11 with ML"
`
		err = os.WriteFile(filepath.Join(versionDir, "runtime.yml"), []byte(config), 0644)
		require.NoError(t, err)
	}

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	// Test: When no version specified, should return latest (1.2.0)
	foundPath, err := resolver.FindRuntimeDirectory("python-3.11-ml")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(runtimeBaseDir, "1.2.0"), foundPath)

	// Test: When @latest specified, should return latest (1.2.0)
	foundPath, err = resolver.FindRuntimeDirectory("python-3.11-ml@latest")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(runtimeBaseDir, "1.2.0"), foundPath)

	// Test: When specific version specified, should return that version
	foundPath, err = resolver.FindRuntimeDirectory("python-3.11-ml@1.0.0")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(runtimeBaseDir, "1.0.0"), foundPath)

	// Test: When non-existent version specified, should error
	_, err = resolver.FindRuntimeDirectory("python-3.11-ml@9.9.9")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runtime not found")
}

func TestResolver_FindRuntimeDirectory_SingleVersion(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create nested version structure with single version
	runtimeName := "openjdk-21"
	versionDir := filepath.Join(runtimesPath, runtimeName, "1.0.0")
	err := os.MkdirAll(versionDir, 0755)
	require.NoError(t, err)

	config := `name: openjdk-21
language: java
version: "1.0.0"
description: "OpenJDK 21"
`
	err = os.WriteFile(filepath.Join(versionDir, "runtime.yml"), []byte(config), 0644)
	require.NoError(t, err)

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	// Test: Should find the single version without @
	foundPath, err := resolver.FindRuntimeDirectory("openjdk-21")
	require.NoError(t, err)
	assert.Equal(t, versionDir, foundPath)

	// Test: Should find with @latest
	foundPath, err = resolver.FindRuntimeDirectory("openjdk-21@latest")
	require.NoError(t, err)
	assert.Equal(t, versionDir, foundPath)

	// Test: Should find with explicit version
	foundPath, err = resolver.FindRuntimeDirectory("openjdk-21@1.0.0")
	require.NoError(t, err)
	assert.Equal(t, versionDir, foundPath)
}

func TestResolver_findLatestVersion(t *testing.T) {
	testPlatform := platform.NewPlatform()
	resolver := NewResolver("/test", testPlatform)

	tests := []struct {
		name     string
		versions []struct {
			path    string
			version string
		}
		expected string
	}{
		{
			name:     "empty list",
			versions: nil,
			expected: "",
		},
		{
			name: "single version",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/1.0.0", version: "1.0.0"},
			},
			expected: "/runtimes/test/1.0.0",
		},
		{
			name: "multiple versions ascending",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/1.0.0", version: "1.0.0"},
				{path: "/runtimes/test/1.1.0", version: "1.1.0"},
				{path: "/runtimes/test/1.2.0", version: "1.2.0"},
			},
			expected: "/runtimes/test/1.2.0",
		},
		{
			name: "multiple versions descending",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/2.0.0", version: "2.0.0"},
				{path: "/runtimes/test/1.5.0", version: "1.5.0"},
				{path: "/runtimes/test/1.0.0", version: "1.0.0"},
			},
			expected: "/runtimes/test/2.0.0",
		},
		{
			name: "multiple versions mixed order",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/1.2.0", version: "1.2.0"},
				{path: "/runtimes/test/1.0.0", version: "1.0.0"},
				{path: "/runtimes/test/1.3.0", version: "1.3.0"},
				{path: "/runtimes/test/1.1.0", version: "1.1.0"},
			},
			expected: "/runtimes/test/1.3.0",
		},
		{
			name: "major version difference",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/1.9.9", version: "1.9.9"},
				{path: "/runtimes/test/2.0.0", version: "2.0.0"},
			},
			expected: "/runtimes/test/2.0.0",
		},
		{
			name: "patch version difference",
			versions: []struct {
				path    string
				version string
			}{
				{path: "/runtimes/test/1.0.1", version: "1.0.1"},
				{path: "/runtimes/test/1.0.10", version: "1.0.10"},
				{path: "/runtimes/test/1.0.2", version: "1.0.2"},
			},
			expected: "/runtimes/test/1.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.findLatestVersion(tt.versions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests for performance-critical methods
func BenchmarkListRuntimes(b *testing.B) {
	tempDir := b.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")

	// Create multiple test runtimes
	for i := 0; i < 50; i++ {
		runtimeDir := filepath.Join(runtimesPath, "runtime-"+string(rune('a'+i)))
		err := os.MkdirAll(runtimeDir, 0755)
		require.NoError(b, err)

		config := `name: runtime-` + string(rune('a'+i)) + `
language: test
version: "1.0"
description: "Test runtime"
mounts: []
`
		err = os.WriteFile(filepath.Join(runtimeDir, "runtime.yml"), []byte(config), 0644)
		require.NoError(b, err)
	}

	testPlatform := platform.NewPlatform()
	resolver := NewResolver(runtimesPath, testPlatform)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ListRuntimes()
		require.NoError(b, err)
	}
}

func TestResolver_ResolveRuntimeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")
	runtimeDir := filepath.Join(runtimesPath, "python-3.11")
	require.NoError(t, os.MkdirAll(runtimeDir, 0755))

	config := `name: python-3.11
language: python
version: "1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(runtimeDir, "runtime.yml"), []byte(config), 0644))

	resolver := NewResolver(runtimesPath, platform.NewPlatform())

	dir, cfg, err := resolver.ResolveRuntimeDirectory("python-3.11")
	require.NoError(t, err)
	assert.Equal(t, runtimeDir, dir)
	require.NotNil(t, cfg)
	assert.Equal(t, "python-3.11", cfg.Name)
}

func TestResolver_ResolveRuntimeDirectory_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")
	require.NoError(t, os.MkdirAll(runtimesPath, 0755))

	resolver := NewResolver(runtimesPath, platform.NewPlatform())

	_, _, err := resolver.ResolveRuntimeDirectory("does-not-exist")
	require.Error(t, err)
}

func TestResolver_ResolveRuntimeDirectory_IncompatibleArchitecture(t *testing.T) {
	tempDir := t.TempDir()
	runtimesPath := filepath.Join(tempDir, "runtimes")
	runtimeDir := filepath.Join(runtimesPath, "python-3.11")
	require.NoError(t, os.MkdirAll(runtimeDir, 0755))

	config := `name: python-3.11
language: python
version: "1.0.0"
requirements:
  architectures: ["mips64"]
`
	require.NoError(t, os.WriteFile(filepath.Join(runtimeDir, "runtime.yml"), []byte(config), 0644))

	resolver := NewResolver(runtimesPath, platform.NewPlatform())

	_, _, err := resolver.ResolveRuntimeDirectory("python-3.11")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "architecture")
}

func TestNormalizeArch(t *testing.T) {
	assert.Equal(t, "amd64", normalizeArch("x86_64"))
	assert.Equal(t, "arm64", normalizeArch("aarch64"))
	assert.Equal(t, "amd64", normalizeArch("amd64"))
	assert.Equal(t, "arm64", normalizeArch("arm64"))
	assert.Equal(t, "riscv64", normalizeArch("riscv64"))
}
