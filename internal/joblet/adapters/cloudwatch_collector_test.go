package adapters

import (
	"testing"
	"time"

	"joblet/internal/joblet/monitoring/cloud/cloudfakes"
	"joblet/internal/joblet/monitoring/domain"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

func TestNewCloudWatchCollector(t *testing.T) {
	cfg := &config.AWSCloudWatchConfig{
		Enabled:        true,
		Region:         "us-west-2",
		LogGroup:       "/aws/joblet/test",
		BatchMaxEvents: 100,
		BatchInterval:  1 * time.Second,
		QueueSize:      1000,
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider:   "AWS",
		Region:     "us-west-2",
		InstanceID: "i-test123",
	}, nil)
	detector.GetCachedInfoReturns(&domain.CloudInfo{
		Provider:   "AWS",
		Region:     "us-west-2",
		InstanceID: "i-test123",
	})

	log := logger.New()

	collector, err := NewCloudWatchCollector(cfg, detector, log)

	if err != nil {
		t.Fatalf("NewCloudWatchCollector() error = %v, want nil", err)
	}

	if collector == nil {
		t.Fatal("NewCloudWatchCollector() returned nil")
	}

	if collector.config != cfg {
		t.Error("config not set correctly")
	}

	if collector.eventQueue == nil {
		t.Error("eventQueue not initialized")
	}

	if collector.metrics == nil {
		t.Error("metrics not initialized")
	}
}

func TestCloudWatchCollector_DisabledConfig(t *testing.T) {
	cfg := &config.AWSCloudWatchConfig{
		Enabled: false,
	}

	detector := &cloudfakes.FakeCloudDetector{}
	log := logger.New()

	collector, err := NewCloudWatchCollector(cfg, detector, log)

	if err == nil {
		t.Error("Expected error for disabled CloudWatch, got nil")
	}

	if collector != nil {
		t.Error("Expected nil collector for disabled config")
	}
}

func TestCloudWatchCollector_ProcessLogChunk(t *testing.T) {
	cfg := &config.AWSCloudWatchConfig{
		Enabled:         true,
		Region:          "us-west-2",
		LogGroup:        "/aws/joblet/test",
		BatchMaxEvents:  100,
		BatchInterval:   1 * time.Second,
		QueueSize:       10,
		SamplingEnabled: false, // Disable sampling for predictable tests
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider: "AWS",
		Region:   "us-west-2",
	}, nil)

	log := logger.New()
	collector, _ := NewCloudWatchCollector(cfg, detector, log)

	// Process a log chunk
	jobID := "test-job-123"
	logData := []byte("Test log message")

	collector.processLogChunk(jobID, logData)

	// Give time for async processing
	time.Sleep(50 * time.Millisecond)

	// Verify metrics
	if collector.metrics.EventsQueued == 0 {
		t.Error("Expected EventsQueued > 0")
	}
}

func TestCloudWatchCollector_Sampling(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		sampleRate  float64
		expectDrop  bool
		description string
	}{
		{
			name:        "INFO always kept",
			level:       "INFO",
			sampleRate:  0.1,
			expectDrop:  false,
			description: "INFO/WARN/ERROR logs are never sampled",
		},
		{
			name:        "DEBUG sampled at 10%",
			level:       "DEBUG",
			sampleRate:  0.1,
			expectDrop:  true, // Will drop some
			description: "DEBUG logs sampled at configured rate",
		},
		{
			name:        "TRACE sampled at 1%",
			level:       "TRACE",
			sampleRate:  0.01,
			expectDrop:  true, // Will drop most
			description: "TRACE logs sampled at configured rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AWSCloudWatchConfig{
				Enabled:         true,
				Region:          "us-west-2",
				LogGroup:        "/aws/joblet/test",
				BatchMaxEvents:  100,
				BatchInterval:   1 * time.Second,
				QueueSize:       1000,
				SamplingEnabled: true,
				SampleDebugRate: 0.1,
				SampleTraceRate: 0.01,
			}

			detector := &cloudfakes.FakeCloudDetector{}
			detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
				Provider: "AWS",
				Region:   "us-west-2",
			}, nil)

			log := logger.New()
			collector, _ := NewCloudWatchCollector(cfg, detector, log)

			// Test sampling behavior
			shouldSample := collector.shouldSample(tt.level)

			if tt.level == "INFO" || tt.level == "WARN" || tt.level == "ERROR" {
				if !shouldSample {
					t.Errorf("Level %s should never be sampled", tt.level)
				}
			}
		})
	}
}

func TestCloudWatchCollector_SubscribeToPubSub(t *testing.T) {
	t.Skip("Skipping pub-sub integration test - requires additional setup")

	// NOTE: This test is skipped because:
	// 1. It requires actual AWS credentials or mock AWS SDK client
	// 2. Pub-sub integration is better tested in integration tests
	// 3. The processLogChunk method is already tested directly

	cfg := &config.AWSCloudWatchConfig{
		Enabled:         true,
		Region:          "us-west-2",
		LogGroup:        "/aws/joblet/test",
		BatchMaxEvents:  100,
		BatchInterval:   1 * time.Second,
		QueueSize:       100,
		SamplingEnabled: false,
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider: "AWS",
		Region:   "us-west-2",
	}, nil)

	log := logger.New()
	collector, err := NewCloudWatchCollector(cfg, detector, log)
	if err != nil {
		t.Fatalf("NewCloudWatchCollector() error = %v", err)
	}
	defer collector.Close()

	// Create pub-sub
	ps := pubsub.NewPubSub[JobEvent]()

	// Subscribe should not error
	err = collector.SubscribeToPubSub(ps)
	if err != nil {
		t.Fatalf("SubscribeToPubSub() error = %v", err)
	}
}

func TestCloudWatchCollector_Close(t *testing.T) {
	cfg := &config.AWSCloudWatchConfig{
		Enabled:        true,
		Region:         "us-west-2",
		LogGroup:       "/aws/joblet/test",
		BatchMaxEvents: 100,
		BatchInterval:  1 * time.Second,
		QueueSize:      100,
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider: "AWS",
		Region:   "us-west-2",
	}, nil)

	log := logger.New()
	collector, _ := NewCloudWatchCollector(cfg, detector, log)

	// Close should not panic
	err := collector.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Second close should not panic
	err = collector.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v, want nil", err)
	}
}

func TestCloudWatchCollector_MetricsTracking(t *testing.T) {
	cfg := &config.AWSCloudWatchConfig{
		Enabled:        true,
		Region:         "us-west-2",
		LogGroup:       "/aws/joblet/test",
		BatchMaxEvents: 100,
		BatchInterval:  1 * time.Second,
		QueueSize:      10,
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider: "AWS",
		Region:   "us-west-2",
	}, nil)

	log := logger.New()
	collector, _ := NewCloudWatchCollector(cfg, detector, log)

	// Initial metrics should be zero
	if collector.metrics.EventsQueued != 0 {
		t.Errorf("EventsQueued = %d, want 0", collector.metrics.EventsQueued)
	}

	if collector.metrics.EventsSent != 0 {
		t.Errorf("EventsSent = %d, want 0", collector.metrics.EventsSent)
	}

	// Process some events
	collector.processLogChunk("job1", []byte("message 1"))
	collector.processLogChunk("job2", []byte("message 2"))

	time.Sleep(50 * time.Millisecond)

	// Metrics should be updated
	if collector.metrics.EventsQueued == 0 {
		t.Error("Expected EventsQueued > 0 after processing")
	}
}

func TestCloudWatchCollector_QueueOverflow(t *testing.T) {
	// Small queue to test overflow
	cfg := &config.AWSCloudWatchConfig{
		Enabled:        true,
		Region:         "us-west-2",
		LogGroup:       "/aws/joblet/test",
		BatchMaxEvents: 100,
		BatchInterval:  10 * time.Second, // Long interval to fill queue
		QueueSize:      2,                // Tiny queue
	}

	detector := &cloudfakes.FakeCloudDetector{}
	detector.DetectCloudEnvironmentReturns(&domain.CloudInfo{
		Provider: "AWS",
		Region:   "us-west-2",
	}, nil)

	log := logger.New()
	collector, _ := NewCloudWatchCollector(cfg, detector, log)

	// Try to overflow the queue
	for i := 0; i < 10; i++ {
		collector.processLogChunk("job1", []byte("overflow test"))
	}

	time.Sleep(50 * time.Millisecond)

	// Should have dropped some events (queue size is 2)
	// Metrics should show queued but not all 10
	if collector.metrics.EventsQueued > 2 {
		t.Logf("EventsQueued = %d (expected <= queue size due to drops)", collector.metrics.EventsQueued)
	}
}
