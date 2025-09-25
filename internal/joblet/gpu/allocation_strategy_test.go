package gpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstFitStrategy(t *testing.T) {
	strategy := &FirstFitStrategy{}

	// Test with sufficient GPUs
	gpus := []*GPU{
		{Index: 0, MemoryMB: 8192, Name: "GPU0"},
		{Index: 1, MemoryMB: 16384, Name: "GPU1"},
		{Index: 2, MemoryMB: 8192, Name: "GPU2"},
	}

	selected, err := strategy.SelectGPUs(gpus, 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(selected))
	assert.Equal(t, 0, selected[0].Index)
	assert.Equal(t, 1, selected[1].Index)
	assert.Equal(t, "first-fit", strategy.Name())
}

func TestFirstFitStrategy_InsufficientGPUs(t *testing.T) {
	strategy := &FirstFitStrategy{}

	gpus := []*GPU{
		{Index: 0, MemoryMB: 8192, Name: "GPU0"},
	}

	selected, err := strategy.SelectGPUs(gpus, 2, 0)
	assert.Error(t, err)
	assert.Nil(t, selected)
	assert.Contains(t, err.Error(), "not enough GPUs available")
}

func TestPackStrategy(t *testing.T) {
	strategy := &PackStrategy{}

	// Test packing - should prefer lower indices (same node)
	gpus := []*GPU{
		{Index: 2, MemoryMB: 8192, Name: "GPU2"},
		{Index: 0, MemoryMB: 16384, Name: "GPU0"},
		{Index: 1, MemoryMB: 8192, Name: "GPU1"},
	}

	selected, err := strategy.SelectGPUs(gpus, 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(selected))
	// Should select lowest indices first (0, 1)
	assert.Equal(t, 0, selected[0].Index)
	assert.Equal(t, 1, selected[1].Index)
	assert.Equal(t, "pack", strategy.Name())
}

func TestSpreadStrategy(t *testing.T) {
	strategy := &SpreadStrategy{}

	// Test spreading - should prefer higher indices (different nodes)
	gpus := []*GPU{
		{Index: 0, MemoryMB: 8192, Name: "GPU0"},
		{Index: 1, MemoryMB: 16384, Name: "GPU1"},
		{Index: 2, MemoryMB: 8192, Name: "GPU2"},
		{Index: 3, MemoryMB: 8192, Name: "GPU3"},
	}

	selected, err := strategy.SelectGPUs(gpus, 2, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(selected))
	// Should spread across GPUs - exact selection depends on spread algorithm
	assert.Equal(t, "spread", strategy.Name())
}

func TestSpreadStrategy_MultipleGPUs(t *testing.T) {
	strategy := &SpreadStrategy{}

	// Test with enough GPUs for spreading
	gpus := make([]*GPU, 8)
	for i := 0; i < 8; i++ {
		gpus[i] = &GPU{Index: i, MemoryMB: 8192, Name: "GPU" + string(rune('0'+i))}
	}

	selected, err := strategy.SelectGPUs(gpus, 4, 0)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(selected))

	// With 8 GPUs and requesting 4, should spread evenly
	actualIndices := make([]int, len(selected))
	for i, gpu := range selected {
		actualIndices[i] = gpu.Index
	}

	// Check that spreading occurred (not just first N in reverse order)
	assert.NotEqual(t, []int{7, 6, 5, 4}, actualIndices, "Should spread, not just take first N")
}

func TestBestFitStrategy(t *testing.T) {
	strategy := &BestFitStrategy{}

	// Test best-fit memory allocation
	gpus := []*GPU{
		{Index: 0, MemoryMB: 8192, Name: "GPU0"},  // Exact match
		{Index: 1, MemoryMB: 16384, Name: "GPU1"}, // Too much
		{Index: 2, MemoryMB: 4096, Name: "GPU2"},  // Too little
		{Index: 3, MemoryMB: 10240, Name: "GPU3"}, // Better fit than GPU1
	}

	// Request 8GB
	selected, err := strategy.SelectGPUs(gpus, 1, 8192)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(selected))
	// Should select GPU0 (exact match)
	assert.Equal(t, 0, selected[0].Index)
	assert.Equal(t, "best-fit", strategy.Name())
}

func TestBestFitStrategy_NoMemoryRequirement(t *testing.T) {
	strategy := &BestFitStrategy{}

	gpus := []*GPU{
		{Index: 0, MemoryMB: 16384, Name: "GPU0"},
		{Index: 1, MemoryMB: 8192, Name: "GPU1"},
		{Index: 2, MemoryMB: 4096, Name: "GPU2"},
	}

	// No memory requirement - should prefer smaller GPUs
	selected, err := strategy.SelectGPUs(gpus, 1, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(selected))
	// Should select GPU2 (smallest)
	assert.Equal(t, 2, selected[0].Index)
}

func TestBestFitStrategy_InsufficientMemory(t *testing.T) {
	strategy := &BestFitStrategy{}

	gpus := []*GPU{
		{Index: 0, MemoryMB: 4096, Name: "GPU0"},
		{Index: 1, MemoryMB: 2048, Name: "GPU1"},
	}

	// Request more memory than any GPU has - should select largest available
	selected, err := strategy.SelectGPUs(gpus, 1, 8192)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(selected))
	// Should select GPU0 (largest available, even though insufficient)
	assert.Equal(t, 0, selected[0].Index)
}

func TestGetAllocationStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		expected string
	}{
		{"FirstFit", "first-fit", "first-fit"},
		{"Pack", "pack", "pack"},
		{"Spread", "spread", "spread"},
		{"BestFit", "best-fit", "best-fit"},
		{"Empty", "", "first-fit"},          // Default
		{"Unknown", "invalid", "first-fit"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := GetAllocationStrategy(tt.strategy)
			assert.Equal(t, tt.expected, strategy.Name())
		})
	}
}

func TestValidateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		wantErr  bool
	}{
		{"ValidFirstFit", "first-fit", false},
		{"ValidPack", "pack", false},
		{"ValidSpread", "spread", false},
		{"ValidBestFit", "best-fit", false},
		{"ValidEmpty", "", false},
		{"Invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrategy(tt.strategy)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAvailableStrategies(t *testing.T) {
	strategies := GetAvailableStrategies()
	expected := []string{"first-fit", "pack", "spread", "best-fit"}
	assert.Equal(t, expected, strategies)
}
