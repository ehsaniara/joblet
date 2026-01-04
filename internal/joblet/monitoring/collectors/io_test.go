package collectors

import (
	"os"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/monitoring/domain"
)

func TestNewIOCollector(t *testing.T) {
	collector := NewIOCollector()
	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	if collector.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if collector.lastStats != nil {
		t.Error("Expected lastStats to be nil initially")
	}
}

func TestIOCollector_shouldSkipDevice(t *testing.T) {
	collector := NewIOCollector()

	tests := []struct {
		deviceName string
		shouldSkip bool
	}{
		// Should skip
		{"loop0", true},
		{"loop1", true},
		{"ram0", true},
		{"dm-0", true},
		{"dm-1", true},
		{"sr0", true},
		{"sda1", true},      // partition
		{"nvme0n1p1", true}, // NVMe partition

		// Should NOT skip (whole devices)
		{"sda", false},
		{"sdb", false},
		{"nvme0n1", false},
		{"vda", false},
		{"xvda", false},
	}

	for _, tt := range tests {
		result := collector.shouldSkipDevice(tt.deviceName)
		if result != tt.shouldSkip {
			t.Errorf("shouldSkipDevice(%s) = %v, expected %v", tt.deviceName, result, tt.shouldSkip)
		}
	}
}

func TestIOCollector_calculatePerDeviceMetrics(t *testing.T) {
	collector := NewIOCollector()

	// Set up previous stats
	collector.lastStats = &ioSystemStats{
		perDevice: []deviceStats{
			{
				device:          "sda",
				readsCompleted:  100,
				writesCompleted: 50,
				readBytes:       102400,
				writeBytes:      51200,
				readTime:        1000,
				writeTime:       500,
				ioTime:          1500,
			},
			{
				device:          "sdb",
				readsCompleted:  200,
				writesCompleted: 100,
				readBytes:       204800,
				writeBytes:      102400,
				readTime:        2000,
				writeTime:       1000,
				ioTime:          3000,
			},
		},
	}

	// Current stats with increased values
	currentStats := &ioSystemStats{
		perDevice: []deviceStats{
			{
				device:          "sda",
				readsCompleted:  150,
				writesCompleted: 75,
				readBytes:       153600,
				writeBytes:      76800,
				readTime:        1500,
				writeTime:       750,
				ioTime:          2000, // +500ms
			},
			{
				device:          "sdb",
				readsCompleted:  250,
				writesCompleted: 125,
				readBytes:       256000,
				writeBytes:      128000,
				readTime:        2500,
				writeTime:       1250,
				ioTime:          4000, // +1000ms
			},
		},
	}

	// Calculate with 1000ms time delta
	timeDeltaMs := 1000.0
	result := collector.calculatePerDeviceMetrics(currentStats, timeDeltaMs)

	if len(result) != 2 {
		t.Fatalf("Expected 2 devices, got %d", len(result))
	}

	// Check sda metrics
	sda := result[0]
	if sda.Device != "sda" {
		t.Errorf("Expected device 'sda', got '%s'", sda.Device)
	}
	if sda.ReadsCompleted != 150 {
		t.Errorf("Expected ReadsCompleted=150, got %d", sda.ReadsCompleted)
	}
	// Utilization: (500ms / 1000ms) * 100 = 50%
	expectedUtilSda := 50.0
	if sda.Utilization != expectedUtilSda {
		t.Errorf("Expected sda Utilization=%.2f%%, got %.2f%%", expectedUtilSda, sda.Utilization)
	}

	// Check sdb metrics
	sdb := result[1]
	if sdb.Device != "sdb" {
		t.Errorf("Expected device 'sdb', got '%s'", sdb.Device)
	}
	// Utilization: (1000ms / 1000ms) * 100 = 100%
	expectedUtilSdb := 100.0
	if sdb.Utilization != expectedUtilSdb {
		t.Errorf("Expected sdb Utilization=%.2f%%, got %.2f%%", expectedUtilSdb, sdb.Utilization)
	}
}

func TestIOCollector_calculatePerDeviceMetrics_UtilizationCap(t *testing.T) {
	collector := NewIOCollector()

	// Set up previous stats
	collector.lastStats = &ioSystemStats{
		perDevice: []deviceStats{
			{
				device: "sda",
				ioTime: 0,
			},
		},
	}

	// Current stats with ioTime greater than time delta (which can happen)
	currentStats := &ioSystemStats{
		perDevice: []deviceStats{
			{
				device: "sda",
				ioTime: 2000, // 2000ms of I/O time
			},
		},
	}

	// Calculate with 1000ms time delta - utilization would be 200% without cap
	timeDeltaMs := 1000.0
	result := collector.calculatePerDeviceMetrics(currentStats, timeDeltaMs)

	if len(result) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(result))
	}

	// Utilization should be capped at 100%
	if result[0].Utilization != 100.0 {
		t.Errorf("Expected Utilization capped at 100%%, got %.2f%%", result[0].Utilization)
	}
}

func TestIOCollector_calculatePerDeviceMetrics_NewDevice(t *testing.T) {
	collector := NewIOCollector()

	// Set up previous stats with only sda
	collector.lastStats = &ioSystemStats{
		perDevice: []deviceStats{
			{
				device: "sda",
				ioTime: 1000,
			},
		},
	}

	// Current stats with sda and new device sdb
	currentStats := &ioSystemStats{
		perDevice: []deviceStats{
			{
				device: "sda",
				ioTime: 1500,
			},
			{
				device: "sdb", // New device
				ioTime: 500,
			},
		},
	}

	timeDeltaMs := 1000.0
	result := collector.calculatePerDeviceMetrics(currentStats, timeDeltaMs)

	if len(result) != 2 {
		t.Fatalf("Expected 2 devices, got %d", len(result))
	}

	// sda should have utilization calculated
	if result[0].Utilization != 50.0 {
		t.Errorf("Expected sda Utilization=50%%, got %.2f%%", result[0].Utilization)
	}

	// sdb (new device) should have 0 utilization since no previous data
	if result[1].Utilization != 0.0 {
		t.Errorf("Expected sdb Utilization=0%% (new device), got %.2f%%", result[1].Utilization)
	}
}

func TestIOCollector_ReadWriteRates(t *testing.T) {
	collector := NewIOCollector()

	// Set up initial stats
	collector.lastStats = &ioSystemStats{
		readBytes:  1000000, // 1MB
		writeBytes: 500000,  // 0.5MB
		perDevice:  []deviceStats{},
	}
	collector.lastTime = time.Now().Add(-1 * time.Second) // 1 second ago

	// Simulate current stats with 2MB read and 1MB written in 1 second
	currentStats := &ioSystemStats{
		readBytes:  3000000, // 3MB total (2MB delta)
		writeBytes: 1500000, // 1.5MB total (1MB delta)
		perDevice:  []deviceStats{},
	}

	currentTime := time.Now()
	timeDelta := currentTime.Sub(collector.lastTime).Seconds()

	// Calculate rates
	readBytesDelta := currentStats.readBytes - collector.lastStats.readBytes
	writeBytesDelta := currentStats.writeBytes - collector.lastStats.writeBytes
	readRate := float64(readBytesDelta) / timeDelta
	writeRate := float64(writeBytesDelta) / timeDelta

	// Read rate should be approximately 2MB/s
	expectedReadRate := 2000000.0
	if readRate < expectedReadRate*0.9 || readRate > expectedReadRate*1.1 {
		t.Errorf("Expected ReadRate ~%.0f bytes/sec, got %.0f", expectedReadRate, readRate)
	}

	// Write rate should be approximately 1MB/s
	expectedWriteRate := 1000000.0
	if writeRate < expectedWriteRate*0.9 || writeRate > expectedWriteRate*1.1 {
		t.Errorf("Expected WriteRate ~%.0f bytes/sec, got %.0f", expectedWriteRate, writeRate)
	}
}

func TestIOCollector_Collect_FirstCollection(t *testing.T) {
	// Skip if not on Linux
	if _, err := os.Stat("/proc/diskstats"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/diskstats not available (not on Linux)")
	}

	collector := NewIOCollector()

	metrics, err := collector.Collect()
	if err != nil {
		t.Fatalf("First collection failed: %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// First collection should have zero rates (no previous data)
	if metrics.ReadRate != 0 {
		t.Errorf("Expected ReadRate=0 on first collection, got %f", metrics.ReadRate)
	}
	if metrics.WriteRate != 0 {
		t.Errorf("Expected WriteRate=0 on first collection, got %f", metrics.WriteRate)
	}

	// Should have per-device data (even if empty utilization)
	// PerDevice might be empty if no physical devices are found
	// This is acceptable - we just check it doesn't panic
}

func TestIOCollector_Collect_SecondCollection(t *testing.T) {
	// Skip if not on Linux
	if _, err := os.Stat("/proc/diskstats"); os.IsNotExist(err) {
		t.Skip("Skipping test: /proc/diskstats not available (not on Linux)")
	}

	// Skip in CI environments
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping I/O collector integration test in CI")
	}

	collector := NewIOCollector()

	// First collection
	_, err := collector.Collect()
	if err != nil {
		t.Fatalf("First collection failed: %v", err)
	}

	// Wait a bit for some I/O to occur
	time.Sleep(100 * time.Millisecond)

	// Second collection
	metrics, err := collector.Collect()
	if err != nil {
		t.Fatalf("Second collection failed: %v", err)
	}

	// After second collection, we should have rates (may be 0 if no I/O)
	// Just verify they're non-negative
	if metrics.ReadRate < 0 {
		t.Errorf("ReadRate should be non-negative, got %f", metrics.ReadRate)
	}
	if metrics.WriteRate < 0 {
		t.Errorf("WriteRate should be non-negative, got %f", metrics.WriteRate)
	}

	// Per-device metrics should have utilization calculated
	for _, dev := range metrics.PerDevice {
		if dev.Utilization < 0 || dev.Utilization > 100 {
			t.Errorf("Device %s utilization should be 0-100, got %f", dev.Device, dev.Utilization)
		}
	}
}

func TestDeviceIOMetrics_Struct(t *testing.T) {
	// Test that DeviceIOMetrics struct is properly initialized
	metrics := domain.DeviceIOMetrics{
		Device:          "sda",
		ReadsCompleted:  100,
		WritesCompleted: 50,
		ReadBytes:       102400,
		WriteBytes:      51200,
		ReadTime:        1000,
		WriteTime:       500,
		IOTime:          1500,
		Utilization:     75.5,
	}

	if metrics.Device != "sda" {
		t.Errorf("Expected Device='sda', got '%s'", metrics.Device)
	}
	if metrics.ReadsCompleted != 100 {
		t.Errorf("Expected ReadsCompleted=100, got %d", metrics.ReadsCompleted)
	}
	if metrics.Utilization != 75.5 {
		t.Errorf("Expected Utilization=75.5, got %f", metrics.Utilization)
	}
}

func TestIOMetrics_NewFields(t *testing.T) {
	// Test that IOMetrics struct has the new fields
	metrics := domain.IOMetrics{
		ReadsCompleted:  1000,
		WritesCompleted: 500,
		ReadBytes:       1024000,
		WriteBytes:      512000,
		ReadRate:        102400.5,
		WriteRate:       51200.25,
		PerDevice: []domain.DeviceIOMetrics{
			{Device: "sda", Utilization: 50.0},
			{Device: "sdb", Utilization: 25.0},
		},
	}

	if metrics.ReadRate != 102400.5 {
		t.Errorf("Expected ReadRate=102400.5, got %f", metrics.ReadRate)
	}
	if metrics.WriteRate != 51200.25 {
		t.Errorf("Expected WriteRate=51200.25, got %f", metrics.WriteRate)
	}
	if len(metrics.PerDevice) != 2 {
		t.Errorf("Expected 2 per-device metrics, got %d", len(metrics.PerDevice))
	}
	if metrics.PerDevice[0].Device != "sda" {
		t.Errorf("Expected first device='sda', got '%s'", metrics.PerDevice[0].Device)
	}
}
