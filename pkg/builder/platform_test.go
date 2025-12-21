//go:build linux

package builder

import (
	"testing"
)

func TestPlatformInfo_GetPlatformString(t *testing.T) {
	tests := []struct {
		name     string
		platform PlatformInfo
		expected string
	}{
		{
			name: "ubuntu-amd64",
			platform: PlatformInfo{
				Distro: "ubuntu",
				Arch:   "amd64",
			},
			expected: "ubuntu-amd64",
		},
		{
			name: "ubuntu-arm64",
			platform: PlatformInfo{
				Distro: "ubuntu",
				Arch:   "arm64",
			},
			expected: "ubuntu-arm64",
		},
		{
			name: "rhel-amd64",
			platform: PlatformInfo{
				Distro: "rhel",
				Arch:   "amd64",
			},
			expected: "rhel-amd64",
		},
		{
			name: "fedora-arm64",
			platform: PlatformInfo{
				Distro: "fedora",
				Arch:   "arm64",
			},
			expected: "fedora-arm64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.platform.GetPlatformString()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPlatformInfo_IsPlatformSupported(t *testing.T) {
	platform := PlatformInfo{
		Distro: "ubuntu",
		Arch:   "amd64",
	}

	tests := []struct {
		name      string
		platforms []string
		expected  bool
	}{
		{
			name:      "empty list - all supported",
			platforms: []string{},
			expected:  true,
		},
		{
			name:      "nil list - all supported",
			platforms: nil,
			expected:  true,
		},
		{
			name:      "platform in list",
			platforms: []string{"ubuntu-amd64", "rhel-amd64"},
			expected:  true,
		},
		{
			name:      "platform not in list",
			platforms: []string{"rhel-amd64", "fedora-amd64"},
			expected:  false,
		},
		{
			name:      "only current platform",
			platforms: []string{"ubuntu-amd64"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := platform.IsPlatformSupported(tt.platforms)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDetectPlatform_Basic(t *testing.T) {
	// This test verifies DetectPlatform runs without error on the current system
	// Since we're running on Linux, it should succeed
	platform, err := DetectPlatform()
	if err != nil {
		t.Fatalf("DetectPlatform failed: %v", err)
	}

	// Verify required fields are set
	if platform.Distro == "" {
		t.Error("expected Distro to be set")
	}
	if platform.Arch == "" {
		t.Error("expected Arch to be set")
	}
	if platform.PkgManager == "" {
		t.Error("expected PkgManager to be set")
	}
	if platform.LibPath == "" {
		t.Error("expected LibPath to be set")
	}

	// Verify arch is valid
	if platform.Arch != "amd64" && platform.Arch != "arm64" {
		t.Errorf("unexpected arch: %s", platform.Arch)
	}

	// Verify package manager is valid
	validPkgManagers := []string{"apt", "yum", "dnf"}
	found := false
	for _, pm := range validPkgManagers {
		if platform.PkgManager == pm {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("unexpected package manager: %s", platform.PkgManager)
	}
}

func TestPlatformInfo_PackageManagerDistros(t *testing.T) {
	// Test that we correctly map distros to package managers
	tests := []struct {
		distro     string
		arch       string
		pkgManager string
		libPath    string
	}{
		{"ubuntu", "amd64", "apt", "/lib/x86_64-linux-gnu"},
		{"ubuntu", "arm64", "apt", "/lib/aarch64-linux-gnu"},
		{"debian", "amd64", "apt", "/lib/x86_64-linux-gnu"},
		{"rhel", "amd64", "yum", "/lib64"},
		{"centos", "amd64", "yum", "/lib64"},
		{"rocky", "amd64", "yum", "/lib64"},
		{"almalinux", "amd64", "yum", "/lib64"},
		{"fedora", "amd64", "dnf", "/lib64"},
		{"amzn", "amd64", "yum", "/lib64"},
	}

	for _, tt := range tests {
		t.Run(tt.distro+"-"+tt.arch, func(t *testing.T) {
			// We can't actually test DetectPlatform with different distros
			// without mocking, but we can verify the struct works correctly
			platform := PlatformInfo{
				Distro:     tt.distro,
				Arch:       tt.arch,
				PkgManager: tt.pkgManager,
				LibPath:    tt.libPath,
			}

			if platform.GetPlatformString() != tt.distro+"-"+tt.arch {
				t.Errorf("unexpected platform string")
			}
		})
	}
}

func TestPlatformInfo_Fields(t *testing.T) {
	platform := PlatformInfo{
		Distro:     "ubuntu",
		Arch:       "amd64",
		PkgManager: "apt",
		LibPath:    "/lib/x86_64-linux-gnu",
	}

	if platform.Distro != "ubuntu" {
		t.Errorf("expected distro 'ubuntu', got %s", platform.Distro)
	}
	if platform.Arch != "amd64" {
		t.Errorf("expected arch 'amd64', got %s", platform.Arch)
	}
	if platform.PkgManager != "apt" {
		t.Errorf("expected pkgManager 'apt', got %s", platform.PkgManager)
	}
	if platform.LibPath != "/lib/x86_64-linux-gnu" {
		t.Errorf("expected libPath '/lib/x86_64-linux-gnu', got %s", platform.LibPath)
	}
}
