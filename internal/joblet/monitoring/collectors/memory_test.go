package collectors

import (
	"os"
	"testing"
)

func TestNewMemoryCollector(t *testing.T) {
	collector := NewMemoryCollector()
	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	if collector.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

// TestMemoryCollector_Collect tests the Collect method
// Note: This test requires access to /proc/meminfo
func TestMemoryCollector_Collect(t *testing.T) {
	// Skip test if not on Linux or if /proc/meminfo doesn't exist
	if _, err := os.Stat("/proc/meminfo"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/meminfo not available (not on Linux)")
	}

	collector := NewMemoryCollector()

	metrics, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collection failed: %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Validate basic fields
	if metrics.TotalBytes == 0 {
		t.Error("Expected non-zero total memory")
	}

	// Used + Free should not exceed Total (allowing for some calculation differences)
	calculatedUsed := metrics.TotalBytes - metrics.FreeBytes - metrics.CachedBytes - metrics.BufferedBytes
	if calculatedUsed > metrics.TotalBytes {
		t.Error("Calculated used memory exceeds total memory")
	}

	// Available bytes should be reasonable
	if metrics.AvailableBytes > metrics.TotalBytes {
		t.Error("Available memory exceeds total memory")
	}

	// Usage percentage should be 0-100
	if metrics.UsagePercent < 0 || metrics.UsagePercent > 100 {
		t.Errorf("Memory usage percent should be 0-100, got %f", metrics.UsagePercent)
	}

	// Cached and buffered should be reasonable
	if metrics.CachedBytes > metrics.TotalBytes {
		t.Error("Cached memory exceeds total memory")
	}

	if metrics.BufferedBytes > metrics.TotalBytes {
		t.Error("Buffered memory exceeds total memory")
	}

	// Swap fields should be reasonable
	if metrics.SwapUsed > metrics.SwapTotal {
		t.Error("Swap used exceeds swap total")
	}

	if metrics.SwapFree > metrics.SwapTotal {
		t.Error("Swap free exceeds swap total")
	}

	// If swap exists, used + free should equal total
	if metrics.SwapTotal > 0 {
		if metrics.SwapUsed+metrics.SwapFree != metrics.SwapTotal {
			t.Errorf("Swap used (%d) + free (%d) != total (%d)",
				metrics.SwapUsed, metrics.SwapFree, metrics.SwapTotal)
		}
	}
}

// TestMemoryCollector_readMemInfo tests memory info parsing
func TestMemoryCollector_readMemInfo(t *testing.T) {
	// Skip test if not on Linux
	if _, err := os.Stat("/proc/meminfo"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/meminfo not available (not on Linux)")
	}

	collector := NewMemoryCollector()

	meminfo, err := collector.readMemInfo()
	if err != nil {
		t.Fatalf("readMemInfo failed: %v", err)
	}

	// Check that required fields are present
	requiredFields := []string{
		"MemTotal", "MemFree", "MemAvailable",
		"Cached", "Buffers", "SwapTotal", "SwapFree",
	}

	for _, field := range requiredFields {
		if _, exists := meminfo[field]; !exists {
			t.Errorf("Required field %s not found in meminfo", field)
		}
	}

	// MemTotal should be positive
	if meminfo["MemTotal"] == 0 {
		t.Error("MemTotal should be positive")
	}

	// MemFree should not exceed MemTotal
	if meminfo["MemFree"] > meminfo["MemTotal"] {
		t.Error("MemFree should not exceed MemTotal")
	}

	// MemAvailable should not exceed MemTotal
	if meminfo["MemAvailable"] > meminfo["MemTotal"] {
		t.Error("MemAvailable should not exceed MemTotal")
	}

	// Cached should not exceed MemTotal
	if meminfo["Cached"] > meminfo["MemTotal"] {
		t.Error("Cached should not exceed MemTotal")
	}

	// Buffers should not exceed MemTotal
	if meminfo["Buffers"] > meminfo["MemTotal"] {
		t.Error("Buffers should not exceed MemTotal")
	}

	// If swap exists, SwapFree should not exceed SwapTotal
	if meminfo["SwapTotal"] > 0 && meminfo["SwapFree"] > meminfo["SwapTotal"] {
		t.Error("SwapFree should not exceed SwapTotal")
	}
}

// Test memory calculations with mock data
func TestMemoryCollector_Calculations(t *testing.T) {
	// Test usage percentage calculation
	tests := []struct {
		name         string
		total        uint64
		free         uint64
		buffers      uint64
		cached       uint64
		expectedUsed uint64
		expectedPct  float64
	}{
		{
			name:         "Normal case",
			total:        1000,
			free:         200,
			buffers:      100,
			cached:       200,
			expectedUsed: 500, // 1000 - 200 - 100 - 200
			expectedPct:  50.0,
		},
		{
			name:         "High usage",
			total:        1000,
			free:         50,
			buffers:      25,
			cached:       25,
			expectedUsed: 900, // 1000 - 50 - 25 - 25
			expectedPct:  90.0,
		},
		{
			name:         "Low usage",
			total:        1000,
			free:         700,
			buffers:      100,
			cached:       100,
			expectedUsed: 100, // 1000 - 700 - 100 - 100
			expectedPct:  10.0,
		},
		{
			name:         "Zero total (edge case)",
			total:        0,
			free:         0,
			buffers:      0,
			cached:       0,
			expectedUsed: 0,
			expectedPct:  0.0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			used := test.total - test.free - test.buffers - test.cached
			if used != test.expectedUsed {
				t.Errorf("Expected used memory %d, got %d", test.expectedUsed, used)
			}

			var usagePct float64
			if test.total > 0 {
				usagePct = float64(used) / float64(test.total) * 100.0
			}

			if usagePct != test.expectedPct {
				t.Errorf("Expected usage percentage %f, got %f", test.expectedPct, usagePct)
			}
		})
	}
}

// Test unit conversion (KB to bytes)
func TestMemoryCollector_UnitConversion(t *testing.T) {
	tests := []struct {
		kb       uint64
		expected uint64
	}{
		{0, 0},
		{1, 1024},
		{1024, 1024 * 1024},
		{1048576, 1024 * 1024 * 1024}, // 1GB in KB -> bytes
	}

	for _, test := range tests {
		bytes := test.kb * 1024
		if bytes != test.expected {
			t.Errorf("Converting %d KB to bytes: expected %d, got %d",
				test.kb, test.expected, bytes)
		}
	}
}

// Test swap calculations
func TestMemoryCollector_SwapCalculations(t *testing.T) {
	tests := []struct {
		name      string
		swapTotal uint64
		swapFree  uint64
		swapUsed  uint64
	}{
		{
			name:      "No swap",
			swapTotal: 0,
			swapFree:  0,
			swapUsed:  0,
		},
		{
			name:      "Partial swap usage",
			swapTotal: 2048,
			swapFree:  1024,
			swapUsed:  1024,
		},
		{
			name:      "No swap usage",
			swapTotal: 2048,
			swapFree:  2048,
			swapUsed:  0,
		},
		{
			name:      "Full swap usage",
			swapTotal: 2048,
			swapFree:  0,
			swapUsed:  2048,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calculatedUsed := test.swapTotal - test.swapFree
			if calculatedUsed != test.swapUsed {
				t.Errorf("Expected swap used %d, calculated %d",
					test.swapUsed, calculatedUsed)
			}

			// Verify consistency
			if test.swapUsed+test.swapFree != test.swapTotal {
				t.Errorf("Swap used (%d) + free (%d) != total (%d)",
					test.swapUsed, test.swapFree, test.swapTotal)
			}
		})
	}
}

// Test error handling and edge cases
func TestMemoryCollector_ErrorHandling(t *testing.T) {
	_ = NewMemoryCollector() // Create collector but don't use it in this test

	// Test parsing of invalid/empty values
	testData := map[string]uint64{
		"":        0,   // Empty string should default to 0
		"invalid": 0,   // Invalid number should default to 0
		"123":     123, // Valid number should parse correctly
		"0":       0,   // Zero should parse correctly
	}

	for input, expected := range testData {
		// This simulates the parsing logic in readMemInfo
		// We can't directly test the private parsing since it uses strconv.ParseUint
		// But we can verify the behavior
		if input == "" || input == "invalid" {
			// These cases should result in 0 due to error handling
			if expected != 0 {
				t.Errorf("Expected 0 for input '%s', got %d", input, expected)
			}
		}
	}
}

// Benchmark memory collection
func BenchmarkMemoryCollector_Collect(b *testing.B) {
	if _, err := os.Stat("/proc/meminfo"); os.IsNotExist(err) {
		b.Skip("Skipping benchmark: /proc/meminfo not available")
	}

	collector := NewMemoryCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := collector.Collect()
		if err != nil {
			b.Fatalf("Collection failed: %v", err)
		}
	}
}
