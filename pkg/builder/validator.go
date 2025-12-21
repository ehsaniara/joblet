//go:build linux

package builder

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Name pattern: lowercase letters, numbers, hyphens, dots (max 64 chars)
	namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)
	// Version pattern: semantic versioning X.Y.Z
	versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// ValidateSpec validates a runtime specification
func ValidateSpec(spec *RuntimeYAMLSpec) error {
	var errors []string

	// Required fields
	if spec.SchemaVersion == "" {
		errors = append(errors, "schema_version is required")
	} else if spec.SchemaVersion != "1.0" {
		errors = append(errors, fmt.Sprintf("unsupported schema_version: %s (expected 1.0)", spec.SchemaVersion))
	}

	if spec.Name == "" {
		errors = append(errors, "name is required")
	} else if !namePattern.MatchString(spec.Name) {
		errors = append(errors, fmt.Sprintf("invalid name: %s (must be lowercase, hyphens, dots, max 64 chars)", spec.Name))
	}

	if spec.Version == "" {
		errors = append(errors, "version is required")
	} else if !versionPattern.MatchString(spec.Version) {
		errors = append(errors, fmt.Sprintf("invalid version: %s (expected semantic version X.Y.Z)", spec.Version))
	}

	if spec.Description == "" {
		errors = append(errors, "description is required")
	} else if len(spec.Description) > 256 {
		errors = append(errors, fmt.Sprintf("description too long: %d chars (max 256)", len(spec.Description)))
	}

	// Validate base
	if err := validateBase(&spec.Base); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate platforms if specified
	if len(spec.Platforms) > 0 {
		for _, platform := range spec.Platforms {
			if !isValidPlatform(platform) {
				errors = append(errors, fmt.Sprintf("unsupported platform: %s", platform))
			}
		}
	}

	// Validate hooks timeout format
	if spec.Hooks != nil && spec.Hooks.Timeout != "" {
		if err := validateTimeout(spec.Hooks.Timeout); err != nil {
			errors = append(errors, fmt.Sprintf("invalid hooks timeout: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

func validateBase(base *BaseConfig) error {
	if base.Language == "" {
		return fmt.Errorf("base.language is required")
	}

	if !isValidLanguage(base.Language) {
		return fmt.Errorf("unsupported language: %s (supported: %s)", base.Language, strings.Join(SupportedLanguages, ", "))
	}

	if base.Version == "" {
		return fmt.Errorf("base.version is required")
	}

	return nil
}

func isValidLanguage(lang string) bool {
	for _, supported := range SupportedLanguages {
		if lang == supported {
			return true
		}
	}
	return false
}

func isValidPlatform(platform string) bool {
	for _, supported := range SupportedPlatforms {
		if platform == supported {
			return true
		}
	}
	return false
}

func validateTimeout(timeout string) error {
	// Simple validation - just check it ends with a valid unit
	if timeout == "" {
		return nil
	}

	validUnits := []string{"s", "m", "h"}
	hasValidUnit := false
	for _, unit := range validUnits {
		if strings.HasSuffix(timeout, unit) {
			hasValidUnit = true
			break
		}
	}

	if !hasValidUnit {
		return fmt.Errorf("timeout must end with s, m, or h (e.g., '30m')")
	}

	return nil
}
