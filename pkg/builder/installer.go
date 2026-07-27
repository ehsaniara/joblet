//go:build linux

package builder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// mightNeedPPA checks if a package might need the deadsnakes PPA
func mightNeedPPA(pkg string) bool {
	// Python 3.10+ packages might need deadsnakes PPA on older Ubuntu
	return strings.HasPrefix(pkg, "python3.1") || strings.HasPrefix(pkg, "python3.2")
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
				// Check if this might be a Python package that needs deadsnakes PPA
				if mightNeedPPA(pkg) {
					logger.Debug("Package %s not in default repos, will try deadsnakes PPA during install", pkg)
					continue
				}
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
