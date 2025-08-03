package collectors

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewCPUCollector(t *testing.T) {
	collector := NewCPUCollector()
	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	if collector.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestCPUCollector_parseUint64(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"123", 123},
		{"0", 0},
		{"999999", 999999},
		{"invalid", 0}, // Should return 0 for invalid input
		{"", 0},        // Should return 0 for empty input
	}

	for _, test := range tests {
		result := parseUint64(test.input)
		if result != test.expected {
			t.Errorf("parseUint64(%s) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestCPUCollector_calculateTotalDelta(t *testing.T) {
	collector := NewCPUCollector()

	current := &cpuStats{
		user:    100,
		nice:    10,
		system:  50,
		idle:    1000,
		iowait:  20,
		irq:     5,
		softirq: 15,
		steal:   2,
	}

	last := &cpuStats{
		user:    80,
		nice:    8,
		system:  40,
		idle:    800,
		iowait:  15,
		irq:     3,
		softirq: 10,
		steal:   1,
	}

	delta := collector.calculateTotalDelta(current, last)

	// Expected: (100+10+50+1000+20+5+15+2) - (80+8+40+800+15+3+10+1)
	// = 1202 - 957 = 245
	expected := float64(245)

	if delta != expected {
		t.Errorf("calculateTotalDelta() = %f, expected %f", delta, expected)
	}
}

func TestCPUCollector_calculateCoreTotalDelta(t *testing.T) {
	collector := NewCPUCollector()

	current := &coreStats{
		user:    50,
		nice:    5,
		system:  25,
		idle:    500,
		iowait:  10,
		irq:     2,
		softirq: 8,
		steal:   1,
	}

	last := &coreStats{
		user:    40,
		nice:    4,
		system:  20,
		idle:    400,
		iowait:  8,
		irq:     1,
		softirq: 6,
		steal:   0,
	}

	delta := collector.calculateCoreTotalDelta(current, last)

	// Expected: (50+5+25+500+10+2+8+1) - (40+4+20+400+8+1+6+0)
	// = 601 - 479 = 122
	expected := float64(122)

	if delta != expected {
		t.Errorf("calculateCoreTotalDelta() = %f, expected %f", delta, expected)
	}
}

// TestCPUCollector_Collect tests the Collect method
// Note: This test requires access to /proc/stat and /proc/loadavg
func TestCPUCollector_Collect(t *testing.T) {
	// Skip test if not on Linux or if /proc/stat doesn't exist
	if _, err := os.Stat("/proc/stat"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/stat not available (not on Linux)")
	}

	// Skip in CI environments to avoid resource constraints and proc filesystem issues
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("CPU collector tests disabled in CI environments")
	}

	collector := NewCPUCollector()

	// First collection
	metrics1, err := collector.Collect()
	if err != nil {
		t.Fatalf("First collection failed: %v", err)
	}

	if metrics1 == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Validate basic fields
	if metrics1.Cores <= 0 {
		t.Error("Expected positive core count")
	}

	if len(metrics1.LoadAverage) != 3 {
		t.Error("Expected load average array of length 3")
	}

	// Load averages should be non-negative
	for i, load := range metrics1.LoadAverage {
		if load < 0 {
			t.Errorf("Load average[%d] should be non-negative, got %f", i, load)
		}
	}

	// Per-core usage should have correct length
	if len(metrics1.PerCoreUsage) != metrics1.Cores {
		t.Errorf("Expected %d per-core usage values, got %d", metrics1.Cores, len(metrics1.PerCoreUsage))
	}

	// Wait a bit and collect again to test percentage calculations
	time.Sleep(100 * time.Millisecond)

	metrics2, err := collector.Collect()
	if err != nil {
		t.Fatalf("Second collection failed: %v", err)
	}

	// After second collection, we should have usage percentages
	if metrics2.UsagePercent < 0 || metrics2.UsagePercent > 100 {
		t.Errorf("CPU usage percent should be 0-100, got %f", metrics2.UsagePercent)
	}

	if metrics2.UserTime < 0 || metrics2.UserTime > 100 {
		t.Errorf("User time should be 0-100, got %f", metrics2.UserTime)
	}

	if metrics2.SystemTime < 0 || metrics2.SystemTime > 100 {
		t.Errorf("System time should be 0-100, got %f", metrics2.SystemTime)
	}

	if metrics2.IdleTime < 0 || metrics2.IdleTime > 100 {
		t.Errorf("Idle time should be 0-100, got %f", metrics2.IdleTime)
	}

	// Per-core usage values should be reasonable
	for i, usage := range metrics2.PerCoreUsage {
		if usage < 0 || usage > 100 {
			t.Errorf("Per-core usage[%d] should be 0-100, got %f", i, usage)
		}
	}
}

// TestCPUCollector_readLoadAverage tests load average parsing
func TestCPUCollector_readLoadAverage(t *testing.T) {
	// Skip test if not on Linux
	if _, err := os.Stat("/proc/loadavg"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/loadavg not available (not on Linux)")
	}

	collector := NewCPUCollector()

	loadAvg, err := collector.readLoadAverage()
	if err != nil {
		t.Fatalf("readLoadAverage failed: %v", err)
	}

	// Load averages should be non-negative
	for i, load := range loadAvg {
		if load < 0 {
			t.Errorf("Load average[%d] should be non-negative, got %f", i, load)
		}
	}

	// Load averages are typically reasonable values (0-100 on most systems)
	// But we won't enforce strict limits as they can be higher under extreme load
}

// Mock test for readCPUStats functionality
func TestCPUCollector_parseCPUStatsLine(t *testing.T) {
	// This tests the parsing logic with mock data
	_ = NewCPUCollector() // Create collector but don't use it in this test

	// Test overall CPU line parsing
	cpuLine := "cpu  12345 678 9012 345678 901 234 567 89 123 456"
	fields := strings.Fields(cpuLine)

	if len(fields) < 8 {
		t.Fatal("Test data invalid")
	}

	// Test parsing of individual fields (this mimics the parsing in readCPUStats)
	user := parseUint64(fields[1])
	nice := parseUint64(fields[2])
	system := parseUint64(fields[3])
	idle := parseUint64(fields[4])
	iowait := parseUint64(fields[5])
	irq := parseUint64(fields[6])
	softirq := parseUint64(fields[7])

	if user != 12345 {
		t.Errorf("Expected user=12345, got %d", user)
	}
	if nice != 678 {
		t.Errorf("Expected nice=678, got %d", nice)
	}
	if system != 9012 {
		t.Errorf("Expected system=9012, got %d", system)
	}
	if idle != 345678 {
		t.Errorf("Expected idle=345678, got %d", idle)
	}
	if iowait != 901 {
		t.Errorf("Expected iowait=901, got %d", iowait)
	}
	if irq != 234 {
		t.Errorf("Expected irq=234, got %d", irq)
	}
	if softirq != 567 {
		t.Errorf("Expected softirq=567, got %d", softirq)
	}

	// Test optional fields
	if len(fields) > 8 {
		steal := parseUint64(fields[8])
		if steal != 89 {
			t.Errorf("Expected steal=89, got %d", steal)
		}
	}

	if len(fields) > 9 {
		guest := parseUint64(fields[9])
		if guest != 123 {
			t.Errorf("Expected guest=123, got %d", guest)
		}
	}

	if len(fields) > 10 {
		guestNice := parseUint64(fields[10])
		if guestNice != 456 {
			t.Errorf("Expected guestNice=456, got %d", guestNice)
		}
	}
}

// Test per-core CPU parsing
func TestCPUCollector_parseCoreCPULine(t *testing.T) {
	// Test per-core CPU line parsing
	coreLine := "cpu0 1234 56 789 12345 67 89 123 45"
	fields := strings.Fields(coreLine)

	if len(fields) < 8 {
		t.Fatal("Test data invalid")
	}

	// Verify it's a core line
	if !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
		t.Error("Expected core CPU line")
	}

	user := parseUint64(fields[1])
	nice := parseUint64(fields[2])
	system := parseUint64(fields[3])
	idle := parseUint64(fields[4])

	if user != 1234 {
		t.Errorf("Expected core user=1234, got %d", user)
	}
	if nice != 56 {
		t.Errorf("Expected core nice=56, got %d", nice)
	}
	if system != 789 {
		t.Errorf("Expected core system=789, got %d", system)
	}
	if idle != 12345 {
		t.Errorf("Expected core idle=12345, got %d", idle)
	}
}

// Test edge cases and error conditions
func TestCPUCollector_EdgeCases(t *testing.T) {
	collector := NewCPUCollector()

	// Test with zero deltas (should not crash)
	current := &cpuStats{user: 100, idle: 100}
	last := &cpuStats{user: 100, idle: 100}

	delta := collector.calculateTotalDelta(current, last)
	if delta != 0 {
		t.Errorf("Expected zero delta, got %f", delta)
	}

	// Test with very small deltas
	current = &cpuStats{user: 101, idle: 101}
	last = &cpuStats{user: 100, idle: 100}

	delta = collector.calculateTotalDelta(current, last)
	if delta != 2 {
		t.Errorf("Expected delta=2, got %f", delta)
	}
}
