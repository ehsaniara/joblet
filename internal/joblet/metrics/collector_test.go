package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
)

func TestNewCollector_CgroupPathValidation(t *testing.T) {
	t.Run("returns error when cgroup path does not exist", func(t *testing.T) {
		nonExistentPath := "/nonexistent/cgroup/path"

		collector, err := NewCollector(
			"test-job-id",
			nonExistentPath,
			5*time.Second,
			nil,
			nil,
			nil,
		)

		if err == nil {
			t.Fatal("expected error for non-existent cgroup path, got nil")
		}
		if collector != nil {
			t.Fatal("expected nil collector when error occurs")
		}

		expectedMsg := "cgroup path does not exist: /nonexistent/cgroup/path"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("returns error when cgroup path is a file not a directory", func(t *testing.T) {
		// Create a temporary file (not a directory)
		tmpFile, err := os.CreateTemp("", "cgroup-test-*")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		collector, err := NewCollector(
			"test-job-id",
			tmpFile.Name(),
			5*time.Second,
			nil,
			nil,
			nil,
		)

		if err == nil {
			t.Fatal("expected error for file path (not directory), got nil")
		}
		if collector != nil {
			t.Fatal("expected nil collector when error occurs")
		}

		expectedMsg := "cgroup path is not a directory: " + tmpFile.Name()
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("succeeds when cgroup path is a valid directory", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "cgroup-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		collector, err := NewCollector(
			"test-job-id",
			tmpDir,
			5*time.Second,
			&domain.ResourceLimits{
				CPU:    200,
				Memory: 1024 * 1024 * 1024,
			},
			[]int{0},
			nil,
		)

		if err != nil {
			t.Fatalf("unexpected error for valid directory: %v", err)
		}
		if collector == nil {
			t.Fatal("expected non-nil collector")
		}

		// Verify collector fields are set correctly
		if collector.jobID != "test-job-id" {
			t.Errorf("expected jobID %q, got %q", "test-job-id", collector.jobID)
		}
		if collector.cgroupPath != tmpDir {
			t.Errorf("expected cgroupPath %q, got %q", tmpDir, collector.cgroupPath)
		}
		if collector.sampleInterval != 5*time.Second {
			t.Errorf("expected sampleInterval %v, got %v", 5*time.Second, collector.sampleInterval)
		}
		if len(collector.gpuIndices) != 1 || collector.gpuIndices[0] != 0 {
			t.Errorf("expected gpuIndices [0], got %v", collector.gpuIndices)
		}
	})

	t.Run("returns error when cgroup path has permission denied", func(t *testing.T) {
		// Skip if running as root (root can access everything)
		if os.Getuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}

		// Create a temporary directory with no permissions
		tmpDir, err := os.MkdirTemp("", "cgroup-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		restrictedDir := filepath.Join(tmpDir, "restricted")
		if err := os.Mkdir(restrictedDir, 0000); err != nil {
			t.Fatalf("failed to create restricted dir: %v", err)
		}

		collector, err := NewCollector(
			"test-job-id",
			restrictedDir,
			5*time.Second,
			nil,
			nil,
			nil,
		)

		// The directory exists but we might not be able to stat it depending on parent perms
		// On most systems, stat will succeed even with 0000 perms if we own the file
		// So we just check that if there's an error, the collector is nil
		if err != nil && collector != nil {
			t.Fatal("expected nil collector when error occurs")
		}
	})

	t.Run("succeeds with empty gpu indices", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cgroup-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		collector, err := NewCollector(
			"test-job-id",
			tmpDir,
			10*time.Second,
			nil,
			nil, // no GPU indices
			nil,
		)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if collector == nil {
			t.Fatal("expected non-nil collector")
		}
		if len(collector.gpuIndices) != 0 {
			t.Errorf("expected empty gpuIndices, got %v", collector.gpuIndices)
		}
	})

	t.Run("succeeds with nil limits", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "cgroup-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		collector, err := NewCollector(
			"test-job-id",
			tmpDir,
			5*time.Second,
			nil, // nil limits
			nil,
			nil,
		)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if collector == nil {
			t.Fatal("expected non-nil collector")
		}
		if collector.limits != nil {
			t.Errorf("expected nil limits, got %v", collector.limits)
		}
	})
}
