//go:build linux

package builder

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// DetectPlatform detects the current platform information
func DetectPlatform() (*PlatformInfo, error) {
	info := &PlatformInfo{}

	// Detect architecture
	switch runtime.GOARCH {
	case "amd64":
		info.Arch = "amd64"
	case "arm64":
		info.Arch = "arm64"
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	// Detect distribution from /etc/os-release
	distro, err := detectDistro()
	if err != nil {
		return nil, fmt.Errorf("failed to detect distribution: %w", err)
	}
	info.Distro = distro

	// Set package manager based on distro
	switch distro {
	case "ubuntu", "debian":
		info.PkgManager = "apt"
		if info.Arch == "amd64" {
			info.LibPath = "/lib/x86_64-linux-gnu"
		} else {
			info.LibPath = "/lib/aarch64-linux-gnu"
		}
	case "rhel", "centos", "rocky", "almalinux":
		info.PkgManager = "yum"
		info.LibPath = "/lib64"
	case "fedora":
		info.PkgManager = "dnf"
		info.LibPath = "/lib64"
	case "amzn":
		info.PkgManager = "yum"
		info.LibPath = "/lib64"
	default:
		return nil, fmt.Errorf("unsupported distribution: %s", distro)
	}

	return info, nil
}

func detectDistro() (string, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("cannot open /etc/os-release: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id := strings.TrimPrefix(line, "ID=")
			id = strings.Trim(id, "\"")
			return id, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading /etc/os-release: %w", err)
	}

	return "", fmt.Errorf("ID not found in /etc/os-release")
}

// GetPlatformString returns the platform string (e.g., "ubuntu-amd64")
func (p *PlatformInfo) GetPlatformString() string {
	return fmt.Sprintf("%s-%s", p.Distro, p.Arch)
}

// IsPlatformSupported checks if the current platform is in the supported list
func (p *PlatformInfo) IsPlatformSupported(platforms []string) bool {
	if len(platforms) == 0 {
		// If no platforms specified, all are supported
		return true
	}

	platformStr := p.GetPlatformString()
	for _, platform := range platforms {
		if platform == platformStr {
			return true
		}
	}
	return false
}
