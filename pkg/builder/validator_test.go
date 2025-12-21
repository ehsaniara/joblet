//go:build linux

package builder

import (
	"path/filepath"
	"testing"
)

func TestValidateSpec_Valid(t *testing.T) {
	spec, err := ParseSpec(filepath.Join("testdata", "python-basic.yaml"))
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	if err := ValidateSpec(spec); err != nil {
		t.Errorf("ValidateSpec failed: %v", err)
	}
}

func TestValidateSpec_InvalidName(t *testing.T) {
	spec, err := ParseSpec(filepath.Join("testdata", "invalid-name.yaml"))
	if err != nil {
		t.Fatalf("ParseSpec failed: %v", err)
	}

	err = ValidateSpec(spec)
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
