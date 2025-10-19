package runtime

import (
	"fmt"
	"regexp"
	"strings"
)

// RuntimeSpec represents a parsed runtime specification
type RuntimeSpec struct {
	// Name is the runtime name (e.g., "python-3.11-ml", "openjdk-21")
	Name string

	// Version is the requested version (e.g., "1.0.0", "latest", "")
	// Empty string means "latest"
	Version string

	// Original is the original unparsed spec
	Original string
}

const (
	// DefaultVersion is used when no version is specified
	DefaultVersion = "latest"
)

// ParseRuntimeSpec parses a runtime specification string.
//
// Supported formats:
//   - "python-3.11-ml@1.0.0"     → name: "python-3.11-ml", version: "1.0.0"
//   - "python-3.11-ml@latest"    → name: "python-3.11-ml", version: "latest"
//   - "python-3.11-ml"           → name: "python-3.11-ml", version: "latest" (default)
//   - "openjdk-21@1.0.0"         → name: "openjdk-21", version: "1.0.0"
//
// The @ notation is similar to npm package versioning.
func ParseRuntimeSpec(spec string) (*RuntimeSpec, error) {
	if spec == "" {
		return nil, fmt.Errorf("runtime spec cannot be empty")
	}

	// Trim whitespace
	spec = strings.TrimSpace(spec)

	// Split on @ to separate name and version
	parts := strings.SplitN(spec, "@", 2)

	name := parts[0]
	version := ""

	if len(parts) == 2 {
		version = parts[1]
	}

	// Validate name (must not be empty)
	if name == "" {
		return nil, fmt.Errorf("runtime name cannot be empty")
	}

	// Validate name format: lowercase letters, numbers, hyphens, dots only
	validName := regexp.MustCompile(`^[a-z0-9][a-z0-9\-\.]*$`)
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("invalid runtime name: %s (must be lowercase, start with letter/number, and contain only lowercase letters, numbers, hyphens, or dots)", name)
	}

	// Default to "latest" if no version specified
	if version == "" {
		version = DefaultVersion
	}

	// Validate version format (semantic version or "latest")
	if version != "latest" {
		if err := validateVersion(version); err != nil {
			return nil, fmt.Errorf("invalid version: %w", err)
		}
	}

	return &RuntimeSpec{
		Name:     name,
		Version:  version,
		Original: spec,
	}, nil
}

// validateVersion validates a semantic version string
// Accepts formats like: 1.0.0, 1.0.0-beta, 1.0.0-rc.1, etc.
func validateVersion(version string) error {
	// Semantic version regex (simplified)
	// Matches: 1.0.0, 1.0.0-beta, 1.0.0-rc.1, 1.0.0-alpha.1+build.123
	semverPattern := `^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.\-]+)?(\+[a-zA-Z0-9\.\-]+)?$`
	validSemver := regexp.MustCompile(semverPattern)

	if !validSemver.MatchString(version) {
		return fmt.Errorf("%s (must be semantic version like 1.0.0 or 'latest')", version)
	}

	return nil
}

// String returns a canonical string representation of the runtime spec
func (rs *RuntimeSpec) String() string {
	return fmt.Sprintf("%s@%s", rs.Name, rs.Version)
}

// IsLatest returns true if the version is "latest"
func (rs *RuntimeSpec) IsLatest() bool {
	return rs.Version == "latest" || rs.Version == ""
}

// FullName returns the full name including version for directory names
// Example: "python-3.11-ml@1.0.0" → "python-3.11-ml-1.0.0"
func (rs *RuntimeSpec) FullName() string {
	if rs.IsLatest() {
		return fmt.Sprintf("%s-latest", rs.Name)
	}
	return fmt.Sprintf("%s-%s", rs.Name, rs.Version)
}

// MustParseRuntimeSpec is like ParseRuntimeSpec but panics on error
// Only use in tests or initialization code where errors are not expected
func MustParseRuntimeSpec(spec string) *RuntimeSpec {
	rs, err := ParseRuntimeSpec(spec)
	if err != nil {
		panic(fmt.Sprintf("invalid runtime spec %q: %v", spec, err))
	}
	return rs
}
