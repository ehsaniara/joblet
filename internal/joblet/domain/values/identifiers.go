package values

import (
	"fmt"
	"regexp"
	"strings"
)

// JobUUID represents a unique job identifier
type JobUUID struct {
	value string
}

// NewJobUUID creates a new JobUUID with validation
func NewJobUUID(id string) (JobUUID, error) {
	if strings.TrimSpace(id) == "" {
		return JobUUID{}, fmt.Errorf("job UUID cannot be empty")
	}

	// Validate UUID format (basic check)
	if len(id) < 8 {
		return JobUUID{}, fmt.Errorf("job UUID too short: %s", id)
	}

	return JobUUID{value: id}, nil
}

// String returns the string representation
func (j JobUUID) String() string {
	return j.value
}

// Value returns the underlying string value
func (j JobUUID) Value() string {
	return j.value
}

// IsEmpty returns true if the UUID is empty
func (j JobUUID) IsEmpty() bool {
	return j.value == ""
}

// ProcessID represents a system process identifier
type ProcessID struct {
	value int32
}

// NewProcessID creates a new ProcessID with validation
func NewProcessID(pid int32) (ProcessID, error) {
	if pid < 0 {
		return ProcessID{}, fmt.Errorf("process ID cannot be negative: %d", pid)
	}
	if pid == 0 {
		return ProcessID{}, fmt.Errorf("process ID cannot be zero")
	}
	return ProcessID{value: pid}, nil
}

// Value returns the PID value
func (p ProcessID) Value() int32 {
	return p.value
}

// String returns the string representation
func (p ProcessID) String() string {
	return fmt.Sprintf("%d", p.value)
}

// IsValid returns true if the PID is valid
func (p ProcessID) IsValid() bool {
	return p.value > 0
}

// NetworkName represents a network identifier
type NetworkName struct {
	value string
}

// NewNetworkName creates a new NetworkName with validation
func NewNetworkName(name string) (NetworkName, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NetworkName{}, fmt.Errorf("network name cannot be empty")
	}

	// Validate network name format (DNS-like)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`, name); !matched {
		return NetworkName{}, fmt.Errorf("invalid network name format: %s", name)
	}

	if len(name) > 63 {
		return NetworkName{}, fmt.Errorf("network name too long: %s", name)
	}

	return NetworkName{value: name}, nil
}

// String returns the string representation
func (n NetworkName) String() string {
	return n.value
}

// Value returns the underlying string value
func (n NetworkName) Value() string {
	return n.value
}

// IsIsolated returns true if this is the special isolated network
func (n NetworkName) IsIsolated() bool {
	return n.value == "isolated"
}

// VolumeName represents a volume identifier
type VolumeName struct {
	value string
}

// NewVolumeName creates a new VolumeName with validation
func NewVolumeName(name string) (VolumeName, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return VolumeName{}, fmt.Errorf("volume name cannot be empty")
	}

	// Validate volume name format (container-like)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`, name); !matched {
		return VolumeName{}, fmt.Errorf("invalid volume name format: %s", name)
	}

	if len(name) > 128 {
		return VolumeName{}, fmt.Errorf("volume name too long: %s", name)
	}

	return VolumeName{value: name}, nil
}

// String returns the string representation
func (v VolumeName) String() string {
	return v.value
}

// Value returns the underlying string value
func (v VolumeName) Value() string {
	return v.value
}

// RuntimeSpec represents a runtime specification
type RuntimeSpec struct {
	value string
}

// NewRuntimeSpec creates a new RuntimeSpec with validation
func NewRuntimeSpec(spec string) (RuntimeSpec, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return RuntimeSpec{}, nil // Empty runtime is valid (use default)
	}

	// Basic validation for runtime format like "python-3.11-ml"
	if len(spec) > 256 {
		return RuntimeSpec{}, fmt.Errorf("runtime spec too long: %s", spec)
	}

	return RuntimeSpec{value: spec}, nil
}

// String returns the string representation
func (r RuntimeSpec) String() string {
	return r.value
}

// Value returns the underlying string value
func (r RuntimeSpec) Value() string {
	return r.value
}

// IsEmpty returns true if no runtime is specified
func (r RuntimeSpec) IsEmpty() bool {
	return r.value == ""
}

// Language extracts the language part from runtime spec (e.g., "python" from "python-3.11-ml")
func (r RuntimeSpec) Language() string {
	if r.IsEmpty() {
		return ""
	}

	parts := strings.Split(r.value, ":")
	return parts[0]
}

// Version extracts the version part from runtime spec (e.g., "3.11-ml" from "python-3.11-ml")
func (r RuntimeSpec) Version() string {
	if r.IsEmpty() {
		return ""
	}

	parts := strings.Split(r.value, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
