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

func TestValidateName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		resourceType string
		maxLength    int
		wantErr      bool
	}{
		{
			name:         "valid name",
			resourceName: "my_resource",
			resourceType: "network",
			maxLength:    64,
			wantErr:      false,
		},
		{
			name:         "valid name with dash",
			resourceName: "my-resource",
			resourceType: "volume",
			maxLength:    64,
			wantErr:      false,
		},
		{
			name:         "empty name",
			resourceName: "",
			resourceType: "job",
			maxLength:    64,
			wantErr:      true,
		},
		{
			name:         "name too long",
			resourceName: "verylongnamethatshouldexceedthemaximumallowedlengthforthisresource",
			resourceType: "job",
			maxLength:    20,
			wantErr:      true,
		},
		{
			name:         "name starts with number",
			resourceName: "1invalid",
			resourceType: "network",
			maxLength:    64,
			wantErr:      true,
		},
		{
			name:         "name with invalid chars",
			resourceName: "invalid@name",
			resourceType: "volume",
			maxLength:    64,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.resourceName, tt.resourceType, tt.maxLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsDangerousPatterns(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "safe value",
			value: "hello world",
			want:  false,
		},
		{
			name:  "command substitution dollar",
			value: "$(rm -rf /)",
			want:  true,
		},
		{
			name:  "command substitution backtick",
			value: "`rm -rf /`",
			want:  true,
		},
		{
			name:  "path traversal",
			value: "../../../etc/passwd",
			want:  true,
		},
		{
			name:  "rm -rf command",
			value: "rm -rf /",
			want:  true,
		},
		{
			name:  "shadow file reference",
			value: "/etc/shadow",
			want:  true,
		},
		{
			name:  "passwd reference",
			value: "cat passwd",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsDangerousPatterns(tt.value); got != tt.want {
				t.Errorf("ContainsDangerousPatterns() = %v, want %v", got, tt.want)
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
