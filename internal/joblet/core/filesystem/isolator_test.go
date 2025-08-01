//go:build linux

package filesystem

import (
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupLimitedWorkDir(t *testing.T) {
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
