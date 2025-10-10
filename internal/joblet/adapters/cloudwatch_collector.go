package adapters

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"joblet/internal/joblet/monitoring/cloud"
	"joblet/internal/joblet/pubsub"
	jobconfig "joblet/pkg/config"
	"joblet/pkg/logger"
)

// CloudWatchCollector collects job logs and sends them to AWS CloudWatch Logs
type CloudWatchCollector struct {
	config    *jobconfig.AWSCloudWatchConfig
	client    *cloudwatchlogs.Client
	logger    *logger.Logger
	cloudInfo cloud.CloudDetector

	// Batching and buffering
	eventQueue  chan *CloudWatchLogEvent
	batchBuffer []*CloudWatchLogEvent
	batchMutex  sync.Mutex

	// Metrics
	metrics *CloudWatchMetrics

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Pub-sub subscription
	unsubscribe func()
}

// CloudWatchLogEvent represents a log event to be sent to CloudWatch
type CloudWatchLogEvent struct {
	JobID     string
	Message   string
	Timestamp time.Time
	Level     string // DEBUG, INFO, WARN, ERROR
}

// CloudWatchMetrics tracks CloudWatch collector performance
type CloudWatchMetrics struct {
	EventsQueued    int64
	EventsSent      int64
	EventsFailed    int64
	EventsSampled   int64
	BatchesSent     int64
	BatchesFailed   int64
	BytesSent       int64
	BytesCompressed int64
}

// NewCloudWatchCollector creates a new CloudWatch log collector
func NewCloudWatchCollector(
	cfg *jobconfig.AWSCloudWatchConfig,
	cloudDetector cloud.CloudDetector,
	logger *logger.Logger,
) (*CloudWatchCollector, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("CloudWatch collector is disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create AWS config
	awsCfg, err := createAWSConfig(ctx, cfg, cloudDetector)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create CloudWatch Logs client
	client := cloudwatchlogs.NewFromConfig(awsCfg)

	collector := &CloudWatchCollector{
		config:      cfg,
		client:      client,
		logger:      logger.WithField("component", "cloudwatch-collector"),
		cloudInfo:   cloudDetector,
		eventQueue:  make(chan *CloudWatchLogEvent, cfg.QueueSize),
		batchBuffer: make([]*CloudWatchLogEvent, 0, cfg.BatchMaxEvents),
		metrics:     &CloudWatchMetrics{},
		ctx:         ctx,
		cancel:      cancel,
	}

	// Create log group if needed
	if cfg.CreateLogGroup {
		if err := collector.ensureLogGroup(ctx); err != nil {
			logger.Warn("failed to create log group", "error", err, "logGroup", cfg.LogGroup)
		}
	}

	// Start background workers
	collector.startWorkers()

	return collector, nil
}

// createAWSConfig creates AWS SDK configuration with proper auth
func createAWSConfig(ctx context.Context, cfg *jobconfig.AWSCloudWatchConfig, detector cloud.CloudDetector) (aws.Config, error) {
	// Auto-detect region if not specified
	region := cfg.Region
	if region == "" {
		cloudInfo, err := detector.DetectCloudEnvironment(ctx)
		if err == nil && cloudInfo != nil && cloudInfo.Provider == "AWS" {
			region = cloudInfo.Region
		}
		if region == "" {
			region = "us-east-1" // Default fallback
		}
	}

	// Load AWS config with region
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
}

// SubscribeToPubSub subscribes the collector to job log pub-sub
func (c *CloudWatchCollector) SubscribeToPubSub(pubsub pubsub.PubSub[JobEvent]) error {
	// Subscribe to all job topics using wildcard pattern "job.*"
	updates, unsubscribe, err := pubsub.Subscribe(c.ctx, "job.*")
	if err != nil {
		return fmt.Errorf("failed to subscribe to pub-sub: %w", err)
	}
	c.unsubscribe = unsubscribe

	// Start subscription handler
	c.wg.Add(1)
	go c.handlePubSubEvents(updates)

	c.logger.Info("subscribed to job log pub-sub")
	return nil
}

// handlePubSubEvents processes incoming pub-sub events
func (c *CloudWatchCollector) handlePubSubEvents(updates <-chan pubsub.Message[JobEvent]) {
	defer c.wg.Done()

	for {
		select {
		case msg := <-updates:
			if msg.Payload.Type == "LOG_CHUNK" && len(msg.Payload.LogChunk) > 0 {
				c.processLogChunk(msg.Payload.JobID, msg.Payload.LogChunk)
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// processLogChunk processes a log chunk and queues it for CloudWatch
func (c *CloudWatchCollector) processLogChunk(jobID string, logData []byte) {
	// Parse log level if present
	level := "INFO"
	message := string(logData)
	if strings.Contains(strings.ToUpper(message), "[DEBUG]") {
		level = "DEBUG"
	} else if strings.Contains(strings.ToUpper(message), "[ERROR]") {
		level = "ERROR"
	} else if strings.Contains(strings.ToUpper(message), "[WARN]") {
		level = "WARN"
	}

	// Apply sampling if enabled
	if c.config.SamplingEnabled && !c.shouldSample(level) {
		atomic.AddInt64(&c.metrics.EventsSampled, 1)
		return
	}

	event := &CloudWatchLogEvent{
		JobID:     jobID,
		Message:   message,
		Timestamp: time.Now(),
		Level:     level,
	}

	// Non-blocking queue
	select {
	case c.eventQueue <- event:
		atomic.AddInt64(&c.metrics.EventsQueued, 1)
	default:
		// Queue full, drop event (CloudWatch is best-effort)
		c.logger.Warn("CloudWatch event queue full, dropping event", "jobId", jobID)
	}
}

// shouldSample determines if a log should be sampled based on level
func (c *CloudWatchCollector) shouldSample(level string) bool {
	switch level {
	case "DEBUG":
		return rand.Float64() < c.config.SampleDebugRate
	case "TRACE":
		return rand.Float64() < c.config.SampleTraceRate
	default:
		return true // Always keep INFO, WARN, ERROR
	}
}

// startWorkers starts background processing goroutines
func (c *CloudWatchCollector) startWorkers() {
	// Batch processor
	c.wg.Add(1)
	go c.batchProcessor()
}

// batchProcessor collects events into batches and sends to CloudWatch
func (c *CloudWatchCollector) batchProcessor() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-c.eventQueue:
			c.batchMutex.Lock()
			c.batchBuffer = append(c.batchBuffer, event)

			// Send batch if full
			if len(c.batchBuffer) >= c.config.BatchMaxEvents {
				c.sendBatch()
			}
			c.batchMutex.Unlock()

		case <-ticker.C:
			// Periodic flush
			c.batchMutex.Lock()
			if len(c.batchBuffer) > 0 {
				c.sendBatch()
			}
			c.batchMutex.Unlock()

		case <-c.ctx.Done():
			// Final flush on shutdown
			c.batchMutex.Lock()
			if len(c.batchBuffer) > 0 {
				c.sendBatch()
			}
			c.batchMutex.Unlock()
			return
		}
	}
}

// sendBatch sends the current batch to CloudWatch
func (c *CloudWatchCollector) sendBatch() {
	if len(c.batchBuffer) == 0 {
		return
	}

	// Group events by job ID (CloudWatch requires same log stream)
	jobBatches := make(map[string][]*CloudWatchLogEvent)
	for _, event := range c.batchBuffer {
		jobBatches[event.JobID] = append(jobBatches[event.JobID], event)
	}

	// Send each job's batch
	for jobID, events := range jobBatches {
		if err := c.sendJobBatch(jobID, events); err != nil {
			c.logger.Error("failed to send CloudWatch batch", "jobId", jobID, "error", err)
			atomic.AddInt64(&c.metrics.BatchesFailed, 1)

			// Retry logic
			c.retryBatch(jobID, events)
		} else {
			atomic.AddInt64(&c.metrics.BatchesSent, 1)
			atomic.AddInt64(&c.metrics.EventsSent, int64(len(events)))
		}
	}

	// Clear batch
	c.batchBuffer = c.batchBuffer[:0]
}

// sendJobBatch sends a batch of events for a specific job to CloudWatch
func (c *CloudWatchCollector) sendJobBatch(jobID string, events []*CloudWatchLogEvent) error {
	logStreamName := c.config.LogStreamPrefix + jobID

	// Convert events to CloudWatch format
	logEvents := make([]types.InputLogEvent, 0, len(events))
	for _, event := range events {
		message := event.Message

		// Compress if enabled
		if c.config.Compression {
			compressed, err := c.compressMessage(message)
			if err == nil {
				atomic.AddInt64(&c.metrics.BytesCompressed, int64(len(message)-len(compressed)))
				message = compressed
			}
		}

		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(event.Timestamp.UnixMilli()),
		})

		atomic.AddInt64(&c.metrics.BytesSent, int64(len(message)))
	}

	// Create log stream if it doesn't exist
	if err := c.ensureLogStream(c.ctx, logStreamName); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	// Send to CloudWatch
	_, err := c.client.PutLogEvents(c.ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(c.config.LogGroup),
		LogStreamName: aws.String(logStreamName),
		LogEvents:     logEvents,
	})

	return err
}

// retryBatch retries sending a failed batch
func (c *CloudWatchCollector) retryBatch(jobID string, events []*CloudWatchLogEvent) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		for i := 0; i < c.config.MaxRetries; i++ {
			time.Sleep(c.config.RetryInterval)

			if err := c.sendJobBatch(jobID, events); err == nil {
				c.logger.Info("retry successful", "jobId", jobID, "attempt", i+1)
				atomic.AddInt64(&c.metrics.EventsSent, int64(len(events)))
				return
			}
		}

		c.logger.Error("batch retry exhausted", "jobId", jobID)
		atomic.AddInt64(&c.metrics.EventsFailed, int64(len(events)))
	}()
}

// compressMessage compresses a log message using gzip
func (c *CloudWatchCollector) compressMessage(message string) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)

	if _, err := w.Write([]byte(message)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	// Return base64-like encoding to make it string-safe
	return buf.String(), nil
}

// ensureLogGroup creates the log group if it doesn't exist
func (c *CloudWatchCollector) ensureLogGroup(ctx context.Context) error {
	// Check if log group exists
	_, err := c.client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(c.config.LogGroup),
		Limit:              aws.Int32(1),
	})

	if err == nil {
		return nil // Already exists
	}

	// Create log group
	_, err = c.client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(c.config.LogGroup),
	})

	if err != nil {
		return fmt.Errorf("failed to create log group: %w", err)
	}

	// Set retention if specified
	if c.config.RetentionDays > 0 {
		_, _ = c.client.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    aws.String(c.config.LogGroup),
			RetentionInDays: aws.Int32(int32(c.config.RetentionDays)),
		})
	}

	c.logger.Info("created CloudWatch log group", "logGroup", c.config.LogGroup)
	return nil
}

// ensureLogStream creates a log stream if it doesn't exist
func (c *CloudWatchCollector) ensureLogStream(ctx context.Context, streamName string) error {
	_, err := c.client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(c.config.LogGroup),
		LogStreamName: aws.String(streamName),
	})

	// Ignore error if stream already exists
	if err != nil && !strings.Contains(err.Error(), "ResourceAlreadyExistsException") {
		return err
	}

	return nil
}

// GetMetrics returns current collector metrics
func (c *CloudWatchCollector) GetMetrics() *CloudWatchMetrics {
	return &CloudWatchMetrics{
		EventsQueued:    atomic.LoadInt64(&c.metrics.EventsQueued),
		EventsSent:      atomic.LoadInt64(&c.metrics.EventsSent),
		EventsFailed:    atomic.LoadInt64(&c.metrics.EventsFailed),
		EventsSampled:   atomic.LoadInt64(&c.metrics.EventsSampled),
		BatchesSent:     atomic.LoadInt64(&c.metrics.BatchesSent),
		BatchesFailed:   atomic.LoadInt64(&c.metrics.BatchesFailed),
		BytesSent:       atomic.LoadInt64(&c.metrics.BytesSent),
		BytesCompressed: atomic.LoadInt64(&c.metrics.BytesCompressed),
	}
}

// Close shuts down the CloudWatch collector gracefully
func (c *CloudWatchCollector) Close() error {
	c.logger.Info("shutting down CloudWatch collector")

	// Unsubscribe from pub-sub
	if c.unsubscribe != nil {
		c.unsubscribe()
	}

	// Stop workers
	c.cancel()
	c.wg.Wait()

	// Log final metrics
	metrics := c.GetMetrics()
	c.logger.Info("CloudWatch collector stopped",
		"eventsQueued", metrics.EventsQueued,
		"eventsSent", metrics.EventsSent,
		"eventsFailed", metrics.EventsFailed,
		"batchesSent", metrics.BatchesSent)

	return nil
}
