//go:build linux

package builder

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
