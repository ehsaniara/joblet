package execution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// TestProcessUploadsRejectsTraversal verifies that a client-supplied "../"
// upload path is rejected before any write and nothing lands outside the
// workspace. processUploads runs as root on the host before chroot, so an
// escaping path would be an arbitrary host file-write.
func TestProcessUploadsRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "jobs", "abc", "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "escaped")

	es := &EnvironmentService{logger: logger.New()}

	// Traversal escape must be rejected and must not write the sentinel.
	err := es.processUploads([]domain.FileUpload{{
		Path:    "../../../escaped",
		Content: []byte("pwned"),
		Mode:    0o644,
	}}, workDir)
	if err == nil {
		t.Fatal("processUploads accepted a traversal path; expected rejection")
	}
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Fatalf("traversal wrote outside the workspace at %s", sentinel)
	}

	// A legitimate in-workspace upload still works and lands inside workDir.
	if err := es.processUploads([]domain.FileUpload{{
		Path:    "src/main.go",
		Content: []byte("package main"),
		Mode:    0o644,
	}}, workDir); err != nil {
		t.Fatalf("processUploads rejected a valid upload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "src", "main.go")); err != nil {
		t.Fatalf("valid upload was not written into the workspace: %v", err)
	}

	// Client setuid bit must be stripped.
	if err := es.processUploads([]domain.FileUpload{{
		Path:    "tool",
		Content: []byte("x"),
		Mode:    0o4755,
	}}, workDir); err != nil {
		t.Fatalf("processUploads rejected a valid upload: %v", err)
	}
	info, err := os.Stat(filepath.Join(workDir, "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid != 0 {
		t.Errorf("setuid bit survived upload: mode=%v", info.Mode())
	}
}
