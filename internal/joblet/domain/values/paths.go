package values

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Path represents a filesystem path with validation
type Path struct {
	value string
}

// NewAbsolutePath creates a new absolute Path with validation
func NewAbsolutePath(path string) (Path, error) {
	if strings.TrimSpace(path) == "" {
		return Path{}, fmt.Errorf("path cannot be empty")
	}

	// Clean the path
	cleaned := filepath.Clean(path)

	if !filepath.IsAbs(cleaned) {
		return Path{}, fmt.Errorf("path must be absolute: %s", path)
	}

	return Path{value: cleaned}, nil
}

// String returns the string representation
func (p Path) String() string {
	return p.value
}

// Value returns the underlying string value
func (p Path) Value() string {
	return p.value
}

// IsEmpty returns true if the path is empty
func (p Path) IsEmpty() bool {
	return p.value == ""
}

// IsAbsolute returns true if the path is absolute
func (p Path) IsAbsolute() bool {
	return filepath.IsAbs(p.value)
}

// Dir returns the directory part of the path
func (p Path) Dir() Path {
	return Path{value: filepath.Dir(p.value)}
}

// Base returns the base filename
func (p Path) Base() string {
	return filepath.Base(p.value)
}

// Join creates a new Path by joining with the given elements
func (p Path) Join(elements ...string) Path {
	allElements := append([]string{p.value}, elements...)
	return Path{value: filepath.Join(allElements...)}
}

// CgroupPath represents a cgroup filesystem path
type CgroupPath struct {
	path Path
}

// NewCgroupPath creates a new CgroupPath with validation
func NewCgroupPath(path string) (CgroupPath, error) {
	if strings.TrimSpace(path) == "" {
		return CgroupPath{}, fmt.Errorf("cgroup path cannot be empty")
	}

	p, err := NewAbsolutePath(path)
	if err != nil {
		return CgroupPath{}, fmt.Errorf("invalid cgroup path: %w", err)
	}

	// Validate that it looks like a cgroup path
	if !strings.HasPrefix(p.Value(), "/sys/fs/cgroup") && !strings.HasPrefix(p.Value(), "/sys/fs/cgroup2") {
		return CgroupPath{}, fmt.Errorf("invalid cgroup path prefix: %s", path)
	}

	return CgroupPath{path: p}, nil
}

// String returns the string representation
func (c CgroupPath) String() string {
	return c.path.String()
}

// Value returns the underlying string value
func (c CgroupPath) Value() string {
	return c.path.Value()
}

// Path returns the underlying Path
func (c CgroupPath) Path() Path {
	return c.path
}

// IsV1 returns true if this is a cgroup v1 path
func (c CgroupPath) IsV1() bool {
	return strings.HasPrefix(c.path.Value(), "/sys/fs/cgroup/")
}

// IsV2 returns true if this is a cgroup v2 path
func (c CgroupPath) IsV2() bool {
	return strings.HasPrefix(c.path.Value(), "/sys/fs/cgroup2/")
}
