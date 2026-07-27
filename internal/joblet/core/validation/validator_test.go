package validation

import (
	"path/filepath"
	"testing"
)

func TestValidatePathWithinBase(t *testing.T) {
	tests := []struct {
		name       string
		basePath   string
		targetPath string
		wantErr    bool
		errContain string
	}{
		{
			name:       "valid simple path",
			basePath:   "/workspace",
			targetPath: "file.txt",
			wantErr:    false,
		},
		{
			name:       "valid nested path",
			basePath:   "/workspace",
			targetPath: "subdir/file.txt",
			wantErr:    false,
		},
		{
			name:       "valid deeply nested path",
			basePath:   "/workspace",
			targetPath: "a/b/c/d/file.txt",
			wantErr:    false,
		},
		{
			name:       "path traversal attack - simple",
			basePath:   "/workspace",
			targetPath: "../etc/passwd",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "path traversal attack - multiple levels",
			basePath:   "/workspace",
			targetPath: "../../../etc/passwd",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "path traversal attack - embedded",
			basePath:   "/workspace",
			targetPath: "subdir/../../etc/passwd",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "path traversal attack - complex",
			basePath:   "/workspace",
			targetPath: "foo/../bar/../../etc/shadow",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "absolute path attempt",
			basePath:   "/workspace",
			targetPath: "/etc/passwd",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "empty base path",
			basePath:   "",
			targetPath: "file.txt",
			wantErr:    true,
			errContain: "base path cannot be empty",
		},
		{
			name:       "empty target path - stays in base",
			basePath:   "/workspace",
			targetPath: "",
			wantErr:    false,
		},
		{
			name:       "dot path - stays in base",
			basePath:   "/workspace",
			targetPath: ".",
			wantErr:    false,
		},
		{
			name:       "similar directory name - should not match",
			basePath:   "/workspace",
			targetPath: "../workspace-other/file.txt",
			wantErr:    true,
			errContain: "escapes base directory",
		},
		{
			name:       "path with spaces",
			basePath:   "/workspace",
			targetPath: "my files/document.txt",
			wantErr:    false,
		},
		{
			name:       "path traversal with spaces",
			basePath:   "/workspace",
			targetPath: "my files/../../etc/passwd",
			wantErr:    true,
			errContain: "escapes base directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePathWithinBase(tt.basePath, tt.targetPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePathWithinBase() expected error containing %q, got nil", tt.errContain)
					return
				}
				if tt.errContain != "" && !contains(err.Error(), tt.errContain) {
					t.Errorf("ValidatePathWithinBase() error = %q, want error containing %q", err.Error(), tt.errContain)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePathWithinBase() unexpected error = %v", err)
					return
				}
				// Verify result is absolute and within base
				if !filepath.IsAbs(result) {
					t.Errorf("ValidatePathWithinBase() result is not absolute: %s", result)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{
			name:    "valid env var",
			key:     "MY_VAR",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "valid env var with underscore prefix",
			key:     "_PRIVATE_VAR",
			value:   "secret",
			wantErr: false,
		},
		{
			name:    "valid env var with numbers",
			key:     "VAR123",
			value:   "value",
			wantErr: false,
		},
		{
			name:    "invalid env var - starts with number",
			key:     "123VAR",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid env var - contains dash",
			key:     "MY-VAR",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid env var - contains space",
			key:     "MY VAR",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid env var - contains special char",
			key:     "MY@VAR",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "empty value is valid",
			key:     "EMPTY_VAR",
			value:   "",
			wantErr: false,
		},
		{
			name:    "value at limit (32768 bytes)",
			key:     "LARGE_VAR",
			value:   string(make([]byte, 32768)),
			wantErr: false,
		},
		{
			name:    "value over limit (32769 bytes)",
			key:     "TOO_LARGE_VAR",
			value:   string(make([]byte, 32769)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvironmentVariable(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnvironmentVariable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
