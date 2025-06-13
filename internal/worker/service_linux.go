//go:build linux

package worker

import (
	"context"
	"errors"
	"fmt"
	"job-worker/internal/config"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/executor"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/resource"
	"job-worker/pkg/logger"
	"job-worker/pkg/os"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	netns          = "/tmp/shared-netns/"
	varRunNetns    = "/var/run/netns/"
	internalSubnet = "172.20.0.0/24" // Default network range for internal communication
	internalGW     = "172.20.0.1"    // Default gateway IP for internal network
	internalIface  = "internal0"     // Default interface name for internal communication
	baseNetwork    = "172.20.0.0/16" // Base network for dynamic allocation
	SYS_SETNS      = 308             // x86_64 syscall number for setns
)

var jobCounter int64

// NetworkConfig holds network configuration for a network group
type NetworkConfig struct {
	Subnet    string `json:"subnet"`    // e.g., "172.20.0.0/24"
	Gateway   string `json:"gateway"`   // e.g., "172.20.0.1"
	Interface string `json:"interface"` // e.g., "internal0"
}

// NetworkGroup represents a network isolation group with configuration
type NetworkGroup struct {
	GroupID       string         `json:"group_id"`
	JobCount      int32          `json:"job_count"`
	NamePath      string         `json:"name_path"`
	CreatedAt     time.Time      `json:"created_at"`
	NetworkConfig *NetworkConfig `json:"network_config"`
}

// SubnetAllocator manages dynamic subnet allocation to avoid conflicts
type SubnetAllocator struct {
	baseNetwork      string
	allocatedSubnets map[string]string // subnet -> groupID mapping
	mutex            sync.RWMutex
}

// NewSubnetAllocator creates a new subnet allocator
func NewSubnetAllocator() *SubnetAllocator {
	return &SubnetAllocator{
		baseNetwork:      baseNetwork,
		allocatedSubnets: make(map[string]string),
	}
}

// AllocateSubnet allocates a unique subnet for a network group
func (sa *SubnetAllocator) AllocateSubnet(groupID string) (*NetworkConfig, error) {
	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	// Check if group already has an allocated subnet
	for subnet, existingGroupID := range sa.allocatedSubnets {
		if existingGroupID == groupID {
			// Extract gateway from subnet
			_, ipNet, err := net.ParseCIDR(subnet)
			if err != nil {
				continue
			}
			gateway := sa.calculateGateway(ipNet)

			return &NetworkConfig{
				Subnet:    subnet,
				Gateway:   gateway,
				Interface: internalIface,
			}, nil
		}
	}

	// Simple allocation strategy: increment the third octet
	// 172.20.0.0/24, 172.20.1.0/24, 172.20.2.0/24, etc.
	for i := 0; i < 256; i++ {
		subnet := fmt.Sprintf("172.20.%d.0/24", i)
		gateway := fmt.Sprintf("172.20.%d.1", i)

		if _, exists := sa.allocatedSubnets[subnet]; !exists {
			sa.allocatedSubnets[subnet] = groupID

			return &NetworkConfig{
				Subnet:    subnet,
				Gateway:   gateway,
				Interface: internalIface,
			}, nil
		}
	}

	return nil, fmt.Errorf("no available subnets in range %s", sa.baseNetwork)
}

// ReleaseSubnet releases a subnet allocation
func (sa *SubnetAllocator) ReleaseSubnet(groupID string) {
	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	for subnet, existingGroupID := range sa.allocatedSubnets {
		if existingGroupID == groupID {
			delete(sa.allocatedSubnets, subnet)
			break
		}
	}
}

// calculateGateway calculates the gateway IP (.1) for a given network
func (sa *SubnetAllocator) calculateGateway(ipNet *net.IPNet) string {
	ip := ipNet.IP.Mask(ipNet.Mask)
	ip[len(ip)-1] = 1 // Set last octet to 1
	return ip.String()
}

type linuxWorker struct {
	store           interfaces.Store
	cgroup          interfaces.Resource
	cmdFactory      os.CommandFactory
	syscall         os.SyscallInterface
	os              os.OsInterface
	logger          *logger.Logger
	networkGroups   map[string]*NetworkGroup
	groupMutex      sync.RWMutex
	subnetAllocator *SubnetAllocator
}

// NewPlatformWorker creates a Linux-specific worker implementation with configurable networking
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	return &linuxWorker{
		store:           store,
		cgroup:          resource.New(),
		cmdFactory:      &os.DefaultCommandFactory{},
		syscall:         &os.DefaultSyscall{},
		os:              &os.DefaultOs{},
		logger:          logger.WithField("component", "worker-linux"),
		networkGroups:   make(map[string]*NetworkGroup),
		subnetAllocator: NewSubnetAllocator(),
	}
}

// Ensure linuxWorker implements both interfaces
var _ interfaces.JobWorker = (*linuxWorker)(nil)
var _ PlatformWorker = (*linuxWorker)(nil)

func (w *linuxWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, networkGroupID string) (*domain.Job, error) {
	select {
	case <-ctx.Done():
		w.logger.Warn("start job cancelled by context", "error", ctx.Err())
		return nil, ctx.Err()
	default:
	}

	jobId := strconv.FormatInt(atomic.AddInt64(&jobCounter, 1), 10)
	jobLogger := w.logger.WithField("jobId", jobId)

	jobLogger.Info("starting new job", "command", command, "args", args, "requestedCPU", maxCPU, "requestedMemory", maxMemory, "requestedIOBPS", maxIOBPS, "networkGroup", networkGroupID)

	// Apply defaults for resource limits
	originalCPU, originalMemory, originalIOBPS := maxCPU, maxMemory, maxIOBPS
	if maxCPU <= 0 {
		maxCPU = config.DefaultCPULimitPercent
	}
	if maxMemory <= 0 {
		maxMemory = config.DefaultMemoryLimitMB
	}
	if maxIOBPS <= 0 {
		maxIOBPS = config.DefaultIOBPS
	}

	if originalCPU != maxCPU || originalMemory != maxMemory || originalIOBPS != maxIOBPS {
		jobLogger.Debug("applied resource defaults", "finalCPU", maxCPU, "finalMemory", maxMemory, "finalIOBPS", maxIOBPS)
	}

	cgroupJobDir := filepath.Join(config.CgroupsBaseDir, "job-"+jobId)

	jobLogger.Debug("creating cgroup", "cgroupPath", cgroupJobDir)

	// Create cgroup with limits
	if err := w.cgroup.Create(cgroupJobDir, maxCPU, maxMemory, maxIOBPS); err != nil {
		jobLogger.Error("failed to create cgroup", "cgroupPath", cgroupJobDir, "error", err)
		return nil, fmt.Errorf("failed to create cgroup for job %s: %w", jobId, err)
	}

	jobLogger.Debug("cgroup created successfully", "cgroupPath", cgroupJobDir, "cpuLimit", maxCPU, "memoryLimit", maxMemory, "ioLimit", maxIOBPS)

	// Handle network namespace setup
	var sysProcAttr *syscall.SysProcAttr
	var nsPath string
	var isNewNetworkGroup bool
	var needsNamespaceJoin bool

	if networkGroupID != "" {
		// Join existing network group or create new one
		var err error
		nsPath, sysProcAttr, isNewNetworkGroup, err = w.handleNetworkGroup(networkGroupID, jobId)
		if err != nil {
			jobLogger.Error("failed to handle network group", "networkGroupID", networkGroupID, "error", err)
			w.cgroup.CleanupCgroup(jobId)
			return nil, fmt.Errorf("failed to handle network group: %w", err)
		}
		needsNamespaceJoin = !isNewNetworkGroup
		jobLogger.Info("network group handling completed", "groupID", networkGroupID, "isNew", isNewNetworkGroup, "needsJoin", needsNamespaceJoin)
	} else {
		// Create isolated namespace (existing behavior)
		sysProcAttr = w.createSysProcAttrWithNamespaces()
		nsPath = filepath.Join(netns, jobId)
		isNewNetworkGroup = true
		needsNamespaceJoin = false
	}

	// Prevent "command not found" errors by resolving command path
	resolvedCommand := command
	if !filepath.IsAbs(command) {
		jobLogger.Debug("resolving command path", "originalCommand", command)

		if path, err := exec.LookPath(command); err == nil {
			resolvedCommand = path
			jobLogger.Debug("command resolved", "originalCommand", command, "resolvedPath", resolvedCommand)
		} else {
			jobLogger.Error("command not found in PATH", "command", command, "error", err)
			w.cgroup.CleanupCgroup(jobId)
			if networkGroupID != "" && isNewNetworkGroup {
				w.cleanupNetworkGroup(networkGroupID)
			}
			return nil, fmt.Errorf("command not found in PATH: %s", command)
		}
	}

	// Create job domain object
	limits := domain.ResourceLimits{
		MaxCPU:    maxCPU,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
	}

	job := &domain.Job{
		Id:             jobId,
		Command:        resolvedCommand,
		Args:           append([]string(nil), args...),
		Limits:         limits,
		Status:         domain.StatusInitializing,
		CgroupPath:     cgroupJobDir,
		StartTime:      time.Now(),
		NetworkGroupID: networkGroupID,
	}
	w.store.CreateNewJob(job)

	jobLogger.Debug("job created in store", "status", string(job.Status), "resolvedCommand", resolvedCommand, "argsCount", len(job.Args))

	// Get path to the job-init binary
	initPath, err := w.getJobInitPath()
	if err != nil {
		jobLogger.Error("failed to get job-init path", "error", err)
		w.cgroup.CleanupCgroup(job.Id)
		if networkGroupID != "" && isNewNetworkGroup {
			w.cleanupNetworkGroup(networkGroupID)
		}
		return nil, fmt.Errorf("failed to get job-init path: %w", err)
	}

	jobLogger.Debug("job-init binary located", "initPath", initPath)

	// Prepare environment for init process with network configuration
	env := w.prepareJobEnvironment(job, cgroupJobDir, networkGroupID, isNewNetworkGroup)

	jobLogger.Debug("environment prepared for init process", "totalEnvVars", len(env))

	// Prepare command
	cmd := w.cmdFactory.CreateCommand(initPath)
	cmd.SetEnv(env)
	cmd.SetStdout(executor.New(w.store, job.Id))
	cmd.SetStderr(executor.New(w.store, job.Id))
	cmd.SetSysProcAttr(sysProcAttr)

	jobLogger.Info("starting init process with namespace isolation",
		"initPath", initPath,
		"targetCommand", job.Command,
		"targetArgs", job.Args,
		"namespaces", "pid,mount,ipc,uts,net",
		"networkGroup", networkGroupID,
		"needsNamespaceJoin", needsNamespaceJoin)

	// Use pre-fork namespace setup approach
	type startResult struct {
		err error
		pid int
	}

	startChan := make(chan startResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				startChan <- startResult{err: fmt.Errorf("panic in start goroutine: %v", r)}
			}
		}()

		// Lock this goroutine to the OS thread
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Join network namespace if needed (before forking)
		if needsNamespaceJoin {
			jobLogger.Debug("joining network namespace before fork", "nsPath", nsPath)
			if err := w.setNetworkNamespace(nsPath); err != nil {
				startChan <- startResult{err: fmt.Errorf("failed to join namespace: %w", err)}
				return
			}
			jobLogger.Debug("successfully joined network namespace")
		}

		// Start the process (which will inherit the current namespace)
		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			startChan <- startResult{err: fmt.Errorf("failed to start command: %w", err)}
			return
		}

		process := cmd.Process()
		if process == nil {
			startChan <- startResult{err: fmt.Errorf("process is nil after start")}
			return
		}

		duration := time.Since(startTime)
		jobLogger.Debug("process started in goroutine", "pid", process.Pid(), "duration", duration)
		startChan <- startResult{pid: process.Pid()}
	}()

	// Wait for the goroutine to complete with timeout
	var actualPid int32
	select {
	case result := <-startChan:
		if result.err != nil {
			jobLogger.Error("failed to start process in goroutine", "error", result.err)

			// Cleanup on failure
			w.cgroup.CleanupCgroup(job.Id)
			if networkGroupID != "" && isNewNetworkGroup {
				w.cleanupNetworkGroup(networkGroupID)
			}

			failedJob := job.DeepCopy()
			if failErr := failedJob.Fail(-1); failErr != nil {
				jobLogger.Warn("domain validation failed for job failure", "domainError", failErr)
				failedJob.Status = domain.StatusFailed
				failedJob.ExitCode = -1
				now := time.Now()
				failedJob.EndTime = &now
			}
			w.store.UpdateJob(failedJob)

			return nil, fmt.Errorf("failed to start process: %w", result.err)
		}

		actualPid = int32(result.pid)
		job.Pid = actualPid

	case <-ctx.Done():
		jobLogger.Warn("context cancelled while starting process")
		w.cgroup.CleanupCgroup(job.Id)
		if networkGroupID != "" && isNewNetworkGroup {
			w.cleanupNetworkGroup(networkGroupID)
		}
		return nil, ctx.Err()

	case <-time.After(10 * time.Second):
		jobLogger.Error("timeout waiting for process to start")
		w.cgroup.CleanupCgroup(job.Id)
		if networkGroupID != "" && isNewNetworkGroup {
			w.cleanupNetworkGroup(networkGroupID)
		}
		return nil, fmt.Errorf("timeout waiting for process to start")
	}

	jobLogger.Info("init process started successfully", "pid", actualPid)

	// Handle namespace symlink creation AFTER we have a valid PID
	if isNewNetworkGroup {
		// Only create symlink for new network groups or isolated jobs
		nsSource := fmt.Sprintf("/proc/%d/ns/net", actualPid)

		// Ensure directory exists
		if err := w.ensureNetnsDir(nsPath); err != nil {
			jobLogger.Error("failed to create netns directory", "error", err)
			w.cleanup(job.Id, int(actualPid))
			return nil, fmt.Errorf("failed to create netns directory: %w", err)
		}

		// For network groups, create a bind mount for persistence
		if networkGroupID != "" {
			// Create the namespace file first
			err := w.os.WriteFile(nsPath, []byte{}, 0644)
			if err != nil {
				jobLogger.Error("failed to create namespace file", "path", nsPath, "error", err)
				w.cleanup(job.Id, int(actualPid))
				return nil, fmt.Errorf("failed to create namespace file: %w", err)
			}

			// Bind mount the namespace
			err = w.syscall.Mount(nsSource, nsPath, "", syscall.MS_BIND, "")
			if err != nil {
				jobLogger.Error("failed to bind mount namespace", "source", nsSource, "target", nsPath, "error", err)
				w.cleanup(job.Id, int(actualPid))
				return nil, fmt.Errorf("failed to bind mount namespace: %w", err)
			}
			jobLogger.Debug("namespace bind mounted successfully", "source", nsSource, "target", nsPath)
		} else {
			// For isolated jobs, just create a symlink
			jobLogger.Debug("creating namespace symlink", "source", nsSource, "target", nsPath)
			err := w.os.Symlink(nsSource, nsPath)
			if err != nil {
				jobLogger.Error("failed to create namespace symlink", "source", nsSource, "target", nsPath, "error", err)
				w.cleanup(job.Id, int(actualPid))
				return nil, fmt.Errorf("failed to create namespace symlink: %w", err)
			}
			jobLogger.Debug("namespace symlink created successfully")
		}
	}

	// Update job counts for network groups
	if networkGroupID != "" {
		w.incrementGroupJobCount(networkGroupID)
	}

	// Mark job as running
	runningJob := job.DeepCopy()
	if e := runningJob.MarkAsRunning(actualPid); e != nil {
		jobLogger.Warn("domain validation failed for running status", "domainError", e)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = actualPid
	}

	runningJob.StartTime = time.Now()
	w.store.UpdateJob(runningJob)

	jobLogger.Info("job marked as running", "pid", runningJob.Pid, "status", string(runningJob.Status))

	// Start monitoring
	go w.waitForCompletion(ctx, cmd, runningJob)

	return runningJob, nil
}

// setNetworkNamespace joins an existing network namespace using setns syscall
func (w *linuxWorker) setNetworkNamespace(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	// Check if namespace file exists
	if _, err := w.os.Stat(nsPath); err != nil {
		return fmt.Errorf("namespace file does not exist: %s (%w)", nsPath, err)
	}

	w.logger.Debug("opening namespace file", "nsPath", nsPath)

	// Open the namespace file
	fd, err := syscall.Open(nsPath, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open namespace file %s: %w", nsPath, err)
	}
	defer syscall.Close(fd)

	w.logger.Debug("calling setns syscall", "fd", fd, "nsPath", nsPath)

	// Call setns system call
	_, _, errno := syscall.Syscall(SYS_SETNS, uintptr(fd), syscall.CLONE_NEWNET, 0)
	if errno != 0 {
		return fmt.Errorf("setns syscall failed for %s: %v", nsPath, errno)
	}

	w.logger.Debug("successfully joined network namespace", "nsPath", nsPath)
	return nil
}

// getNetworkConfigForGroup returns network configuration for a group
func (w *linuxWorker) getNetworkConfigForGroup(networkGroupID string) (*NetworkConfig, error) {
	// Option 1: Check for environment variable override
	envSubnetKey := fmt.Sprintf("NETWORK_GROUP_%s_SUBNET", strings.ToUpper(networkGroupID))
	envGatewayKey := fmt.Sprintf("NETWORK_GROUP_%s_GATEWAY", strings.ToUpper(networkGroupID))

	if customSubnet := w.os.Getenv(envSubnetKey); customSubnet != "" {
		w.logger.Debug("using custom subnet from environment", "groupID", networkGroupID, "subnet", customSubnet)

		gateway := w.os.Getenv(envGatewayKey)
		if gateway == "" {
			// Calculate default gateway (.1) for the subnet
			if _, ipNet, err := net.ParseCIDR(customSubnet); err == nil {
				ip := ipNet.IP.Mask(ipNet.Mask)
				ip[len(ip)-1] = 1 // Set last octet to 1
				gateway = ip.String()
			} else {
				return nil, fmt.Errorf("invalid custom subnet %s: %w", customSubnet, err)
			}
		}

		config := &NetworkConfig{
			Subnet:    customSubnet,
			Gateway:   gateway,
			Interface: internalIface,
		}

		if err := w.validateNetworkConfig(config); err != nil {
			return nil, fmt.Errorf("invalid custom network config: %w", err)
		}

		return config, nil
	}

	// Option 2: Dynamic allocation (prevents conflicts between groups)
	if w.subnetAllocator != nil {
		config, err := w.subnetAllocator.AllocateSubnet(networkGroupID)
		if err == nil {
			w.logger.Debug("allocated dynamic subnet", "groupID", networkGroupID, "subnet", config.Subnet, "gateway", config.Gateway)
			return config, nil
		}
		w.logger.Warn("failed to allocate dynamic subnet, using default", "groupID", networkGroupID, "error", err)
	}

	// Option 3: Default configuration
	config := &NetworkConfig{
		Subnet:    internalSubnet,
		Gateway:   internalGW,
		Interface: internalIface,
	}

	w.logger.Debug("using default network configuration", "groupID", networkGroupID, "subnet", config.Subnet, "gateway", config.Gateway)
	return config, nil
}

// validateNetworkConfig validates the network configuration
func (w *linuxWorker) validateNetworkConfig(config *NetworkConfig) error {
	if config == nil {
		return fmt.Errorf("network config cannot be nil")
	}

	// Validate subnet
	_, ipNet, err := net.ParseCIDR(config.Subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet %s: %w", config.Subnet, err)
	}

	// Validate gateway IP
	gatewayIP := net.ParseIP(config.Gateway)
	if gatewayIP == nil {
		return fmt.Errorf("invalid gateway IP %s", config.Gateway)
	}

	// Check if gateway is within subnet
	if !ipNet.Contains(gatewayIP) {
		return fmt.Errorf("gateway IP %s is not within subnet %s", config.Gateway, config.Subnet)
	}

	// Validate interface name
	if config.Interface == "" {
		return fmt.Errorf("interface name cannot be empty")
	}

	return nil
}

// prepareJobEnvironment prepares environment variables including network configuration
func (w *linuxWorker) prepareJobEnvironment(job *domain.Job, cgroupJobDir string, networkGroupID string, isNewNetworkGroup bool) []string {
	env := w.os.Environ()
	jobEnvVars := []string{
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", cgroupJobDir),
	}

	// Add network group environment variables for job-init
	if networkGroupID != "" {
		jobEnvVars = append(jobEnvVars, fmt.Sprintf("NETWORK_GROUP_ID=%s", networkGroupID))
		jobEnvVars = append(jobEnvVars, fmt.Sprintf("IS_NEW_NETWORK_GROUP=%t", isNewNetworkGroup))

		// Add network configuration if group exists
		w.groupMutex.RLock()
		if group, exists := w.networkGroups[networkGroupID]; exists && group.NetworkConfig != nil {
			jobEnvVars = append(jobEnvVars,
				fmt.Sprintf("INTERNAL_SUBNET=%s", group.NetworkConfig.Subnet),
				fmt.Sprintf("INTERNAL_GATEWAY=%s", group.NetworkConfig.Gateway),
				fmt.Sprintf("INTERNAL_INTERFACE=%s", group.NetworkConfig.Interface),
			)
			w.logger.Debug("added network config to environment",
				"groupID", networkGroupID,
				"subnet", group.NetworkConfig.Subnet,
				"gateway", group.NetworkConfig.Gateway,
				"interface", group.NetworkConfig.Interface)
		}
		w.groupMutex.RUnlock()
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}
	jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)))

	return append(env, jobEnvVars...)
}

func (w *linuxWorker) StopJob(ctx context.Context, jobId string) error {
	log := w.logger.WithField("jobId", jobId)

	select {
	case <-ctx.Done():
		log.Warn("stop job cancelled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	log.Info("stop job request received")

	job, exists := w.store.GetJob(jobId)
	if !exists {
		log.Warn("job not found for stop operation")
		return fmt.Errorf("job not found: %s", jobId)
	}

	// Clean up isolated job namespace symlink
	if job.NetworkGroupID == "" {
		nsPath := filepath.Join(netns, jobId)
		w.os.Remove(nsPath)
	}

	if !job.IsRunning() {
		log.Warn("attempted to stop non-running job", "currentStatus", string(job.Status))
		return fmt.Errorf("job is not running: %s (current status: %s)", jobId, job.Status)
	}

	log.Info("stopping running job", "pid", job.Pid, "status", string(job.Status))

	stopStartTime := time.Now()

	// Send SIGTERM to process group
	log.Debug("sending SIGTERM to process group", "processGroup", -int(job.Pid))
	if err := w.syscall.Kill(-int(job.Pid), syscall.SIGTERM); err != nil {
		log.Warn("failed to send SIGTERM to process group", "processGroup", job.Pid, "error", err)

		// If killing the group failed, try killing just the main process
		if errIn := w.syscall.Kill(int(job.Pid), syscall.SIGTERM); errIn != nil {
			log.Warn("failed to send SIGTERM to main process", "pid", job.Pid, "error", errIn)
		}
	}

	// Wait for graceful shutdown
	gracefulWaitTime := 100 * time.Millisecond
	log.Debug("waiting for graceful shutdown", "timeout", gracefulWaitTime)
	time.Sleep(gracefulWaitTime)

	// Check if process is still alive
	processAlive := w.processExists(int(job.Pid))
	gracefulDuration := time.Since(stopStartTime)

	if !processAlive {
		log.Info("process terminated gracefully", "pid", job.Pid, "duration", gracefulDuration)

		stoppedJob := job.DeepCopy()
		if stopErr := stoppedJob.Stop(); stopErr != nil {
			// If domain validation fails
			log.Warn("domain validation failed for graceful stop", "domainError", stopErr)
			stoppedJob.Status = domain.StatusStopped
			stoppedJob.ExitCode = 0
			now := time.Now()
			stoppedJob.EndTime = &now
		} else {
			stoppedJob.ExitCode = 0
		}
		w.store.UpdateJob(stoppedJob)

		if job.CgroupPath != "" {
			log.Debug("cleaning up cgroup after graceful stop", "cgroupPath", job.CgroupPath)
			w.cgroup.CleanupCgroup(job.Id)
		}

		// Handle network group cleanup
		if job.NetworkGroupID != "" {
			w.decrementGroupJobCount(job.NetworkGroupID)
		}

		return nil
	}

	// Send SIGKILL to process group (force kill)
	log.Warn("process still alive after graceful shutdown, force killing", "pid", job.Pid, "gracefulDuration", gracefulDuration)

	if err := w.syscall.Kill(-int(job.Pid), syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to process group", "processGroup", job.Pid, "error", err)

		// Then try killing just the main process
		if errIn := w.syscall.Kill(int(job.Pid), syscall.SIGKILL); errIn != nil {
			log.Error("failed to kill process", "pid", job.Pid, "error", errIn)
			return fmt.Errorf("failed to kill process: %v", errIn)
		}
	}

	stoppedJob := job.DeepCopy()
	if stopErr := stoppedJob.Stop(); stopErr != nil {
		// If domain validation fails
		log.Warn("domain validation failed for forced stop", "domainError", stopErr)
		stoppedJob.Status = domain.StatusStopped
		// Forced kill
		stoppedJob.ExitCode = -1
		now := time.Now()
		stoppedJob.EndTime = &now
	}
	w.store.UpdateJob(stoppedJob)

	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup after forced stop", "cgroupPath", job.CgroupPath)
		w.cgroup.CleanupCgroup(job.Id)
	}

	// Handle network group cleanup
	if job.NetworkGroupID != "" {
		w.decrementGroupJobCount(job.NetworkGroupID)
	}

	totalDuration := time.Since(stopStartTime)
	log.Info("job stopped successfully", "method", "forced", "totalDuration", totalDuration)
	return nil
}

// Helper method to ensure netns directory exists
func (w *linuxWorker) ensureNetnsDir(nsPath string) error {
	dir := filepath.Dir(nsPath)

	// Check if directory exists
	if _, err := w.os.Stat(dir); err != nil {
		if w.os.IsNotExist(err) {
			// Directory doesn't exist, try to create it
			if mkdirErr := w.os.MkdirAll(dir, 0755); mkdirErr != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, mkdirErr)
			}
			w.logger.Debug("created netns directory", "path", dir)
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
	}

	return nil
}

// Enhanced network group handling with configurable networking
func (w *linuxWorker) handleNetworkGroup(networkGroupID, jobId string) (string, *syscall.SysProcAttr, bool, error) {
	w.groupMutex.Lock()
	defer w.groupMutex.Unlock()

	nsPath := filepath.Join(varRunNetns, networkGroupID)

	// Check if group exists
	if group, exists := w.networkGroups[networkGroupID]; exists {
		w.logger.Info("joining existing network group",
			"groupID", networkGroupID,
			"currentJobs", group.JobCount,
			"subnet", group.NetworkConfig.Subnet,
			"gateway", group.NetworkConfig.Gateway)

		// Verify the namespace file still exists
		if _, err := w.os.Stat(group.NamePath); err != nil {
			w.logger.Warn("network group namespace file missing, recreating group", "groupID", networkGroupID, "path", group.NamePath)
			delete(w.networkGroups, networkGroupID)
			// Fall through to create new group
		} else {
			// Create sysProcAttr for joining existing namespace
			// No new network namespace - we'll join existing namespace using setns before fork
			sysProcAttr := w.syscall.CreateProcessGroup()
			sysProcAttr.Cloneflags = syscall.CLONE_NEWPID |
				syscall.CLONE_NEWNS |
				syscall.CLONE_NEWIPC |
				syscall.CLONE_NEWUTS
			// Note: No CLONE_NEWNET - we'll join existing namespace using setns before fork

			return group.NamePath, sysProcAttr, false, nil
		}
	}

	// Create new network group with configuration
	w.logger.Info("creating new network group", "groupID", networkGroupID)

	// Get network configuration for this group
	networkConfig, err := w.getNetworkConfigForGroup(networkGroupID)
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to get network config for group %s: %w", networkGroupID, err)
	}

	// Validate network configuration
	if err := w.validateNetworkConfig(networkConfig); err != nil {
		return "", nil, false, fmt.Errorf("invalid network config for group %s: %w", networkGroupID, err)
	}

	// Create with new network namespace
	sysProcAttr := w.createSysProcAttrWithNamespaces()

	// Ensure /var/run/netns directory exists
	if err := w.ensureNetnsDir(nsPath); err != nil {
		return "", nil, false, fmt.Errorf("failed to ensure netns directory: %w", err)
	}

	// Create the group with network configuration
	group := &NetworkGroup{
		GroupID:       networkGroupID,
		JobCount:      0,
		NamePath:      nsPath,
		CreatedAt:     time.Now(),
		NetworkConfig: networkConfig,
	}

	w.networkGroups[networkGroupID] = group

	w.logger.Info("network group created with configuration",
		"groupID", networkGroupID,
		"subnet", networkConfig.Subnet,
		"gateway", networkConfig.Gateway,
		"interface", networkConfig.Interface)

	return nsPath, sysProcAttr, true, nil
}

func (w *linuxWorker) incrementGroupJobCount(networkGroupID string) {
	w.groupMutex.Lock()
	defer w.groupMutex.Unlock()

	if group, exists := w.networkGroups[networkGroupID]; exists {
		atomic.AddInt32(&group.JobCount, 1)
		w.logger.Debug("incremented job count for network group",
			"groupID", networkGroupID, "newCount", group.JobCount)
	}
}

func (w *linuxWorker) decrementGroupJobCount(networkGroupID string) {
	w.groupMutex.Lock()
	defer w.groupMutex.Unlock()

	if group, exists := w.networkGroups[networkGroupID]; exists {
		newCount := atomic.AddInt32(&group.JobCount, -1)
		w.logger.Debug("decremented job count for network group",
			"groupID", networkGroupID, "newCount", newCount)

		// Cleanup empty group
		if newCount <= 0 {
			w.logger.Info("cleaning up empty network group", "groupID", networkGroupID)

			// Release subnet allocation
			if w.subnetAllocator != nil {
				w.subnetAllocator.ReleaseSubnet(networkGroupID)
			}

			w.cleanupNetworkGroup(networkGroupID)
			delete(w.networkGroups, networkGroupID)
		}
	}
}

func (w *linuxWorker) cleanupNetworkGroup(networkGroupID string) {
	if group, exists := w.networkGroups[networkGroupID]; exists {
		// Unmount the bind mount first
		if err := w.syscall.Unmount(group.NamePath, 0); err != nil {
			w.logger.Warn("failed to unmount network group namespace",
				"groupID", networkGroupID, "path", group.NamePath, "error", err)
		}

		// Remove the namespace file
		if err := w.os.Remove(group.NamePath); err != nil {
			w.logger.Warn("failed to remove network group namespace file",
				"groupID", networkGroupID, "path", group.NamePath, "error", err)
		}

		w.logger.Info("network group cleaned up",
			"groupID", networkGroupID,
			"subnet", group.NetworkConfig.Subnet)
	}
}

func (w *linuxWorker) getJobInitPath() (string, error) {
	w.logger.Debug("searching for job-init binary")

	execPath, err := w.os.Executable()
	if err != nil {
		w.logger.Warn("failed to get executable path", "error", err)
	} else {
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		w.logger.Debug("checking job-init in executable directory", "execPath", execPath, "initPath", initPath)
		if _, e := w.os.Stat(initPath); e == nil {
			w.logger.Info("job-init found in executable directory", "path", initPath)
			return initPath, nil
		}
	}

	w.logger.Error("job-init binary not found in any location")
	return "", fmt.Errorf("job-init binary not found in executable dir")
}

func (w *linuxWorker) createSysProcAttrWithNamespaces() *syscall.SysProcAttr {
	sysProcAttr := w.syscall.CreateProcessGroup()

	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // PID namespace - ALWAYS isolated
		syscall.CLONE_NEWNS | // Mount namespace - ALWAYS isolated
		syscall.CLONE_NEWIPC | // IPC namespace - ALWAYS isolated
		syscall.CLONE_NEWUTS | // UTS namespace - ALWAYS isolated
		syscall.CLONE_NEWNET // Network namespace - isolated by default

	w.logger.Debug("enabled namespace isolation",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"namespaces", "pid,mount,ipc,uts,net")

	return sysProcAttr
}

func (w *linuxWorker) waitForCompletion(ctx context.Context, cmd os.Command, job *domain.Job) {
	log := w.logger.WithField("jobId", job.Id)
	startTime := time.Now()

	log.Debug("starting job monitoring", "pid", job.Pid, "command", job.Command, "networkGroup", job.NetworkGroupID)

	defer func() {
		// Cleanup network group reference
		if job.NetworkGroupID != "" {
			w.decrementGroupJobCount(job.NetworkGroupID)
		}

		// Cleanup isolated job namespace
		if job.NetworkGroupID == "" {
			nsPath := filepath.Join(netns, job.Id)
			w.os.Remove(nsPath)
		}

		if r := recover(); r != nil {
			duration := time.Since(startTime)
			log.Error("panic in job monitoring", "panic", r, "duration", duration)
			w.cleanup(job.Id, int(job.Pid))
		}
	}()

	err := cmd.Wait()
	duration := time.Since(startTime)

	if err != nil {
		var exitCode int32
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}

		log.Info("job completed with error", "exitCode", exitCode, "duration", duration, "error", err)
		completedJob := job.DeepCopy()
		if failErr := completedJob.Fail(exitCode); failErr != nil {
			log.Warn("domain validation failed for job failure", "domainError", failErr, "exitCode", exitCode)
			completedJob.Status = domain.StatusFailed
			completedJob.ExitCode = exitCode
			now := time.Now()
			completedJob.EndTime = &now
		}
		w.store.UpdateJob(completedJob)
	} else {
		log.Info("job completed successfully", "duration", duration)
		completedJob := job.DeepCopy()
		if completeErr := completedJob.Complete(0); completeErr != nil {
			log.Warn("domain validation failed for job completion", "domainError", completeErr)
			completedJob.Status = domain.StatusCompleted
			completedJob.ExitCode = 0
			now := time.Now()
			completedJob.EndTime = &now
		}
		w.store.UpdateJob(completedJob)
	}

	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup", "cgroupPath", job.CgroupPath)
		w.cgroup.CleanupCgroup(job.Id)
	}

	log.Debug("job monitoring completed", "totalDuration", time.Since(startTime))
}

func (w *linuxWorker) cleanup(jobId string, pid int) {
	log := w.logger.WithFields("jobId", jobId, "pid", pid)
	log.Warn("starting emergency cleanup")

	// Get job to check network group
	var networkGroupID string
	if job, exists := w.store.GetJob(jobId); exists {
		networkGroupID = job.NetworkGroupID
	}

	// Force kill the process group
	if err := w.syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		log.Warn("failed to emergency kill process group", "processGroup", -pid, "error", err)

		// Then try individual process
		if e := w.syscall.Kill(pid, syscall.SIGKILL); e != nil {
			log.Error("failed to emergency kill individual process", "pid", pid, "error", e)
		}
	}

	log.Debug("cleaning up cgroup in emergency cleanup")
	w.cgroup.CleanupCgroup(jobId)

	// Cleanup network resources
	if networkGroupID != "" {
		w.decrementGroupJobCount(networkGroupID)
	} else {
		// Remove isolated job namespace
		nsPath := filepath.Join(netns, jobId)
		w.os.Remove(nsPath)
	}

	// Update job status using domain method
	if job, exists := w.store.GetJob(jobId); exists {
		failedJob := job.DeepCopy()

		if failErr := failedJob.Fail(-1); failErr != nil {
			// When domain validation fails
			log.Warn("domain validation failed in emergency cleanup", "domainError", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}

		w.store.UpdateJob(failedJob)
		log.Info("job marked as failed in emergency cleanup")
	} else {
		log.Warn("job not found during emergency cleanup")
	}

	log.Info("emergency cleanup completed")
}

func (w *linuxWorker) processExists(pid int) bool {
	err := w.syscall.Kill(pid, 0)
	if err == nil {
		w.logger.Debug("process exists check: process found", "pid", pid)
		return true
	}

	if errors.Is(err, syscall.ESRCH) {
		w.logger.Debug("process exists check: no such process", "pid", pid)
		return false
	}

	if errors.Is(err, syscall.EPERM) {
		w.logger.Debug("process exists check: permission denied (process exists)", "pid", pid)
		return true
	}

	w.logger.Debug("process exists check: other error, assuming process doesn't exist", "pid", pid, "error", err)
	return false
}
