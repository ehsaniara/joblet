package version

import (
	"fmt"
	"runtime"
)

var (
	// These values are set at build time via -ldflags
	Version   = "dev"     // Version is the semantic version (e.g., v4.3.3)
	GitCommit = "unknown" // GitCommit is the git commit hash
	GitTag    = "unknown" // GitTag is the git tag if available
	BuildDate = "unknown" // BuildDate is when the binary was built
	Component = "unknown" // Component identifies which component this is (rnx, joblet, api)
)

// BuildInfo represents the complete build information
type BuildInfo struct {
	Version      string `json:"version"`
	GitCommit    string `json:"git_commit"`
	GitTag       string `json:"git_tag"`
	BuildDate    string `json:"build_date"`
	Component    string `json:"component"`
	GoVersion    string `json:"go_version"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
}

// GetBuildInfo returns complete build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:      Version,
		GitCommit:    GitCommit,
		GitTag:       GitTag,
		BuildDate:    BuildDate,
		Component:    Component,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// GetVersion returns the version string
func GetVersion() string {
	if Version != "dev" {
		return Version
	}
	if GitTag != "unknown" && GitTag != "" {
		return GitTag
	}
	return fmt.Sprintf("dev-%s", GitCommit)
}

// GetShortVersion returns a concise version string for display
func GetShortVersion() string {
	version := GetVersion()
	if GitCommit != "unknown" && len(GitCommit) >= 7 {
		return fmt.Sprintf("%s (%s)", version, GitCommit[:7])
	}
	return version
}

// GetLongVersion returns detailed version information for --version output
func GetLongVersion() string {
	info := GetBuildInfo()

	var output string
	output += fmt.Sprintf("%s version %s\n", info.Component, GetShortVersion())

	if info.BuildDate != "unknown" {
		output += fmt.Sprintf("Built: %s\n", info.BuildDate)
	}

	if info.GitCommit != "unknown" {
		output += fmt.Sprintf("Commit: %s\n", info.GitCommit)
	}

	output += fmt.Sprintf("Go: %s\n", info.GoVersion)
	output += fmt.Sprintf("Platform: %s/%s\n", info.Platform, info.Architecture)

	return output
}
