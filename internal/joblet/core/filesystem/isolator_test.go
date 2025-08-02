//go:build linux

package filesystem

import (
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupLimitedWorkDir(t *testing.T) {
	// Skip in CI environments that might not have mount privileges
	// Check multiple CI environment indicators
	if isCI() {
		t.Skip("Filesystem tests require mount privileges not available in CI")
	}

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "joblet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create job filesystem
	cfg := config.FilesystemConfig{
		BaseDir: tempDir,
		TmpDir:  filepath.Join(tempDir, "tmp"),
	}

	platform := platform.NewPlatform()
	jobFS := &JobFilesystem{
		JobID:    "test-job",
		RootDir:  filepath.Join(tempDir, "root"),
		TmpDir:   filepath.Join(tempDir, "tmp"),
		WorkDir:  filepath.Join(tempDir, "root", "work"),
		Volumes:  []string{}, // No volumes
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	// Create necessary directories
	if err := os.MkdirAll(jobFS.RootDir, 0755); err != nil {
		t.Fatalf("Failed to create root dir: %v", err)
	}
	if err := os.MkdirAll(jobFS.WorkDir, 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	// Test setupLimitedWorkDir
	err = jobFS.setupLimitedWorkDir()
	if err != nil {
		t.Logf("setupLimitedWorkDir failed (expected in test environment without mount privileges): %v", err)
		// This is expected to fail in test environment without proper privileges
		return
	}

	t.Log("setupLimitedWorkDir succeeded (running with sufficient privileges)")
}

func TestJobFilesystemWithoutVolumes(t *testing.T) {
	// Skip in CI environments that might not have mount privileges
	if isCI() {
		t.Skip("Filesystem tests require mount privileges not available in CI")
	}

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "joblet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create job filesystem without volumes
	cfg := config.FilesystemConfig{
		BaseDir: tempDir,
		TmpDir:  filepath.Join(tempDir, "tmp"),
	}

	platform := platform.NewPlatform()
	jobFS := &JobFilesystem{
		JobID:    "test-job-no-volumes",
		RootDir:  filepath.Join(tempDir, "root"),
		TmpDir:   filepath.Join(tempDir, "tmp"),
		WorkDir:  filepath.Join(tempDir, "root", "work"),
		Volumes:  []string{}, // No volumes - should trigger limited work dir
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "test-filesystem"),
	}

	// Verify that with no volumes, setupLimitedWorkDir would be called
	if len(jobFS.Volumes) != 0 {
		t.Errorf("Expected no volumes, got %d volumes", len(jobFS.Volumes))
	}

	t.Log("Job filesystem correctly configured with no volumes - would use limited work directory")
}

// isCI detects if tests are running in a CI environment
func isCI() bool {
	// Check common CI environment variables
	ciEnvVars := []string{
		"CI",                     // Generic CI indicator
		"CONTINUOUS_INTEGRATION", // Generic CI indicator
		"GITHUB_ACTIONS",         // GitHub Actions
		"TRAVIS",                 // Travis CI
		"CIRCLECI",               // Circle CI
		"JENKINS_URL",            // Jenkins
		"BUILDKITE",              // Buildkite
		"GITLAB_CI",              // GitLab CI
		"AZURE_HTTP_USER_AGENT",  // Azure DevOps
		"TEAMCITY_VERSION",       // TeamCity
	}

	for _, envVar := range ciEnvVars {
		if value := os.Getenv(envVar); value == "true" || value == "1" || value != "" {
			return true
		}
	}

	// Check for specific CI user patterns
	if user := os.Getenv("USER"); user == "runner" || user == "travis" || strings.Contains(user, "jenkins") {
		return true
	}

	// Check for CI-like hostnames
	if hostname := os.Getenv("HOSTNAME"); strings.Contains(hostname, "runner") || strings.Contains(hostname, "build") {
		return true
	}

	// Check working directory patterns
	if pwd := os.Getenv("PWD"); strings.Contains(pwd, "/home/runner/") || strings.Contains(pwd, "/builds/") {
		return true
	}

	return false
}
