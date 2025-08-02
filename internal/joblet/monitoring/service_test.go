package monitoring

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if !config.Enabled {
		t.Error("Expected monitoring to be enabled by default")
	}

	if config.Collection.SystemInterval <= 0 {
		t.Error("Expected positive system interval")
	}

	if config.Collection.ProcessInterval <= 0 {
		t.Error("Expected positive process interval")
	}

	if !config.Collection.CloudDetection {
		t.Error("Expected cloud detection to be enabled by default")
	}

}

func TestNewService(t *testing.T) {
	config := DefaultConfig()
	service := NewService(config)

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	if service.config != config {
		t.Error("Config not properly stored")
	}

	if service.hostCollector == nil {
		t.Error("Expected host collector to be initialized")
	}

	if service.cpuCollector == nil {
		t.Error("Expected CPU collector to be initialized")
	}

	if service.memoryCollector == nil {
		t.Error("Expected memory collector to be initialized")
	}

	if service.diskCollector == nil {
		t.Error("Expected disk collector to be initialized")
	}

	if service.networkCollector == nil {
		t.Error("Expected network collector to be initialized")
	}

	if service.ioCollector == nil {
		t.Error("Expected I/O collector to be initialized")
	}

	if service.processCollector == nil {
		t.Error("Expected process collector to be initialized")
	}

	if service.cloudDetector == nil {
		t.Error("Expected cloud detector to be initialized")
	}
}

func TestNewService_NilConfig(t *testing.T) {
	service := NewService(nil)

	if service == nil {
		t.Fatal("Expected non-nil service even with nil config")
	}

	if service.config == nil {
		t.Error("Expected default config to be used")
	}

	// Should use default config values
	if !service.config.Enabled {
		t.Error("Expected default config to have monitoring enabled")
	}
}

func TestService_IsRunning(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI environments to avoid resource constraints
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring service tests disabled in CI environments")
	}

	// Use config with cloud detection disabled to avoid network timeouts
	config := DefaultConfig()
	config.Collection.CloudDetection = false
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	// Initially not running
	if service.IsRunning() {
		t.Error("Expected service to not be running initially")
	}

	// Start service
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Should be running
	if !service.IsRunning() {
		t.Error("Expected service to be running after Start()")
	}

	// Stop service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Should not be running
	if service.IsRunning() {
		t.Error("Expected service to not be running after Stop()")
	}
}

func TestService_StartStop(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI environments to avoid resource constraints
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring service tests disabled in CI environments")
	}

	service := NewService(DefaultConfig())

	// Test starting
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Test double start (should be safe)
	err = service.Start()
	if err != nil {
		t.Errorf("Double start should not error: %v", err)
	}

	// Test stopping
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Test double stop (should be safe)
	err = service.Stop()
	if err != nil {
		t.Errorf("Double stop should not error: %v", err)
	}
}

func TestService_DisabledService(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	service := NewService(config)

	err := service.Start()
	if err != nil {
		t.Errorf("Starting disabled service should not error: %v", err)
	}

	// Should not be running even after "start"
	if service.IsRunning() {
		t.Error("Disabled service should not be running")
	}

	err = service.Stop()
	if err != nil {
		t.Errorf("Stopping disabled service should not error: %v", err)
	}
}

func TestService_GetLatestMetrics(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI environments to avoid resource constraints
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring service tests disabled in CI environments")
	}

	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	// Initially no metrics
	latest := service.GetLatestMetrics()
	if latest != nil {
		t.Error("Expected nil metrics initially")
	}

	// Start service and wait a bit for collection
	_ = service.Start()
	time.Sleep(100 * time.Millisecond)

	// Should have metrics now (might still be nil if collection hasn't completed)
	_ = service.GetLatestMetrics()
	// We can't guarantee metrics will be available immediately, so just check it doesn't crash
}

func TestService_GetSystemStatus(t *testing.T) {
	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	status := service.GetSystemStatus()

	if status == nil {
		t.Fatal("Expected non-nil system status")
	}

	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	// Initially should not be available (no metrics collected yet)
	if status.Available {
		// This might be true if collection happens very quickly
		// Just verify the structure is there
		if status.Host.Hostname == "" && status.Host.OS == "" {
			t.Error("If available=true, should have host info")
		}
	}

}

func TestService_GetCloudInfo(t *testing.T) {
	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	// Initially no cloud info
	cloudInfo := service.GetCloudInfo()
	if cloudInfo != nil {
		// Cloud info might be available if detection is very fast
		// Just verify it's structured correctly
		if cloudInfo.Provider == "" {
			t.Error("If cloud info is available, should have provider")
		}
	}
}

// Test concurrent access to service methods
func TestService_ConcurrentAccess(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI environments to avoid resource constraints
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring service tests disabled in CI environments")
	}

	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	// Start service
	_ = service.Start()

	// Run multiple goroutines accessing service methods
	done := make(chan bool, 3)

	// Goroutine 1: GetLatestMetrics
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_ = service.GetLatestMetrics()
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 2: GetSystemStatus
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_ = service.GetSystemStatus()
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 3: GetCloudInfo
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_ = service.GetCloudInfo()
			time.Sleep(time.Millisecond)
		}
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should still be running
	if !service.IsRunning() {
		t.Error("Service should still be running after concurrent access")
	}
}

// Test service with fast collection intervals
func TestService_FastCollection(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI environments to avoid resource constraints
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("CI") == "true" || os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring service tests disabled in CI environments")
	}

	config := DefaultConfig()
	config.Collection.SystemInterval = 10 * time.Millisecond
	config.Collection.ProcessInterval = 50 * time.Millisecond

	service := NewService(config)
	defer func() { _ = service.Stop() }()

	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for several collections
	time.Sleep(100 * time.Millisecond)

	// Should have collected some metrics
	latest := service.GetLatestMetrics()
	// Might have metrics if collection is working
	_ = latest
}

// Benchmark service operations
func BenchmarkService_GetLatestMetrics(b *testing.B) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		b.Skip("Monitoring system requires Linux")
	}

	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	_ = service.Start()
	time.Sleep(50 * time.Millisecond) // Let it collect some data

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetLatestMetrics()
	}
}

func BenchmarkService_GetSystemStatus(b *testing.B) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		b.Skip("Monitoring system requires Linux")
	}

	service := NewService(DefaultConfig())
	defer func() { _ = service.Stop() }()

	_ = service.Start()
	time.Sleep(50 * time.Millisecond) // Let it collect some data

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetSystemStatus()
	}
}
