package adapters

import (
	"testing"
	"time"

	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Simple integration test focusing on async log system integration
func TestJobStoreAdapter_AsyncLogSystemIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Test that async log system is properly created and integrated
	logConfig := &config.LogPersistenceConfig{
		Directory:         tempDir,
		RetentionDays:     1,
		RotationSizeBytes: 1024,
		QueueSize:         100,
		MemoryLimit:       1024 * 1024,
		BatchSize:         10,
		FlushInterval:     50 * time.Millisecond,
		OverflowStrategy:  "compress",
	}

	testLogger := logger.New()

	// Test async log system creation
	asyncLogSystem := NewAsyncLogSystem(logConfig, testLogger)
	if asyncLogSystem == nil {
		t.Fatal("Failed to create async log system")
	}
	defer asyncLogSystem.Close()

	// Test basic functionality
	testJobID := "integration-test-job"
	testData := []byte("Integration test log message\n")

	// This should never block
	start := time.Now()
	asyncLogSystem.WriteLog(testJobID, testData)
	elapsed := time.Since(start)

	// Should complete in microseconds, not milliseconds
	if elapsed > time.Millisecond {
		t.Errorf("WriteLog took too long: %v (should be < 1ms)", elapsed)
	}

	// Wait for background processing
	time.Sleep(200 * time.Millisecond)

	// Verify metrics show data was processed
	metrics := asyncLogSystem.GetMetrics()
	if metrics.TotalBytesWritten == 0 {
		t.Error("Expected bytes to be written through async system")
	}

	t.Logf("Integration test completed: %d bytes processed", metrics.TotalBytesWritten)
}

func TestAsyncLogSystem_ConfigurationIntegration(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name        string
		config      *config.LogPersistenceConfig
		expectError bool
	}{
		{
			name: "default configuration",
			config: &config.LogPersistenceConfig{
				Directory: tempDir,
			},
		},
		{
			name: "custom small queue",
			config: &config.LogPersistenceConfig{
				Directory:        tempDir,
				QueueSize:        10,
				BatchSize:        2,
				FlushInterval:    10 * time.Millisecond,
				OverflowStrategy: "spill",
			},
		},
		{
			name: "high performance config",
			config: &config.LogPersistenceConfig{
				Directory:        tempDir,
				QueueSize:        10000,
				MemoryLimit:      100 * 1024 * 1024, // 100MB
				BatchSize:        100,
				FlushInterval:    1 * time.Millisecond,
				OverflowStrategy: "compress",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testLogger := logger.New()
			asyncLogSystem := NewAsyncLogSystem(tc.config, testLogger)

			if asyncLogSystem == nil {
				if !tc.expectError {
					t.Fatal("Expected async log system to be created")
				}
				return
			}
			defer asyncLogSystem.Close()

			// Test basic functionality with this configuration
			testData := []byte("Config test message\n")
			asyncLogSystem.WriteLog("config-test", testData)

			// Short wait for processing
			time.Sleep(50 * time.Millisecond)

			metrics := asyncLogSystem.GetMetrics()
			if metrics.TotalBytesWritten == 0 {
				t.Error("Expected data to be processed")
			}
		})
	}
}

func TestAsyncLogSystem_PerformanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()

	config := &config.LogPersistenceConfig{
		Directory:        tempDir,
		QueueSize:        100000,
		MemoryLimit:      1024 * 1024 * 1024,
		BatchSize:        100,
		FlushInterval:    100 * time.Millisecond,
		OverflowStrategy: "compress",
	}

	testLogger := logger.New()
	asyncLogSystem := NewAsyncLogSystem(config, testLogger)
	defer asyncLogSystem.Close()

	// Test performance with rapid writes
	numWrites := 10000
	testData := []byte("Performance test log message\n")

	start := time.Now()
	for i := 0; i < numWrites; i++ {
		asyncLogSystem.WriteLog("perf-test", testData)
	}
	elapsed := time.Since(start)

	// Should handle 10k writes very quickly
	throughput := float64(numWrites) / elapsed.Seconds()
	t.Logf("Throughput: %.0f writes/second", throughput)

	if throughput < 100000 { // Should handle at least 100k writes/second
		t.Errorf("Performance too low: %.0f writes/second", throughput)
	}

	// Wait for background processing to complete
	time.Sleep(500 * time.Millisecond)

	metrics := asyncLogSystem.GetMetrics()
	expectedBytes := int64(numWrites * len(testData))
	if metrics.TotalBytesWritten != expectedBytes {
		t.Errorf("Expected %d bytes written, got %d", expectedBytes, metrics.TotalBytesWritten)
	}
}
