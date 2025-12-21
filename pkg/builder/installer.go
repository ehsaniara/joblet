//go:build linux

package builder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// InstallSystemPackages installs system packages using the appropriate package manager
func InstallSystemPackages(ctx context.Context, platform *PlatformInfo, packages []string, logger BuildLogger) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Info("Installing system packages: %s", strings.Join(packages, ", "))

	var cmd *exec.Cmd
	switch platform.PkgManager {
	case "apt":
		// Update package list first
		updateCmd := exec.CommandContext(ctx, "apt-get", "update", "-qq")
		if output, err := updateCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("apt-get update failed: %w\nOutput: %s", err, string(output))
		}

		args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
		cmd = exec.CommandContext(ctx, "apt-get", args...)

	case "yum":
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.CommandContext(ctx, "yum", args...)

	case "dnf":
		args := append([]string{"install", "-y"}, packages...)
		cmd = exec.CommandContext(ctx, "dnf", args...)

	default:
		return fmt.Errorf("unsupported package manager: %s", platform.PkgManager)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("package installation failed: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("Package installation output: %s", string(output))
	return nil
}

// InstallPipPackages installs Python packages using pip
func InstallPipPackages(ctx context.Context, packages []string, pipOptions string, isolatedDir string, pythonVersion string, logger BuildLogger) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Info("Installing pip packages: %s", strings.Join(packages, ", "))

	// Build pip install command using the specific Python version's pip
	// Use python3.X -m pip to ensure we use the correct Python version
	pythonBinary := fmt.Sprintf("python%s", pythonVersion)
	args := []string{"-m", "pip", "install", "--no-cache-dir"}

	// Add pip options if specified
	if pipOptions != "" {
		optionParts := strings.Fields(pipOptions)
		args = append(args, optionParts...)
	}

	// Add target directory for isolated installation
	args = append(args, "--target", fmt.Sprintf("%s/usr/local/lib/python3/site-packages", isolatedDir))

	// Add packages
	args = append(args, packages...)

	cmd := exec.CommandContext(ctx, pythonBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fail fast on pip errors
		return fmt.Errorf("pip install failed: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("Pip installation output: %s", string(output))
	return nil
}

// InstallNpmPackages installs Node.js packages using npm
func InstallNpmPackages(ctx context.Context, packages []string, isolatedDir string, logger BuildLogger) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Info("Installing npm packages: %s", strings.Join(packages, ", "))

	// Build npm install command
	args := []string{"install", "-g", "--prefix", fmt.Sprintf("%s/usr/local", isolatedDir)}
	args = append(args, packages...)

	cmd := exec.CommandContext(ctx, "npm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fail fast on npm errors
		return fmt.Errorf("npm install failed: %w\nOutput: %s", err, string(output))
	}

	logger.Debug("npm installation output: %s", string(output))
	return nil
}

// ValidatePackageAvailability checks if packages are available in the package manager
func ValidatePackageAvailability(ctx context.Context, platform *PlatformInfo, packages []string, logger BuildLogger) error {
	if len(packages) == 0 {
		return nil
	}

	logger.Debug("Validating package availability...")

	var cmd *exec.Cmd
	switch platform.PkgManager {
	case "apt":
		// Use apt-cache to check package availability
		for _, pkg := range packages {
			cmd = exec.CommandContext(ctx, "apt-cache", "show", pkg)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("package not found: %s", pkg)
			}
		}

	case "yum", "dnf":
		// Use yum/dnf info to check package availability
		for _, pkg := range packages {
			cmd = exec.CommandContext(ctx, platform.PkgManager, "info", pkg)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("package not found: %s", pkg)
			}
		}

	default:
		return fmt.Errorf("unsupported package manager: %s", platform.PkgManager)
	}

	return nil
}
