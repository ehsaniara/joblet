//go:build linux

package builder

import (
	"time"
)

// RuntimeYAMLSpec represents the parsed runtime.yaml specification
type RuntimeYAMLSpec struct {
	SchemaVersion string            `yaml:"schema_version"` // Must be "1.0"
	Name          string            `yaml:"name"`           // e.g., "python-3.11-ml" (lowercase, hyphens, dots, max 64)
	Version       string            `yaml:"version"`        // Semantic version: X.Y.Z
	Description   string            `yaml:"description"`    // Max 256 chars
	Base          BaseConfig        `yaml:"base"`
	Pip           []string          `yaml:"pip,omitempty"`
	PipOptions    string            `yaml:"pip_options,omitempty"` // e.g., "--index-url https://..."
	Npm           []string          `yaml:"npm,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Requirements  *Requirements     `yaml:"requirements,omitempty"`
	Platforms     []string          `yaml:"platforms,omitempty"` // e.g., ["ubuntu-amd64", "rhel-arm64"]
	Hooks         *Hooks            `yaml:"hooks,omitempty"`
	Libraries     []string          `yaml:"libraries,omitempty"` // Additional library patterns to copy, e.g., ["libcustom*", "libopenblas*"]
}

// BaseConfig defines the base language and version
type BaseConfig struct {
	Language string `yaml:"language"` // python | java | node | go | rust
	Version  string `yaml:"version"`  // e.g., "3.11", "21", "20"
}

// Requirements specifies runtime requirements
type Requirements struct {
	GPU         bool   `yaml:"gpu,omitempty"`
	CUDAVersion string `yaml:"cuda_version,omitempty"`
	MinMemory   string `yaml:"min_memory,omitempty"` // e.g., "256MB", "2GB"
}

// Hooks defines pre and post install hooks
type Hooks struct {
	Timeout     string `yaml:"timeout,omitempty"`      // e.g., "30m", defaults to 20m
	PreInstall  string `yaml:"pre_install,omitempty"`  // Script to run before system packages
	PostInstall string `yaml:"post_install,omitempty"` // Script to run after all packages
}

// PlatformInfo contains detected platform information
type PlatformInfo struct {
	Distro     string // ubuntu, rhel, amzn
	Arch       string // amd64, arm64
	PkgManager string // apt, yum, dnf
	LibPath    string // /lib/x86_64-linux-gnu, /lib64
}

// BuildContext holds state during build
type BuildContext struct {
	Spec        *RuntimeYAMLSpec
	Platform    *PlatformInfo
	RuntimeDir  string // /opt/joblet/runtimes/{name}/{version}
	IsolatedDir string // /opt/joblet/runtimes/{name}/{version}/isolated
	DryRun      bool
	Verbose     bool

	// Isolation
	IsolatedEnv     *IsolatedEnvironment // OverlayFS isolated environment for package installation
	IsolationTmpDir string               // Temporary directory for overlay filesystem
}

// BuildResult contains build outcome
type BuildResult struct {
	Success       bool
	Name          string
	Version       string
	InstallPath   string
	FileCount     int
	TotalSize     int64
	SizeBytes     int64
	TotalDuration time.Duration
	Phases        []PhaseResult
	Error         error
}

// PhaseResult contains result of a single build phase
type PhaseResult struct {
	Phase    int
	Name     string
	Success  bool
	Duration time.Duration
	Message  string
	Error    error
}

// BuildPhase represents a build phase
type BuildPhase int

const (
	PhaseParseValidate BuildPhase = iota + 1
	PhaseDetectPlatform
	PhaseCheckDiskSpace
	PhaseValidatePackages
	PhasePrepareDirectories
	PhasePreInstallHook
	PhaseInstallBase
	PhaseInstallPackages
	PhasePostInstallHook
	PhaseCopyBinaries
	PhaseCopyLibraries
	PhaseCopyConfiguration
	PhaseGenerateConfig
	PhaseValidateBuild
)

// String returns the phase name
func (p BuildPhase) String() string {
	names := map[BuildPhase]string{
		PhaseParseValidate:      "Parse & Validate",
		PhaseDetectPlatform:     "Detect Platform",
		PhaseCheckDiskSpace:     "Check Disk Space",
		PhaseValidatePackages:   "Validate Packages",
		PhasePrepareDirectories: "Prepare Directories",
		PhasePreInstallHook:     "Pre-install Hook",
		PhaseInstallBase:        "Install Base",
		PhaseInstallPackages:    "Install Packages",
		PhasePostInstallHook:    "Post-install Hook",
		PhaseCopyBinaries:       "Copy Binaries",
		PhaseCopyLibraries:      "Copy Libraries",
		PhaseCopyConfiguration:  "Copy Configuration",
		PhaseGenerateConfig:     "Generate Config",
		PhaseValidateBuild:      "Validate Build",
	}
	if name, ok := names[p]; ok {
		return name
	}
	return "Unknown"
}

// Supported languages
var SupportedLanguages = []string{"python", "java", "node", "go", "rust"}

// Supported platforms
var SupportedPlatforms = []string{
	"ubuntu-amd64",
	"ubuntu-arm64",
	"rhel-amd64",
	"rhel-arm64",
	"amzn-amd64",
	"amzn-arm64",
}

// Default hook timeout
const DefaultHookTimeout = 20 * time.Minute

// RuntimesBasePath is the base path for all runtimes
const RuntimesBasePath = "/opt/joblet/runtimes"

// MinDiskSpaceBytes is the minimum free disk space required (1GB)
const MinDiskSpaceBytes = 1 * 1024 * 1024 * 1024

// LanguageProfile defines packages and binaries for a language
type LanguageProfile struct {
	Language        string
	Version         string
	AptPackages     []string          // Packages for apt-based systems
	YumPackages     []string          // Packages for yum/dnf-based systems
	Binaries        []string          // Required binaries to copy
	LibraryPatterns []string          // Glob patterns for libraries
	Environment     map[string]string // Environment variables to set
}
