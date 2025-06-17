//go:build linux

package jobinit

import (
	"fmt"
	"job-worker/pkg/logger"
	"job-worker/pkg/os"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type linuxJobInitializer struct {
	osInterface      os.OsInterface
	syscallInterface os.SyscallInterface
	execInterface    os.ExecInterface
	logger           *logger.Logger
}

// NewJobInitializer creates a Linux-specific job initializer
func NewJobInitializer() JobInitializer {
	return &linuxJobInitializer{
		osInterface:      &os.DefaultOs{},
		syscallInterface: &os.DefaultSyscall{},
		execInterface:    &os.DefaultExec{},
		logger:           logger.New(),
	}
}

// Ensure linuxJobInitializer implements JobInitializer
var _ JobInitializer = (*linuxJobInitializer)(nil)

// LoadConfigFromEnv loads job configuration from environment variables
func (j *linuxJobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := j.osInterface.Getenv("JOB_ID")
	command := j.osInterface.Getenv("JOB_COMMAND")
	cgroupPath := j.osInterface.Getenv("JOB_CGROUP_PATH")
	argsCountStr := j.osInterface.Getenv("JOB_ARGS_COUNT")

	// Network group variables (parent process handles namespace joining)
	networkGroupID := j.osInterface.Getenv("NETWORK_GROUP_ID")
	isNewNetworkGroupStr := j.osInterface.Getenv("IS_NEW_NETWORK_GROUP")

	jobLogger := j.logger.WithField("jobId", jobID)

	if jobID == "" || command == "" || cgroupPath == "" {
		jobLogger.Error("missing required environment variables",
			"jobId", jobID,
			"command", command,
			"cgroupPath", cgroupPath)

		return nil, fmt.Errorf("missing required environment variables (JOB_ID=%s, JOB_COMMAND=%s, JOB_CGROUP_PATH=%s)",
			jobID, command, cgroupPath)
	}

	var args []string

	// Parse arguments from individual environment variables
	if argsCountStr != "" {
		argsCount, err := strconv.Atoi(argsCountStr)
		if err != nil {
			jobLogger.Error("invalid JOB_ARGS_COUNT", "argsCount", argsCountStr, "error", err)
			return nil, fmt.Errorf("invalid JOB_ARGS_COUNT: %v", err)
		}

		args = make([]string, argsCount)
		for i := 0; i < argsCount; i++ {
			argKey := fmt.Sprintf("JOB_ARG_%d", i)
			args[i] = j.osInterface.Getenv(argKey)
		}

		jobLogger.Debug("loaded job arguments", "argsCount", argsCount, "args", args)
	}

	// Parse network group settings
	var isNewNetworkGroup bool
	if isNewNetworkGroupStr != "" {
		isNewNetworkGroup = strings.ToLower(isNewNetworkGroupStr) == "true"
	}

	jobLogger.Debug("loaded job configuration",
		"command", command,
		"cgroupPath", cgroupPath,
		"argsCount", len(args),
		"networkGroupID", networkGroupID,
		"isNewNetworkGroup", isNewNetworkGroup)

	return &JobConfig{
		JobID:             jobID,
		Command:           command,
		Args:              args,
		CgroupPath:        cgroupPath,
		NetworkGroupID:    networkGroupID,
		IsNewNetworkGroup: isNewNetworkGroup,
	}, nil
}

// ExecuteJob sets up cgroup, network namespace, and executes the job command
func (j *linuxJobInitializer) ExecuteJob(config *JobConfig) error {
	jobLogger := j.logger.WithField("jobId", config.JobID)

	// Validate config
	if config == nil {
		j.logger.Error("job config cannot be nil")
		return fmt.Errorf("job config cannot be nil")
	}

	if config.JobID == "" || config.Command == "" || config.CgroupPath == "" {
		jobLogger.Error("invalid job config", "command", config.Command, "cgroupPath", config.CgroupPath)
		return fmt.Errorf("invalid job config: jobID=%s, command=%s, cgroupPath=%s",
			config.JobID, config.Command, config.CgroupPath)
	}

	jobLogger.Info("executing job with proper namespace isolation",
		"jobID", config.JobID,
		"command", config.Command,
		"networkGroup", config.NetworkGroupID)

	// CRITICAL: Clean up any leaked interfaces first
	if err := j.cleanupLeakedInterfaces(); err != nil {
		jobLogger.Warn("failed to cleanup leaked interfaces", "error", err)
	}

	// Handle network setup if needed (parent already handled namespace joining)
	if err := j.setupNetworking(config); err != nil {
		jobLogger.Error("failed to setup networking", "error", err)
		return fmt.Errorf("failed to setup networking: %w", err)
	}

	// Add ourselves to the cgroup BEFORE executing the real command
	if err := j.JoinCgroup(config.CgroupPath); err != nil {
		jobLogger.Error("failed to join cgroup", "cgroupPath", config.CgroupPath, "error", err)
		return fmt.Errorf("failed to join cgroup %s: %w", config.CgroupPath, err)
	}

	// Resolve command path if it's not absolute
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		jobLogger.Error("command resolution failed", "command", config.Command, "error", err)
		return fmt.Errorf("command not found: %w", err)
	}

	if commandPath != config.Command {
		jobLogger.Debug("resolved command path", "original", config.Command, "resolved", commandPath)
	}

	// Prepare arguments for executing - first arg should be the command name
	execArgs := append([]string{config.Command}, config.Args...)
	jobLogger.Debug("executing command",
		"commandPath", commandPath,
		"execArgs", execArgs,
		"envCount", len(j.osInterface.Environ()))

	// Now exec the real command - this replaces our process
	// The exec process will inherit our cgroup membership and network namespace
	if e := j.syscallInterface.Exec(commandPath, execArgs, j.osInterface.Environ()); e != nil {
		jobLogger.Error("exec failed", "commandPath", commandPath, "command", config.Command, "error", e)
		return fmt.Errorf("failed to exec command %s (resolved to %s): %w",
			config.Command, commandPath, e)
	}

	// This should never be reached since exec replaces the process
	return nil
}

// cleanupLeakedInterfaces removes any interfaces that shouldn't be in this namespace
func (j *linuxJobInitializer) cleanupLeakedInterfaces() error {
	j.logger.Debug("cleaning up any leaked interfaces before setup")

	// Get current interfaces
	cmd := exec.Command("ip", "link", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		j.logger.Warn("failed to list interfaces", "error", err)
		return nil // Don't fail job for this
	}

	lines := strings.Split(string(output), "\n")
	myJobID := j.osInterface.Getenv("JOB_ID")

	for _, line := range lines {
		// Look for interfaces that match job pattern but aren't ours
		if strings.Contains(line, "job-") && !strings.Contains(line, "job-"+myJobID) {
			// Extract interface name
			if parts := strings.Fields(line); len(parts) >= 2 {
				interfaceName := strings.TrimSuffix(parts[1], ":")
				if strings.HasPrefix(interfaceName, "job-") && interfaceName != "job-"+myJobID {
					j.logger.Warn("found leaked interface, removing", "interface", interfaceName)
					j.removeInterface(interfaceName)
				}
			}
		}
	}

	return nil
}

// removeInterface safely removes a network interface
func (j *linuxJobInitializer) removeInterface(interfaceName string) {
	// Try to bring interface down first
	if err := j.executeCommand("ip", "link", "set", interfaceName, "down"); err != nil {
		j.logger.Debug("failed to bring interface down", "interface", interfaceName, "error", err)
	}

	// Remove the interface
	if err := j.executeCommand("ip", "link", "delete", interfaceName); err != nil {
		j.logger.Debug("failed to delete interface", "interface", interfaceName, "error", err)
	} else {
		j.logger.Info("removed leaked interface", "interface", interfaceName)
	}
}

// setupNetworking handles both automatic IP setup and basic network configuration
func (j *linuxJobInitializer) setupNetworking(config *JobConfig) error {
	// Check for automatic IP setup first
	if j.osInterface.Getenv("AUTO_SETUP_IP") == "true" {
		if err := j.setupAutomaticIP(); err != nil {
			j.logger.Error("failed to setup automatic IP", "error", err)
			return fmt.Errorf("failed to setup automatic IP: %w", err)
		}
		return nil
	}

	// For isolated jobs or manual setup, do nothing
	j.logger.Debug("no automatic IP setup requested")
	return nil
}

// setupAutomaticIP configures the network interface with server-assigned IP
func (j *linuxJobInitializer) setupAutomaticIP() error {
	assignedIP := j.osInterface.Getenv("JOB_ASSIGNED_IP")
	jobID := j.osInterface.Getenv("JOB_ID")
	subnet := j.osInterface.Getenv("INTERNAL_SUBNET")
	gateway := j.osInterface.Getenv("INTERNAL_GATEWAY")

	if assignedIP == "" || jobID == "" {
		return fmt.Errorf("missing required IP configuration: IP=%s, JobID=%s", assignedIP, jobID)
	}

	// Create unique interface name for THIS job only
	interfaceName := fmt.Sprintf("job-%s", jobID)

	j.logger.Info("setting up automatic IP with proper isolation",
		"jobID", jobID,
		"assignedIP", assignedIP,
		"interface", interfaceName)

	// Calculate interface IP with subnet mask
	interfaceIP := assignedIP + "/24" // Default
	if subnet != "" {
		if _, ipNet, err := net.ParseCIDR(subnet); err == nil {
			maskSize, _ := ipNet.Mask.Size()
			interfaceIP = fmt.Sprintf("%s/%d", assignedIP, maskSize)
		}
	}

	// STEP 1: Verify we start with ONLY loopback
	j.logCurrentInterfaces("BEFORE_setup")
	if err := j.verifyCleanNamespace(); err != nil {
		j.logger.Warn("namespace not clean", "error", err)
		// Don't fail - just warn and continue
	}

	// STEP 2: Setup loopback (should already exist but ensure it's up)
	if err := j.executeCommand("ip", "link", "set", "lo", "up"); err != nil {
		j.logger.Warn("failed to bring up loopback", "error", err)
	}

	// STEP 3: Create ONLY our job's interface
	// Use dummy interface instead of veth to avoid peer leakage
	if err := j.executeCommand("ip", "link", "add", interfaceName, "type", "dummy"); err != nil {
		j.logger.Error("failed to create job interface", "interface", interfaceName, "error", err)
		return fmt.Errorf("failed to create interface %s: %w", interfaceName, err)
	}

	// STEP 4: Assign IP to our interface
	if err := j.executeCommand("ip", "addr", "add", interfaceIP, "dev", interfaceName); err != nil {
		j.logger.Error("failed to assign IP", "interface", interfaceName, "ip", interfaceIP, "error", err)
		return fmt.Errorf("failed to assign IP %s to %s: %w", interfaceIP, interfaceName, err)
	}

	// STEP 5: Bring interface up
	if err := j.executeCommand("ip", "link", "set", interfaceName, "up"); err != nil {
		j.logger.Error("failed to bring interface up", "interface", interfaceName, "error", err)
		return fmt.Errorf("failed to bring up interface %s: %w", interfaceName, err)
	}

	// STEP 6: Add route to gateway if specified
	if gateway != "" {
		if err := j.executeCommand("ip", "route", "add", "default", "via", gateway, "dev", interfaceName); err != nil {
			j.logger.Warn("failed to add default route", "gateway", gateway, "error", err)
			// Don't fail for routing issues
		}
	}

	// STEP 7: Verify we now have exactly 2 interfaces
	j.logCurrentInterfaces("AFTER_setup")
	if err := j.verifyCorrectInterfaceCount(interfaceName, assignedIP); err != nil {
		return fmt.Errorf("interface verification failed: %w", err)
	}

	j.logger.Info("automatic IP setup completed successfully",
		"interface", interfaceName,
		"ip", assignedIP,
		"expectedInterfaces", "2 (lo + job interface)")

	return nil
}

// verifyCleanNamespace ensures we start with only loopback
func (j *linuxJobInitializer) verifyCleanNamespace() error {
	cmd := exec.Command("ip", "link", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	interfaceCount := 0
	hasLoopback := false
	extraInterfaces := []string{}

	for _, line := range lines {
		if strings.Contains(line, ": lo:") {
			hasLoopback = true
			interfaceCount++
		} else if strings.HasPrefix(line, " ") {
			// Skip continuation lines
			continue
		} else if strings.Contains(line, ": ") {
			interfaceCount++
			// Extract interface name for logging
			if parts := strings.Fields(line); len(parts) >= 2 {
				interfaceName := strings.TrimSuffix(parts[1], ":")
				if interfaceName != "lo" {
					extraInterfaces = append(extraInterfaces, interfaceName)
				}
			}
		}
	}

	if !hasLoopback {
		return fmt.Errorf("loopback interface not found")
	}

	if len(extraInterfaces) > 0 {
		j.logger.Warn("found extra interfaces in namespace",
			"extraInterfaces", extraInterfaces,
			"totalInterfaces", interfaceCount)
		// Clean them up
		for _, iface := range extraInterfaces {
			j.cleanupInterface(iface)
		}
	}

	j.logger.Debug("namespace verification",
		"hasLoopback", hasLoopback,
		"totalInterfaces", interfaceCount,
		"extraInterfaces", len(extraInterfaces))

	return nil
}

// verifyCorrectInterfaceCount ensures we have exactly 2 interfaces after setup
func (j *linuxJobInitializer) verifyCorrectInterfaceCount(expectedInterface, expectedIP string) error {
	cmd := exec.Command("ip", "addr", "show")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to verify interfaces: %w", err)
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	interfaceCount := 0
	hasLoopback := false
	hasJobInterface := false
	hasExpectedIP := false

	for _, line := range lines {
		// Count interfaces (lines starting with number)
		if strings.Contains(line, ": lo:") {
			hasLoopback = true
			interfaceCount++
		} else if strings.Contains(line, ": "+expectedInterface+":") {
			hasJobInterface = true
			interfaceCount++
		} else if strings.HasPrefix(line, " ") {
			// Skip continuation lines but check for IP
			if strings.Contains(line, expectedIP) {
				hasExpectedIP = true
			}
		} else if strings.Contains(line, ": ") && !strings.Contains(line, ": lo:") && !strings.Contains(line, ": "+expectedInterface+":") {
			interfaceCount++
			// Log unexpected interface
			if parts := strings.Fields(line); len(parts) >= 2 {
				unexpectedInterface := strings.TrimSuffix(parts[1], ":")
				j.logger.Warn("unexpected interface found", "interface", unexpectedInterface)
			}
		}
	}

	// Verify we have exactly what we expect
	if !hasLoopback {
		return fmt.Errorf("loopback interface missing")
	}

	if !hasJobInterface {
		return fmt.Errorf("job interface %s missing", expectedInterface)
	}

	if !hasExpectedIP {
		return fmt.Errorf("expected IP %s not found on interface", expectedIP)
	}

	if interfaceCount != 2 {
		return fmt.Errorf("expected exactly 2 interfaces (lo + %s), found %d", expectedInterface, interfaceCount)
	}

	j.logger.Info("interface verification passed",
		"hasLoopback", hasLoopback,
		"hasJobInterface", hasJobInterface,
		"hasExpectedIP", hasExpectedIP,
		"totalInterfaces", interfaceCount)

	return nil
}

// cleanupInterface removes an unexpected interface
func (j *linuxJobInitializer) cleanupInterface(interfaceName string) {
	j.logger.Info("cleaning up unexpected interface", "interface", interfaceName)

	// Try to bring interface down first
	if err := j.executeCommand("ip", "link", "set", interfaceName, "down"); err != nil {
		j.logger.Debug("failed to bring interface down", "interface", interfaceName, "error", err)
	}

	// Remove the interface
	if err := j.executeCommand("ip", "link", "delete", interfaceName); err != nil {
		j.logger.Debug("failed to delete interface", "interface", interfaceName, "error", err)
	} else {
		j.logger.Info("successfully removed unexpected interface", "interface", interfaceName)
	}
}

// logCurrentInterfaces - enhanced version for debugging
func (j *linuxJobInitializer) logCurrentInterfaces(stage string) {
	cmd := exec.Command("ip", "addr", "show")
	if output, err := cmd.CombinedOutput(); err == nil {
		lines := strings.Split(string(output), "\n")
		interfaceNames := []string{}

		for _, line := range lines {
			if strings.Contains(line, ": ") && !strings.HasPrefix(line, " ") {
				if parts := strings.Fields(line); len(parts) >= 2 {
					interfaceName := strings.TrimSuffix(parts[1], ":")
					interfaceNames = append(interfaceNames, interfaceName)
				}
			}
		}

		j.logger.Info("current network interfaces",
			"stage", stage,
			"interfaces", interfaceNames,
			"count", len(interfaceNames))

		// Also log full output for debugging
		j.logger.Debug("full interface output", "stage", stage, "output", string(output))
	}
}

// executeCommand runs a system command
func (j *linuxJobInitializer) executeCommand(name string, args ...string) error {
	j.logger.Debug("executing command", "command", name, "args", args)

	// Find the command path
	commandPath, err := j.execInterface.LookPath(name)
	if err != nil {
		return fmt.Errorf("command %s not found: %w", name, err)
	}

	// Create the command
	cmd := exec.Command(commandPath, args...)
	cmd.Env = j.osInterface.Environ()

	// Run the command and wait for completion
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s failed: %w", name, err)
	}

	j.logger.Debug("command executed successfully", "command", name)
	return nil
}

// Run is the main entry point
func (j *linuxJobInitializer) Run() error {
	j.logger.Info("job-init starting on Linux, parent handles namespaces")

	if err := j.setupNamespaceEnvironment(); err != nil {
		j.logger.Error("failed to setup namespace environment", "error", err)
		return err
	}

	if err := j.ValidateEnvironment(); err != nil {
		j.logger.Error("environment validation failed", "error", err)
		return fmt.Errorf("environment validation failed: %w", err)
	}

	config, err := j.LoadConfigFromEnv()
	if err != nil {
		j.logger.Error("failed to load config", "error", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	if e := j.ExecuteJob(config); e != nil {
		j.logger.Error("failed to execute job", "error", e)
		return fmt.Errorf("failed to execute job: %w", e)
	}

	return nil
}

// ValidateEnvironment checks if all required environment variables are set
func (j *linuxJobInitializer) ValidateEnvironment() error {
	requiredVars := []string{"JOB_ID", "JOB_COMMAND", "JOB_CGROUP_PATH"}

	for _, varName := range requiredVars {
		if j.osInterface.Getenv(varName) == "" {
			j.logger.Error("required environment variable not set", "variable", varName)
			return fmt.Errorf("required environment variable %s is not set", varName)
		}
	}

	j.logger.Debug("environment validation passed", "requiredVars", requiredVars)
	return nil
}

func (j *linuxJobInitializer) setupNamespaceEnvironment() error {
	pid := j.osInterface.Getpid()
	j.logger.Debug("setting up namespace environment", "pid", pid)

	// Check if we're in a PID namespace (PID 1 indicates namespace)
	if pid == 1 {
		j.logger.Info("detected PID 1 - we're the init process in a PID namespace, remounting /proc")

		// Make all mounts private to prevent propagation to parent
		if err := j.syscallInterface.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
			j.logger.Warn("failed to make mounts private", "error", err)
		}

		if err := j.remountProc(); err != nil {
			return fmt.Errorf("failed to remount /proc: %w", err)
		}

		j.logger.Info("namespace environment setup completed successfully")
	} else {
		j.logger.Debug("PID not indicating namespace isolation, skipping /proc remount", "pid", pid)
	}

	return nil
}

func (j *linuxJobInitializer) remountProc() error {
	j.logger.Debug("attempting to remount /proc for namespace isolation")

	// direct remount
	if err := j.syscallInterface.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		j.logger.Debug("direct /proc mount failed, trying unmount first", "error", err)

		// unmounting first, then remounting
		if unmountErr := j.syscallInterface.Unmount("/proc", syscall.MNT_DETACH); unmountErr != nil {
			j.logger.Debug("/proc unmount failed (this might be normal)", "error", unmountErr)
		}

		// mounting again after unmount
		if err := j.syscallInterface.Mount("proc", "/proc", "proc", 0, ""); err != nil {
			return fmt.Errorf("failed to remount /proc after unmount: %w", err)
		}
	}

	j.logger.Info("/proc successfully remounted for namespace isolation")
	return nil
}

func (j *linuxJobInitializer) JoinCgroup(cgroupPath string) error {
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	pid := j.osInterface.Getpid()
	pidBytes := []byte(strconv.Itoa(pid))

	log := j.logger.WithFields("pid", pid, "cgroupPath", cgroupPath)
	log.Debug("joining cgroup", "procsFile", procsFile)

	// Retry logic in case cgroup is still being set up
	var lastErr error
	for i := 0; i < 10; i++ {
		if err := j.osInterface.WriteFile(procsFile, pidBytes, 0644); err != nil {
			lastErr = err

			// Exponential backoff: 1ms, 2ms, 4ms, 8ms, etc.
			backoff := time.Duration(1<<uint(i)) * time.Millisecond
			log.Debug("cgroup join attempt failed, retrying",
				"attempt", i+1,
				"error", err,
				"backoff", backoff)
			time.Sleep(backoff)
			continue
		}

		log.Info("successfully joined cgroup", "attempts", i+1)
		return nil
	}

	log.Error("failed to join cgroup after retries", "attempts", 10, "lastError", lastErr)
	return fmt.Errorf("failed to join cgroup after 10 retries: %w", lastErr)
}

func (j *linuxJobInitializer) resolveCommandPath(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	if filepath.IsAbs(command) {
		if _, err := j.osInterface.Stat(command); err != nil {
			j.logger.Error("absolute command path not found", "command", command, "error", err)
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		j.logger.Debug("using absolute command path", "command", command)
		return command, nil
	}

	if resolvedPath, err := j.execInterface.LookPath(command); err == nil {
		j.logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	j.logger.Debug("checking common command locations", "command", command, "paths", commonPaths)

	for _, path := range commonPaths {
		if _, err := j.osInterface.Stat(path); err == nil {
			j.logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	j.logger.Error("command not found anywhere",
		"command", command,
		"searchedPaths", commonPaths,
		"searchedPATH", true)

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}
