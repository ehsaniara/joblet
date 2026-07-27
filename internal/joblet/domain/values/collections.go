package values

import (
	"fmt"
	"sort"
	"strings"
)

// VolumeNames represents a collection of volume names
type VolumeNames struct {
	volumes []VolumeName
}

// NewVolumeNames creates a new VolumeNames collection from string slice
func NewVolumeNames(names []string) (VolumeNames, error) {
	var volumes []VolumeName

	for _, name := range names {
		vol, err := NewVolumeName(name)
		if err != nil {
			return VolumeNames{}, fmt.Errorf("invalid volume name '%s': %w", name, err)
		}
		volumes = append(volumes, vol)
	}

	return VolumeNames{volumes: volumes}, nil
}

// Count returns the number of volumes
func (vn VolumeNames) Count() int {
	return len(vn.volumes)
}

// IsEmpty returns true if no volumes are specified
func (vn VolumeNames) IsEmpty() bool {
	return len(vn.volumes) == 0
}

// ToSlice returns the volumes as a slice
func (vn VolumeNames) ToSlice() []VolumeName {
	result := make([]VolumeName, len(vn.volumes))
	copy(result, vn.volumes)
	return result
}

// ToStringSlice returns the volume names as string slice
func (vn VolumeNames) ToStringSlice() []string {
	result := make([]string, len(vn.volumes))
	for i, vol := range vn.volumes {
		result[i] = vol.Value()
	}
	return result
}

// String returns a comma-separated string representation
func (vn VolumeNames) String() string {
	if vn.IsEmpty() {
		return ""
	}

	names := vn.ToStringSlice()
	sort.Strings(names)
	return strings.Join(names, ",")
}

// Validate checks if all volume names are valid and unique
func (vn VolumeNames) Validate() error {
	seen := make(map[string]bool)

	for _, vol := range vn.volumes {
		name := vol.Value()
		if seen[name] {
			return fmt.Errorf("duplicate volume name: %s", name)
		}
		seen[name] = true
	}

	return nil
}

// Environment represents job environment variables as a value object
type Environment struct {
	variables map[string]string
}

// NewEnvironment creates a new Environment from a map
func NewEnvironment(vars map[string]string) Environment {
	// Create a copy to avoid mutation
	envVars := make(map[string]string)
	for k, v := range vars {
		envVars[k] = v
	}
	return Environment{variables: envVars}
}

// ToMap returns the environment as a map (copy)
func (e Environment) ToMap() map[string]string {
	result := make(map[string]string)
	for k, v := range e.variables {
		result[k] = v
	}
	return result
}

// ToSlice returns the environment as a slice of "KEY=VALUE" strings
func (e Environment) ToSlice() []string {
	result := make([]string, 0, len(e.variables))
	for k, v := range e.variables {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result
}

// Count returns the number of environment variables
func (e Environment) Count() int {
	return len(e.variables)
}

// IsEmpty returns true if no environment variables are set
func (e Environment) IsEmpty() bool {
	return len(e.variables) == 0
}
