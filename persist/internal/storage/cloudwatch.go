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
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// CloudWatchBackend implements the Backend interface for AWS CloudWatch Logs and Metrics
type CloudWatchBackend struct {
	config        *config.CloudWatchConfig
	logsClient    *cloudwatchlogs.Client
	metricsClient *cloudwatch.Client
	logger        *logger.Logger

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

	// Region must be set in config (by installation script)
	if cwConfig.Region == "" {
		return nil, fmt.Errorf("cloudwatch.region is required - must be set by installation script")
	}

	// Set defaults for prefixes
	if cwConfig.LogGroupPrefix == "" {
		cwConfig.LogGroupPrefix = "/joblet"
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

	// Set default retention if not specified
	// 0 or not set = default to 7 days
	// -1 = never expire (don't set retention policy)
	// positive = expire after N days
	if cwConfig.LogRetentionDays == 0 {
		cwConfig.LogRetentionDays = 7 // Default: 7 days retention
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

	// Create CloudWatch Metrics client
	metricsClient := cloudwatch.NewFromConfig(awsCfg)

	backend := &CloudWatchBackend{
		config:         &cwConfig,
		logsClient:     logsClient,
		metricsClient:  metricsClient,
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

// getLogGroupForRead returns the log group for read operations.
// Uses the passed nodeID if set (for multi-node queries), otherwise uses the local nodeID.
func (b *CloudWatchBackend) getLogGroupForRead(nodeID string) string {
	effectiveNodeID := nodeID
	if effectiveNodeID == "" {
		effectiveNodeID = b.config.NodeID
	}
	return fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, effectiveNodeID)
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
	// Single log group per node: /joblet/{nodeID}/jobs
	// Separate log stream per job: {jobID}-{streamType}
	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-%s", jobID, streamType)

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
		"job_uuid", jobID,
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

// ensureLogGroup creates a log group if it doesn't exist and sets retention policy
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

	groupAlreadyExisted := false
	if err != nil {
		// Check if error is "already exists" - this is not a real error
		if strings.Contains(err.Error(), "ResourceAlreadyExistsException") {
			groupAlreadyExisted = true
			// Continue to set retention policy even for existing groups
		} else {
			return fmt.Errorf("failed to create log group: %w", err)
		}
	}

	// Cache the fact that we've created/verified this group
	b.cacheMutex.Lock()
	b.createdGroups[logGroup] = true
	b.cacheMutex.Unlock()

	// Set retention policy if configured (skip if -1 = never expire)
	action := "created"
	if groupAlreadyExisted {
		action = "verified"
	}

	if b.config.LogRetentionDays > 0 {
		_, err := b.logsClient.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
			LogGroupName:    aws.String(logGroup),
			RetentionInDays: aws.Int32(int32(b.config.LogRetentionDays)),
		})
		if err != nil {
			b.logger.Warn("failed to set retention policy", "logGroup", logGroup, "retentionDays", b.config.LogRetentionDays, "error", err)
			// Don't fail - log group was created/verified successfully
		} else {
			b.logger.Info(fmt.Sprintf("%s CloudWatch log group with retention", action), "logGroup", logGroup, "retentionDays", b.config.LogRetentionDays)
		}
	} else {
		b.logger.Info(fmt.Sprintf("%s CloudWatch log group", action), "logGroup", logGroup, "retention", "never expire")
	}

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

// WriteMetrics writes metrics to CloudWatch Logs (for raw data retrieval) and CloudWatch Metrics API (for dashboards/alarms)
func (b *CloudWatchBackend) WriteMetrics(jobID string, metrics []*ipcpb.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	ctx := context.Background()

	// First, write to CloudWatch Logs for raw data retrieval
	// This stores the exact data points we can query later
	if err := b.writeMetricsToLogs(ctx, jobID, metrics); err != nil {
		return fmt.Errorf("failed to write metrics to logs: %w", err)
	}

	// Also write to CloudWatch Metrics API for dashboards and alarms (fire-and-forget, errors logged but not returned)
	go b.writeMetricsToMetricsAPI(ctx, jobID, metrics)

	return nil
}

// writeMetricsToLogs writes metrics to CloudWatch Logs as JSON for raw data retrieval
func (b *CloudWatchBackend) writeMetricsToLogs(ctx context.Context, jobID string, metrics []*ipcpb.Metric) error {
	// Log group and stream for metrics
	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-metrics", jobID)

	// Ensure log group and stream exist
	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	// Sort metrics by timestamp
	sortedMetrics := make([]*ipcpb.Metric, len(metrics))
	copy(sortedMetrics, metrics)
	sort.Slice(sortedMetrics, func(i, j int) bool {
		return sortedMetrics[i].Timestamp < sortedMetrics[j].Timestamp
	})

	// Convert to CloudWatch log events (JSON format)
	logEvents := make([]types.InputLogEvent, 0, len(sortedMetrics))
	for _, metric := range sortedMetrics {
		if metric.Data == nil {
			continue
		}

		// Build JSON message with all metric data
		data := metric.Data
		diskReadBytes := int64(0)
		diskWriteBytes := int64(0)
		diskReadOps := int64(0)
		diskWriteOps := int64(0)
		if data.DiskIo != nil {
			diskReadBytes = data.DiskIo.ReadBytes
			diskWriteBytes = data.DiskIo.WriteBytes
			diskReadOps = data.DiskIo.ReadOps
			diskWriteOps = data.DiskIo.WriteOps
		}
		netRxBytes := int64(0)
		netTxBytes := int64(0)
		if data.NetworkIo != nil {
			netRxBytes = data.NetworkIo.RxBytes
			netTxBytes = data.NetworkIo.TxBytes
		}

		message := fmt.Sprintf(`{"type":"metrics","ts":%d,"cpu":%.6f,"mem":%d,"gpu":%.6f,"disk_r":%d,"disk_w":%d,"disk_r_ops":%d,"disk_w_ops":%d,"net_rx":%d,"net_tx":%d}`,
			metric.Timestamp,
			data.CpuUsage,
			data.MemoryUsage,
			data.GpuUsage,
			diskReadBytes,
			diskWriteBytes,
			diskReadOps,
			diskWriteOps,
			netRxBytes,
			netTxBytes,
		)
		timestamp := metric.Timestamp / 1_000_000 // Convert nanos to millis
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	// Batch write events
	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put metric log events: %w", err)
		}
	}

	b.logger.Debug("wrote metrics to CloudWatch Logs",
		"job_uuid", jobID,
		"count", len(metrics),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// writeMetricsToMetricsAPI writes metrics to CloudWatch Metrics API for dashboards and alarms
// This is fire-and-forget - errors are logged but don't fail the write operation
func (b *CloudWatchBackend) writeMetricsToMetricsAPI(ctx context.Context, jobID string, metrics []*ipcpb.Metric) {
	// Convert metrics to CloudWatch metric data
	metricData := make([]cloudwatchtypes.MetricDatum, 0, len(metrics)*9) // Up to 9 metrics per sample

	// Base dimensions for all metrics
	baseDimensions := []cloudwatchtypes.Dimension{
		{
			Name:  aws.String("JobUUID"),
			Value: aws.String(jobID),
		},
		{
			Name:  aws.String("NodeID"),
			Value: aws.String(b.config.NodeID),
		},
	}

	// Add custom dimensions from config
	for key, value := range b.config.MetricDimensions {
		baseDimensions = append(baseDimensions, cloudwatchtypes.Dimension{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	for _, metric := range metrics {
		if metric.Data == nil {
			continue
		}

		// Convert nanoseconds to time.Time for CloudWatch
		timestamp := time.Unix(0, metric.Timestamp)

		data := metric.Data

		// CPU Usage (value is in cores, convert to percent for CloudWatch)
		// e.g., 0.5 cores = 50%, 2.0 cores = 200%
		metricData = append(metricData, cloudwatchtypes.MetricDatum{
			MetricName: aws.String("CPUUsage"),
			Unit:       cloudwatchtypes.StandardUnitPercent,
			Value:      aws.Float64(data.CpuUsage * 100),
			Timestamp:  &timestamp,
			Dimensions: baseDimensions,
		})

		// Memory Usage (convert to MB for better CloudWatch visualization)
		memoryMB := float64(data.MemoryUsage) / 1024 / 1024
		metricData = append(metricData, cloudwatchtypes.MetricDatum{
			MetricName: aws.String("MemoryUsage"),
			Unit:       cloudwatchtypes.StandardUnitMegabytes,
			Value:      aws.Float64(memoryMB),
			Timestamp:  &timestamp,
			Dimensions: baseDimensions,
		})

		// GPU Usage (only sent when GPU is allocated to the job)
		// GpuUsage > 0 indicates GPU is being used; 0 means no GPU allocated
		if data.GpuUsage > 0 {
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("GPUUsage"),
				Unit:       cloudwatchtypes.StandardUnitPercent,
				Value:      aws.Float64(data.GpuUsage * 100), // Convert 0.0-1.0 to 0-100
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
		}

		// Disk I/O metrics (always send, including zero values for complete time series)
		if data.DiskIo != nil {
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("DiskReadBytes"),
				Unit:       cloudwatchtypes.StandardUnitBytes,
				Value:      aws.Float64(float64(data.DiskIo.ReadBytes)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("DiskWriteBytes"),
				Unit:       cloudwatchtypes.StandardUnitBytes,
				Value:      aws.Float64(float64(data.DiskIo.WriteBytes)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("DiskReadOps"),
				Unit:       cloudwatchtypes.StandardUnitCount,
				Value:      aws.Float64(float64(data.DiskIo.ReadOps)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("DiskWriteOps"),
				Unit:       cloudwatchtypes.StandardUnitCount,
				Value:      aws.Float64(float64(data.DiskIo.WriteOps)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
		}

		// Network I/O metrics (always send, including zero values for complete time series)
		if data.NetworkIo != nil {
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("NetworkRxBytes"),
				Unit:       cloudwatchtypes.StandardUnitBytes,
				Value:      aws.Float64(float64(data.NetworkIo.RxBytes)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
			metricData = append(metricData, cloudwatchtypes.MetricDatum{
				MetricName: aws.String("NetworkTxBytes"),
				Unit:       cloudwatchtypes.StandardUnitBytes,
				Value:      aws.Float64(float64(data.NetworkIo.TxBytes)),
				Timestamp:  &timestamp,
				Dimensions: baseDimensions,
			})
		}
	}

	if len(metricData) == 0 {
		return
	}

	// Batch write metrics (CloudWatch allows up to 1000 metrics per request, but we use smaller batches)
	batchSize := b.config.MetricBatchSize
	for i := 0; i < len(metricData); i += batchSize {
		end := i + batchSize
		if end > len(metricData) {
			end = len(metricData)
		}
		batch := metricData[i:end]

		_, err := b.metricsClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(b.config.MetricNamespace),
			MetricData: batch,
		})
		if err != nil {
			b.logger.Warn("failed to put metric data to CloudWatch Metrics API (non-fatal)",
				"job_uuid", jobID,
				"error", err)
		}
	}
}

// WriteExecEvents writes process execution events to CloudWatch Logs
func (b *CloudWatchBackend) WriteExecEvents(jobID string, events []*ipcpb.ExecEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	// Log group and stream for exec events
	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-exec-events", jobID)

	// Ensure log group and stream exist
	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	// Sort events by timestamp
	sortedEvents := make([]*ipcpb.ExecEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	// Convert to CloudWatch log events (JSON format)
	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		// Format: JSON for structured logging
		message := fmt.Sprintf(`{"type":"exec","pid":%d,"ppid":%d,"uid":%d,"gid":%d,"comm":%q,"filename":%q,"args":%q}`,
			event.Pid, event.Ppid, event.Uid, event.Gid, event.Comm, event.Filename, strings.Join(event.Args, " "))
		timestamp := event.Timestamp / 1_000_000 // Convert nanos to millis
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	// Batch write events
	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put exec events: %w", err)
		}
	}

	b.logger.Debug("wrote exec events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteConnectEvents writes network connection events to CloudWatch Logs
func (b *CloudWatchBackend) WriteConnectEvents(jobID string, events []*ipcpb.ConnectEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	// Log group and stream for connect events
	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-connect-events", jobID)

	// Ensure log group and stream exist
	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	// Sort events by timestamp
	sortedEvents := make([]*ipcpb.ConnectEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	// Convert to CloudWatch log events (JSON format)
	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		// Format: JSON for structured logging
		message := fmt.Sprintf(`{"type":"connect","pid":%d,"comm":%q,"src":"%s:%d","dst":"%s:%d","proto":%q}`,
			event.Pid, event.Comm, event.SrcAddr, event.SrcPort, event.DstAddr, event.DstPort, event.Protocol)
		timestamp := event.Timestamp / 1_000_000 // Convert nanos to millis
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	// Batch write events
	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put connect events: %w", err)
		}
	}

	b.logger.Debug("wrote connect events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteFileEvents writes file access events to CloudWatch Logs
func (b *CloudWatchBackend) WriteFileEvents(jobID string, events []*ipcpb.FileEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-file-events", jobID)

	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	sortedEvents := make([]*ipcpb.FileEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message := fmt.Sprintf(`{"type":"file","pid":%d,"comm":%q,"path":%q,"op":%q,"bytes":%d}`,
			event.Pid, event.Comm, event.Path, event.Operation, event.Bytes)
		timestamp := event.Timestamp / 1_000_000
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put file events: %w", err)
		}
	}

	b.logger.Debug("wrote file events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteAcceptEvents writes incoming connection accept events to CloudWatch Logs
func (b *CloudWatchBackend) WriteAcceptEvents(jobID string, events []*ipcpb.AcceptEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-accept-events", jobID)

	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	sortedEvents := make([]*ipcpb.AcceptEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message := fmt.Sprintf(`{"type":"accept","pid":%d,"comm":%q,"src":"%s:%d","dst":"%s:%d","proto":%q}`,
			event.Pid, event.Comm, event.SrcAddr, event.SrcPort, event.DstAddr, event.DstPort, event.Protocol)
		timestamp := event.Timestamp / 1_000_000
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put accept events: %w", err)
		}
	}

	b.logger.Debug("wrote accept events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteSocketDataEvents writes sendto/recvfrom events to CloudWatch Logs
func (b *CloudWatchBackend) WriteSocketDataEvents(jobID string, events []*ipcpb.SocketDataEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-socket-data-events", jobID)

	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	sortedEvents := make([]*ipcpb.SocketDataEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message := fmt.Sprintf(`{"type":"socket_data","pid":%d,"comm":%q,"dir":%q,"addr":"%s:%d","proto":%q,"bytes":%d}`,
			event.Pid, event.Comm, event.Direction, event.Addr, event.Port, event.Protocol, event.Bytes)
		timestamp := event.Timestamp / 1_000_000
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put socket data events: %w", err)
		}
	}

	b.logger.Debug("wrote socket data events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteMmapEvents writes memory mapping events to CloudWatch Logs
func (b *CloudWatchBackend) WriteMmapEvents(jobID string, events []*ipcpb.MmapEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-mmap-events", jobID)

	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	sortedEvents := make([]*ipcpb.MmapEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message := fmt.Sprintf(`{"type":"mmap","pid":%d,"comm":%q,"addr":%d,"len":%d,"prot":%d,"flags":%d,"file":%q}`,
			event.Pid, event.Comm, event.Addr, event.Length, event.Prot, event.Flags, event.Filename)
		timestamp := event.Timestamp / 1_000_000
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put mmap events: %w", err)
		}
	}

	b.logger.Debug("wrote mmap events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// WriteMprotectEvents writes memory protection change events to CloudWatch Logs
func (b *CloudWatchBackend) WriteMprotectEvents(jobID string, events []*ipcpb.MprotectEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx := context.Background()

	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)
	logStream := fmt.Sprintf("%s-mprotect-events", jobID)

	if err := b.ensureLogGroup(ctx, logGroup); err != nil {
		return fmt.Errorf("failed to ensure log group: %w", err)
	}
	if err := b.ensureLogStream(ctx, logGroup, logStream); err != nil {
		return fmt.Errorf("failed to ensure log stream: %w", err)
	}

	sortedEvents := make([]*ipcpb.MprotectEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	logEvents := make([]types.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message := fmt.Sprintf(`{"type":"mprotect","pid":%d,"comm":%q,"addr":%d,"len":%d,"prot":%d}`,
			event.Pid, event.Comm, event.Addr, event.Length, event.Prot)
		timestamp := event.Timestamp / 1_000_000
		logEvents = append(logEvents, types.InputLogEvent{
			Message:   aws.String(message),
			Timestamp: aws.Int64(timestamp),
		})
	}

	batchSize := b.config.LogBatchSize
	for i := 0; i < len(logEvents); i += batchSize {
		end := i + batchSize
		if end > len(logEvents) {
			end = len(logEvents)
		}
		if err := b.putLogEvents(ctx, logGroup, logStream, logEvents[i:end]); err != nil {
			return fmt.Errorf("failed to put mprotect events: %w", err)
		}
	}

	b.logger.Debug("wrote mprotect events to CloudWatch",
		"job_uuid", jobID,
		"count", len(events),
		"logGroup", logGroup,
		"logStream", logStream)

	return nil
}

// ReadLogs reads log lines from CloudWatch Logs
func (b *CloudWatchBackend) ReadLogs(ctx context.Context, query *LogQuery) (*EventReader[*ipcpb.LogLine], error) {
	reader := NewEventReader[*ipcpb.LogLine](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readLogsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readLogsFromStream retrieves logs from CloudWatch and sends them to the channel
func (b *CloudWatchBackend) readLogsFromStream(ctx context.Context, query *LogQuery, ch chan<- *ipcpb.LogLine) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)

	// Determine stream type suffix
	streamSuffix := "stdout"
	if query.Stream == ipcpb.StreamType_STREAM_TYPE_STDERR {
		streamSuffix = "stderr"
	}
	logStream := fmt.Sprintf("%s-%s", query.JobUUID, streamSuffix)

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
			JobUuid:   query.JobUUID,
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
func (b *CloudWatchBackend) ReadMetrics(ctx context.Context, query *MetricQuery) (*EventReader[*ipcpb.Metric], error) {
	reader := NewEventReader[*ipcpb.Metric](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMetricsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readMetricsFromStream retrieves metrics from CloudWatch Logs and sends them to the channel
func (b *CloudWatchBackend) readMetricsFromStream(ctx context.Context, query *MetricQuery, ch chan<- *ipcpb.Metric) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-metrics", query.JobUUID)

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

	// Paginate through all log events
	var allEvents []types.OutputLogEvent
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			// Check for stream not found - not an error, just no metrics
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("metrics stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get metric log events: %w", err)
		}

		allEvents = append(allEvents, resp.Events...)

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken

		// Respect limit if specified
		if query.Limit > 0 && len(allEvents) >= query.Limit {
			allEvents = allEvents[:query.Limit]
			break
		}
	}

	// Parse and send metrics
	for _, event := range allEvents {
		metric, err := parseMetricFromJSON(*event.Message, query.JobUUID)
		if err != nil {
			b.logger.Warn("failed to parse metric from log", "error", err)
			continue
		}

		select {
		case ch <- metric:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// parseMetricFromJSON parses a metric from CloudWatch Logs JSON format
func parseMetricFromJSON(jsonStr string, jobID string) (*ipcpb.Metric, error) {
	var data struct {
		Type      string  `json:"type"`
		Timestamp int64   `json:"ts"`
		CPU       float64 `json:"cpu"`
		Memory    int64   `json:"mem"`
		GPU       float64 `json:"gpu"`
		DiskRead  int64   `json:"disk_r"`
		DiskWrite int64   `json:"disk_w"`
		DiskROps  int64   `json:"disk_r_ops"`
		DiskWOps  int64   `json:"disk_w_ops"`
		NetRx     int64   `json:"net_rx"`
		NetTx     int64   `json:"net_tx"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metric: %w", err)
	}

	return &ipcpb.Metric{
		JobUuid:   jobID,
		Timestamp: data.Timestamp,
		Data: &ipcpb.MetricData{
			CpuUsage:    data.CPU,
			MemoryUsage: data.Memory,
			GpuUsage:    data.GPU,
			DiskIo: &ipcpb.DiskIO{
				ReadBytes:  data.DiskRead,
				WriteBytes: data.DiskWrite,
				ReadOps:    data.DiskROps,
				WriteOps:   data.DiskWOps,
			},
			NetworkIo: &ipcpb.NetworkIO{
				RxBytes: data.NetRx,
				TxBytes: data.NetTx,
			},
		},
	}, nil
}

// DeleteJob deletes all CloudWatch log streams for a job
// Note: CloudWatch Metrics API data cannot be deleted individually (managed by retention policy)
func (b *CloudWatchBackend) DeleteJob(jobID string) error {
	ctx := context.Background()
	// Single log group per node - only delete job-specific log streams
	logGroup := fmt.Sprintf("%s/%s/jobs", b.config.LogGroupPrefix, b.config.NodeID)

	// Define the log streams for this job (stdout, stderr, metrics, and all eBPF telemetry events)
	streams := []string{
		fmt.Sprintf("%s-stdout", jobID),
		fmt.Sprintf("%s-stderr", jobID),
		fmt.Sprintf("%s-metrics", jobID),
		fmt.Sprintf("%s-exec-events", jobID),
		fmt.Sprintf("%s-connect-events", jobID),
		fmt.Sprintf("%s-file-events", jobID),
		fmt.Sprintf("%s-accept-events", jobID),
		fmt.Sprintf("%s-socket-data-events", jobID),
		fmt.Sprintf("%s-mmap-events", jobID),
		fmt.Sprintf("%s-mprotect-events", jobID),
	}

	// Delete each log stream for this job
	var errs []error
	for _, streamName := range streams {
		_, err := b.logsClient.DeleteLogStream(ctx, &cloudwatchlogs.DeleteLogStreamInput{
			LogGroupName:  aws.String(logGroup),
			LogStreamName: aws.String(streamName),
		})
		if err != nil {
			// Ignore ResourceNotFoundException - stream may not have been created
			if !strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Warn("failed to delete log stream", "logStream", streamName, "error", err)
				errs = append(errs, fmt.Errorf("stream %s: %w", streamName, err))
			} else {
				b.logger.Debug("log stream not found (already deleted or never created)", "logStream", streamName)
			}
		} else {
			b.logger.Debug("deleted log stream", "logStream", streamName)
		}

		// Clear from cache
		streamKey := fmt.Sprintf("%s/%s", logGroup, streamName)
		b.cacheMutex.Lock()
		delete(b.createdStreams, streamKey)
		b.cacheMutex.Unlock()

		// Clear sequence tokens
		b.tokenMutex.Lock()
		delete(b.sequenceTokens, streamKey)
		b.tokenMutex.Unlock()
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to delete some log streams: %v", errs)
	}

	b.logger.Info("deleted CloudWatch log streams for job", "job_uuid", jobID, "logGroup", logGroup)
	return nil
}

// Close closes the CloudWatch backend (no-op for CloudWatch client)
func (b *CloudWatchBackend) Close() error {
	b.logger.Info("CloudWatch backend closed")
	return nil
}

// ReadExecEvents reads process execution events from CloudWatch Logs
func (b *CloudWatchBackend) ReadExecEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ExecEvent], error) {
	reader := NewEventReader[*ipcpb.ExecEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readExecEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readExecEventsFromStream retrieves exec events from CloudWatch Logs
func (b *CloudWatchBackend) readExecEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ExecEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-exec-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			// Check for stream not found - not an error, just no events
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("exec events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get exec events: %w", err)
		}

		for _, event := range resp.Events {
			execEvent, err := parseExecEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse exec event", "error", err)
				continue
			}

			select {
			case ch <- execEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadConnectEvents reads network connection events from CloudWatch Logs
func (b *CloudWatchBackend) ReadConnectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ConnectEvent], error) {
	reader := NewEventReader[*ipcpb.ConnectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readConnectEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readConnectEventsFromStream retrieves connect events from CloudWatch Logs
func (b *CloudWatchBackend) readConnectEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ConnectEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-connect-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			// Check for stream not found - not an error, just no events
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("connect events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get connect events: %w", err)
		}

		for _, event := range resp.Events {
			connectEvent, err := parseConnectEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse connect event", "error", err)
				continue
			}

			select {
			case ch <- connectEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadFileEvents reads file access events from CloudWatch Logs
func (b *CloudWatchBackend) ReadFileEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.FileEvent], error) {
	reader := NewEventReader[*ipcpb.FileEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readFileEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readFileEventsFromStream retrieves file events from CloudWatch Logs
func (b *CloudWatchBackend) readFileEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.FileEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-file-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("file events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get file events: %w", err)
		}

		for _, event := range resp.Events {
			fileEvent, err := parseFileEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse file event", "error", err)
				continue
			}

			select {
			case ch <- fileEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadAcceptEvents reads incoming connection accept events from CloudWatch Logs
func (b *CloudWatchBackend) ReadAcceptEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.AcceptEvent], error) {
	reader := NewEventReader[*ipcpb.AcceptEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readAcceptEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readAcceptEventsFromStream retrieves accept events from CloudWatch Logs
func (b *CloudWatchBackend) readAcceptEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.AcceptEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-accept-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("accept events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get accept events: %w", err)
		}

		for _, event := range resp.Events {
			acceptEvent, err := parseAcceptEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse accept event", "error", err)
				continue
			}

			select {
			case ch <- acceptEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadSocketDataEvents reads sendto/recvfrom events from CloudWatch Logs
func (b *CloudWatchBackend) ReadSocketDataEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.SocketDataEvent], error) {
	reader := NewEventReader[*ipcpb.SocketDataEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readSocketDataEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readSocketDataEventsFromStream retrieves socket data events from CloudWatch Logs
func (b *CloudWatchBackend) readSocketDataEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.SocketDataEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-socket-data-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("socket data events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get socket data events: %w", err)
		}

		for _, event := range resp.Events {
			socketDataEvent, err := parseSocketDataEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse socket data event", "error", err)
				continue
			}

			select {
			case ch <- socketDataEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadMmapEvents reads memory mapping events from CloudWatch Logs
func (b *CloudWatchBackend) ReadMmapEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MmapEvent], error) {
	reader := NewEventReader[*ipcpb.MmapEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMmapEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readMmapEventsFromStream retrieves mmap events from CloudWatch Logs
func (b *CloudWatchBackend) readMmapEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MmapEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-mmap-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("mmap events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get mmap events: %w", err)
		}

		for _, event := range resp.Events {
			mmapEvent, err := parseMmapEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse mmap event", "error", err)
				continue
			}

			select {
			case ch <- mmapEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// ReadMprotectEvents reads memory protection change events from CloudWatch Logs
func (b *CloudWatchBackend) ReadMprotectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MprotectEvent], error) {
	reader := NewEventReader[*ipcpb.MprotectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMprotectEventsFromStream(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readMprotectEventsFromStream retrieves mprotect events from CloudWatch Logs
func (b *CloudWatchBackend) readMprotectEventsFromStream(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MprotectEvent) error {
	// Use passed nodeID for multi-node queries, falls back to local config.NodeID
	logGroup := b.getLogGroupForRead(query.NodeID)
	logStream := fmt.Sprintf("%s-mprotect-events", query.JobUUID)

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

	// Paginate through all log events
	var nextToken *string

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := b.logsClient.GetLogEvents(ctx, input)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceNotFoundException") {
				b.logger.Debug("mprotect events stream not found", "job_uuid", query.JobUUID)
				return nil
			}
			return fmt.Errorf("failed to get mprotect events: %w", err)
		}

		for _, event := range resp.Events {
			mprotectEvent, err := parseMprotectEventFromJSON(*event.Message, query.JobUUID, *event.Timestamp*1_000_000)
			if err != nil {
				b.logger.Warn("failed to parse mprotect event", "error", err)
				continue
			}

			select {
			case ch <- mprotectEvent:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Check if we've reached the end (no new events or token unchanged)
		if resp.NextForwardToken == nil || (nextToken != nil && *nextToken == *resp.NextForwardToken) {
			break
		}
		nextToken = resp.NextForwardToken
	}

	return nil
}

// parseExecEventFromJSON parses an exec event from CloudWatch JSON format
func parseExecEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.ExecEvent, error) {
	var data struct {
		Type     string `json:"type"`
		Pid      uint32 `json:"pid"`
		Ppid     uint32 `json:"ppid"`
		Uid      uint32 `json:"uid"`
		Gid      uint32 `json:"gid"`
		Comm     string `json:"comm"`
		Filename string `json:"filename"`
		Args     string `json:"args"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exec event: %w", err)
	}

	return &ipcpb.ExecEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Ppid:      data.Ppid,
		Uid:       data.Uid,
		Gid:       data.Gid,
		Comm:      data.Comm,
		Filename:  data.Filename,
		Args:      strings.Fields(data.Args),
	}, nil
}

// parseConnectEventFromJSON parses a connect event from CloudWatch JSON format
func parseConnectEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.ConnectEvent, error) {
	var data struct {
		Type     string `json:"type"`
		Pid      uint32 `json:"pid"`
		Comm     string `json:"comm"`
		Src      string `json:"src"`
		Dst      string `json:"dst"`
		Protocol string `json:"proto"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connect event: %w", err)
	}

	// Parse src:port and dst:port
	srcAddr, srcPort := parseAddrPort(data.Src)
	dstAddr, dstPort := parseAddrPort(data.Dst)

	return &ipcpb.ConnectEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		SrcAddr:   srcAddr,
		SrcPort:   srcPort,
		DstAddr:   dstAddr,
		DstPort:   dstPort,
		Protocol:  data.Protocol,
	}, nil
}

// parseAddrPort splits "addr:port" into address and port
func parseAddrPort(addrPort string) (string, uint32) {
	parts := strings.Split(addrPort, ":")
	if len(parts) != 2 {
		return addrPort, 0
	}
	port, _ := strconv.ParseUint(parts[1], 10, 32)
	return parts[0], uint32(port)
}

// parseFileEventFromJSON parses a file event from CloudWatch JSON format
func parseFileEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.FileEvent, error) {
	var data struct {
		Type      string `json:"type"`
		Pid       uint32 `json:"pid"`
		Comm      string `json:"comm"`
		Path      string `json:"path"`
		Operation string `json:"op"`
		Bytes     int64  `json:"bytes"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file event: %w", err)
	}

	return &ipcpb.FileEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		Path:      data.Path,
		Operation: data.Operation,
		Bytes:     data.Bytes,
	}, nil
}

// parseAcceptEventFromJSON parses an accept event from CloudWatch JSON format
func parseAcceptEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.AcceptEvent, error) {
	var data struct {
		Type     string `json:"type"`
		Pid      uint32 `json:"pid"`
		Comm     string `json:"comm"`
		Src      string `json:"src"`
		Dst      string `json:"dst"`
		Protocol string `json:"proto"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accept event: %w", err)
	}

	srcAddr, srcPort := parseAddrPort(data.Src)
	dstAddr, dstPort := parseAddrPort(data.Dst)

	return &ipcpb.AcceptEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		SrcAddr:   srcAddr,
		SrcPort:   srcPort,
		DstAddr:   dstAddr,
		DstPort:   dstPort,
		Protocol:  data.Protocol,
	}, nil
}

// parseSocketDataEventFromJSON parses a socket data event from CloudWatch JSON format
func parseSocketDataEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.SocketDataEvent, error) {
	var data struct {
		Type      string `json:"type"`
		Pid       uint32 `json:"pid"`
		Comm      string `json:"comm"`
		Direction string `json:"dir"`
		Addr      string `json:"addr"`
		Protocol  string `json:"proto"`
		Bytes     int64  `json:"bytes"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal socket data event: %w", err)
	}

	addr, port := parseAddrPort(data.Addr)

	return &ipcpb.SocketDataEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		Direction: data.Direction,
		Addr:      addr,
		Port:      port,
		Protocol:  data.Protocol,
		Bytes:     data.Bytes,
	}, nil
}

// parseMmapEventFromJSON parses an mmap event from CloudWatch JSON format
func parseMmapEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.MmapEvent, error) {
	var data struct {
		Type     string `json:"type"`
		Pid      uint32 `json:"pid"`
		Comm     string `json:"comm"`
		Addr     uint64 `json:"addr"`
		Length   uint64 `json:"len"`
		Prot     uint32 `json:"prot"`
		Flags    uint32 `json:"flags"`
		Filename string `json:"file"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mmap event: %w", err)
	}

	return &ipcpb.MmapEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		Addr:      data.Addr,
		Length:    data.Length,
		Prot:      data.Prot,
		Flags:     data.Flags,
		Filename:  data.Filename,
	}, nil
}

// parseMprotectEventFromJSON parses an mprotect event from CloudWatch JSON format
func parseMprotectEventFromJSON(jsonStr string, jobID string, timestamp int64) (*ipcpb.MprotectEvent, error) {
	var data struct {
		Type   string `json:"type"`
		Pid    uint32 `json:"pid"`
		Comm   string `json:"comm"`
		Addr   uint64 `json:"addr"`
		Length uint64 `json:"len"`
		Prot   uint32 `json:"prot"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mprotect event: %w", err)
	}

	return &ipcpb.MprotectEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Pid:       data.Pid,
		Comm:      data.Comm,
		Addr:      data.Addr,
		Length:    data.Length,
		Prot:      data.Prot,
	}, nil
}
