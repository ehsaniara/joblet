//go:build linux

package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMounts_EmptyDir(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "generator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mounts := generateMounts(tmpDir)
	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts for empty dir, got %d", len(mounts))
	}
}

func TestGenerateMounts_WithDirs(t *testing.T) {
	// Create a temp directory with some subdirs
	tmpDir, err := os.MkdirTemp("", "generator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some directories that should be mounted
	dirsToCreate := []string{
		"usr/local/bin",
		"usr/bin",
		"lib",
		"etc/ssl",
	}

	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	mounts := generateMounts(tmpDir)

	if len(mounts) != 4 {
		t.Errorf("expected 4 mounts, got %d", len(mounts))
	}

	// Verify mount properties
	for _, m := range mounts {
		if m.Source == "" {
			t.Error("mount source should not be empty")
		}
		if m.Target == "" {
			t.Error("mount target should not be empty")
		}
		// All standard mounts except /tmp should be read-only
		if m.Target == "/tmp" || m.Target == "/var/tmp" {
			if m.ReadOnly {
				t.Errorf("mount %s should be read-write", m.Target)
			}
		} else {
			if !m.ReadOnly {
				t.Errorf("mount %s should be read-only", m.Target)
			}
		}
	}
}

func TestGenerateEnvironment_Python(t *testing.T) {
	buildCtx := &BuildContext{
		Spec: &RuntimeYAMLSpec{
			Base: BaseConfig{
				Language: "python",
				Version:  "3.11",
			},
			Environment: map[string]string{
				"MY_VAR": "my_value",
			},
		},
	}

	env := generateEnvironment(buildCtx)

	// Check user-defined variable
	if v, ok := env["MY_VAR"]; !ok || v != "my_value" {
		t.Errorf("expected MY_VAR=my_value, got %s", v)
	}

	// Check PATH is set
	if _, ok := env["PATH"]; !ok {
		t.Error("expected PATH to be set")
	}

	// Check PYTHONUNBUFFERED is set
	if v, ok := env["PYTHONUNBUFFERED"]; !ok || v != "1" {
		t.Errorf("expected PYTHONUNBUFFERED=1, got %s", v)
	}

	// Check PYTHONPATH is set
	if _, ok := env["PYTHONPATH"]; !ok {
		t.Error("expected PYTHONPATH to be set")
	}
}

func TestGenerateEnvironment_Java(t *testing.T) {
	// Create temp dir with Java structure
	tmpDir, err := os.MkdirTemp("", "generator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Java directory
	javaDir := filepath.Join(tmpDir, "usr/lib/jvm/java-21-openjdk-amd64")
	if err := os.MkdirAll(javaDir, 0755); err != nil {
		t.Fatalf("failed to create java dir: %v", err)
	}

	buildCtx := &BuildContext{
		IsolatedDir: tmpDir,
		Spec: &RuntimeYAMLSpec{
			Base: BaseConfig{
				Language: "java",
				Version:  "21",
			},
			Environment: map[string]string{},
		},
	}

	env := generateEnvironment(buildCtx)

	// Check JAVA_HOME is detected
	if v, ok := env["JAVA_HOME"]; !ok {
		t.Error("expected JAVA_HOME to be set")
	} else if v != "/usr/lib/jvm/java-21-openjdk-amd64" {
		t.Errorf("expected JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64, got %s", v)
	}
}

func TestGenerateEnvironment_Node(t *testing.T) {
	buildCtx := &BuildContext{
		Spec: &RuntimeYAMLSpec{
			Base: BaseConfig{
				Language: "node",
				Version:  "20",
			},
			Environment: map[string]string{},
		},
	}

	env := generateEnvironment(buildCtx)

	// Check NODE_ENV is set
	if v, ok := env["NODE_ENV"]; !ok || v != "production" {
		t.Errorf("expected NODE_ENV=production, got %s", v)
	}
}

func TestGenerateEnvironment_UserOverride(t *testing.T) {
	buildCtx := &BuildContext{
		Spec: &RuntimeYAMLSpec{
			Base: BaseConfig{
				Language: "python",
				Version:  "3.11",
			},
			Environment: map[string]string{
				"PYTHONUNBUFFERED": "0", // User override
				"PATH":             "/custom/path",
			},
		},
	}

	env := generateEnvironment(buildCtx)

	// User-defined values should be preserved
	if v := env["PYTHONUNBUFFERED"]; v != "0" {
		t.Errorf("expected PYTHONUNBUFFERED=0 (user override), got %s", v)
	}

	if v := env["PATH"]; v != "/custom/path" {
		t.Errorf("expected PATH=/custom/path (user override), got %s", v)
	}
}

func TestDetectJavaHome_Found(t *testing.T) {
	// Create temp dir with Java structure
	tmpDir, err := os.MkdirTemp("", "java-home-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Java 17 directory
	javaDir := filepath.Join(tmpDir, "usr/lib/jvm/java-17-openjdk-amd64")
	if err := os.MkdirAll(javaDir, 0755); err != nil {
		t.Fatalf("failed to create java dir: %v", err)
	}

	javaHome := detectJavaHome(tmpDir)
	if javaHome != "/usr/lib/jvm/java-17-openjdk-amd64" {
		t.Errorf("expected /usr/lib/jvm/java-17-openjdk-amd64, got %s", javaHome)
	}
}

func TestDetectJavaHome_NotFound(t *testing.T) {
	// Create empty temp dir
	tmpDir, err := os.MkdirTemp("", "java-home-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	javaHome := detectJavaHome(tmpDir)
	if javaHome != "" {
		t.Errorf("expected empty string, got %s", javaHome)
	}
}

func TestMountSpec_Structure(t *testing.T) {
	mount := MountSpec{
		Source:   "isolated/usr/bin",
		Target:   "/usr/bin",
		ReadOnly: true,
	}

	if mount.Source != "isolated/usr/bin" {
		t.Errorf("expected source 'isolated/usr/bin', got %s", mount.Source)
	}
	if mount.Target != "/usr/bin" {
		t.Errorf("expected target '/usr/bin', got %s", mount.Target)
	}
	if !mount.ReadOnly {
		t.Error("expected readonly=true")
	}
}

func TestRuntimeConfig_Structure(t *testing.T) {
	config := RuntimeConfig{
		Name:        "test-runtime",
		Language:    "python",
		Version:     "1.0.0",
		Description: "Test runtime",
		Mounts: []MountSpec{
			{Source: "isolated/usr/bin", Target: "/usr/bin", ReadOnly: true},
		},
		Environment: map[string]string{
			"PATH": "/usr/bin",
		},
		Packages: []string{"numpy", "pandas"},
		Requirements: RuntimeRequirements{
			Architectures: []string{"amd64"},
			GPU:           false,
		},
		BuildInfo: BuildInfoSpec{
			BuiltAt:   "2024-01-01T00:00:00Z",
			BuiltWith: "joblet-builder",
			Platform:  "ubuntu-amd64",
		},
	}

	if config.Name != "test-runtime" {
		t.Error("config name mismatch")
	}
	if len(config.Mounts) != 1 {
		t.Error("expected 1 mount")
	}
	if len(config.Packages) != 2 {
		t.Error("expected 2 packages")
	}
}
