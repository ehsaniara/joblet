//go:build linux

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ParseSpec parses a runtime.yaml file and returns the specification
func ParseSpec(path string) (*RuntimeYAMLSpec, error) {
	// Resolve path - could be a directory or a file
	specPath := path
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	// If it's a directory, look for runtime.yaml
	if info.IsDir() {
		specPath = filepath.Join(path, "runtime.yaml")
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			// Try runtime.yml
			specPath = filepath.Join(path, "runtime.yml")
			if _, err := os.Stat(specPath); os.IsNotExist(err) {
				return nil, fmt.Errorf("no runtime.yaml or runtime.yml found in %s", path)
			}
		}
	}

	// Read the file
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	// Parse YAML
	var spec RuntimeYAMLSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &spec, nil
}

// ParseSpecFromBytes parses a runtime.yaml from bytes
func ParseSpecFromBytes(data []byte) (*RuntimeYAMLSpec, error) {
	var spec RuntimeYAMLSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return &spec, nil
}

// ParseRuntimeYAML is an alias for ParseSpecFromBytes for compatibility
func ParseRuntimeYAML(data []byte) (*RuntimeYAMLSpec, error) {
	return ParseSpecFromBytes(data)
}
