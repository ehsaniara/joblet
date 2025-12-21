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
