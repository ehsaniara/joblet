//go:build linux

package builder

import (
	"path/filepath"
	"testing"
)

func TestParseSpec_BasicPython(t *testing.T) {
	spec, err := ParseSpec(filepath.Join("testdata", "python-basic.yaml"))
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if spec.Name != "python-3.11" {
		t.Errorf("expected name 'python-3.11', got %q", spec.Name)
	}
	if spec.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", spec.Version)
	}
	if spec.Base.Language != "python" {
		t.Errorf("expected language 'python', got %q", spec.Base.Language)
	}
	if spec.Base.Version != "3.11" {
		t.Errorf("expected base version '3.11', got %q", spec.Base.Version)
	}
}

func TestParseSpec_PythonML(t *testing.T) {
	spec, err := ParseSpec(filepath.Join("testdata", "python-ml.yaml"))
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if spec.Name != "python-ml" {
		t.Errorf("expected name 'python-ml', got %q", spec.Name)
	}
	if len(spec.Pip) != 2 {
		t.Errorf("expected 2 pip packages, got %d", len(spec.Pip))
	}
	if spec.Hooks == nil {
		t.Error("expected hooks to be set")
	} else if spec.Hooks.PreInstall == "" {
		t.Error("expected pre_install hook to be set")
	}
}

func TestParseSpec_Directory(t *testing.T) {
	// Create a temp directory with runtime.yaml
	spec, err := ParseSpec("testdata")
	if err != nil {
		t.Fatalf("ParseSpec from directory failed: %v", err)
	}

	// Should find one of the yaml files in testdata (first alphabetically or by mod time)
	if spec == nil {
		t.Error("expected spec to be parsed")
	}
}

func TestParseSpec_FileNotFound(t *testing.T) {
	_, err := ParseSpec("testdata/nonexistent.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
