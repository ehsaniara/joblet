package values

import (
	"fmt"
	"strconv"
	"strings"
)

// CPUPercentage represents CPU usage as a percentage
type CPUPercentage struct {
	value int32
}

// NewCPUPercentage creates a new CPU percentage value
func NewCPUPercentage(percent int32) (CPUPercentage, error) {
	if percent < 0 {
		return CPUPercentage{}, fmt.Errorf("CPU percentage cannot be negative: %d", percent)
	}
	// Allow > 100% for multi-core systems
	if percent > 10000 { // 100 cores max
		return CPUPercentage{}, fmt.Errorf("CPU percentage too high: %d", percent)
	}
	return CPUPercentage{value: percent}, nil
}

// Value returns the percentage value
func (c CPUPercentage) Value() int32 {
	return c.value
}

// Cores returns the approximate number of cores
func (c CPUPercentage) Cores() float32 {
	return float32(c.value) / 100.0
}

// String implements Stringer
func (c CPUPercentage) String() string {
	return fmt.Sprintf("%d%%", c.value)
}

// IsUnlimited returns true if no limit is set
func (c CPUPercentage) IsUnlimited() bool {
	return c.value == 0
}

// CPUCoreSet represents a set of CPU cores
type CPUCoreSet struct {
	cores []int
	spec  string
}

// ParseCPUCoreSet parses a CPU core specification
// Formats: "0-3", "1,3,5", "2", or "" for no restriction
func ParseCPUCoreSet(spec string) (CPUCoreSet, error) {
	if spec == "" {
		return CPUCoreSet{}, nil
	}

	var cores []int

	// Handle range format: "0-3"
	if strings.Contains(spec, "-") {
		parts := strings.Split(spec, "-")
		if len(parts) != 2 {
			return CPUCoreSet{}, fmt.Errorf("invalid CPU range format: %s", spec)
		}

		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

		if err1 != nil || err2 != nil {
			return CPUCoreSet{}, fmt.Errorf("invalid CPU range numbers: %s", spec)
		}

		if start < 0 || end < 0 {
			return CPUCoreSet{}, fmt.Errorf("CPU core numbers cannot be negative")
		}

		if start > end {
			return CPUCoreSet{}, fmt.Errorf("invalid CPU range: start > end")
		}

		for i := start; i <= end; i++ {
			cores = append(cores, i)
		}
	} else {
		// Handle list format: "0,2,4,6"
		for _, coreStr := range strings.Split(spec, ",") {
			core, err := strconv.Atoi(strings.TrimSpace(coreStr))
			if err != nil {
				return CPUCoreSet{}, fmt.Errorf("invalid CPU core number: %s", coreStr)
			}
			if core < 0 {
				return CPUCoreSet{}, fmt.Errorf("CPU core number cannot be negative: %d", core)
			}
			cores = append(cores, core)
		}
	}

	// Check for duplicates
	seen := make(map[int]bool)
	for _, core := range cores {
		if seen[core] {
			return CPUCoreSet{}, fmt.Errorf("duplicate CPU core specification: %d", core)
		}
		seen[core] = true
	}

	return CPUCoreSet{cores: cores, spec: spec}, nil
}

// Cores returns the list of CPU cores
func (c CPUCoreSet) Cores() []int {
	return c.cores
}

// Count returns the number of cores
func (c CPUCoreSet) Count() int {
	return len(c.cores)
}

// IsEmpty returns true if no cores are specified
func (c CPUCoreSet) IsEmpty() bool {
	return len(c.cores) == 0
}

// Contains checks if a core is in the set
func (c CPUCoreSet) Contains(core int) bool {
	for _, c := range c.cores {
		if c == core {
			return true
		}
	}
	return false
}

// String returns the original specification
func (c CPUCoreSet) String() string {
	return c.spec
}

// Validate checks if the core set is valid for the system
func (c CPUCoreSet) Validate(availableCores int) error {
	for _, core := range c.cores {
		if core >= availableCores {
			return fmt.Errorf("core %d not available (system has %d cores)", core, availableCores)
		}
	}
	return nil
}
