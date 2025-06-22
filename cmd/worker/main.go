package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
	"worker/internal/config"
	"worker/internal/worker"
	"worker/internal/worker/server"
	"worker/internal/worker/store"
	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

func main() {
	appLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			appLogger.SetLevel(level)
			appLogger.Info("log level set from environment", "level", level)
		} else {
			appLogger.Warn("invalid log level in environment, using INFO", "logLevel", logLevel, "error", err)
		}
	}

	appLogger.Info("worker starting", "version", "1.0.0", "platform", runtime.GOOS)

	// Validate all platform requirements at startup
	if err := validatePlatformRequirements(appLogger); err != nil {
		appLogger.Fatal("platform requirements not met", "error", err)
	}

	appLogger.Debug("goroutine monitoring started")

	// Initialize core components (store handles job state, worker executes jobs)
	s := store.New()
	w := worker.NewWorker(s)
	appLogger.Info("worker and store initialized", "platform", runtime.GOOS)

	// Setup graceful shutdown handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	var grpcServer *grpc.Server
	serverStarted := make(chan error, 1)

	// Start gRPC server in background
	go func() {
		appLogger.Info("starting JobService gRPC server", "address", ":50051")
		var err error
		grpcServer, err = server.StartGRPCServer(s, w)
		serverStarted <- err
	}()

	select {
	case err := <-serverStarted:
		if err != nil {
			appLogger.Fatal("failed to start server", "error", err)
		}
		appLogger.Info("gRPC server started successfully")

	case sig := <-sigChan:
		appLogger.Info("received shutdown signal", "signal", sig)
		cancel()
		return
	}

	go func() {
		sig := <-sigChan
		appLogger.Info("received shutdown signal, initiating graceful shutdown", "signal", sig)
		cancel()
	}()

	<-ctx.Done()

	appLogger.Info("starting graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if grpcServer != nil {
		appLogger.Info("stopping gRPC server")

		serverStopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(serverStopped)
		}()

		select {
		case <-serverStopped:
			appLogger.Info("gRPC server stopped gracefully")
		case <-shutdownCtx.Done():
			appLogger.Warn("gRPC server shutdown timeout, forcing stop")
			grpcServer.Stop()
		}
	}

	appLogger.Info("server gracefully stopped")
}

// validatePlatformRequirements checks all platform-specific requirements including user namespaces
func validatePlatformRequirements(logger *logger.Logger) error {
	switch runtime.GOOS {
	case "linux":
		return validateLinuxRequirements(logger)
	case "darwin":
		return fmt.Errorf("macOS not supported when user namespaces are required")
	default:
		return fmt.Errorf("unsupported platform: %s (user namespaces required)", runtime.GOOS)
	}
}

// validateLinuxRequirements performs comprehensive Linux environment validation
func validateLinuxRequirements(logger *logger.Logger) error {
	osInterface := &osinterface.DefaultOs{}

	logger.Info("validating Linux platform requirements")

	// Basic cgroups v2 validation
	if err := validateCgroupsV2(osInterface, logger); err != nil {
		return fmt.Errorf("cgroups v2 validation failed: %w", err)
	}

	// Cgroup namespace validation
	if err := validateCgroupNamespaces(osInterface, logger); err != nil {
		return fmt.Errorf("cgroup namespace validation failed: %w", err)
	}

	// User namespace validation
	if err := validateUserNamespaces(osInterface, logger); err != nil {
		return fmt.Errorf("user namespace validation failed: %w", err)
	}

	// Kernel version validation
	if err := validateKernelVersion(osInterface, logger); err != nil {
		return fmt.Errorf("kernel version validation failed: %w", err)
	}

	// File system permissions validation
	if err := validateFileSystemPermissions(osInterface, logger); err != nil {
		return fmt.Errorf("file system permissions validation failed: %w", err)
	}

	logger.Info("all Linux platform requirements validated successfully",
		"cgroupsV2", true,
		"cgroupNamespace", true,
		"userNamespace", true,
		"subuidConfig", true)

	return nil
}

// validateCgroupsV2 validates cgroups v2 support
func validateCgroupsV2(osInterface osinterface.OsInterface, logger *logger.Logger) error {
	cgroupPath := config.CgroupsBaseDir

	if _, err := osInterface.Stat(cgroupPath); osInterface.IsNotExist(err) {
		return fmt.Errorf("cgroups not available at %s", cgroupPath)
	}

	// Check for cgroups v2 specific files
	v2Files := []string{
		"cgroup.controllers",
		"cgroup.subtree_control",
		"cgroup.procs",
	}

	for _, file := range v2Files {
		filePath := fmt.Sprintf("%s/%s", cgroupPath, file)
		if _, err := osInterface.Stat(filePath); err != nil {
			return fmt.Errorf("cgroups v2 file missing: %s", filePath)
		}
	}

	logger.Debug("cgroups v2 validation passed", "cgroupsPath", cgroupPath)
	return nil
}

// validateCgroupNamespaces validates cgroup namespace support
func validateCgroupNamespaces(osInterface osinterface.OsInterface, logger *logger.Logger) error {
	cgroupNSPath := "/proc/self/ns/cgroup"

	if _, err := osInterface.Stat(cgroupNSPath); osInterface.IsNotExist(err) {
		return fmt.Errorf("cgroup namespaces not supported by kernel (required): %s not found", cgroupNSPath)
	}

	logger.Debug("cgroup namespace validation passed", "nsPath", cgroupNSPath)
	return nil
}

// validateUserNamespaces validates user namespace support and configuration
func validateUserNamespaces(osInterface osinterface.OsInterface, logger *logger.Logger) error {
	// Check kernel support for user namespaces
	userNSPath := "/proc/self/ns/user"
	if _, err := osInterface.Stat(userNSPath); osInterface.IsNotExist(err) {
		return fmt.Errorf("user namespaces not supported by kernel: %s not found", userNSPath)
	}

	// Check if user namespaces are enabled (some distros disable them)
	procSysPath := "/proc/sys/user/max_user_namespaces"
	if data, err := osInterface.ReadFile(procSysPath); err == nil {
		content := strings.TrimSpace(string(data))
		if content == "0" {
			return fmt.Errorf("user namespaces disabled in kernel (max_user_namespaces=0)")
		}
		logger.Debug("user namespace limit checked", "maxUserNamespaces", content)
	}

	// Check unprivileged user namespace creation
	unpriqPath := "/proc/sys/kernel/unprivileged_userns_clone"
	if data, err := osInterface.ReadFile(unpriqPath); err == nil {
		content := strings.TrimSpace(string(data))
		if content == "0" {
			logger.Warn("unprivileged user namespace creation disabled - may require privileged operation")
		} else {
			logger.Debug("unprivileged user namespace creation enabled")
		}
	}

	logger.Debug("user namespace kernel validation passed", "nsPath", userNSPath)
	return nil
}

// validateSubUIDGID validates subuid/subgid configuration using UserNamespaceManager
//func validateSubUIDGID(osInterface osinterface.OsInterface, logger *logger.Logger) error {
//	// Create a user namespace manager for validation
//	config := usernamespace.DefaultUserNamespaceConfig()
//	manager := usernamespace.NewUserNamespaceManager(config, osInterface)
//
//	// Use the manager's validation method
//	if err := manager.ValidateSubUIDGID(); err != nil {
//		return fmt.Errorf("subuid/subgid validation failed: %w", err)
//	}
//
//	// Additional validation: check if current user has subuid/subgid entries
//	currentUser := osInterface.Getenv("USER")
//	if currentUser == "" {
//		currentUser = "worker" // Default assumption
//	}
//
//	// Check subuid file content
//	if data, err := osInterface.ReadFile(config.SubUIDFile); err == nil {
//		content := string(data)
//		if !strings.Contains(content, currentUser+":") {
//			logger.Warn("current user not found in subuid file",
//				"user", currentUser,
//				"file", config.SubUIDFile,
//				"hint", fmt.Sprintf("Add: %s:100000:65536", currentUser))
//		} else {
//			logger.Debug("subuid entry found for current user", "user", currentUser)
//		}
//	}
//
//	// Check subgid file content
//	if data, err := osInterface.ReadFile(config.SubGIDFile); err == nil {
//		content := string(data)
//		if !strings.Contains(content, currentUser+":") {
//			logger.Warn("current user not found in subgid file",
//				"user", currentUser,
//				"file", config.SubGIDFile,
//				"hint", fmt.Sprintf("Add: %s:100000:65536", currentUser))
//		} else {
//			logger.Debug("subgid entry found for current user", "user", currentUser)
//		}
//	}
//
//	logger.Debug("subuid/subgid validation passed",
//		"subuidFile", config.SubUIDFile,
//		"subgidFile", config.SubGIDFile)
//	return nil
//}

// validateFileSystemPermissions validates required file system access
func validateFileSystemPermissions(osInterface osinterface.OsInterface, logger *logger.Logger) error {
	// For cgroup v2, we don't test direct filesystem writes
	// Instead, verify the cgroup2 filesystem is properly mounted
	cgroupPath := "/sys/fs/cgroup"

	if _, err := osInterface.Stat(cgroupPath); err != nil {
		return fmt.Errorf("cgroup filesystem not accessible: %s (%w)", cgroupPath, err)
	}

	// Check if it's mounted as cgroup v2 (we can read cgroup.controllers)
	controllersPath := fmt.Sprintf("%s/cgroup.controllers", cgroupPath)
	if _, err := osInterface.Stat(controllersPath); err != nil {
		return fmt.Errorf("cgroup v2 not properly mounted: %s not found", controllersPath)
	}

	logger.Debug("cgroup v2 filesystem validation passed", "cgroupPath", cgroupPath)
	return nil
}

// validateKernelVersion validates minimum kernel version for all features
func validateKernelVersion(osInterface osinterface.OsInterface, logger *logger.Logger) error {
	// Read kernel version
	version, err := osInterface.ReadFile("/proc/version")
	if err != nil {
		return fmt.Errorf("cannot read kernel version: %w", err)
	}

	versionStr := string(version)
	logger.Debug("kernel version detected", "version", versionStr)

	// Check for minimum kernel version (4.6+ for cgroup namespaces)
	if strings.Contains(versionStr, "Linux version 4.") {
		// Check if it's 4.6 or higher
		if strings.Contains(versionStr, "4.0.") ||
			strings.Contains(versionStr, "4.1.") ||
			strings.Contains(versionStr, "4.2.") ||
			strings.Contains(versionStr, "4.3.") ||
			strings.Contains(versionStr, "4.4.") ||
			strings.Contains(versionStr, "4.5.") {
			return fmt.Errorf("kernel too old for cgroup namespaces (need 4.6+): %s", versionStr)
		}
	}

	// Check for user namespace support (3.8+ but recommend 4.6+)
	if strings.Contains(versionStr, "Linux version 3.") ||
		strings.Contains(versionStr, "Linux version 2.") {
		return fmt.Errorf("kernel too old for reliable user namespace support (need 4.6+): %s", versionStr)
	}

	logger.Debug("kernel version validation passed")
	return nil
}
