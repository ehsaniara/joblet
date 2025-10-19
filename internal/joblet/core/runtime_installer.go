//go:build linux

package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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
	"github.com/ehsaniara/joblet/pkg/registry"
	"github.com/ehsaniara/joblet/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// RuntimeInstallationStreamer interface for streaming runtime installation progress
type RuntimeInstallationStreamer interface {
	SendProgress(message string) error
	SendLog(data []byte) error
}

// RuntimeInstaller handles runtime installation
type RuntimeInstaller struct {
	config             *config.Config
	logger             *logger.Logger
	platform           platform.Platform
	registryClient     *registry.Client
	registryDownloader *registry.Downloader
	registryURL        string // External registry URL (e.g., https://github.com/ehsaniara/joblet-runtimes)
}

// NewRuntimeInstaller creates a new runtime installer
func NewRuntimeInstaller(config *config.Config, logger *logger.Logger, platform platform.Platform) *RuntimeInstaller {
	// Default registry URL
	registryURL := "https://github.com/ehsaniara/joblet-runtimes"

	return &RuntimeInstaller{
		config:             config,
		logger:             logger.WithField("component", "runtime-installer"),
		platform:           platform,
		registryClient:     registry.NewClient(),
		registryDownloader: registry.NewDownloader(),
		registryURL:        registryURL,
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

// RuntimeInstallFromRegistryRequest represents a registry installation request
type RuntimeInstallFromRegistryRequest struct {
	RuntimeSpec    string                      // Runtime spec with version (e.g., "python-3.11-ml@1.0.0")
	ForceReinstall bool                        // Force reinstallation if already exists
	RegistryURL    string                      // Custom registry URL (default: https://github.com/ehsaniara/joblet-runtimes)
	Streamer       RuntimeInstallationStreamer // Optional streaming support
}

// InstallFromRegistry installs a runtime from the external registry
// This method does NOT fallback to GitHub - if not found in registry, it returns an error
func (ri *RuntimeInstaller) InstallFromRegistry(ctx context.Context, req *RuntimeInstallFromRegistryRequest) (*RuntimeInstallResult, error) {
	startTime := time.Now()

	// Use custom registry URL if provided, otherwise use default
	registryURL := req.RegistryURL
	if registryURL == "" {
		registryURL = ri.registryURL // Default: https://github.com/ehsaniara/joblet-runtimes
	}

	ri.logger.Info("installing runtime from external registry", "spec", req.RuntimeSpec, "registry", registryURL)

	// Send progress update
	if req.Streamer != nil {
		_ = req.Streamer.SendProgress(fmt.Sprintf("📦 Parsing runtime specification: %s", req.RuntimeSpec))
	}

	// Parse runtime spec to extract name and version
	spec, err := runtime.ParseRuntimeSpec(req.RuntimeSpec)
	if err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Invalid runtime specification: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("invalid runtime specification: %w", err)
	}

	ri.logger.Info("parsed runtime spec", "name", spec.Name, "version", spec.Version)

	// Send progress update
	if req.Streamer != nil {
		_ = req.Streamer.SendProgress(fmt.Sprintf("📡 Fetching registry from %s", registryURL))
	}

	// Resolve version from registry
	resolvedSpec, entry, err := ri.registryClient.ResolveVersion(ctx, spec, registryURL)
	if err != nil {
		// Runtime not found in registry - NO FALLBACK
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Runtime not found in registry: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("runtime not found in registry: %w", err)
	}

	ri.logger.Info("resolved runtime version", "name", resolvedSpec.Name, "version", resolvedSpec.Version, "url", entry.DownloadURL)

	// Send progress update
	if req.Streamer != nil {
		if spec.IsLatest() {
			_ = req.Streamer.SendProgress(fmt.Sprintf("✅ Resolved @latest → %s", resolvedSpec.Version))
		}
		_ = req.Streamer.SendProgress(fmt.Sprintf("📥 Downloading: %s (%d MB)", resolvedSpec.FullName(), entry.Size/1024/1024))
	}

	// Determine installation path using nested version structure
	// Format: /opt/joblet/runtimes/<name>/<version>/
	// Example: /opt/joblet/runtimes/python-3.11/1.2.0/
	installPath := filepath.Join(ri.config.Runtime.BasePath, resolvedSpec.Name, resolvedSpec.Version)

	// Check if already installed
	if !req.ForceReinstall {
		if _, err := os.Stat(installPath); err == nil {
			ri.logger.Info("runtime already installed", "path", installPath)
			return &RuntimeInstallResult{
				RuntimeSpec: req.RuntimeSpec,
				Success:     true,
				Message:     fmt.Sprintf("Runtime %s@%s is already installed", resolvedSpec.Name, resolvedSpec.Version),
				InstallPath: installPath,
				Duration:    time.Since(startTime),
			}, nil
		}
	} else {
		// Force reinstall - remove existing version
		if err := os.RemoveAll(installPath); err != nil {
			ri.logger.Warn("failed to remove existing runtime", "path", installPath, "error", err)
		}
	}

	// Create temporary download directory
	tmpDir, err := os.MkdirTemp("", "runtime-download-*")
	if err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Failed to create temp directory: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download package with checksum verification
	packagePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tar.gz", resolvedSpec.FullName()))

	progressCallback := func(progress registry.DownloadProgress) {
		if req.Streamer != nil && progress.Percentage >= 0 {
			_ = req.Streamer.SendProgress(fmt.Sprintf("⬇️  Downloading: %d%% (%d MB / %d MB)",
				progress.Percentage,
				progress.BytesDownloaded/1024/1024,
				progress.TotalBytes/1024/1024))
		}
	}

	if err := ri.registryDownloader.DownloadAndVerify(ctx, entry.DownloadURL, entry.Checksum, packagePath, progressCallback); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Download failed: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("download failed: %w", err)
	}

	ri.logger.Info("package downloaded and verified", "path", packagePath, "checksum", entry.Checksum)

	// Send progress update
	if req.Streamer != nil {
		_ = req.Streamer.SendProgress("🔐 Checksum verified successfully")
		_ = req.Streamer.SendProgress("📦 Extracting runtime package...")
	}

	// Create temporary extraction directory for source packages
	// We'll determine if it's a source or pre-built package after extraction
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Failed to create extraction directory: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Extract package to temporary directory first
	if err := ri.extractTarGz(packagePath, extractDir); err != nil {
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     fmt.Sprintf("Extraction failed: %v", err),
			Duration:    time.Since(startTime),
		}, fmt.Errorf("extraction failed: %w", err)
	}

	ri.logger.Info("runtime package extracted to temp directory", "path", extractDir)

	// Check if it's a pre-built package (has runtime.yml) or source package (has setup.sh)
	extractedRuntimeYml := filepath.Join(extractDir, "runtime.yml")
	extractedSetupScript := filepath.Join(extractDir, "setup.sh")

	if _, err := os.Stat(extractedRuntimeYml); err == nil {
		// Pre-built package detected - just copy to install path
		ri.logger.Info("pre-built package detected", "path", extractDir)

		if req.Streamer != nil {
			_ = req.Streamer.SendProgress("📦 Pre-built package detected - installing...")
		}

		// Create installation directory
		if err := os.MkdirAll(installPath, 0755); err != nil {
			return &RuntimeInstallResult{
				RuntimeSpec: req.RuntimeSpec,
				Success:     false,
				Message:     fmt.Sprintf("Failed to create installation directory: %v", err),
				Duration:    time.Since(startTime),
			}, fmt.Errorf("failed to create installation directory: %w", err)
		}

		// Copy all files from extractDir to installPath
		copyCmd := exec.Command("cp", "-r", extractDir+"/.", installPath)
		if output, err := copyCmd.CombinedOutput(); err != nil {
			return &RuntimeInstallResult{
				RuntimeSpec: req.RuntimeSpec,
				Success:     false,
				Message:     fmt.Sprintf("Failed to copy runtime: %v", err),
				Duration:    time.Since(startTime),
			}, fmt.Errorf("failed to copy runtime: %w, output: %s", err, output)
		}

		ri.logger.Info("pre-built runtime installed", "path", installPath)

	} else if _, err := os.Stat(extractedSetupScript); err == nil {
		// Source package detected - run setup script on target server
		ri.logger.Info("source package detected", "path", extractDir)

		if req.Streamer != nil {
			_ = req.Streamer.SendProgress("📦 Source package detected - building runtime on target server...")
		}

		// Execute setup script to build runtime
		if err := ri.executeSetupScriptForRegistry(ctx, extractDir, resolvedSpec, req.Streamer); err != nil {
			return &RuntimeInstallResult{
				RuntimeSpec: req.RuntimeSpec,
				Success:     false,
				Message:     fmt.Sprintf("Setup script failed: %v", err),
				Duration:    time.Since(startTime),
			}, fmt.Errorf("setup script failed: %w", err)
		}

		// Verify runtime.yml was created in final install path
		finalRuntimeYml := filepath.Join(installPath, "runtime.yml")
		if _, err := os.Stat(finalRuntimeYml); os.IsNotExist(err) {
			return &RuntimeInstallResult{
				RuntimeSpec: req.RuntimeSpec,
				Success:     false,
				Message:     "Setup script did not create runtime.yml in installation directory",
				Duration:    time.Since(startTime),
			}, fmt.Errorf("setup script did not create runtime.yml at %s", finalRuntimeYml)
		}

		ri.logger.Info("runtime built successfully from source package", "path", installPath)

	} else {
		// No runtime.yml and no setup script - invalid package
		return &RuntimeInstallResult{
			RuntimeSpec: req.RuntimeSpec,
			Success:     false,
			Message:     "Invalid runtime package: no runtime.yml or setup.sh found after extraction",
			Duration:    time.Since(startTime),
		}, fmt.Errorf("invalid runtime package: no runtime.yml or setup.sh in %s", extractDir)
	}

	// Send progress update
	if req.Streamer != nil {
		_ = req.Streamer.SendProgress(fmt.Sprintf("✅ Runtime installed: %s", installPath))
	}

	return &RuntimeInstallResult{
		RuntimeSpec: req.RuntimeSpec,
		Success:     true,
		Message:     fmt.Sprintf("Runtime %s@%s installed successfully", resolvedSpec.Name, resolvedSpec.Version),
		InstallPath: installPath,
		Duration:    time.Since(startTime),
	}, nil
}

// extractTarGz extracts a .tar.gz file to the specified destination
// Automatically strips a single top-level directory if all files are nested under it
func (ri *RuntimeInstaller) extractTarGz(tarGzPath, destPath string) error {
	// First pass: detect if there's a common top-level directory
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz file: %w", err)
	}

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}

	tarReader := tar.NewReader(gzReader)

	var topLevelPrefix string
	firstEntry := true
	entryCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			gzReader.Close()
			file.Close()
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		entryCount++

		// Normalize path separators
		normalizedName := filepath.ToSlash(header.Name)
		parts := strings.Split(normalizedName, "/")

		if firstEntry {
			// Set prefix from first entry
			if len(parts) >= 1 && parts[0] != "" {
				topLevelPrefix = parts[0] + "/"
			}
			firstEntry = false
		} else if topLevelPrefix != "" {
			// Verify all entries share the same prefix
			// Entry either IS the prefix directory or starts with it
			if normalizedName != strings.TrimSuffix(topLevelPrefix, "/") && !strings.HasPrefix(normalizedName, topLevelPrefix) {
				topLevelPrefix = "" // No common prefix
			}
		}
	}

	gzReader.Close()
	file.Close()

	// Second pass: extract files
	file, err = os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("failed to reopen tar.gz file: %w", err)
	}
	defer file.Close()

	gzReader, err = gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader = tar.NewReader(gzReader)

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract each file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Strip top-level directory if present
		relativePath := header.Name
		if topLevelPrefix != "" {
			relativePath = strings.TrimPrefix(relativePath, topLevelPrefix)
			if relativePath == "" {
				continue // Skip the top-level directory itself
			}
		}

		// Construct target path
		targetPath := filepath.Join(destPath, relativePath)

		// Security check: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destPath)) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			// Copy file contents
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close()

		case tar.TypeSymlink:
			// Create symlink
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}

		default:
			ri.logger.Warn("unsupported tar entry type", "type", header.Typeflag, "name", header.Name)
		}
	}

	return nil
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

	// Bind mount host filesystem directories (READ-ONLY to prevent host contamination)
	hostDirs := []string{"/usr", "/lib", "/lib64", "/bin", "/sbin", "/etc", "/var"}
	for _, dir := range hostDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			target := filepath.Join(chrootDir, dir)
			// CRITICAL: Mount as READ-ONLY to prevent host contamination
			// Setup scripts can read host binaries/libraries but cannot modify them
			if err := syscall.Mount(dir, target, "", syscall.MS_BIND, ""); err != nil {
				ri.logger.Debug("host bind mount failed", "source", dir, "target", target, "error", err)
				continue
			}
			// Make the bind mount read-only (requires remount)
			if err := syscall.Mount("", target, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
				ri.logger.Debug("failed to make bind mount read-only", "target", target, "error", err)
				// Don't add to mountedPaths if remount failed
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

	// For GitHub installations, pass empty version (setup script will determine from runtime.yml)
	return ri.runChrootCommand(ctx, chrootDir, "/bin/bash", []string{"/tmp/run-setup.sh"}, req.RuntimeSpec, "", req.Streamer)
}

// runChrootCommand runs a command in chroot with real-time streaming
func (ri *RuntimeInstaller) runChrootCommand(ctx context.Context, chrootDir, command string, args []string, runtimeSpec string, runtimeVersion string, streamer RuntimeInstallationStreamer) (string, error) {
	cmd := exec.CommandContext(ctx, "chroot", append([]string{chrootDir, command}, args...)...)

	env := []string{
		"PATH=/usr/bin:/bin:/sbin:/usr/sbin",
		"HOME=/root",
		fmt.Sprintf("RUNTIME_SPEC=%s", runtimeSpec),
		fmt.Sprintf("RUNTIME_VERSION=%s", runtimeVersion),
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

// executeSetupScriptForRegistry executes setup script for source packages downloaded from registry
// IMPORTANT: This runs the setup script inside a chroot environment to prevent host contamination
func (ri *RuntimeInstaller) executeSetupScriptForRegistry(ctx context.Context, sourcePath string, spec *runtime.RuntimeSpec, streamer RuntimeInstallationStreamer) error {
	// Detect platform
	platform, err := ri.detectPlatform()
	if err != nil {
		return fmt.Errorf("failed to detect platform: %w", err)
	}

	ri.logger.Info("detected platform", "platform", platform)

	if streamer != nil {
		_ = streamer.SendProgress(fmt.Sprintf("🖥️  Detected platform: %s", platform))
		_ = streamer.SendProgress("🔒 Creating isolated chroot environment...")
	}

	// Create chroot environment (same as GitHub installation flow)
	chrootDir, cleanup, err := ri.createSimpleChroot()
	if err != nil {
		return fmt.Errorf("failed to create chroot environment: %w", err)
	}
	defer cleanup()

	ri.logger.Info("created chroot environment", "path", chrootDir)

	if streamer != nil {
		_ = streamer.SendProgress("📦 Copying source package to chroot...")
	}

	// Copy source package to chroot /tmp
	chrootSourcePath := filepath.Join(chrootDir, "tmp", "runtime-source")
	if err := os.MkdirAll(chrootSourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create chroot source directory: %w", err)
	}

	// Copy all source files to chroot
	copyCmd := exec.Command("cp", "-r", sourcePath+"/.", chrootSourcePath)
	if output, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy source to chroot: %w, output: %s", err, output)
	}

	ri.logger.Info("copied source package to chroot", "dest", chrootSourcePath)

	if streamer != nil {
		_ = streamer.SendProgress("🔨 Running setup script in isolated environment...")
	}

	// Look for platform-specific setup script first, fallback to generic setup.sh
	setupScripts := []string{
		fmt.Sprintf("setup-%s.sh", platform),
		"setup.sh",
	}

	var setupScript string
	for _, script := range setupScripts {
		scriptPath := filepath.Join(chrootSourcePath, script)
		if _, err := os.Stat(scriptPath); err == nil {
			setupScript = script
			ri.logger.Info("found setup script", "name", script)
			break
		}
	}

	if setupScript == "" {
		return fmt.Errorf("no setup script found for platform %s", platform)
	}

	// Create wrapper script to execute setup in chroot
	wrapperScript := fmt.Sprintf(`#!/bin/bash
set -e
cd /tmp/runtime-source
if [ -f %s ]; then
    echo "🔒 Executing %s in isolated chroot environment"
    echo "JOBLET_CHROOT=true (host is protected)"
    /bin/chmod +x %s || true
    ./%s
else
    echo "ERROR: Setup script not found: %s"
    exit 1
fi
`, setupScript, setupScript, setupScript, setupScript, setupScript)

	wrapperPath := filepath.Join(chrootDir, "tmp", "run-setup.sh")
	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	// Execute setup script inside chroot
	// IMPORTANT: Pass NAME and VERSION separately
	// Setup scripts use RUNTIME_SPEC for installation directory and RUNTIME_VERSION for runtime.yml
	_, err = ri.runChrootCommand(
		ctx,
		chrootDir,
		"/bin/bash",
		[]string{"/tmp/run-setup.sh"},
		spec.Name,    // Just the name, e.g., "python-3.11-ml"
		spec.Version, // Package version, e.g., "1.3.1"
		streamer,
	)
	if err != nil {
		return fmt.Errorf("setup script execution failed in chroot: %w", err)
	}

	// Fix runtime.yml version to match package version (setup scripts may use wrong version)
	// The version in runtime.yml must match the package version (spec.Version) for resolver to work
	runtimeYmlPath := filepath.Join(chrootDir, "opt", "joblet", "runtimes", spec.Name, "runtime.yml")
	if err := ri.fixRuntimeYmlVersion(runtimeYmlPath, spec.Version); err != nil {
		ri.logger.Warn("failed to fix runtime.yml version", "error", err)
		// Non-fatal - continue with installation
	}

	if streamer != nil {
		_ = streamer.SendProgress("✅ Runtime built successfully in isolated environment")
		_ = streamer.SendProgress("📁 Moving runtime from chroot to host...")
	}

	// Copy runtime from chroot to host (same as GitHub installation flow)
	if err := ri.copyRuntimeToHost(chrootDir, spec.Name); err != nil {
		return fmt.Errorf("failed to copy runtime from chroot: %w", err)
	}

	if streamer != nil {
		_ = streamer.SendProgress("📁 Moving to nested version structure...")
	}

	// Setup scripts install to flat structure: /opt/joblet/runtimes/<name>/
	// We need to move it to nested structure: /opt/joblet/runtimes/<name>/<version>/
	flatInstallPath := filepath.Join(ri.config.Runtime.BasePath, spec.Name)
	nestedInstallPath := filepath.Join(ri.config.Runtime.BasePath, spec.Name, spec.Version)

	// Check if flat install path exists
	if _, err := os.Stat(flatInstallPath); os.IsNotExist(err) {
		return fmt.Errorf("setup script did not create runtime at expected path: %s", flatInstallPath)
	}

	// Check if it's actually a directory (not already the nested structure)
	runtimeYmlFlat := filepath.Join(flatInstallPath, "runtime.yml")
	if _, err := os.Stat(runtimeYmlFlat); err == nil {
		// Flat structure exists, need to move to nested
		ri.logger.Info("moving runtime from flat to nested structure",
			"from", flatInstallPath,
			"to", nestedInstallPath)

		// Create parent directory for nested structure
		if err := os.MkdirAll(filepath.Dir(nestedInstallPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for nested structure: %w", err)
		}

		// Use temp location to avoid "cannot move directory into itself" error
		tempPath := flatInstallPath + "-temp-" + fmt.Sprint(time.Now().Unix())

		// Move flat path to temp
		if err := os.Rename(flatInstallPath, tempPath); err != nil {
			return fmt.Errorf("failed to move to temp location: %w", err)
		}

		// Recreate parent directory
		if err := os.MkdirAll(flatInstallPath, 0755); err != nil {
			// Try to restore
			_ = os.Rename(tempPath, flatInstallPath)
			return fmt.Errorf("failed to recreate parent directory: %w", err)
		}

		// Move temp to nested location
		if err := os.Rename(tempPath, nestedInstallPath); err != nil {
			// Try to restore
			os.RemoveAll(flatInstallPath)
			_ = os.Rename(tempPath, flatInstallPath)
			return fmt.Errorf("failed to move to nested location: %w", err)
		}

		ri.logger.Info("runtime moved to nested structure", "path", nestedInstallPath)
	}

	if streamer != nil {
		_ = streamer.SendProgress(fmt.Sprintf("✅ Runtime installed at: %s", nestedInstallPath))
	}

	return nil
}

// detectPlatform detects the OS and architecture
// Returns format: ubuntu-amd64, rhel-amd64, amzn-amd64, ubuntu-arm64, etc.
func (ri *RuntimeInstaller) detectPlatform() (string, error) {
	// Detect architecture using uname
	cmd := exec.Command("uname", "-m")
	archOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect architecture: %w", err)
	}

	arch := strings.TrimSpace(string(archOutput))
	archMap := map[string]string{
		"x86_64":  "amd64",
		"amd64":   "amd64",
		"aarch64": "arm64",
		"arm64":   "arm64",
	}
	normalizedArch := archMap[arch]
	if normalizedArch == "" {
		normalizedArch = arch
	}

	// Detect OS distribution
	osRelease, err := ri.readOSRelease()
	if err != nil {
		return "", fmt.Errorf("failed to read os-release: %w", err)
	}

	// Map ID to platform name
	osID := strings.ToLower(osRelease["ID"])
	platformMap := map[string]string{
		"ubuntu":    "ubuntu",
		"debian":    "ubuntu", // Treat Debian like Ubuntu
		"rhel":      "rhel",
		"centos":    "rhel", // Treat CentOS like RHEL
		"rocky":     "rhel", // Treat Rocky like RHEL
		"almalinux": "rhel", // Treat AlmaLinux like RHEL
		"amzn":      "amzn",
		"amazon":    "amzn",
	}

	platformOS := platformMap[osID]
	if platformOS == "" {
		// Fallback to ID from os-release
		platformOS = osID
	}

	platform := fmt.Sprintf("%s-%s", platformOS, normalizedArch)
	return platform, nil
}

// readOSRelease reads /etc/os-release file
func (ri *RuntimeInstaller) readOSRelease() (map[string]string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove quotes
		value = strings.Trim(value, `"'`)
		result[key] = value
	}

	return result, nil
}

// fixRuntimeYmlVersion updates the version field in runtime.yml to match the package version
// This is needed because setup scripts may write the wrong version (e.g., Python version instead of package version)
func (ri *RuntimeInstaller) fixRuntimeYmlVersion(runtimeYmlPath string, correctVersion string) error {
	// Read runtime.yml
	data, err := os.ReadFile(runtimeYmlPath)
	if err != nil {
		return fmt.Errorf("failed to read runtime.yml: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse runtime.yml: %w", err)
	}

	// Update version field
	config["version"] = correctVersion

	// Write back
	newData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal runtime.yml: %w", err)
	}

	if err := os.WriteFile(runtimeYmlPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write runtime.yml: %w", err)
	}

	ri.logger.Info("fixed runtime.yml version", "path", runtimeYmlPath, "version", correctVersion)
	return nil
}
