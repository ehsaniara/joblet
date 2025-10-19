package runtime

import (
	"testing"
)

func TestParseRuntimeSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantVersion string
		wantErr     bool
	}{
		// Valid cases with explicit version
		{
			name:        "version specified",
			input:       "python-3.11-ml@1.0.0",
			wantName:    "python-3.11-ml",
			wantVersion: "1.0.0",
			wantErr:     false,
		},
		{
			name:        "explicit latest",
			input:       "python-3.11-ml@latest",
			wantName:    "python-3.11-ml",
			wantVersion: "latest",
			wantErr:     false,
		},
		{
			name:        "no version defaults to latest",
			input:       "python-3.11-ml",
			wantName:    "python-3.11-ml",
			wantVersion: "latest",
			wantErr:     false,
		},
		{
			name:        "openjdk with version",
			input:       "openjdk-21@1.0.0",
			wantName:    "openjdk-21",
			wantVersion: "1.0.0",
			wantErr:     false,
		},
		{
			name:        "semver with prerelease",
			input:       "python-3.11-ml@1.0.0-beta.1",
			wantName:    "python-3.11-ml",
			wantVersion: "1.0.0-beta.1",
			wantErr:     false,
		},
		{
			name:        "semver with build metadata",
			input:       "python-3.11-ml@1.0.0+build.123",
			wantName:    "python-3.11-ml",
			wantVersion: "1.0.0+build.123",
			wantErr:     false,
		},
		{
			name:        "semver with prerelease and build",
			input:       "python-3.11-ml@1.0.0-rc.1+build.123",
			wantName:    "python-3.11-ml",
			wantVersion: "1.0.0-rc.1+build.123",
			wantErr:     false,
		},
		{
			name:        "runtime with dots in name",
			input:       "python-3.11-ml@2.0.0",
			wantName:    "python-3.11-ml",
			wantVersion: "2.0.0",
			wantErr:     false,
		},

		// Invalid cases
		{
			name:    "empty spec",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only @",
			input:   "@",
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   "@1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid version format",
			input:   "python-3.11-ml@v1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid version format - no patch",
			input:   "python-3.11-ml@1.0",
			wantErr: true,
		},
		{
			name:    "invalid name - starts with hyphen",
			input:   "-python@1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid name - special chars",
			input:   "python_ml@1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRuntimeSpec(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRuntimeSpec(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRuntimeSpec(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.Name != tt.wantName {
				t.Errorf("ParseRuntimeSpec(%q).Name = %q, want %q", tt.input, got.Name, tt.wantName)
			}

			if got.Version != tt.wantVersion {
				t.Errorf("ParseRuntimeSpec(%q).Version = %q, want %q", tt.input, got.Version, tt.wantVersion)
			}

			if got.Original != tt.input {
				t.Errorf("ParseRuntimeSpec(%q).Original = %q, want %q", tt.input, got.Original, tt.input)
			}
		})
	}
}

func TestRuntimeSpec_String(t *testing.T) {
	tests := []struct {
		name    string
		spec    *RuntimeSpec
		want    string
	}{
		{
			name: "with version",
			spec: &RuntimeSpec{
				Name:    "python-3.11-ml",
				Version: "1.0.0",
			},
			want: "python-3.11-ml@1.0.0",
		},
		{
			name: "with latest",
			spec: &RuntimeSpec{
				Name:    "python-3.11-ml",
				Version: "latest",
			},
			want: "python-3.11-ml@latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.String()
			if got != tt.want {
				t.Errorf("RuntimeSpec.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeSpec_IsLatest(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "latest keyword",
			version: "latest",
			want:    true,
		},
		{
			name:    "empty defaults to latest",
			version: "",
			want:    true,
		},
		{
			name:    "specific version",
			version: "1.0.0",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &RuntimeSpec{
				Name:    "test",
				Version: tt.version,
			}
			got := spec.IsLatest()
			if got != tt.want {
				t.Errorf("RuntimeSpec.IsLatest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuntimeSpec_FullName(t *testing.T) {
	tests := []struct {
		name    string
		spec    *RuntimeSpec
		want    string
	}{
		{
			name: "with version",
			spec: &RuntimeSpec{
				Name:    "python-3.11-ml",
				Version: "1.0.0",
			},
			want: "python-3.11-ml-1.0.0",
		},
		{
			name: "with latest",
			spec: &RuntimeSpec{
				Name:    "python-3.11-ml",
				Version: "latest",
			},
			want: "python-3.11-ml-latest",
		},
		{
			name: "empty version defaults to latest",
			spec: &RuntimeSpec{
				Name:    "python-3.11-ml",
				Version: "",
			},
			want: "python-3.11-ml-latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.FullName()
			if got != tt.want {
				t.Errorf("RuntimeSpec.FullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMustParseRuntimeSpec(t *testing.T) {
	// Valid case - should not panic
	t.Run("valid spec", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MustParseRuntimeSpec panicked unexpectedly: %v", r)
			}
		}()
		spec := MustParseRuntimeSpec("python-3.11-ml@1.0.0")
		if spec.Name != "python-3.11-ml" {
			t.Errorf("MustParseRuntimeSpec returned wrong name: %q", spec.Name)
		}
	})

	// Invalid case - should panic
	t.Run("invalid spec panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("MustParseRuntimeSpec should have panicked")
			}
		}()
		MustParseRuntimeSpec("@invalid")
	})
}
