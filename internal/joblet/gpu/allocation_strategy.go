package gpu

import (
	"fmt"
	"sort"
)

// AllocationStrategy defines the GPU allocation strategy
type AllocationStrategy string

const (
	// StrategyFirstFit allocates the first available GPUs (default)
	StrategyFirstFit AllocationStrategy = "first-fit"

	// StrategyPack tries to pack jobs onto the fewest GPUs/nodes
	// This maximizes GPU utilization per node and leaves other nodes free
	StrategyPack AllocationStrategy = "pack"

	// StrategySpread tries to spread jobs across as many GPUs/nodes as possible
	// This provides better isolation and thermal distribution
	StrategySpread AllocationStrategy = "spread"

	// StrategyBestFit allocates GPUs with memory closest to requirement
	// This optimizes memory usage and prevents waste
	StrategyBestFit AllocationStrategy = "best-fit"
)

// GPUAllocationStrategy interface for different allocation strategies
type GPUAllocationStrategy interface {
	// SelectGPUs selects GPUs based on the strategy
	SelectGPUs(availableGPUs []*GPU, gpuCount int, gpuMemoryMB int64) ([]*GPU, error)
	// Name returns the strategy name
	Name() string
}

// FirstFitStrategy allocates the first available GPUs
type FirstFitStrategy struct{}

func (s *FirstFitStrategy) SelectGPUs(availableGPUs []*GPU, gpuCount int, gpuMemoryMB int64) ([]*GPU, error) {
	if len(availableGPUs) < gpuCount {
		return nil, fmt.Errorf("not enough GPUs available: need %d, have %d", gpuCount, len(availableGPUs))
	}

	// Simply return the first N GPUs
	return availableGPUs[:gpuCount], nil
}

func (s *FirstFitStrategy) Name() string {
	return string(StrategyFirstFit)
}

// PackStrategy tries to pack jobs onto GPUs with highest current utilization
// This keeps jobs consolidated on fewer physical nodes
type PackStrategy struct{}

func (s *PackStrategy) SelectGPUs(availableGPUs []*GPU, gpuCount int, gpuMemoryMB int64) ([]*GPU, error) {
	if len(availableGPUs) < gpuCount {
		return nil, fmt.Errorf("not enough GPUs available: need %d, have %d", gpuCount, len(availableGPUs))
	}

	// Create a copy for sorting
	sortedGPUs := make([]*GPU, len(availableGPUs))
	copy(sortedGPUs, availableGPUs)

	// Sort GPUs to prefer:
	// 1. GPUs on nodes that already have other allocated GPUs (pack onto same nodes)
	// 2. Lower indexed GPUs (typically on same node)
	sort.Slice(sortedGPUs, func(i, j int) bool {
		// Prefer lower indices (typically same physical node)
		return sortedGPUs[i].Index < sortedGPUs[j].Index
	})

	return sortedGPUs[:gpuCount], nil
}

func (s *PackStrategy) Name() string {
	return string(StrategyPack)
}

// SpreadStrategy tries to spread jobs across different GPUs/nodes
// This provides better thermal distribution and fault isolation
type SpreadStrategy struct{}

func (s *SpreadStrategy) SelectGPUs(availableGPUs []*GPU, gpuCount int, gpuMemoryMB int64) ([]*GPU, error) {
	if len(availableGPUs) < gpuCount {
		return nil, fmt.Errorf("not enough GPUs available: need %d, have %d", gpuCount, len(availableGPUs))
	}

	// Create a copy for sorting
	sortedGPUs := make([]*GPU, len(availableGPUs))
	copy(sortedGPUs, availableGPUs)

	// Sort GPUs to prefer:
	// 1. GPUs on nodes that have fewer allocated GPUs (spread across nodes)
	// 2. Higher indexed GPUs (typically on different nodes)
	sort.Slice(sortedGPUs, func(i, j int) bool {
		// Prefer higher indices (typically different physical nodes)
		// This spreads allocations across the GPU topology
		return sortedGPUs[i].Index > sortedGPUs[j].Index
	})

	// If we need multiple GPUs, try to pick them from different ranges
	if gpuCount > 1 && len(sortedGPUs) >= gpuCount*2 {
		selected := make([]*GPU, 0, gpuCount)
		step := len(sortedGPUs) / gpuCount

		for i := 0; i < gpuCount; i++ {
			idx := i * step
			if idx < len(sortedGPUs) {
				selected = append(selected, sortedGPUs[idx])
			}
		}

		// Fill any remaining slots if needed
		for i := len(selected); i < gpuCount && i < len(sortedGPUs); i++ {
			selected = append(selected, sortedGPUs[i])
		}

		return selected, nil
	}

	return sortedGPUs[:gpuCount], nil
}

func (s *SpreadStrategy) Name() string {
	return string(StrategySpread)
}

// BestFitStrategy allocates GPUs with memory closest to the requirement
// This minimizes memory waste
type BestFitStrategy struct{}

func (s *BestFitStrategy) SelectGPUs(availableGPUs []*GPU, gpuCount int, gpuMemoryMB int64) ([]*GPU, error) {
	if len(availableGPUs) < gpuCount {
		return nil, fmt.Errorf("not enough GPUs available: need %d, have %d", gpuCount, len(availableGPUs))
	}

	// Create a copy for sorting
	sortedGPUs := make([]*GPU, len(availableGPUs))
	copy(sortedGPUs, availableGPUs)

	// Sort GPUs by how close their memory is to the requirement
	// If no memory requirement, sort by memory size (prefer smaller GPUs)
	sort.Slice(sortedGPUs, func(i, j int) bool {
		if gpuMemoryMB > 0 {
			// Calculate waste (unused memory)
			wasteI := sortedGPUs[i].MemoryMB - gpuMemoryMB
			wasteJ := sortedGPUs[j].MemoryMB - gpuMemoryMB

			// Both must meet requirement
			if wasteI >= 0 && wasteJ >= 0 {
				// Prefer GPU with less waste
				return wasteI < wasteJ
			}
			// Prefer GPU that meets requirement
			if wasteI >= 0 {
				return true
			}
			if wasteJ >= 0 {
				return false
			}
			// Neither meets requirement, prefer larger
			return sortedGPUs[i].MemoryMB > sortedGPUs[j].MemoryMB
		} else {
			// No memory requirement, prefer smaller GPUs
			return sortedGPUs[i].MemoryMB < sortedGPUs[j].MemoryMB
		}
	})

	return sortedGPUs[:gpuCount], nil
}

func (s *BestFitStrategy) Name() string {
	return string(StrategyBestFit)
}

// GetAllocationStrategy returns a GPU allocation strategy based on the name
func GetAllocationStrategy(strategy string) GPUAllocationStrategy {
	switch AllocationStrategy(strategy) {
	case StrategyPack:
		return &PackStrategy{}
	case StrategySpread:
		return &SpreadStrategy{}
	case StrategyBestFit:
		return &BestFitStrategy{}
	case StrategyFirstFit, "":
		return &FirstFitStrategy{}
	default:
		// Default to first-fit for unknown strategies
		return &FirstFitStrategy{}
	}
}

// ValidateStrategy checks if a strategy name is valid
func ValidateStrategy(strategy string) error {
	switch AllocationStrategy(strategy) {
	case StrategyFirstFit, StrategyPack, StrategySpread, StrategyBestFit, "":
		return nil
	default:
		return fmt.Errorf("unknown GPU allocation strategy: %s", strategy)
	}
}

// GetAvailableStrategies returns all available allocation strategies
func GetAvailableStrategies() []string {
	return []string{
		string(StrategyFirstFit),
		string(StrategyPack),
		string(StrategySpread),
		string(StrategyBestFit),
	}
}
