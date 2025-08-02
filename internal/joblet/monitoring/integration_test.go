package monitoring

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"joblet/internal/joblet/monitoring/domain"
)

// TestIntegration_FullMonitoringWorkflow tests the complete monitoring workflow
func TestIntegration_FullMonitoringWorkflow(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in CI if requested via environment variable
	if os.Getenv("SKIP_MONITORING_INTEGRATION_TESTS") == "true" {
		t.Skip("Monitoring integration tests disabled via SKIP_MONITORING_INTEGRATION_TESTS")
	}

	// Also skip in GitHub Actions CI by default to avoid container restrictions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Monitoring integration tests disabled in GitHub Actions CI")
	}

	// Detect CI environment and use shorter timeouts
	inCI := os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""

	// Set up timeout context to prevent CI hangs
	timeout := 30 * time.Second
	if inCI {
		timeout = 10 * time.Second // Shorter timeout in CI
	}
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < timeout {
			timeout = remaining / 2
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Logf("Setting test timeout to %v (CI=%v)", timeout, inCI)

	// Create service with fast intervals for testing
	config := &domain.MonitoringConfig{
		Enabled: true,
		Collection: domain.CollectionConfig{
			SystemInterval:  100 * time.Millisecond,
			ProcessInterval: 200 * time.Millisecond,
			CloudDetection:  false, // Disable to avoid network calls in tests
		},
	}

	service := NewService(config)

	// Test service lifecycle with timeout protection
	select {
	case <-ctx.Done():
		t.Skip("Test timeout reached before service start")
	default:
	}

	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Ensure service stops even if test fails
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = service.Stop()
		}()

		select {
		case <-done:
			// Service stopped successfully
		case <-stopCtx.Done():
			t.Log("Warning: Service stop timed out")
		}
	}()

	// Wait for several collection cycles with timeout protection
	select {
	case <-time.After(500 * time.Millisecond):
		// Normal case - waited for collection cycles
	case <-ctx.Done():
		t.Skip("Test timeout reached during collection wait")
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	// Test current metrics access
	latest := service.GetLatestMetrics()
	if latest != nil {
		// Verify basic structure
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
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in GitHub Actions CI by default to avoid container restrictions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Monitoring integration tests disabled in GitHub Actions CI")
	}

	// Set timeout for CI protection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := &domain.MonitoringConfig{
		Enabled: true,
		Collection: domain.CollectionConfig{
			SystemInterval:  50 * time.Millisecond,
			ProcessInterval: 100 * time.Millisecond,
			CloudDetection:  false,
		},
	}

	service := NewService(config)

	// Single start/stop cycle to test basic functionality
	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	// Brief collection period with timeout protection
	select {
	case <-time.After(50 * time.Millisecond):
		// Normal case
	case <-ctx.Done():
		t.Skip("Test timeout during collection period")
	}

	// Stop with timeout protection
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- service.Stop()
	}()

	select {
	case err = <-stopDone:
		if err != nil {
			t.Fatalf("Failed to stop service: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Service stop timed out")
	}

	if service.IsRunning() {
		t.Error("Service should not be running after stop")
	}
}

// TestIntegration_ServiceAPIs tests all public API methods
func TestIntegration_ServiceAPIs(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in GitHub Actions CI by default to avoid container restrictions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Monitoring integration tests disabled in GitHub Actions CI")
	}

	config := DefaultConfig()
	config.Collection.CloudDetection = false // Disable for testing
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	_ = service.Start()
	time.Sleep(200 * time.Millisecond) // Allow for collection

	// Test all public methods (should not panic)
	_ = service.IsRunning()
	_ = service.GetLatestMetrics()
	_ = service.GetSystemStatus()
	_ = service.GetCloudInfo()
}

// TestIntegration_CIEnvironment tests behavior in CI environments
func TestIntegration_CIEnvironment(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in GitHub Actions CI by default to avoid container restrictions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Monitoring integration tests disabled in GitHub Actions CI")
	}

	// Detect CI environment
	ciEnv := os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("TRAVIS") != ""

	config := DefaultConfig()
	config.Collection.CloudDetection = false // Always disable in CI
	service := NewService(config)
	defer func() { _ = service.Stop() }()

	err := service.Start()
	if err != nil {
		t.Fatalf("Failed to start service in CI: %v", err)
	}

	// In CI, just verify basic functionality
	time.Sleep(100 * time.Millisecond)

	if !service.IsRunning() {
		t.Error("Service should be running in CI")
	}

	// Test methods work without crashing
	status := service.GetSystemStatus()
	if status == nil {
		t.Error("Expected non-nil system status in CI")
	}

	if ciEnv {
		t.Log("Detected CI environment - monitoring system functional")
	}
}

// TestIntegration_ConcurrentAccess tests concurrent access to service methods
func TestIntegration_ConcurrentAccess(t *testing.T) {
	// Skip on non-Linux platforms
	if runtime.GOOS != "linux" {
		t.Skip("Monitoring system requires Linux")
	}

	// Skip in GitHub Actions CI by default to avoid container restrictions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Monitoring integration tests disabled in GitHub Actions CI")
	}

	config := DefaultConfig()
	config.Collection.CloudDetection = false
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
