//go:build linux

package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// ExecuteHook executes a hook script with the given timeout
func ExecuteHook(ctx context.Context, hookName string, script string, buildCtx *BuildContext, logger BuildLogger) error {
	if script == "" {
		return nil
	}

	// Determine timeout
	timeout := DefaultHookTimeout
	if buildCtx.Spec.Hooks != nil && buildCtx.Spec.Hooks.Timeout != "" {
		parsedTimeout, err := time.ParseDuration(buildCtx.Spec.Hooks.Timeout)
		if err != nil {
			logger.Warn("Invalid hook timeout, using default: %v", err)
		} else {
			timeout = parsedTimeout
		}
	}

	logger.Info("Executing %s hook (timeout: %s)", hookName, timeout)

	// Create context with timeout
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create the command
	cmd := exec.CommandContext(hookCtx, "/bin/bash", "-c", script)

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("RUNTIME_DIR=%s", buildCtx.RuntimeDir),
		fmt.Sprintf("ISOLATED_DIR=%s", buildCtx.IsolatedDir),
		fmt.Sprintf("RUNTIME_NAME=%s", buildCtx.Spec.Name),
		fmt.Sprintf("RUNTIME_VERSION=%s", buildCtx.Spec.Version),
		fmt.Sprintf("PLATFORM=%s", buildCtx.Platform.Distro),
		fmt.Sprintf("ARCHITECTURE=%s", buildCtx.Platform.Arch),
		fmt.Sprintf("PKG_MANAGER=%s", buildCtx.Platform.PkgManager),
	)

	// Capture output
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		logger.Debug("Hook output:\n%s", string(output))
	}

	if err != nil {
		// Check if it was a timeout
		if hookCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%s hook timed out after %s", hookName, timeout)
		}
		return fmt.Errorf("%s hook failed: %w\nOutput: %s", hookName, err, string(output))
	}

	logger.Info("%s hook completed successfully", hookName)
	return nil
}

// ExecutePreInstallHook executes the pre-install hook
func ExecutePreInstallHook(ctx context.Context, buildCtx *BuildContext, logger BuildLogger) error {
	if buildCtx.Spec.Hooks == nil || buildCtx.Spec.Hooks.PreInstall == "" {
		return nil
	}
	return ExecuteHook(ctx, "pre_install", buildCtx.Spec.Hooks.PreInstall, buildCtx, logger)
}

// ExecutePostInstallHook executes the post-install hook
func ExecutePostInstallHook(ctx context.Context, buildCtx *BuildContext, logger BuildLogger) error {
	if buildCtx.Spec.Hooks == nil || buildCtx.Spec.Hooks.PostInstall == "" {
		return nil
	}
	return ExecuteHook(ctx, "post_install", buildCtx.Spec.Hooks.PostInstall, buildCtx, logger)
}
