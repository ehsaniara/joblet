package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUploadPath(t *testing.T) {
	base := "/opt/joblet/jobs/abc/work"

	// Paths that must be rejected as workspace escapes (issue #1 path traversal)
	rejected := []string{
		"../../../../etc/cron.d/x",
		"../../../../../opt/joblet/bin/joblet",
		"../outside",
		"/etc/passwd",
		"a/../../../etc/x",
		"",
	}
	for _, p := range rejected {
		if got, err := ResolveUploadPath(base, p); err == nil {
			t.Errorf("ResolveUploadPath(%q) = %q, want error (escapes workspace)", p, got)
		}
	}

	// Legitimate in-workspace paths must resolve within base
	allowed := map[string]string{
		"script.py":        filepath.Join(base, "script.py"),
		"src/main.go":      filepath.Join(base, "src/main.go"),
		"a/../b/file.txt":  filepath.Join(base, "b/file.txt"), // cleans to within base
		"./data.csv":       filepath.Join(base, "data.csv"),
		"nested/deep/x.sh": filepath.Join(base, "nested/deep/x.sh"),
	}
	for in, want := range allowed {
		got, err := ResolveUploadPath(base, in)
		if err != nil {
			t.Errorf("ResolveUploadPath(%q) unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ResolveUploadPath(%q) = %q, want %q", in, got, want)
		}
		// Invariant: the result never escapes base
		rel, err := filepath.Rel(base, got)
		if err != nil || rel == ".." || len(rel) >= 2 && rel[:2] == ".." {
			t.Errorf("ResolveUploadPath(%q) = %q escaped base %q (rel=%q)", in, got, base, rel)
		}
	}
}

func TestSanitizeUploadMode(t *testing.T) {
	// setuid (04000), setgid (02000), sticky (01000) must be stripped
	cases := map[uint32]os.FileMode{
		0o755:      0o755,
		0o644:      0o644,
		0o4755:     0o755, // setuid stripped
		0o2755:     0o755, // setgid stripped
		0o1777:     0o777, // sticky stripped
		0o104755:   0o755, // file-type + setuid bits stripped
		0xFFFFFFFF: 0o777, // garbage clamps to perm bits
	}
	for in, want := range cases {
		if got := SanitizeUploadMode(in); got != want {
			t.Errorf("SanitizeUploadMode(%o) = %o, want %o", in, got, want)
		}
	}
}
