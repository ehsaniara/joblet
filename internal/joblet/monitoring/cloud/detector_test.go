package cloud

import (
	"context"
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	detector := NewDetector()

	if detector == nil {
		t.Fatal("NewDetector() returned nil")
	}

	if detector.logger == nil {
		t.Error("logger not initialized")
	}

	if detector.client == nil {
		t.Error("HTTP client not initialized")
	}

	if detector.client.Timeout != DefaultDetectionTimeout {
		t.Errorf("client timeout = %v, want %v", detector.client.Timeout, DefaultDetectionTimeout)
	}
}

func TestDetector_CachingBehavior(t *testing.T) {
	detector := NewDetector()

	// First detection (will likely fail on non-cloud machine, but with short timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	info1, err1 := detector.DetectCloudEnvironment(ctx)

	// Second detection should return cached result immediately
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	startTime := time.Now()
	info2, err2 := detector.DetectCloudEnvironment(ctx2)
	duration := time.Since(startTime)

	// Second call should be instant (< 10ms) due to cache
	if duration > 10*time.Millisecond {
		t.Errorf("Second call took %v, expected instant cache hit", duration)
	}

	// Both calls should return same result
	if (info1 == nil) != (info2 == nil) {
		t.Error("Cached result differs from original")
	}

	if (err1 == nil) != (err2 == nil) {
		t.Error("Cached error differs from original")
	}

	// Check that lastScan was updated
	if detector.lastScan.IsZero() {
		t.Error("lastScan not updated after detection")
	}

	// Check cache retrieval
	cached := detector.GetCachedInfo()
	if info1 == nil && cached != nil {
		t.Error("GetCachedInfo() should return nil when no cloud detected")
	}
	if info1 != nil && cached == nil {
		t.Error("GetCachedInfo() should return cached info")
	}
}

func TestDetector_CacheExpiration(t *testing.T) {
	detector := NewDetector()

	// Simulate expired cache
	detector.lastScan = time.Now().Add(-2 * time.Hour)
	detector.cached = nil

	// Detection should run again (not use expired cache) with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, _ = detector.DetectCloudEnvironment(ctx)

	// lastScan should be updated to recent time
	timeSinceLastScan := time.Since(detector.lastScan)
	if timeSinceLastScan > 1*time.Second {
		t.Errorf("lastScan not updated, time since = %v", timeSinceLastScan)
	}
}

func TestDetector_Constants(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		check func(interface{}) bool
	}{
		{
			name:  "AWSIMDSEndpoint",
			value: AWSIMDSEndpoint,
			check: func(v interface{}) bool { return v == "http://169.254.169.254" },
		},
		{
			name:  "AWSIMDSTokenTTL",
			value: AWSIMDSTokenTTL,
			check: func(v interface{}) bool { return v == "21600" },
		},
		{
			name:  "AWSIMDSTimeout",
			value: AWSIMDSTimeout,
			check: func(v interface{}) bool { return v == 2*time.Second },
		},
		{
			name:  "DefaultDetectionTimeout",
			value: DefaultDetectionTimeout,
			check: func(v interface{}) bool { return v == 5*time.Second },
		},
		{
			name:  "DetectionCacheDuration",
			value: DetectionCacheDuration,
			check: func(v interface{}) bool { return v == 1*time.Hour },
		},
		{
			name:  "ProviderAWS",
			value: ProviderAWS,
			check: func(v interface{}) bool { return v == "AWS" },
		},
		{
			name:  "ProviderAzure",
			value: ProviderAzure,
			check: func(v interface{}) bool { return v == "Azure" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.value) {
				t.Errorf("Constant %s has unexpected value: %v", tt.name, tt.value)
			}
		})
	}
}

func TestDetector_CheckDMIVendor(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		name           string
		expectedVendor string
		description    string
	}{
		{
			name:           "Check AWS EC2 DMI",
			expectedVendor: "Amazon EC2",
			description:    "Should detect Amazon EC2 in DMI",
		},
		{
			name:           "Check Microsoft Azure DMI",
			expectedVendor: "Microsoft Corporation",
			description:    "Should detect Microsoft in DMI for Azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will likely return false on non-cloud machines
			// Just testing that the method doesn't panic
			result := detector.checkDMIVendor(tt.expectedVendor)
			t.Logf("checkDMIVendor(%q) = %v", tt.expectedVendor, result)
		})
	}
}

func TestDetector_GetDMIValue(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		field       string
		description string
	}{
		{"sys_vendor", "System vendor"},
		{"product_name", "Product name"},
		{"bios_vendor", "BIOS vendor"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			// Just test that it doesn't panic
			value := detector.getDMIValue(tt.field)
			t.Logf("getDMIValue(%q) = %q", tt.field, value)

			// Value can be empty on non-physical machines
			// Just verify it returns a string
			if value != "" {
				t.Logf("Found DMI value: %s = %s", tt.field, value)
			}
		})
	}
}

func TestDetector_GetCachedInfo_NilWhenNoDetection(t *testing.T) {
	detector := NewDetector()

	// Before any detection, cache should be nil
	cached := detector.GetCachedInfo()
	if cached != nil {
		t.Error("GetCachedInfo() should return nil before detection")
	}
}

func TestDetector_OnPremisesDetection(t *testing.T) {
	detector := NewDetector()
	// Use short timeout to avoid hanging on network calls
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// On a non-cloud machine, this should return nil or detect a cloud environment
	info, _ := detector.DetectCloudEnvironment(ctx)

	// Either we detect nothing (on-premises) or detect a cloud
	if info != nil {
		t.Logf("Cloud detected: Provider=%s, Region=%s, InstanceID=%s",
			info.Provider, info.Region, info.InstanceID)

		// Verify structure if cloud detected
		if info.Provider == "" {
			t.Error("Provider should not be empty when cloud detected")
		}
	} else {
		t.Logf("No cloud environment detected (on-premises or detection timed out)")
	}
}

// Benchmark tests

func BenchmarkDetector_DetectCloudEnvironment(b *testing.B) {
	detector := NewDetector()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.DetectCloudEnvironment(ctx)
	}
}

func BenchmarkDetector_GetCachedInfo(b *testing.B) {
	detector := NewDetector()
	ctx := context.Background()

	// Prime the cache
	_, _ = detector.DetectCloudEnvironment(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detector.GetCachedInfo()
	}
}

func BenchmarkDetector_CheckDMIVendor(b *testing.B) {
	detector := NewDetector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detector.checkDMIVendor("Amazon EC2")
	}
}
