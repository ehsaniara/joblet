package monitoring

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/monitoring/domain"
)

// Test helper - skip if not on Linux or in CI
func skipIfNotLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Requires Linux")
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Disabled in CI")
	}
}

// Test helper - create test config
func testConfig() *domain.MonitoringConfig {
	return &domain.MonitoringConfig{
		Enabled: true,
		Collection: domain.CollectionConfig{
			SystemInterval: 100 * time.Millisecond,
			CloudDetection: false, // Disable to avoid network calls
		},
	}
}

// TestIntegration_FullMonitoringWorkflow tests the complete monitoring workflow
func TestIntegration_FullMonitoringWorkflow(t *testing.T) {
	skipIfNotLinux(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	service := NewService(testConfig())
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer func() { _ = service.Stop() }()

	// Wait for collection cycles
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("Test timeout")
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	// Test current metrics access
	latest := service.GetLatestMetrics()
	if latest != nil {
		if latest.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}
		if latest.Host.Hostname == "" {
			t.Error("Expected hostname to be populated")
		}
	}

	// Test system status
	status := service.GetSystemStatus()
	if status == nil {
		t.Error("Expected non-nil system status")
	} else if status.Available && status.Host.Hostname == "" {
		t.Error("If status is available, host info should be populated")
	}

	// Test cloud info (should be nil with detection disabled)
	cloudInfo := service.GetCloudInfo()
	if cloudInfo != nil {
		t.Error("Expected nil cloud info with detection disabled")
	}
}

// TestIntegration_QuickLifecycle tests service start/stop operations
func TestIntegration_QuickLifecycle(t *testing.T) {
	skipIfNotLinux(t)

	service := NewService(testConfig())

	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	time.Sleep(50 * time.Millisecond)

	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	if service.IsRunning() {
		t.Error("Service should not be running after stop")
	}
}

// TestIntegration_ServiceAPIs tests all public API methods
func TestIntegration_ServiceAPIs(t *testing.T) {
	skipIfNotLinux(t)

	config := testConfig()
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	_ = service.Start()
	time.Sleep(200 * time.Millisecond)

	// Test all public methods (should not panic)
	_ = service.IsRunning()
	_ = service.GetLatestMetrics()
	_ = service.GetSystemStatus()
	_ = service.GetCloudInfo()
}

// TestIntegration_CIEnvironment tests behavior in CI environments
func TestIntegration_CIEnvironment(t *testing.T) {
	skipIfNotLinux(t)

	config := testConfig()
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	status := service.GetSystemStatus()
	if status == nil {
		t.Error("Expected non-nil system status")
	}
}

// TestIntegration_ConcurrentAccess tests concurrent access to service methods
func TestIntegration_ConcurrentAccess(t *testing.T) {
	skipIfNotLinux(t)

	config := testConfig()
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	_ = service.Start()
	time.Sleep(100 * time.Millisecond)

	// Run concurrent operations
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 5; j++ {
				_ = service.GetLatestMetrics()
				_ = service.GetSystemStatus()
				_ = service.GetCloudInfo()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should still be running
	if !service.IsRunning() {
		t.Error("Service should still be running after concurrent access")
	}
}
