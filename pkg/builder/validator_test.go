//go:build linux

package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func loadSpecFromFile(t *testing.T, name string) *RuntimeYAMLSpec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}
	spec, err := ParseSpecFromBytes(data)
	if err != nil {
		t.Fatalf("ParseSpecFromBytes failed: %v", err)
	}
	return spec
}

func TestValidateSpec_Valid(t *testing.T) {
	spec := loadSpecFromFile(t, "python-basic.yaml")

	if err := ValidateSpec(spec); err != nil {
		t.Errorf("ValidateSpec failed: %v", err)
	}
}

func TestValidateSpec_InvalidName(t *testing.T) {
	spec := loadSpecFromFile(t, "invalid-name.yaml")

	err := ValidateSpec(spec)
	if err == nil {
		t.Error("expected error for invalid name")
	}
}

func TestValidateSpec_MissingName(t *testing.T) {
	spec := &RuntimeYAMLSpec{
		SchemaVersion: "1.0",
		Version:       "1.0.0",
		Description:   "Test",
		Base: BaseConfig{
			Language: "python",
			Version:  "3.11",
		},
	}

	err := ValidateSpec(spec)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateSpec_InvalidVersion(t *testing.T) {
	spec := &RuntimeYAMLSpec{
		SchemaVersion: "1.0",
		Name:          "test-runtime",
		Version:       "invalid",
		Description:   "Test",
		Base: BaseConfig{
			Language: "python",
			Version:  "3.11",
		},
	}

	err := ValidateSpec(spec)
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestValidateSpec_InvalidLanguage(t *testing.T) {
	spec := &RuntimeYAMLSpec{
		SchemaVersion: "1.0",
		Name:          "test-runtime",
		Version:       "1.0.0",
		Description:   "Test",
		Base: BaseConfig{
			Language: "invalid",
			Version:  "1.0",
		},
	}

	err := ValidateSpec(spec)
	if err == nil {
		t.Error("expected error for invalid language")
	}
}

func TestValidateSpec_UnsupportedSchemaVersion(t *testing.T) {
	spec := &RuntimeYAMLSpec{
		SchemaVersion: "2.0",
		Name:          "test-runtime",
		Version:       "1.0.0",
		Description:   "Test",
		Base: BaseConfig{
			Language: "python",
			Version:  "3.11",
		},
	}

	err := ValidateSpec(spec)
	if err == nil {
		t.Error("expected error for unsupported schema version")
	}
}
