//go:build linux

package core

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// RuntimeInstallationStreamer interface for streaming runtime installation progress
type RuntimeInstallationStreamer interface {
	SendProgress(message string) error
	SendLog(data []byte) error
}

// RuntimeInstaller handles runtime installation
type RuntimeInstaller struct {
	config   *config.Config
	logger   *logger.Logger
	platform platform.Platform
}

// NewRuntimeInstaller creates a new runtime installer
func NewRuntimeInstaller(config *config.Config, logger *logger.Logger, platform platform.Platform) *RuntimeInstaller {
	return &RuntimeInstaller{
		config:   config,
		logger:   logger.WithField("component", "runtime-installer"),
		platform: platform,
	}
}

// RuntimeInstallRequest represents a runtime installation request
type RuntimeInstallRequest struct {
	RuntimeSpec    string
	Repository     string
	Branch         string
	Path           string
	ForceReinstall bool
	Streamer       RuntimeInstallationStreamer
}

// RuntimeInstallResult represents the result of a runtime installation
type RuntimeInstallResult struct {
	RuntimeSpec string
	Success     bool
	Message     string
	InstallPath string
	Duration    time.Duration
	LogOutput   string
}

// RuntimeFile represents an uploaded runtime file
type RuntimeFile struct {
	Path       string
	Content    []byte
	Executable bool
}

// RuntimeInstallFromLocalRequest represents a local installation request
type RuntimeInstallFromLocalRequest struct {
	RuntimeSpec    string
	Files          []*RuntimeFile
	ForceReinstall bool
	Streamer       RuntimeInstallationStreamer
}

// InstallFromGithub installs a runtime from GitHub repository
func (ri *RuntimeInstaller) InstallFromGithub(ctx context.Context, req *RuntimeInstallRequest) (*RuntimeInstallResult, error) {
	startTime := time.Now()
	ri.logger.Info("installing runtime from GitHub", "spec", req.RuntimeSpec, "repo", req.Repository, "branch", req.Branch)

	// Handle force reinstall
	if req.ForceReinstall {
		runtimePath := filepath.Join(ri.config.Runtime.BasePath, req.RuntimeSpec)
		if err := ri.platform.RemoveAll(runtimePath); err != nil {
			ri.logger.Warn("failed to remove existing runtime", "path", runtimePath, "error", err)
		}
	}

	// Set defaults
	if req.Repository == "" {
		req.Repository = "ehsaniara/joblet"
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.Path == "" {
		req.Path = "runtimes"
	}

	// Create chroot environment
	chrootDir, cleanup, err := ri.createSimpleChroot()
	if err != nil {
		return nil, fmt.Errorf("failed to create chroot: %w", err)
	}
	defer cleanup()

	// Download and extract repository
	if err := ri.downloadAndExtractRepo(ctx, chrootDir, req); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Failed to download repository: %v", err),
			Duration:    time.Since(startTime),
		}, err
	}

	// Execute setup script
	output, err := ri.executeSetupScript(ctx, chrootDir, req)
	if err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Setup script failed: %v", err),
			LogOutput:   output,
			Duration:    time.Since(startTime),
		}, err
	}

	// Copy runtime to host
	if err := ri.copyRuntimeToHost(chrootDir, req.RuntimeSpec); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Failed to copy runtime: %v", err),
			LogOutput:   output,
			Duration:    time.Since(startTime),
		}, err
	}

	return &RuntimeInstallResult{
		RuntimeSpec: req.RuntimeSpec,
		Success:     true,
		Message:     "Runtime installed successfully",
		InstallPath: filepath.Join(ri.config.Runtime.BasePath, req.RuntimeSpec, "runtime.yml"),
		Duration:    time.Since(startTime),
		LogOutput:   output,
	}, nil
}

// InstallFromLocal installs a runtime from uploaded files
func (ri *RuntimeInstaller) InstallFromLocal(ctx context.Context, req *RuntimeInstallFromLocalRequest) (*RuntimeInstallResult, error) {
	startTime := time.Now()
	ri.logger.Info("installing runtime from local files", "spec", req.RuntimeSpec, "files", len(req.Files))

	// Handle force reinstall
	if req.ForceReinstall {
		runtimePath := filepath.Join(ri.config.Runtime.BasePath, req.RuntimeSpec)
		if err := ri.platform.RemoveAll(runtimePath); err != nil {
			ri.logger.Warn("failed to remove existing runtime", "path", runtimePath, "error", err)
		}
	}

	// Create chroot environment
	chrootDir, cleanup, err := ri.createSimpleChroot()
	if err != nil {
		return nil, fmt.Errorf("failed to create chroot: %w", err)
	}
	defer cleanup()

	// Write files to chroot
	targetDir := filepath.Join(chrootDir, "tmp", "runtime-scripts")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	for _, file := range req.Files {
		filePath := filepath.Join(targetDir, file.Path)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", file.Path, err)
		}

		mode := os.FileMode(0644)
		if file.Executable || strings.HasSuffix(file.Path, ".sh") {
			mode = 0755
		}
		if err := os.WriteFile(filePath, file.Content, mode); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
	}

	// Execute setup script
	output, err := ri.executeLocalSetupScript(ctx, chrootDir, req)
	if err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Setup script failed: %v", err),
			LogOutput:   output,
			Duration:    time.Since(startTime),
		}, err
	}

	// Copy runtime to host
	if err := ri.copyRuntimeToHost(chrootDir, req.RuntimeSpec); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Failed to copy runtime: %v", err),
			LogOutput:   output,
			Duration:    time.Since(startTime),
		}, err
	}

	return &RuntimeInstallResult{
		RuntimeSpec: req.RuntimeSpec,
		Success:     true,
		Message:     "Runtime installed successfully from local files",
		InstallPath: filepath.Join(ri.config.Runtime.BasePath, req.RuntimeSpec, "runtime.yml"),
		Duration:    time.Since(startTime),
		LogOutput:   output,
	}, nil
}

// createSimpleChroot creates a chroot environment with full host access
func (ri *RuntimeInstaller) createSimpleChroot() (string, func(), error) {
	chrootDir, err := os.MkdirTemp("", "runtime-chroot-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create essential directories
	dirs := []string{
		"proc", "sys", "dev", "dev/pts", "tmp", "var/tmp", "run",
		"opt/joblet/runtimes", "usr", "lib", "lib64", "bin", "sbin", "etc",
		"var", "var/lib", "var/cache", "home", "root",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(chrootDir, dir), 0755); err != nil {
			os.RemoveAll(chrootDir)
			return "", nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create device nodes for proper operation
	if err := ri.createDeviceNodes(chrootDir); err != nil {
		return "", nil, fmt.Errorf("failed to create device nodes: %w", err)
	}

	// Mount special filesystems
	specialMounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
	}{
		{"/proc", filepath.Join(chrootDir, "proc"), "proc", 0},
		{"/sys", filepath.Join(chrootDir, "sys"), "sysfs", 0},
		{"/dev", filepath.Join(chrootDir, "dev"), "", syscall.MS_BIND},
		{"/dev/pts", filepath.Join(chrootDir, "dev/pts"), "", syscall.MS_BIND},
		{"/run", filepath.Join(chrootDir, "run"), "", syscall.MS_BIND},
		{"tmpfs", filepath.Join(chrootDir, "tmp"), "tmpfs", 0},
	}

	var mountedPaths []string
	for _, m := range specialMounts {
		if m.fstype != "" && m.fstype != "tmpfs" {
			if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, ""); err != nil {
				ri.logger.Debug("special mount failed", "source", m.source, "target", m.target, "error", err)
				continue
			}
		} else if m.fstype == "tmpfs" {
			if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, "size=1G"); err != nil {
				ri.logger.Debug("tmpfs mount failed", "target", m.target, "error", err)
				continue
			}
		} else {
			if err := syscall.Mount(m.source, m.target, "", m.flags, ""); err != nil {
				ri.logger.Debug("bind mount failed", "source", m.source, "target", m.target, "error", err)
				continue
			}
		}
		mountedPaths = append(mountedPaths, m.target)
	}

	// Bind mount host filesystem directories (read-write for runtime installation)
	hostDirs := []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"}
	for _, dir := range hostDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			target := filepath.Join(chrootDir, dir)
			// Mount as read-write for runtime installation needs
			if err := syscall.Mount(dir, target, "", syscall.MS_BIND, ""); err != nil {
				ri.logger.Debug("host bind mount failed", "source", dir, "target", target, "error", err)
				continue
			}
			mountedPaths = append(mountedPaths, target)
		}
	}

	// The bind mounts above provide access to host binaries
	// Setup scripts will find what they need and copy to their isolated/ directory

	cleanup := func() {
		// Unmount all mounted paths in reverse order
		for i := len(mountedPaths) - 1; i >= 0; i-- {
			if err := syscall.Unmount(mountedPaths[i], syscall.MNT_DETACH); err != nil {
				// Log but don't fail on unmount errors during cleanup
				fmt.Printf("Warning: failed to unmount %s: %v\n", mountedPaths[i], err)
			}
		}
		os.RemoveAll(chrootDir)
	}

	return chrootDir, cleanup, nil
}

// createDeviceNodes creates essential device nodes in chroot
func (ri *RuntimeInstaller) createDeviceNodes(chrootDir string) error {
	devices := []struct {
		path  string
		major uint32
		minor uint32
		mode  uint32
	}{
		{"dev/null", 1, 3, syscall.S_IFCHR | 0666},
		{"dev/zero", 1, 5, syscall.S_IFCHR | 0666},
		{"dev/random", 1, 8, syscall.S_IFCHR | 0666},
		{"dev/urandom", 1, 9, syscall.S_IFCHR | 0666},
	}

	for _, dev := range devices {
		devPath := filepath.Join(chrootDir, dev.path)
		if err := syscall.Mknod(devPath, dev.mode, int(ri.makedev(dev.major, dev.minor))); err != nil {
			ri.logger.Debug("failed to create device node", "path", devPath, "error", err)
		}
	}
	return nil
}

// makedev creates a device number from major and minor numbers
func (ri *RuntimeInstaller) makedev(major, minor uint32) uint64 {
	return uint64(major)<<8 | uint64(minor)
}

// downloadAndExtractRepo downloads and extracts GitHub repository
func (ri *RuntimeInstaller) downloadAndExtractRepo(ctx context.Context, chrootDir string, req *RuntimeInstallRequest) error {
	if req.Streamer != nil {
		if err := req.Streamer.SendProgress("📦 Cloning repository..."); err != nil {
			return fmt.Errorf("failed to send progress: %w", err)
		}
	}

	// Download as ZIP
	zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/%s.zip", req.Repository, req.Branch)
	zipPath := filepath.Join(chrootDir, "tmp", "repo.zip")

	resp, err := http.Get(zipURL)
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download repository: status %d", resp.StatusCode)
	}

	// Save ZIP file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save zip file: %w", err)
	}

	if req.Streamer != nil {
		if err := req.Streamer.SendProgress("Extracting repository..."); err != nil {
			return fmt.Errorf("failed to send progress: %w", err)
		}
	}

	// Extract ZIP
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	extractDir := filepath.Join(chrootDir, "tmp", "runtime-build")
	repoName := strings.Split(req.Repository, "/")[1]
	expectedPrefix := fmt.Sprintf("%s-%s/", repoName, req.Branch)

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, expectedPrefix) {
			continue
		}

		relativePath := strings.TrimPrefix(file.Name, expectedPrefix)
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(extractDir, relativePath)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(targetPath)
		if err != nil {
			src.Close()
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}

		if err := dst.Chmod(file.Mode()); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	if req.Streamer != nil {
		if err := req.Streamer.SendProgress("Repository extracted successfully"); err != nil {
			// Log but don't fail on progress send errors
			fmt.Printf("Warning: failed to send progress: %v\n", err)
		}
	}

	return nil
}

// executeSetupScript executes the setup.sh script for GitHub installations
func (ri *RuntimeInstaller) executeSetupScript(ctx context.Context, chrootDir string, req *RuntimeInstallRequest) (string, error) {
	if req.Streamer != nil {
		if err := req.Streamer.SendProgress("🏗️  Running setup script..."); err != nil {
			return "", fmt.Errorf("failed to send progress: %w", err)
		}
	}

	workDir := fmt.Sprintf("/tmp/runtime-build/%s/%s", req.Path, req.RuntimeSpec)

	script := fmt.Sprintf(`#!/bin/bash
set -e
cd %s
if [ -f setup.sh ]; then
    echo "Executing setup.sh in %s"
    /bin/chmod +x setup.sh || true
    ./setup.sh
else
    echo "No setup script found in %s"
    /bin/ls -la || ls -la
    exit 1
fi
`, workDir, workDir, workDir)

	scriptPath := filepath.Join(chrootDir, "tmp", "run-setup.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", fmt.Errorf("failed to write setup script: %w", err)
	}

	return ri.runChrootCommand(ctx, chrootDir, "/bin/bash", []string{"/tmp/run-setup.sh"}, req.RuntimeSpec, req.Streamer)
}

// executeLocalSetupScript executes the setup.sh script for local installations
func (ri *RuntimeInstaller) executeLocalSetupScript(ctx context.Context, chrootDir string, req *RuntimeInstallFromLocalRequest) (string, error) {
	if req.Streamer != nil {
		if err := req.Streamer.SendProgress("🏗️  Running local setup script..."); err != nil {
			return "", fmt.Errorf("failed to send progress: %w", err)
		}
	}

	script := `#!/bin/bash
set -e
cd /tmp/runtime-scripts
if [ -f setup.sh ]; then
    echo "Executing setup.sh in /tmp/runtime-scripts"
    /bin/chmod +x setup.sh || true
    ./setup.sh
else
    echo "No setup script found"
    /bin/ls -la || ls -la
    exit 1
fi
`

	scriptPath := filepath.Join(chrootDir, "tmp", "run-setup.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", fmt.Errorf("failed to write setup script: %w", err)
	}

	return ri.runChrootCommand(ctx, chrootDir, "/bin/bash", []string{"/tmp/run-setup.sh"}, req.RuntimeSpec, req.Streamer)
}

// runChrootCommand runs a command in chroot with real-time streaming
func (ri *RuntimeInstaller) runChrootCommand(ctx context.Context, chrootDir, command string, args []string, runtimeSpec string, streamer RuntimeInstallationStreamer) (string, error) {
	cmd := exec.CommandContext(ctx, "chroot", append([]string{chrootDir, command}, args...)...)

	env := []string{
		"PATH=/usr/bin:/bin:/sbin:/usr/sbin",
		"HOME=/root",
		fmt.Sprintf("RUNTIME_SPEC=%s", runtimeSpec),
		"RUNTIME_DIR=/opt/joblet/runtimes",
		fmt.Sprintf("BUILD_ID=install-%d", time.Now().Unix()),
		"JOBLET_CHROOT=true",
	}
	cmd.Env = env

	// If no streamer, use simple CombinedOutput
	if streamer == nil {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("command execution failed: %w", err)
		}
		return string(output), nil
	}

	// Set up pipes for real-time streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Stream output in real-time
	var outputBuffer strings.Builder
	done := make(chan bool, 2)

	// Stream stdout
	go func() {
		defer func() { done <- true }()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				data := buf[:n]
				outputBuffer.Write(data)
				if err := streamer.SendLog(data); err != nil {
					// Log but don't fail on log send errors
					fmt.Printf("Warning: failed to send log: %v\n", err)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Stream stderr
	go func() {
		defer func() { done <- true }()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				data := buf[:n]
				outputBuffer.Write(data)
				if err := streamer.SendLog(data); err != nil {
					// Log but don't fail on log send errors
					fmt.Printf("Warning: failed to send log: %v\n", err)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for streaming to complete
	<-done
	<-done

	// Wait for command to finish
	err = cmd.Wait()
	output := outputBuffer.String()

	if err != nil {
		return output, fmt.Errorf("command execution failed: %w", err)
	}

	return output, nil
}

// copyRuntimeToHost copies runtime from chroot to host
func (ri *RuntimeInstaller) copyRuntimeToHost(chrootDir, runtimeSpec string) error {
	hostPath := filepath.Join(ri.config.Runtime.BasePath, runtimeSpec)
	chrootPath := filepath.Join(chrootDir, "opt/joblet/runtimes", runtimeSpec)

	// Create host directory
	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return fmt.Errorf("failed to create host directory: %w", err)
	}

	// Copy runtime files
	copyCmd := exec.Command("cp", "-r", chrootPath+"/.", hostPath)
	if output, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy runtime: %w, output: %s", err, output)
	}

	// Verify runtime.yml exists
	runtimeYML := filepath.Join(hostPath, "runtime.yml")
	if _, err := os.Stat(runtimeYML); err != nil {
		return fmt.Errorf("runtime.yml not created at %s", runtimeYML)
	}

	return nil
}
