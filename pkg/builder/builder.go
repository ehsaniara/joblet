//go:build linux

package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Builder orchestrates the runtime build process
type Builder struct {
	runtimesPath string
	logger       BuildLogger
}

// NewBuilder creates a new runtime builder with custom runtimes path and logger
func NewBuilder(runtimesPath string, logger BuildLogger) *Builder {
	if runtimesPath == "" {
		runtimesPath = RuntimesBasePath
	}
	if logger == nil {
		logger = NewBuildLogger(false)
	}
	return &Builder{
		runtimesPath: runtimesPath,
		logger:       logger,
	}
}

// NewBuilderWithDefaults creates a new runtime builder with default settings
func NewBuilderWithDefaults(dryRun, verbose bool) *Builder {
	return &Builder{
		runtimesPath: RuntimesBasePath,
		logger:       NewBuildLogger(verbose),
	}
}

const totalPhases = 14

// Build executes the 14-phase build process from YAML content
func (b *Builder) Build(ctx context.Context, yamlContent string, dryRun bool) (*BuildResult, error) {
	startTime := time.Now()
	result := &BuildResult{
		Phases: make([]PhaseResult, 0, totalPhases),
	}

	// Phase 1: Parse & Validate
	spec, err := b.phase1ParseValidateContent(yamlContent, result)
	if err != nil {
		return result, err
	}

	// Phase 2: Detect Platform
	platform, err := b.phase2DetectPlatform(spec, result)
	if err != nil {
		return result, err
	}

	// Create build context
	runtimeDir := filepath.Join(b.runtimesPath, spec.Name, spec.Version)
	isolatedDir := filepath.Join(runtimeDir, "isolated")

	buildCtx := &BuildContext{
		Spec:        spec,
		Platform:    platform,
		RuntimeDir:  runtimeDir,
		IsolatedDir: isolatedDir,
		DryRun:      dryRun,
		Verbose:     false,
	}

	// Phase 3: Check Disk Space
	if err := b.phase3CheckDiskSpace(buildCtx, result); err != nil {
		return result, err
	}

	// Get language profile
	profile, err := GetLanguageProfile(spec.Base.Language, spec.Base.Version)
	if err != nil {
		return result, fmt.Errorf("failed to get language profile: %w", err)
	}

	// Phase 4: Validate Packages
	if err := b.phase4ValidatePackages(ctx, buildCtx, profile, result); err != nil {
		return result, err
	}

	// Dry run stops here
	if dryRun {
		b.logger.Info("Dry run completed. No changes were made.")
		result.Success = true
		result.Name = spec.Name
		result.Version = spec.Version
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Phase 5: Prepare Directories
	if err := b.phase5PrepareDirectories(buildCtx, result); err != nil {
		return result, err
	}

	// Setup isolated environment for system package installation
	// This ensures packages are installed in an OverlayFS chroot, not on host
	if err := b.setupIsolatedEnvironment(buildCtx); err != nil {
		return result, fmt.Errorf("failed to setup isolated environment: %w", err)
	}
	// Ensure cleanup happens even on error
	defer b.cleanupIsolatedEnvironment(buildCtx)

	// Phase 6: Pre-install Hook
	if err := b.phase6PreInstallHook(ctx, buildCtx, result); err != nil {
		return result, err
	}

	// Phase 7: Install Base (now runs in isolated environment)
	if err := b.phase7InstallBaseIsolated(ctx, buildCtx, profile, result); err != nil {
		return result, err
	}

	// Phase 8: Install Packages
	if err := b.phase8InstallPackages(ctx, buildCtx, result); err != nil {
		return result, err
	}

	// Phase 9: Post-install Hook
	if err := b.phase9PostInstallHook(ctx, buildCtx, result); err != nil {
		return result, err
	}

	// Phase 10: Copy Binaries (from isolated environment)
	if err := b.phase10CopyBinariesFromIsolated(buildCtx, profile, result); err != nil {
		return result, err
	}

	// Phase 11: Copy Libraries (from isolated environment)
	if err := b.phase11CopyLibrariesFromIsolated(buildCtx, profile, result); err != nil {
		return result, err
	}

	// Phase 12: Copy Configuration
	if err := b.phase12CopyConfiguration(buildCtx, result); err != nil {
		return result, err
	}

	// Phase 13: Generate Config
	if err := b.phase13GenerateConfig(buildCtx, result); err != nil {
		return result, err
	}

	// Phase 14: Validate Build
	if err := b.phase14ValidateBuild(buildCtx, result); err != nil {
		return result, err
	}

	result.Success = true
	result.Name = spec.Name
	result.Version = spec.Version
	result.InstallPath = runtimeDir
	result.SizeBytes = result.TotalSize
	result.TotalDuration = time.Since(startTime)

	b.logger.Info("Build completed successfully!")
	b.logger.Info("Runtime: %s@%s", spec.Name, spec.Version)
	b.logger.Info("Location: %s", runtimeDir)
	b.logger.Info("Duration: %s", result.TotalDuration.Round(time.Second))

	return result, nil
}

// BuildFromFile executes the 14-phase build process from a file path
func (b *Builder) BuildFromFile(ctx context.Context, specPath string, dryRun bool) (*BuildResult, error) {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}
	return b.Build(ctx, string(content), dryRun)
}

func (b *Builder) phase1ParseValidateContent(yamlContent string, result *BuildResult) (*RuntimeYAMLSpec, error) {
	phaseStart := time.Now()
	b.logger.Phase(1, totalPhases, PhaseParseValidate.String(), "Parsing and validating runtime specification")

	spec, err := ParseRuntimeYAML([]byte(yamlContent))
	if err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   1,
			Name:    PhaseParseValidate.String(),
			Success: false,
			Error:   err,
		})
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	if err := ValidateSpec(spec); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   1,
			Name:    PhaseParseValidate.String(),
			Success: false,
			Error:   err,
		})
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    1,
		Name:     PhaseParseValidate.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("Parsed %s@%s", spec.Name, spec.Version),
	})

	b.logger.Info("Validated: %s@%s (%s %s)", spec.Name, spec.Version, spec.Base.Language, spec.Base.Version)
	return spec, nil
}

func (b *Builder) phase2DetectPlatform(spec *RuntimeYAMLSpec, result *BuildResult) (*PlatformInfo, error) {
	phaseStart := time.Now()
	b.logger.Phase(2, totalPhases, PhaseDetectPlatform.String(), "Detecting platform")

	platform, err := DetectPlatform()
	if err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   2,
			Name:    PhaseDetectPlatform.String(),
			Success: false,
			Error:   err,
		})
		return nil, fmt.Errorf("platform detection failed: %w", err)
	}

	// Check if platform is supported
	if !platform.IsPlatformSupported(spec.Platforms) {
		err := fmt.Errorf("platform %s is not supported by this runtime", platform.GetPlatformString())
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   2,
			Name:    PhaseDetectPlatform.String(),
			Success: false,
			Error:   err,
		})
		return nil, err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    2,
		Name:     PhaseDetectPlatform.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  platform.GetPlatformString(),
	})

	b.logger.Info("Platform: %s (pkg manager: %s)", platform.GetPlatformString(), platform.PkgManager)
	return platform, nil
}

func (b *Builder) phase3CheckDiskSpace(buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(3, totalPhases, PhaseCheckDiskSpace.String(), "Checking disk space")

	// Check disk space - try RuntimesBasePath first, fall back to "/" if it doesn't exist
	checkPath := filepath.Dir(RuntimesBasePath)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkPath, &stat); err != nil {
		// Fall back to root directory for disk space check (useful in dry-run mode)
		checkPath = "/"
		if err := syscall.Statfs(checkPath, &stat); err != nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   3,
				Name:    PhaseCheckDiskSpace.String(),
				Success: false,
				Error:   err,
			})
			return fmt.Errorf("failed to check disk space: %w", err)
		}
	}

	available := stat.Bavail * uint64(stat.Bsize)
	if available < MinDiskSpaceBytes {
		err := fmt.Errorf("insufficient disk space: %d MB available, need at least %d MB", available/1024/1024, MinDiskSpaceBytes/1024/1024)
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   3,
			Name:    PhaseCheckDiskSpace.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    3,
		Name:     PhaseCheckDiskSpace.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("%d MB available", available/1024/1024),
	})

	b.logger.Info("Disk space: %d MB available", available/1024/1024)
	return nil
}

func (b *Builder) phase4ValidatePackages(ctx context.Context, buildCtx *BuildContext, profile *LanguageProfile, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(4, totalPhases, PhaseValidatePackages.String(), "Validating package availability")

	// Get packages based on package manager
	var packages []string
	switch buildCtx.Platform.PkgManager {
	case "apt":
		packages = profile.AptPackages
	case "yum", "dnf":
		packages = profile.YumPackages
	}

	if err := ValidatePackageAvailability(ctx, buildCtx.Platform, packages, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   4,
			Name:    PhaseValidatePackages.String(),
			Success: false,
			Error:   err,
		})
		return fmt.Errorf("package validation failed: %w", err)
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    4,
		Name:     PhaseValidatePackages.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("%d packages validated", len(packages)),
	})

	b.logger.Info("Validated %d packages", len(packages))
	return nil
}

func (b *Builder) phase5PrepareDirectories(buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(5, totalPhases, PhasePrepareDirectories.String(), "Preparing directories")

	// Create runtime directory structure
	dirs := []string{
		buildCtx.RuntimeDir,
		buildCtx.IsolatedDir,
		filepath.Join(buildCtx.IsolatedDir, "bin"),
		filepath.Join(buildCtx.IsolatedDir, "lib"),
		filepath.Join(buildCtx.IsolatedDir, "lib64"),
		filepath.Join(buildCtx.IsolatedDir, "usr", "bin"),
		filepath.Join(buildCtx.IsolatedDir, "usr", "lib"),
		filepath.Join(buildCtx.IsolatedDir, "usr", "local", "lib"),
		filepath.Join(buildCtx.IsolatedDir, "etc"),
		filepath.Join(buildCtx.IsolatedDir, "tmp"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   5,
				Name:    PhasePrepareDirectories.String(),
				Success: false,
				Error:   err,
			})
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    5,
		Name:     PhasePrepareDirectories.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  buildCtx.RuntimeDir,
	})

	b.logger.Info("Created directory structure: %s", buildCtx.RuntimeDir)
	return nil
}

func (b *Builder) phase6PreInstallHook(ctx context.Context, buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(6, totalPhases, PhasePreInstallHook.String(), "Running pre-install hook")

	if buildCtx.Spec.Hooks == nil || buildCtx.Spec.Hooks.PreInstall == "" {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:    6,
			Name:     PhasePreInstallHook.String(),
			Success:  true,
			Duration: time.Since(phaseStart),
			Message:  "No pre-install hook defined",
		})
		b.logger.Info("No pre-install hook defined, skipping")
		return nil
	}

	if err := ExecutePreInstallHook(ctx, buildCtx, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   6,
			Name:    PhasePreInstallHook.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    6,
		Name:     PhasePreInstallHook.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Completed",
	})

	return nil
}

func (b *Builder) phase8InstallPackages(ctx context.Context, buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(8, totalPhases, PhaseInstallPackages.String(), "Installing language packages")

	// Install pip packages inside the isolated environment
	// This uses the Python/pip installed in phase 7, NOT the host's Python
	if len(buildCtx.Spec.Pip) > 0 {
		if buildCtx.IsolatedEnv == nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   8,
				Name:    PhaseInstallPackages.String(),
				Success: false,
				Error:   fmt.Errorf("isolated environment not available for pip installation"),
			})
			return fmt.Errorf("isolated environment not available for pip installation")
		}

		if err := buildCtx.IsolatedEnv.InstallPipPackagesIsolated(buildCtx.Spec.Pip, buildCtx.Spec.PipOptions, buildCtx.Spec.Base.Version); err != nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   8,
				Name:    PhaseInstallPackages.String(),
				Success: false,
				Error:   err,
			})
			return err
		}
	}

	// Install npm packages inside the isolated environment
	// This uses the npm installed in phase 7, NOT the host's npm
	if len(buildCtx.Spec.Npm) > 0 {
		if buildCtx.IsolatedEnv == nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   8,
				Name:    PhaseInstallPackages.String(),
				Success: false,
				Error:   fmt.Errorf("isolated environment not available for npm installation"),
			})
			return fmt.Errorf("isolated environment not available for npm installation")
		}

		if err := buildCtx.IsolatedEnv.InstallNpmPackagesIsolated(buildCtx.Spec.Npm); err != nil {
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   8,
				Name:    PhaseInstallPackages.String(),
				Success: false,
				Error:   err,
			})
			return err
		}
	}

	totalPackages := len(buildCtx.Spec.Pip) + len(buildCtx.Spec.Npm)
	result.Phases = append(result.Phases, PhaseResult{
		Phase:    8,
		Name:     PhaseInstallPackages.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("Installed %d packages", totalPackages),
	})

	b.logger.Info("Installed %d language packages", totalPackages)
	return nil
}

func (b *Builder) phase9PostInstallHook(ctx context.Context, buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(9, totalPhases, PhasePostInstallHook.String(), "Running post-install hook")

	if buildCtx.Spec.Hooks == nil || buildCtx.Spec.Hooks.PostInstall == "" {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:    9,
			Name:     PhasePostInstallHook.String(),
			Success:  true,
			Duration: time.Since(phaseStart),
			Message:  "No post-install hook defined",
		})
		b.logger.Info("No post-install hook defined, skipping")
		return nil
	}

	if err := ExecutePostInstallHook(ctx, buildCtx, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   9,
			Name:    PhasePostInstallHook.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    9,
		Name:     PhasePostInstallHook.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Completed",
	})

	return nil
}

func (b *Builder) phase12CopyConfiguration(buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(12, totalPhases, PhaseCopyConfiguration.String(), "Copying configuration")

	if err := CopyConfiguration(buildCtx.IsolatedDir, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   12,
			Name:    PhaseCopyConfiguration.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    12,
		Name:     PhaseCopyConfiguration.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Completed",
	})

	return nil
}

func (b *Builder) phase13GenerateConfig(buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(13, totalPhases, PhaseGenerateConfig.String(), "Generating runtime config")

	if err := GenerateRuntimeConfig(buildCtx, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   13,
			Name:    PhaseGenerateConfig.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    13,
		Name:     PhaseGenerateConfig.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Generated runtime.yml",
	})

	return nil
}

func (b *Builder) phase14ValidateBuild(buildCtx *BuildContext, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(14, totalPhases, PhaseValidateBuild.String(), "Validating build")

	// Check that runtime.yml exists
	configPath := filepath.Join(buildCtx.RuntimeDir, "runtime.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err := fmt.Errorf("runtime.yml not found")
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   14,
			Name:    PhaseValidateBuild.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	// Check that isolated directory has content
	isolatedBinDir := filepath.Join(buildCtx.IsolatedDir, "usr", "bin")
	entries, err := os.ReadDir(isolatedBinDir)
	if err != nil || len(entries) == 0 {
		err := fmt.Errorf("no binaries found in isolated directory")
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   14,
			Name:    PhaseValidateBuild.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	// Calculate total size
	var totalSize int64
	var fileCount int
	_ = filepath.Walk(buildCtx.RuntimeDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	result.TotalSize = totalSize
	result.FileCount = fileCount

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    14,
		Name:     PhaseValidateBuild.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("%d files, %d MB", fileCount, totalSize/1024/1024),
	})

	b.logger.Info("Build validated: %d files, %d MB", fileCount, totalSize/1024/1024)
	return nil
}

// setupIsolatedEnvironment creates an OverlayFS-based chroot for isolated package installation
// This ensures system packages are NOT installed on the host
func (b *Builder) setupIsolatedEnvironment(buildCtx *BuildContext) error {
	b.logger.Info("Setting up isolated build environment (OverlayFS)")

	// Create temporary directory for overlay filesystem
	tmpDir, err := os.MkdirTemp("", "joblet-build-overlay-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory for overlay: %w", err)
	}
	buildCtx.IsolationTmpDir = tmpDir

	// Create isolated environment
	isolatedEnv, err := NewIsolatedEnvironment(tmpDir, b.logger)
	if err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to create isolated environment: %w", err)
	}

	// Setup the overlay filesystem
	if err := isolatedEnv.Setup(); err != nil {
		os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to setup isolated environment: %w", err)
	}

	buildCtx.IsolatedEnv = isolatedEnv
	b.logger.Info("Isolated environment ready - host will NOT be modified")
	return nil
}

// cleanupIsolatedEnvironment cleans up the OverlayFS environment
func (b *Builder) cleanupIsolatedEnvironment(buildCtx *BuildContext) {
	if buildCtx.IsolatedEnv != nil {
		b.logger.Debug("Cleaning up isolated environment")
		if err := buildCtx.IsolatedEnv.Cleanup(); err != nil {
			b.logger.Warn("Failed to cleanup isolated environment: %v", err)
		}
		buildCtx.IsolatedEnv = nil
	}
}

// phase7InstallBaseIsolated installs base language packages in the isolated environment
func (b *Builder) phase7InstallBaseIsolated(ctx context.Context, buildCtx *BuildContext, profile *LanguageProfile, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(7, totalPhases, PhaseInstallBase.String(), "Installing base language packages (isolated)")

	if buildCtx.IsolatedEnv == nil {
		return fmt.Errorf("isolated environment not initialized")
	}

	var packages []string
	switch buildCtx.Platform.PkgManager {
	case "apt":
		packages = profile.AptPackages
	case "yum", "dnf":
		packages = profile.YumPackages
	}

	// Install packages in the isolated environment (not on host!)
	if err := buildCtx.IsolatedEnv.InstallPackagesIsolated(buildCtx.Platform.PkgManager, packages); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   7,
			Name:    PhaseInstallBase.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    7,
		Name:     PhaseInstallBase.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  fmt.Sprintf("Installed %d packages (isolated)", len(packages)),
	})

	b.logger.Info("Installed %d base packages in isolated environment", len(packages))
	return nil
}

// phase10CopyBinariesFromIsolated copies binaries from the isolated environment
func (b *Builder) phase10CopyBinariesFromIsolated(buildCtx *BuildContext, profile *LanguageProfile, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(10, totalPhases, PhaseCopyBinaries.String(), "Copying binaries from isolated environment")

	if buildCtx.IsolatedEnv == nil {
		return fmt.Errorf("isolated environment not initialized")
	}

	// Copy binaries from the merged overlay view
	if err := CopyBinariesFromPath(buildCtx.IsolatedEnv.GetMergedPath(""), buildCtx.Platform, profile, buildCtx.IsolatedDir, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   10,
			Name:    PhaseCopyBinaries.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	// For Java runtimes, copy the entire JVM installation from the isolated environment
	if err := CopyJavaRuntimeFromPath(buildCtx.IsolatedEnv.GetMergedPath(""), profile, buildCtx.IsolatedDir, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   10,
			Name:    PhaseCopyBinaries.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	// For Python runtimes, copy the stdlib and site-packages from the isolated environment
	if err := CopyPythonRuntimeFromPath(buildCtx.IsolatedEnv.GetMergedPath(""), profile, buildCtx.IsolatedDir, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   10,
			Name:    PhaseCopyBinaries.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    10,
		Name:     PhaseCopyBinaries.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Completed (from isolated)",
	})

	return nil
}

// phase11CopyLibrariesFromIsolated copies libraries from the isolated environment
func (b *Builder) phase11CopyLibrariesFromIsolated(buildCtx *BuildContext, profile *LanguageProfile, result *BuildResult) error {
	phaseStart := time.Now()
	b.logger.Phase(11, totalPhases, PhaseCopyLibraries.String(), "Copying libraries from isolated environment")

	if buildCtx.IsolatedEnv == nil {
		return fmt.Errorf("isolated environment not initialized")
	}

	// Copy libraries from the merged overlay view
	// Include any additional library patterns specified in runtime.yaml
	extraLibPatterns := buildCtx.Spec.Libraries
	if err := CopyLibrariesFromPath(buildCtx.IsolatedEnv.GetMergedPath(""), buildCtx.Platform, profile, buildCtx.IsolatedDir, extraLibPatterns, b.logger); err != nil {
		result.Phases = append(result.Phases, PhaseResult{
			Phase:   11,
			Name:    PhaseCopyLibraries.String(),
			Success: false,
			Error:   err,
		})
		return err
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase:    11,
		Name:     PhaseCopyLibraries.String(),
		Success:  true,
		Duration: time.Since(phaseStart),
		Message:  "Completed (from isolated)",
	})

	return nil
}
