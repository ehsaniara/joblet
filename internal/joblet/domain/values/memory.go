package values

import (
	"fmt"
)

// MemorySize represents memory size with unit conversions
type MemorySize struct {
	bytes int64
}

// MemoryUnit represents different memory units
type MemoryUnit string

const (
	Byte     MemoryUnit = "B"
	Kilobyte MemoryUnit = "KB"
	Megabyte MemoryUnit = "MB"
	Gigabyte MemoryUnit = "GB"
)

// NewMemorySize creates a new memory size from bytes
func NewMemorySize(bytes int64) (MemorySize, error) {
	if bytes < 0 {
		return MemorySize{}, fmt.Errorf("memory size cannot be negative: %d", bytes)
	}
	return MemorySize{bytes: bytes}, nil
}

// NewMemorySizeFromMB creates memory size from megabytes
func NewMemorySizeFromMB(mb int32) (MemorySize, error) {
	if mb < 0 {
		return MemorySize{}, fmt.Errorf("memory size cannot be negative: %d MB", mb)
	}
	bytes := int64(mb) * 1024 * 1024
	return MemorySize{bytes: bytes}, nil
}

// Bytes returns the size in bytes
func (m MemorySize) Bytes() int64 {
	return m.bytes
}

// Megabytes returns the size in megabytes
func (m MemorySize) Megabytes() int32 {
	return int32(m.bytes / (1024 * 1024))
}

// Gigabytes returns the size in gigabytes
func (m MemorySize) Gigabytes() float64 {
	return float64(m.bytes) / (1024 * 1024 * 1024)
}

// IsUnlimited returns true if no limit is set
func (m MemorySize) IsUnlimited() bool {
	return m.bytes == 0
}

// String returns a readable string
func (m MemorySize) String() string {
	if m.bytes == 0 {
		return "0"
	}

	const unit = 1024
	if m.bytes < unit {
		return fmt.Sprintf("%d B", m.bytes)
	}

	div, exp := int64(unit), 0
	for n := m.bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(m.bytes)/float64(div), "KMGTPE"[exp])
}

// Add adds two memory sizes
func (m MemorySize) Add(other MemorySize) MemorySize {
	return MemorySize{bytes: m.bytes + other.bytes}
}

// Subtract subtracts another memory size
func (m MemorySize) Subtract(other MemorySize) (MemorySize, error) {
	if other.bytes > m.bytes {
		return MemorySize{}, fmt.Errorf("cannot subtract %s from %s", other, m)
	}
	return MemorySize{bytes: m.bytes - other.bytes}, nil
}

// Validate checks if the memory size is within acceptable bounds
func (m MemorySize) Validate(min, max MemorySize) error {
	if m.bytes < min.bytes {
		return fmt.Errorf("memory size %s is below minimum %s", m, min)
	}
	if max.bytes > 0 && m.bytes > max.bytes {
		return fmt.Errorf("memory size %s exceeds maximum %s", m, max)
	}
	return nil
}
