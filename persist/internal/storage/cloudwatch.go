package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// CloudWatchBackend implements the Backend interface for AWS CloudWatch Logs and Metrics
type CloudWatchBackend struct {
	config     *config.CloudWatchConfig
	logsClient *cloudwatchlogs.Client
	logger     *logger.Logger

	// Cache for log group/stream creation
	createdGroups  map[string]bool
	createdStreams map[string]bool
	cacheMutex     sync.RWMutex

	// Sequence tokens for log streams (required by CloudWatch Logs API)
	sequenceTokens map[string]*string
	tokenMutex     sync.RWMutex
}

// NewCloudWatchBackend creates a new CloudWatch storage backend
func NewCloudWatchBackend(cfg *config.StorageConfig, nodeID string, log *logger.Logger) (Backend, error) {
	if log == nil {
		log = logger.New().WithField("component", "cloudwatch-backend")
	}

	// Get CloudWatch config
	cwConfig := cfg.CloudWatch

	// Set nodeID (inherited from server config)
	cwConfig.NodeID = nodeID
	if cwConfig.Region == "" {
		// Auto-detect region from EC2 metadata
		region, err := detectEC2Region(context.Background())
		if err != nil {
			log.Warn("failed to auto-detect AWS region, using us-east-1 as default", "error", err)
			cwConfig.Region = "us-east-1"
		} else {
			cwConfig.Region = region
			log.Info("auto-detected AWS region from EC2 metadata", "region", region)
		}
	}

	// Set defaults for prefixes
	if cwConfig.LogGroupPrefix == "" {
		cwConfig.LogGroupPrefix = "/joblet/jobs"
	}
	if cwConfig.LogStreamPrefix == "" {
		cwConfig.LogStreamPrefix = "job-"
	}
	if cwConfig.MetricNamespace == "" {
		cwConfig.MetricNamespace = "Joblet/Jobs"
	}

	// Set default batch sizes
	if cwConfig.LogBatchSize == 0 {
		cwConfig.LogBatchSize = 100 // CloudWatch Logs max is 10,000 events per batch
	}
	if cwConfig.MetricBatchSize == 0 {
		cwConfig.MetricBatchSize = 20 // CloudWatch Metrics max is 1,000 per batch
	}

	// Load AWS configuration using default credential chain
	// This supports IAM roles, instance profiles, environment variables, and shared credentials file
	log.Info("using AWS default credential chain (IAM role, instance profile, or environment variables)")
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cwConfig.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create CloudWatch Logs client
	logsClient := cloudwatchlogs.NewFromConfig(awsCfg)

	backend := &CloudWatchBackend{
		config:         &cwConfig,
		logsClient:     logsClient,
		logger:         log,
		createdGroups:  make(map[string]bool),
		createdStreams: make(map[string]bool),
		sequenceTokens: make(map[string]*string),
	}

	log.Info("CloudWatch backend initialized successfully",
		"region", cwConfig.Region,
		"logGroupPrefix", cwConfig.LogGroupPrefix,
		"metricNamespace", cwConfig.MetricNamespace)

	return backend, nil
}

// detectEC2Region attempts to detect the AWS region from EC2 metadata service
func detectEC2Region(ctx context.Context) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := imds.NewFromConfig(cfg)
	result, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get region from EC2 metadata: %w", err)
	}

	return result.Region, nil
}

// WriteLogs writes log lines to CloudWatch Logs
func (b *CloudWatchBackend) WriteLogs(jobID string, logs []*ipcpb.LogLine) error {
	if len(logs) == 0 {
		return nil
	}

	// Group logs by stream type (stdout/stderr)
	stdoutLogs := make([]*ipcpb.LogLine, 0)
	stderrLogs := make([]*ipcpb.LogLine, 0)

	for _, log := range logs {
		switch log.Stream {
		case ipcpb.StreamType_STREAM_TYPE_STDOUT:
			stdoutLogs = append(stdoutLogs, log)
		case ipcpb.StreamType_STREAM_TYPE_STDERR:
			stderrLogs = append(stderrLogs, log)
		}
	}

	// Write to separate log streams
	var errs []error
	if len(stdoutLogs) > 0 {
		if err := b.writeLogsToStream(jobID, "stdout", stdoutLogs); err != nil {
			errs = append(errs, fmt.Errorf("stdout: %w", err))
		}
	}
	if len(stderrLogs) > 0 {
		if err := b.writeLogsToStream(jobID, "stderr", stderrLogs); err != nil {
			errs = append(errs, fmt.Errorf("stderr: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to write logs: %v", errs)
	}

	return nil
}

// writeLogsToStream writes logs to a specific CloudWatch log stream
func (b *CloudWatchBackend) writeLogsToStream(jobID, streamType string, logs []*ipcpb.LogLine) error {
	ctx := context.Background()

	// Determine log group and stream names
	// Include nodeID for multi-node deployments: /joblet/{nodeID}/jobs/{jobID}
	logGroup := fmt.Sprintf("%s/%s/jobs/%s", b.config.LogGroupPrefix, b.config.NodeID, jobID)
	logStream := fmt.Sprintf("%s%s-%s", b.config.LogStreamPrefix, jobID, streamType)

	// Ensure log group exists
	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}

	// Ensure log stream exists
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	// Sort logs by timestamp (CloudWatch requires chronological order)
	sortedLogs := make([]*ipcpb.LogLine, len(logs))
	copy(sortedLogs, logs)
	sort.Slice(sortedLogs, func(i, j int) bool {
		return sortedLogs[i].Timestamp < sortedLogs[j].Timestamp
	})

	// Convert to CloudWatch log events
	events := make([]types.InputLogEvent, 0, len(sortedLogs))
	for _, log := range sortedLogs {
		// Convert nanoseconds to milliseconds for CloudWatch
		timestamp := log.Timestamp / 1_000_000
		events = append(events, types.InputLogEvent{
			Message:   aws.String(string(log.Content)),
			Timestamp: aws.Int64(timestamp),
		})
	}

	// Batch write events (respect CloudWatch limits)
	batchSize := b.config.LogBatchSize
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		if err := b.putLogEvents(ctx, logGroup, logStream, batch); err != nil {
			return fmt.Errorf("failed to put log events (batch %d-%d): %w", i, end, err)
		}
	}

	b.logger.Debug("wrote logs to CloudWatch",
		"jobId", jobID,
		"stream", streamType,
		"count", len(logs),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// putLogEvents sends log events to CloudWatch with sequence token handling
func (b *CloudWatchBackend) putLogEvents(ctx context.Context, logGroup, logStream string, events []types.InputLogEvent) error {
	// Get current sequence token
	b.tokenMutex.RLock()
	streamKey := fmt.Sprintf("%s/%s", logGroup, logStream)
	sequenceToken := b.sequenceTokens[streamKey]
	b.tokenMutex.RUnlock()

	// Put log events
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		LogEvents:     events,
		SequenceToken: sequenceToken,
	}

	resp, err := b.logsClient.PutLogEvents(ctx, input)
	if err != nil {
		// Handle invalid sequence token error by retrying with the expected token
		var invalidSeqErr *types.InvalidSequenceTokenException
		if errTyped := err; errTyped != nil {
			// Try to extract expected sequence token from error
			// CloudWatch returns the expected token in the error message
			b.logger.Warn("invalid sequence token, retrying", "error", err)
			// For simplicity, we'll get the latest token by describing the stream
			describeResp, describeErr := b.logsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
				LogGroupName:        aws.String(logGroup),
				LogStreamNamePrefix: aws.String(logStream),
			})
			if describeErr == nil && len(describeResp.LogStreams) > 0 {
				sequenceToken = describeResp.LogStreams[0].UploadSequenceToken
				input.SequenceToken = sequenceToken
				resp, err = b.logsClient.PutLogEvents(ctx, input)
			}
		}

		if err != nil {
			return fmt.Errorf("failed to put log events: %w (invalidSeqErr: %v)", err, invalidSeqErr)
		}
	}

	// Update sequence token for next call
	if resp.NextSequenceToken != nil {
		b.tokenMutex.Lock()
		b.sequenceTokens[streamKey] = resp.NextSequenceToken
		b.tokenMutex.Unlock()
	}

	return nil
}

// ensureLogGroup creates a log group if it doesn't exist
func (b *CloudWatchBackend) ensureLogGroup(ctx context.Context, logGroup string) error {
	// Check cache first
	b.cacheMutex.RLock()
	exists := b.createdGroups[logGroup]
	b.cacheMutex.RUnlock()

	if exists {
		return nil
	}

	// Create log group (idempotent - no error if already exists)
	_, err := b.logsClient.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroup),
	})

	if err != nil {
		// Check if error is "already exists" - this is not a real error
		if strings.Contains(err.Error(), "ResourceAlreadyExistsException") {
			b.cacheMutex.Lock()
			b.createdGroups[logGroup] = true
			b.cacheMutex.Unlock()
			return nil
		}
		return fmt.Errorf("failed to create log group: %w", err)
	}

	// Cache the fact that we've created this group
	b.cacheMutex.Lock()
	b.createdGroups[logGroup] = true
	b.cacheMutex.Unlock()

	b.logger.Info("created CloudWatch log group", "logGroup", logGroup)
	return nil
}

// ensureLogStream creates a log stream if it doesn't exist
func (b *CloudWatchBackend) ensureLogStream(ctx context.Context, logGroup, logStream string) error {
	// Check cache first
	streamKey := fmt.Sprintf("%s/%s", logGroup, logStream)
	b.cacheMutex.RLock()
	exists := b.createdStreams[streamKey]
	b.cacheMutex.RUnlock()

	if exists {
		return nil
	}

	// Create log stream (idempotent - no error if already exists)
	_, err := b.logsClient.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
	})

	if err != nil {
		// Check if error is "already exists" - this is not a real error
		if strings.Contains(err.Error(), "ResourceAlreadyExistsException") {
			b.cacheMutex.Lock()
			b.createdStreams[streamKey] = true
			b.cacheMutex.Unlock()
			return nil
		}
		return fmt.Errorf("failed to create log stream: %w", err)
	}

	// Cache the fact that we've created this stream
	b.cacheMutex.Lock()
	b.createdStreams[streamKey] = true
	b.cacheMutex.Unlock()

	b.logger.Info("created CloudWatch log stream", "logGroup", logGroup, "logStream", logStream)
	return nil
}

// WriteMetrics writes metrics to CloudWatch (stored as JSON in log events for now)
// TODO: In a future version, consider using CloudWatch Metrics API for proper metric storage
func (b *CloudWatchBackend) WriteMetrics(jobID string, metrics []*ipcpb.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	ctx := context.Background()

	// Store metrics as JSON log events in a dedicated metrics log stream
	// Include nodeID for multi-node deployments
	logGroup := fmt.Sprintf("%s/%s/jobs/%s", b.config.LogGroupPrefix, b.config.NodeID, jobID)
	logStream := fmt.Sprintf("%s%s-metrics", b.config.LogStreamPrefix, jobID)

	// Ensure log group and stream exist
	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group for metrics: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream for metrics: %w", err)
	}

	// Convert metrics to log events (JSON format)
	events := make([]types.InputLogEvent, 0, len(metrics))
	for _, metric := range metrics {
		// Serialize metric as JSON
		jsonData, err := json.Marshal(metric)
		if err != nil {
			b.logger.Warn("failed to marshal metric to JSON", "error", err)
			continue
		}

		// Convert timestamp to milliseconds
		timestamp := metric.Timestamp / 1_000_000
		events = append(events, types.InputLogEvent{
			Message:   aws.String(string(jsonData)),
			Timestamp: aws.Int64(timestamp),
		})
	}

	// Batch write events
	batchSize := b.config.MetricBatchSize
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		if err := b.putLogEvents(ctx, logGroup, logStream, batch); err != nil {
			return fmt.Errorf("failed to put metric events (batch %d-%d): %w", i, end, err)
		}
	}

	b.logger.Debug("wrote metrics to CloudWatch",
		"jobId", jobID,
		"count", len(metrics),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// ReadLogs reads log lines from CloudWatch Logs
func (b *CloudWatchBackend) ReadLogs(ctx context.Context, query *LogQuery) (*LogReader, error) {
	reader := &LogReader{
		Channel: make(chan *ipcpb.LogLine, 100),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	go func() {
		defer close(reader.Channel)
		defer close(reader.Error)
		defer close(reader.Done)

		if err := b.readLogsFromStream(ctx, query, reader.Channel); err != nil {
			reader.Error <- err
		}
	}()

	return reader, nil
}

// readLogsFromStream retrieves logs from CloudWatch and sends them to the channel
func (b *CloudWatchBackend) readLogsFromStream(ctx context.Context, query *LogQuery, ch chan<- *ipcpb.LogLine) error {
	// Include nodeID for multi-node deployments
	logGroup := fmt.Sprintf("%s/%s/jobs/%s", b.config.LogGroupPrefix, b.config.NodeID, query.JobID)

	// Determine stream type suffix
	streamSuffix := "stdout"
	if query.Stream == ipcpb.StreamType_STREAM_TYPE_STDERR {
		streamSuffix = "stderr"
	}
	logStream := fmt.Sprintf("%s%s-%s", b.config.LogStreamPrefix, query.JobID, streamSuffix)

	// Build GetLogEvents input
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartFromHead: aws.Bool(true),
	}

	if query.StartTime != nil {
		// Convert nanoseconds to milliseconds
		startMs := *query.StartTime / 1_000_000
		input.StartTime = aws.Int64(startMs)
	}
	if query.EndTime != nil {
		endMs := *query.EndTime / 1_000_000
		input.EndTime = aws.Int64(endMs)
	}
	if query.Limit > 0 {
		input.Limit = aws.Int32(int32(query.Limit))
	}

	// Retrieve logs
	resp, err := b.logsClient.GetLogEvents(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get log events: %w", err)
	}

	// Send log events to channel
	for _, event := range resp.Events {
		// Convert back to nanoseconds
		timestampNs := *event.Timestamp * 1_000_000

		logLine := &ipcpb.LogLine{
			JobId:     query.JobID,
			Stream:    query.Stream,
			Content:   []byte(*event.Message),
			Timestamp: timestampNs,
		}

		select {
		case ch <- logLine:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// ReadMetrics reads metrics from CloudWatch Logs (stored as JSON)
func (b *CloudWatchBackend) ReadMetrics(ctx context.Context, query *MetricQuery) (*MetricReader, error) {
	reader := &MetricReader{
		Channel: make(chan *ipcpb.Metric, 100),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	go func() {
		defer close(reader.Channel)
		defer close(reader.Error)
		defer close(reader.Done)

		if err := b.readMetricsFromStream(ctx, query, reader.Channel); err != nil {
			reader.Error <- err
		}
	}()

	return reader, nil
}

// readMetricsFromStream retrieves metrics from CloudWatch and sends them to the channel
func (b *CloudWatchBackend) readMetricsFromStream(ctx context.Context, query *MetricQuery, ch chan<- *ipcpb.Metric) error {
	// Include nodeID for multi-node deployments
	logGroup := fmt.Sprintf("%s/%s/jobs/%s", b.config.LogGroupPrefix, b.config.NodeID, query.JobID)
	logStream := fmt.Sprintf("%s%s-metrics", b.config.LogStreamPrefix, query.JobID)

	// Build GetLogEvents input
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartFromHead: aws.Bool(true),
	}

	if query.StartTime != nil {
		startMs := *query.StartTime / 1_000_000
		input.StartTime = aws.Int64(startMs)
	}
	if query.EndTime != nil {
		endMs := *query.EndTime / 1_000_000
		input.EndTime = aws.Int64(endMs)
	}
	if query.Limit > 0 {
		input.Limit = aws.Int32(int32(query.Limit))
	}

	// Retrieve metrics
	resp, err := b.logsClient.GetLogEvents(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get metric events: %w", err)
	}

	// Parse JSON metrics and send to channel
	for _, event := range resp.Events {
		var metric ipcpb.Metric
		if err := json.Unmarshal([]byte(*event.Message), &metric); err != nil {
			b.logger.Warn("failed to unmarshal metric from JSON", "error", err)
			continue
		}

		select {
		case ch <- &metric:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// DeleteJob deletes all CloudWatch log streams for a job
func (b *CloudWatchBackend) DeleteJob(jobID string) error {
	ctx := context.Background()
	// Include nodeID for multi-node deployments
	logGroup := fmt.Sprintf("%s/%s/jobs/%s", b.config.LogGroupPrefix, b.config.NodeID, jobID)

	// List all log streams in the group
	listResp, err := b.logsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroup),
	})

	if err != nil {
		// If log group doesn't exist, that's fine
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			b.logger.Debug("log group not found (already deleted or never created)", "logGroup", logGroup)
			return nil
		}
		return fmt.Errorf("failed to list log streams for deletion: %w", err)
	}

	// Delete each log stream
	for _, stream := range listResp.LogStreams {
		_, err := b.logsClient.DeleteLogStream(ctx, &cloudwatchlogs.DeleteLogStreamInput{
			LogGroupName:  aws.String(logGroup),
			LogStreamName: stream.LogStreamName,
		})
		if err != nil {
			b.logger.Warn("failed to delete log stream", "logStream", *stream.LogStreamName, "error", err)
		} else {
			b.logger.Debug("deleted log stream", "logStream", *stream.LogStreamName)
		}
	}

	// Delete the log group itself
	_, err = b.logsClient.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroup),
	})
	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			// Already deleted
			return nil
		}
		return fmt.Errorf("failed to delete log group: %w", err)
	}

	// Clear from cache
	b.cacheMutex.Lock()
	delete(b.createdGroups, logGroup)
	for key := range b.createdStreams {
		if strings.HasPrefix(key, logGroup+"/") {
			delete(b.createdStreams, key)
		}
	}
	b.cacheMutex.Unlock()

	// Clear sequence tokens
	b.tokenMutex.Lock()
	for key := range b.sequenceTokens {
		if strings.HasPrefix(key, logGroup+"/") {
			delete(b.sequenceTokens, key)
		}
	}
	b.tokenMutex.Unlock()

	b.logger.Info("deleted CloudWatch log group for job", "jobId", jobID, "logGroup", logGroup)
	return nil
}

// Close closes the CloudWatch backend (no-op for CloudWatch client)
func (b *CloudWatchBackend) Close() error {
	b.logger.Info("CloudWatch backend closed")
	return nil
}

// Helper function to convert string timestamp to int64 nanoseconds
func parseTimestampToNanos(ts string) (int64, error) {
	// Try parsing as RFC3339
	t, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return t.UnixNano(), nil
	}

	// Try parsing as Unix timestamp (seconds)
	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err == nil {
		return seconds * 1_000_000_000, nil
	}

	return 0, fmt.Errorf("failed to parse timestamp: %s", ts)
}
