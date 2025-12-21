//go:build linux

package builder

import (
	"testing"
)

func TestGetLanguageProfile_Python(t *testing.T) {
	tests := []struct {
		version         string
		expectedPkgPart string
	}{
		{"3.11", "python3.11"},
		{"3.12", "python3.12"},
		{"3.9", "python3.9"},
	}

	for _, tt := range tests {
		t.Run("python-"+tt.version, func(t *testing.T) {
			profile, err := GetLanguageProfile("python", tt.version)
			if err != nil {
				t.Fatalf("GetLanguageProfile failed: %v", err)
			}

			if profile.Language != "python" {
				t.Errorf("expected language 'python', got %q", profile.Language)
			}

			if profile.Version != tt.version {
				t.Errorf("expected version %q, got %q", tt.version, profile.Version)
			}

			// Check apt packages contain the expected Python package
			found := false
			for _, pkg := range profile.AptPackages {
				if pkg == tt.expectedPkgPart {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected apt packages to contain %q, got %v", tt.expectedPkgPart, profile.AptPackages)
			}

			// Check binaries
			if len(profile.Binaries) == 0 {
				t.Error("expected binaries to be set")
			}

			// Check environment has PYTHONUNBUFFERED
			if _, ok := profile.Environment["PYTHONUNBUFFERED"]; !ok {
				t.Error("expected PYTHONUNBUFFERED environment variable")
			}
		})
	}
}

func TestGetLanguageProfile_Java(t *testing.T) {
	tests := []struct {
		version         string
		expectedPkgPart string
	}{
		{"17", "openjdk-17-jdk"},
		{"21", "openjdk-21-jdk"},
		{"11", "openjdk-11-jdk"},
	}

	for _, tt := range tests {
		t.Run("java-"+tt.version, func(t *testing.T) {
			profile, err := GetLanguageProfile("java", tt.version)
			if err != nil {
				t.Fatalf("GetLanguageProfile failed: %v", err)
			}

			if profile.Language != "java" {
				t.Errorf("expected language 'java', got %q", profile.Language)
			}

			// Check apt packages contain the expected JDK package
			found := false
			for _, pkg := range profile.AptPackages {
				if pkg == tt.expectedPkgPart {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected apt packages to contain %q, got %v", tt.expectedPkgPart, profile.AptPackages)
			}

			// Check binaries include java and javac
			hasJava := false
			hasJavac := false
			for _, bin := range profile.Binaries {
				if bin == "java" {
					hasJava = true
				}
				if bin == "javac" {
					hasJavac = true
				}
			}
			if !hasJava {
				t.Error("expected binaries to include 'java'")
			}
			if !hasJavac {
				t.Error("expected binaries to include 'javac'")
			}

			// Check JAVA_HOME is set
			if _, ok := profile.Environment["JAVA_HOME"]; !ok {
				t.Error("expected JAVA_HOME environment variable")
			}
		})
	}
}

func TestGetLanguageProfile_Node(t *testing.T) {
	profile, err := GetLanguageProfile("node", "20")
	if err != nil {
		t.Fatalf("GetLanguageProfile failed: %v", err)
	}

	if profile.Language != "node" {
		t.Errorf("expected language 'node', got %q", profile.Language)
	}

	// Check binaries include node and npm
	hasNode := false
	hasNpm := false
	for _, bin := range profile.Binaries {
		if bin == "node" {
			hasNode = true
		}
		if bin == "npm" {
			hasNpm = true
		}
	}
	if !hasNode {
		t.Error("expected binaries to include 'node'")
	}
	if !hasNpm {
		t.Error("expected binaries to include 'npm'")
	}

	// Check NODE_ENV is set
	if val, ok := profile.Environment["NODE_ENV"]; !ok || val != "production" {
		t.Error("expected NODE_ENV=production environment variable")
	}
}

func TestGetLanguageProfile_Go(t *testing.T) {
	profile, err := GetLanguageProfile("go", "1.21")
	if err != nil {
		t.Fatalf("GetLanguageProfile failed: %v", err)
	}

	if profile.Language != "go" {
		t.Errorf("expected language 'go', got %q", profile.Language)
	}

	// Check binaries include go
	hasGo := false
	for _, bin := range profile.Binaries {
		if bin == "go" {
			hasGo = true
			break
		}
	}
	if !hasGo {
		t.Error("expected binaries to include 'go'")
	}

	// Check GOPATH is set
	if _, ok := profile.Environment["GOPATH"]; !ok {
		t.Error("expected GOPATH environment variable")
	}
}

func TestGetLanguageProfile_Rust(t *testing.T) {
	profile, err := GetLanguageProfile("rust", "1.75")
	if err != nil {
		t.Fatalf("GetLanguageProfile failed: %v", err)
	}

	if profile.Language != "rust" {
		t.Errorf("expected language 'rust', got %q", profile.Language)
	}

	// Check binaries include rustc and cargo
	hasRustc := false
	hasCargo := false
	for _, bin := range profile.Binaries {
		if bin == "rustc" {
			hasRustc = true
		}
		if bin == "cargo" {
			hasCargo = true
		}
	}
	if !hasRustc {
		t.Error("expected binaries to include 'rustc'")
	}
	if !hasCargo {
		t.Error("expected binaries to include 'cargo'")
	}

	// Check CARGO_HOME is set
	if _, ok := profile.Environment["CARGO_HOME"]; !ok {
		t.Error("expected CARGO_HOME environment variable")
	}
}

func TestGetLanguageProfile_UnsupportedLanguage(t *testing.T) {
	_, err := GetLanguageProfile("cobol", "85")
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestLanguageProfile_HasRequiredFields(t *testing.T) {
	languages := []struct {
		lang    string
		version string
	}{
		{"python", "3.11"},
		{"java", "21"},
		{"node", "20"},
		{"go", "1.21"},
		{"rust", "1.75"},
	}

	for _, l := range languages {
		t.Run(l.lang, func(t *testing.T) {
			profile, err := GetLanguageProfile(l.lang, l.version)
			if err != nil {
				t.Fatalf("GetLanguageProfile failed: %v", err)
			}

			if len(profile.AptPackages) == 0 {
				t.Error("expected AptPackages to be non-empty")
			}

			if len(profile.YumPackages) == 0 {
				t.Error("expected YumPackages to be non-empty")
			}

			if len(profile.Binaries) == 0 {
				t.Error("expected Binaries to be non-empty")
			}

			if profile.Environment == nil {
				t.Error("expected Environment to be non-nil")
			}
		})
	}
}
