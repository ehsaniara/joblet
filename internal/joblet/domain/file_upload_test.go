package domain

import (
	"os"
	"testing"
)

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
