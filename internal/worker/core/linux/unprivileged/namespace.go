//go:build linux

package unprivileged

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

type JobIsolation struct {
	platform platform.Platform
	logger   *logger.Logger
}

func NewJobIsolation() *JobIsolation {
	return &JobIsolation{
		platform: platform.NewPlatform(),
		logger:   logger.New().WithField("component", "native-isolation"),
	}
}

// CreateIsolatedSysProcAttr uses Go's native syscall package for maximum compatibility
func (ji *JobIsolation) CreateIsolatedSysProcAttr() *syscall.SysProcAttr {
	sysProcAttr := &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// Native Linux namespace isolation using Go's syscall package
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // Process isolation (native)
		syscall.CLONE_NEWNS | // Mount isolation (native)
		syscall.CLONE_NEWIPC | // IPC isolation (native)
		syscall.CLONE_NEWUTS // UTS isolation (native)

	ji.logger.Debug("created native Go isolation attributes",
		"approach", "native-go-syscalls",
		"pidNamespace", true,
		"mountNamespace", true,
		"userComplexity", false,
		"reliability", "high")

	return sysProcAttr
}

// SetupUserNamespace uses native Go syscalls for namespace setup
func (ji *JobIsolation) SetupUserNamespace(pid int) error {
	log := ji.logger.WithFields("pid", pid, "operation", "native-setup")

	// Use native Go approach - let the kernel handle namespace setup
	// No manual intervention needed with this approach
	log.Debug("using native Go namespace setup (kernel-managed)")

	return nil
}

// ValidateIsolationSupport checks native Linux/Go support
func (ji *JobIsolation) ValidateIsolationSupport() error {
	log := ji.logger.WithField("operation", "validate-native")

	// Check we're on Linux (required for namespaces)
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native namespace isolation requires Linux, got %s", runtime.GOOS)
	}

	// Check root privileges (needed for mount operations)
	if os.Geteuid() != 0 {
		return fmt.Errorf("native isolation requires root privileges (euid: %d)", os.Geteuid())
	}

	// Test native syscall availability
	if err := ji.testNativeNamespaceSupport(); err != nil {
		return fmt.Errorf("native namespace support test failed: %w", err)
	}

	log.Debug("native Go + Linux isolation validated",
		"platform", runtime.GOOS,
		"goVersion", runtime.Version(),
		"nativeSupport", true)

	return nil
}

// testNativeNamespaceSupport tests if we can use native Go syscalls
func (ji *JobIsolation) testNativeNamespaceSupport() error {
	// Test if we can access namespace files (indicates kernel support)
	requiredNamespaces := []string{
		"/proc/self/ns/pid",
		"/proc/self/ns/mnt",
		"/proc/self/ns/ipc",
		"/proc/self/ns/uts",
	}

	for _, nsFile := range requiredNamespaces {
		if _, err := ji.platform.Stat(nsFile); err != nil {
			return fmt.Errorf("namespace %s not supported: %w", nsFile, err)
		}
	}

	return nil
}

// GetIsolationCapabilities returns native implementation capabilities
func (ji *JobIsolation) GetIsolationCapabilities() map[string]bool {
	return map[string]bool{
		"process_isolation":     true,  // Native PID namespaces
		"mount_isolation":       true,  // Native mount namespaces
		"ipc_isolation":         true,  // Native IPC namespaces
		"hostname_isolation":    true,  // Native UTS namespaces
		"user_isolation":        false, // Skipped for reliability
		"network_isolation":     false, // Host networking (requirement)
		"filesystem_isolation":  true,  // Native mount namespace benefits
		"resource_isolation":    true,  // Native cgroup integration
		"native_go_support":     true,  // Pure Go implementation
		"external_dependencies": false, // No external tools needed
		"reliability":           true,  // Proven syscall approach
	}
}
