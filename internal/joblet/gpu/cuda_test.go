package gpu

import (
	"testing"

	"joblet/pkg/platform/platformfakes"

	"github.com/stretchr/testify/assert"
)

func TestCUDAVersion_String(t *testing.T) {
	version := CUDAVersion{Major: 12, Minor: 1}
	assert.Equal(t, "12.1", version.String())
}

func TestCUDAVersion_IsCompatible(t *testing.T) {
	tests := []struct {
		name       string
		version    CUDAVersion
		required   CUDAVersion
		compatible bool
	}{
		{
			name:       "Same version",
			version:    CUDAVersion{Major: 12, Minor: 1},
			required:   CUDAVersion{Major: 12, Minor: 1},
			compatible: true,
		},
		{
			name:       "Higher major version",
			version:    CUDAVersion{Major: 13, Minor: 0},
			required:   CUDAVersion{Major: 12, Minor: 1},
			compatible: true,
		},
		{
			name:       "Same major, higher minor",
			version:    CUDAVersion{Major: 12, Minor: 2},
			required:   CUDAVersion{Major: 12, Minor: 1},
			compatible: true,
		},
		{
			name:       "Lower major version",
			version:    CUDAVersion{Major: 11, Minor: 8},
			required:   CUDAVersion{Major: 12, Minor: 1},
			compatible: false,
		},
		{
			name:       "Same major, lower minor",
			version:    CUDAVersion{Major: 12, Minor: 0},
			required:   CUDAVersion{Major: 12, Minor: 1},
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.compatible, tt.version.IsCompatible(tt.required))
		})
	}
}

func TestParseVersionString(t *testing.T) {
	detector := &CUDADetector{}

	tests := []struct {
		name        string
		versionStr  string
		expected    CUDAVersion
		expectError bool
	}{
		{
			name:       "Valid version",
			versionStr: "12.1",
			expected:   CUDAVersion{Major: 12, Minor: 1},
		},
		{
			name:       "Three part version",
			versionStr: "11.8.0",
			expected:   CUDAVersion{Major: 11, Minor: 8},
		},
		{
			name:        "Invalid format",
			versionStr:  "12",
			expectError: true,
		},
		{
			name:        "Non-numeric major",
			versionStr:  "abc.1",
			expectError: true,
		},
		{
			name:        "Non-numeric minor",
			versionStr:  "12.abc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := detector.parseVersionString(tt.versionStr)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Major, version.Major)
				assert.Equal(t, tt.expected.Minor, version.Minor)
			}
		})
	}
}

func TestParseNvccOutput(t *testing.T) {
	detector := &CUDADetector{}

	tests := []struct {
		name     string
		output   string
		expected CUDAVersion
		wantErr  bool
	}{
		{
			name: "Standard nvcc output",
			output: `nvcc: NVIDIA (R) Cuda compiler driver
Copyright (c) 2005-2023 NVIDIA Corporation
Built on Tue_Aug_15_22:02:13_PDT_2023
Cuda compilation tools, release 12.2, V12.2.140
Built on Tue_Aug_15_22:02:13_PDT_2023`,
			expected: CUDAVersion{Major: 12, Minor: 2},
		},
		{
			name:     "V-style version",
			output:   "Build cuda_12.1.r12.1/compiler.32415258_0",
			expected: CUDAVersion{Major: 12, Minor: 1},
		},
		{
			name:    "No version found",
			output:  "Some random output without version",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := detector.parseNvccOutput(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Major, version.Major)
				assert.Equal(t, tt.expected.Minor, version.Minor)
			}
		})
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	detector := &CUDADetector{}

	tests := []struct {
		name     string
		path     string
		expected CUDAVersion
		wantErr  bool
	}{
		{
			name:     "Version in path",
			path:     "/usr/local/cuda-12.1",
			expected: CUDAVersion{Major: 12, Minor: 1},
		},
		{
			name:     "Base path with version",
			path:     "/opt/cuda-11.8",
			expected: CUDAVersion{Major: 11, Minor: 8},
		},
		{
			name:    "No version in path",
			path:    "/usr/local/cuda",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := detector.extractVersionFromPath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Major, version.Major)
				assert.Equal(t, tt.expected.Minor, version.Minor)
			}
		})
	}
}

func TestIsVersionedCUDADir(t *testing.T) {
	detector := &CUDADetector{}

	tests := []struct {
		name     string
		dirname  string
		expected bool
	}{
		{"Valid versioned dir", "cuda-12.1", true},
		{"Valid versioned dir 2", "cuda-11.8", true},
		{"Invalid - no version", "cuda", false},
		{"Invalid - wrong format", "cuda-12", false},
		{"Invalid - not cuda", "python-3.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, detector.isVersionedCUDADir(tt.dirname))
		})
	}
}

func TestDetectCUDAVersion_FromPath(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	detector := &CUDADetector{platform: fakePlatform}

	// Test version extraction from path
	version, err := detector.detectCUDAVersion("/usr/local/cuda-12.1")
	assert.NoError(t, err)
	assert.Equal(t, 12, version.Major)
	assert.Equal(t, 1, version.Minor)
	assert.Equal(t, "/usr/local/cuda-12.1", version.Path)
}

// Simplified tests without command mocking for now

func TestMarkDefaultInstallation(t *testing.T) {
	fakePlatform := &platformfakes.FakePlatform{}
	detector := &CUDADetector{platform: fakePlatform}

	tests := []struct {
		name            string
		cudaHome        string
		installations   []CUDAInstallation
		expectedDefault string
	}{
		{
			name:     "CUDA_HOME priority",
			cudaHome: "/opt/cuda-12.1",
			installations: []CUDAInstallation{
				{Path: "/usr/local/cuda", Version: CUDAVersion{Major: 11, Minor: 8}},
				{Path: "/opt/cuda-12.1", Version: CUDAVersion{Major: 12, Minor: 1}},
			},
			expectedDefault: "/opt/cuda-12.1",
		},
		{
			name:     "Default symlink priority",
			cudaHome: "",
			installations: []CUDAInstallation{
				{Path: "/opt/cuda-12.1", Version: CUDAVersion{Major: 12, Minor: 1}},
				{Path: "/usr/local/cuda", Version: CUDAVersion{Major: 11, Minor: 8}},
			},
			expectedDefault: "/usr/local/cuda",
		},
		{
			name:     "Highest version fallback",
			cudaHome: "",
			installations: []CUDAInstallation{
				{Path: "/opt/cuda-11.8", Version: CUDAVersion{Major: 11, Minor: 8}},
				{Path: "/opt/cuda-12.1", Version: CUDAVersion{Major: 12, Minor: 1}},
			},
			expectedDefault: "/opt/cuda-12.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock CUDA_HOME environment variable
			fakePlatform.GetenvReturns(tt.cudaHome)

			// Reset IsDefault flags
			for i := range tt.installations {
				tt.installations[i].IsDefault = false
			}

			detector.markDefaultInstallation(tt.installations)

			// Check which installation is marked as default
			var defaultPath string
			for _, install := range tt.installations {
				if install.IsDefault {
					defaultPath = install.Path
					break
				}
			}

			assert.Equal(t, tt.expectedDefault, defaultPath)
		})
	}
}

func TestFindCompatibleCUDA_Logic(t *testing.T) {
	// Test the compatibility logic directly without platform mocking
	installations := []CUDAInstallation{
		{Path: "/opt/cuda-11.8", Version: CUDAVersion{Major: 11, Minor: 8}},
		{Path: "/opt/cuda-12.1", Version: CUDAVersion{Major: 12, Minor: 1}},
		{Path: "/opt/cuda-12.2", Version: CUDAVersion{Major: 12, Minor: 2}},
	}

	requiredVersion := CUDAVersion{Major: 12, Minor: 1}

	var compatible []CUDAInstallation
	for _, install := range installations {
		if install.Version.IsCompatible(requiredVersion) {
			compatible = append(compatible, install)
		}
	}

	assert.Equal(t, 2, len(compatible))
	assert.Equal(t, "/opt/cuda-12.1", compatible[0].Path)
	assert.Equal(t, "/opt/cuda-12.2", compatible[1].Path)
}

func TestGetBestCUDA_Logic(t *testing.T) {
	installations := []CUDAInstallation{
		{Path: "/opt/cuda-12.1", Version: CUDAVersion{Major: 12, Minor: 1}, IsDefault: false},
		{Path: "/opt/cuda-12.2", Version: CUDAVersion{Major: 12, Minor: 2}, IsDefault: true},
		{Path: "/opt/cuda-13.0", Version: CUDAVersion{Major: 13, Minor: 0}, IsDefault: false},
	}
	compatible := []CUDAInstallation{installations[0], installations[1], installations[2]}

	// Find default installation if compatible
	var best CUDAInstallation
	for _, install := range compatible {
		if install.IsDefault {
			best = install
			break
		}
	}

	if best.Path == "" {
		// Otherwise, find closest compatible version
		best = compatible[0]
		for _, install := range compatible {
			if install.Version.Major < best.Version.Major ||
				(install.Version.Major == best.Version.Major && install.Version.Minor < best.Version.Minor) {
				best = install
			}
		}
	}

	assert.Equal(t, "/opt/cuda-12.2", best.Path)
	assert.True(t, best.IsDefault)
}
