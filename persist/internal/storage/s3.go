package storage

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// S3Backend implements the Backend interface for AWS S3
type S3Backend struct {
	config   *config.S3Config
	s3Client *s3.Client
	logger   *logger.Logger

	// Write buffers - one per job/stream combination
	buffers   map[string]*s3Buffer
	buffersMu sync.RWMutex

	// Background flush goroutine
	flushTicker *time.Ticker
	flushDone   chan struct{}
}

// s3Buffer holds buffered data for a specific job/stream
type s3Buffer struct {
	jobID      string
	streamType string // "stdout", "stderr", "metrics", "exec-events", etc.
	data       *bytes.Buffer
	gzWriter   *gzip.Writer
	count      int       // Number of records in buffer
	lastWrite  time.Time // Last write timestamp
	mu         sync.Mutex
}

// NewS3Backend creates a new S3 storage backend
func NewS3Backend(cfg *config.StorageConfig, nodeID string, log *logger.Logger) (Backend, error) {
	if log == nil {
		log = logger.New().WithField("component", "s3-backend")
	}

	s3Config := cfg.S3

	// Set nodeID (inherited from server config)
	s3Config.NodeID = nodeID

	// Validate required fields
	if s3Config.Region == "" {
		return nil, fmt.Errorf("s3.region is required")
	}
	if s3Config.Bucket == "" {
		return nil, fmt.Errorf("s3.bucket is required")
	}

	// Set defaults
	if s3Config.KeyPrefix == "" {
		s3Config.KeyPrefix = "jobs/"
	}
	// Ensure prefix ends with /
	if !strings.HasSuffix(s3Config.KeyPrefix, "/") {
		s3Config.KeyPrefix += "/"
	}
	if s3Config.FlushInterval == 0 {
		s3Config.FlushInterval = 30 // 30 seconds
	}
	if s3Config.FlushThreshold == 0 {
		s3Config.FlushThreshold = 5 * 1024 * 1024 // 5MB
	}
	if s3Config.MaxBufferSize == 0 {
		s3Config.MaxBufferSize = 50 * 1024 * 1024 // 50MB
	}
	if s3Config.StorageClass == "" {
		s3Config.StorageClass = "STANDARD"
	}

	// Load AWS configuration
	log.Info("using AWS default credential chain (IAM role, instance profile, or environment variables)")
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(s3Config.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)

	backend := &S3Backend{
		config:    &s3Config,
		s3Client:  s3Client,
		logger:    log,
		buffers:   make(map[string]*s3Buffer),
		flushDone: make(chan struct{}),
	}

	// Start background flush goroutine
	backend.flushTicker = time.NewTicker(time.Duration(s3Config.FlushInterval) * time.Second)
	go backend.backgroundFlush()

	log.Info("S3 backend initialized successfully",
		"region", s3Config.Region,
		"bucket", s3Config.Bucket,
		"keyPrefix", s3Config.KeyPrefix,
		"storageClass", s3Config.StorageClass)

	return backend, nil
}

// backgroundFlush periodically flushes all buffers
func (b *S3Backend) backgroundFlush() {
	for {
		select {
		case <-b.flushTicker.C:
			b.flushAllBuffers(false)
		case <-b.flushDone:
			return
		}
	}
}

// flushAllBuffers flushes all buffers to S3
func (b *S3Backend) flushAllBuffers(force bool) {
	b.buffersMu.RLock()
	bufferKeys := make([]string, 0, len(b.buffers))
	for key := range b.buffers {
		bufferKeys = append(bufferKeys, key)
	}
	b.buffersMu.RUnlock()

	for _, key := range bufferKeys {
		b.buffersMu.RLock()
		buf, exists := b.buffers[key]
		b.buffersMu.RUnlock()

		if !exists {
			continue
		}

		buf.mu.Lock()
		shouldFlush := force || buf.data.Len() > 0
		if shouldFlush {
			if err := b.flushBuffer(buf); err != nil {
				b.logger.Warn("failed to flush buffer", "key", key, "error", err)
			}
		}
		buf.mu.Unlock()
	}
}

// getOrCreateBuffer gets or creates a buffer for a job/stream
func (b *S3Backend) getOrCreateBuffer(jobID, streamType string) *s3Buffer {
	key := fmt.Sprintf("%s/%s", jobID, streamType)

	b.buffersMu.RLock()
	buf, exists := b.buffers[key]
	b.buffersMu.RUnlock()

	if exists {
		return buf
	}

	b.buffersMu.Lock()
	defer b.buffersMu.Unlock()

	// Double-check after acquiring write lock
	if buf, exists := b.buffers[key]; exists {
		return buf
	}

	// Create new buffer
	data := &bytes.Buffer{}
	buf = &s3Buffer{
		jobID:      jobID,
		streamType: streamType,
		data:       data,
		gzWriter:   gzip.NewWriter(data),
		lastWrite:  time.Now(),
	}
	b.buffers[key] = buf

	return buf
}

// flushBuffer uploads buffer contents to S3 and resets the buffer
// Uses time-partitioned keys to avoid expensive read-modify-write operations
// Caller must hold buf.mu lock
func (b *S3Backend) flushBuffer(buf *s3Buffer) error {
	if buf.data.Len() == 0 {
		return nil
	}

	// Close gzip writer to finalize the stream
	if err := buf.gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Build time-partitioned S3 key: {prefix}{nodeID}/{jobID}/{streamType}/{timestamp}.jsonl.gz
	// Using nanosecond timestamp ensures uniqueness even with rapid flushes
	timestamp := time.Now().UnixNano()
	key := fmt.Sprintf("%s%s/%s/%s/%d.jsonl.gz",
		b.config.KeyPrefix,
		b.config.NodeID,
		buf.jobID,
		buf.streamType,
		timestamp,
	)

	// Upload to S3 - no need to read existing data, just create new object
	if err := b.putObject(context.Background(), key, buf.data.Bytes()); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	b.logger.Debug("flushed buffer to S3",
		"job_uuid", buf.jobID,
		"stream", buf.streamType,
		"key", key,
		"count", buf.count,
		"size", buf.data.Len())

	// Reset buffer
	buf.data = &bytes.Buffer{}
	buf.gzWriter = gzip.NewWriter(buf.data)
	buf.count = 0
	buf.lastWrite = time.Now()

	return nil
}

// putObject uploads data to S3
func (b *S3Backend) putObject(ctx context.Context, key string, data []byte) error {
	input := &s3.PutObjectInput{
		Bucket:       aws.String(b.config.Bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String("application/gzip"),
		StorageClass: types.StorageClass(b.config.StorageClass),
	}

	// Add server-side encryption if configured
	if b.config.ServerSideEncryption == "AES256" {
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	} else if b.config.ServerSideEncryption == "aws:kms" {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		if b.config.KMSKeyID != "" {
			input.SSEKMSKeyId = aws.String(b.config.KMSKeyID)
		}
	}

	_, err := b.s3Client.PutObject(ctx, input)
	return err
}

// getObject retrieves data from S3
func (b *S3Backend) getObject(ctx context.Context, key string) ([]byte, error) {
	resp, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// writeToBuffer writes data to the appropriate buffer
func (b *S3Backend) writeToBuffer(jobID, streamType string, data []byte) error {
	buf := b.getOrCreateBuffer(jobID, streamType)

	buf.mu.Lock()
	defer buf.mu.Unlock()

	// Check if buffer would exceed max size
	if buf.data.Len()+len(data) > b.config.MaxBufferSize {
		// Flush first
		if err := b.flushBuffer(buf); err != nil {
			return fmt.Errorf("failed to flush buffer before write: %w", err)
		}
	}

	// Write to buffer
	if _, err := buf.gzWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write to buffer: %w", err)
	}
	buf.count++
	buf.lastWrite = time.Now()

	// Flush if threshold reached
	if buf.data.Len() >= b.config.FlushThreshold {
		if err := b.flushBuffer(buf); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}

	return nil
}

// WriteLogs writes log lines to S3
func (b *S3Backend) WriteLogs(jobID string, logs []*ipcpb.LogLine) error {
	if len(logs) == 0 {
		return nil
	}

	// Sort logs by timestamp
	sortedLogs := make([]*ipcpb.LogLine, len(logs))
	copy(sortedLogs, logs)
	sort.Slice(sortedLogs, func(i, j int) bool {
		return sortedLogs[i].Timestamp < sortedLogs[j].Timestamp
	})

	// Group logs by stream type
	for _, log := range sortedLogs {
		streamType := "stdout"
		if log.Stream == ipcpb.StreamType_STREAM_TYPE_STDERR {
			streamType = "stderr"
		}

		data, err := json.Marshal(log)
		if err != nil {
			return fmt.Errorf("failed to marshal log: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, streamType, data); err != nil {
			return fmt.Errorf("failed to write log to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote logs to buffer", "job_uuid", jobID, "count", len(logs))
	return nil
}

// WriteMetrics writes metrics to S3
func (b *S3Backend) WriteMetrics(jobID string, metrics []*ipcpb.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Sort metrics by timestamp
	sortedMetrics := make([]*ipcpb.Metric, len(metrics))
	copy(sortedMetrics, metrics)
	sort.Slice(sortedMetrics, func(i, j int) bool {
		return sortedMetrics[i].Timestamp < sortedMetrics[j].Timestamp
	})

	for _, metric := range sortedMetrics {
		data, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "metrics", data); err != nil {
			return fmt.Errorf("failed to write metric to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote metrics to buffer", "job_uuid", jobID, "count", len(metrics))
	return nil
}

// WriteExecEvents writes exec events to S3
func (b *S3Backend) WriteExecEvents(jobID string, events []*ipcpb.ExecEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.ExecEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal exec event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "exec-events", data); err != nil {
			return fmt.Errorf("failed to write exec event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote exec events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteConnectEvents writes connect events to S3
func (b *S3Backend) WriteConnectEvents(jobID string, events []*ipcpb.ConnectEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.ConnectEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal connect event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "connect-events", data); err != nil {
			return fmt.Errorf("failed to write connect event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote connect events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteFileEvents writes file events to S3
func (b *S3Backend) WriteFileEvents(jobID string, events []*ipcpb.FileEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.FileEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal file event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "file-events", data); err != nil {
			return fmt.Errorf("failed to write file event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote file events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteAcceptEvents writes accept events to S3
func (b *S3Backend) WriteAcceptEvents(jobID string, events []*ipcpb.AcceptEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.AcceptEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal accept event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "accept-events", data); err != nil {
			return fmt.Errorf("failed to write accept event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote accept events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteSocketDataEvents writes socket data events to S3
func (b *S3Backend) WriteSocketDataEvents(jobID string, events []*ipcpb.SocketDataEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.SocketDataEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal socket data event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "socket-data-events", data); err != nil {
			return fmt.Errorf("failed to write socket data event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote socket data events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteMmapEvents writes mmap events to S3
func (b *S3Backend) WriteMmapEvents(jobID string, events []*ipcpb.MmapEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.MmapEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal mmap event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "mmap-events", data); err != nil {
			return fmt.Errorf("failed to write mmap event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote mmap events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// WriteMprotectEvents writes mprotect events to S3
func (b *S3Backend) WriteMprotectEvents(jobID string, events []*ipcpb.MprotectEvent) error {
	if len(events) == 0 {
		return nil
	}

	sortedEvents := make([]*ipcpb.MprotectEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	for _, event := range sortedEvents {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal mprotect event: %w", err)
		}
		data = append(data, '\n')

		if err := b.writeToBuffer(jobID, "mprotect-events", data); err != nil {
			return fmt.Errorf("failed to write mprotect event to buffer: %w", err)
		}
	}

	b.logger.Debug("wrote mprotect events to buffer", "job_uuid", jobID, "count", len(events))
	return nil
}

// getS3KeyPrefix returns the S3 key prefix for listing time-partitioned objects
func (b *S3Backend) getS3KeyPrefix(nodeID, jobID, streamType string) string {
	effectiveNodeID := nodeID
	if effectiveNodeID == "" {
		effectiveNodeID = b.config.NodeID
	}
	return fmt.Sprintf("%s%s/%s/%s/",
		b.config.KeyPrefix,
		effectiveNodeID,
		jobID,
		streamType,
	)
}

// listObjects lists all objects with the given prefix, sorted by key (chronological order)
func (b *S3Backend) listObjects(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(b.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.config.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}

	// Keys are already sorted by S3, but ensure chronological order
	// Since keys end with nanosecond timestamps, lexicographic sort works
	sort.Strings(keys)

	return keys, nil
}

// ReadLogs reads logs from S3
func (b *S3Backend) ReadLogs(ctx context.Context, query *LogQuery) (*EventReader[*ipcpb.LogLine], error) {
	reader := NewEventReader[*ipcpb.LogLine](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readLogsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readLogsFromS3 retrieves logs from S3 and sends them to the channel
// Reads from multiple time-partitioned objects in chronological order
func (b *S3Backend) readLogsFromS3(ctx context.Context, query *LogQuery, ch chan<- *ipcpb.LogLine) error {
	// Determine stream type
	streamType := "stdout"
	if query.Stream == ipcpb.StreamType_STREAM_TYPE_STDERR {
		streamType = "stderr"
	}

	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, streamType)

	// List all objects for this stream
	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		// Check for access errors vs no data
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("logs not found in S3", "job_uuid", query.JobUUID, "stream", streamType)
			return nil
		}
		return fmt.Errorf("failed to list logs from S3: %w", err)
	}

	if len(keys) == 0 {
		b.logger.Debug("no log objects found in S3", "job_uuid", query.JobUUID, "stream", streamType)
		return nil
	}

	count := 0
	skipped := 0

	// Read each object in chronological order
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if we've reached the limit
		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get log object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var logLine ipcpb.LogLine
			if err := json.Unmarshal(line, &logLine); err != nil {
				b.logger.Warn("failed to unmarshal log line", "error", err)
				continue
			}

			// Apply time range filter
			if query.StartTime != nil && logLine.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && logLine.Timestamp > *query.EndTime {
				continue
			}

			// Apply text filter
			if query.Filter != "" && !strings.Contains(string(logLine.Content), query.Filter) {
				continue
			}

			// Apply offset
			if skipped < query.Offset {
				skipped++
				continue
			}

			// Apply limit
			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &logLine:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		if err := scanner.Err(); err != nil {
			b.logger.Warn("error reading log object", "key", key, "error", err)
		}

		gzReader.Close()
	}

	return nil
}

// ReadMetrics reads metrics from S3
func (b *S3Backend) ReadMetrics(ctx context.Context, query *MetricQuery) (*EventReader[*ipcpb.Metric], error) {
	reader := NewEventReader[*ipcpb.Metric](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMetricsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

// readMetricsFromS3 retrieves metrics from S3 and sends them to the channel
// Reads from multiple time-partitioned objects in chronological order
func (b *S3Backend) readMetricsFromS3(ctx context.Context, query *MetricQuery, ch chan<- *ipcpb.Metric) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "metrics")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("metrics not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list metrics from S3: %w", err)
	}

	if len(keys) == 0 {
		b.logger.Debug("no metric objects found in S3", "job_uuid", query.JobUUID)
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get metric object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var metric ipcpb.Metric
			if err := json.Unmarshal(line, &metric); err != nil {
				b.logger.Warn("failed to unmarshal metric", "error", err)
				continue
			}

			if query.StartTime != nil && metric.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && metric.Timestamp > *query.EndTime {
				continue
			}

			if skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &metric:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		if err := scanner.Err(); err != nil {
			b.logger.Warn("error reading metric object", "key", key, "error", err)
		}

		gzReader.Close()
	}

	return nil
}

// ReadExecEvents reads exec events from S3
func (b *S3Backend) ReadExecEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ExecEvent], error) {
	reader := NewEventReader[*ipcpb.ExecEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readExecEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readExecEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ExecEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "exec-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("exec events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list exec events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get exec event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.ExecEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal exec event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadConnectEvents reads connect events from S3
func (b *S3Backend) ReadConnectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ConnectEvent], error) {
	reader := NewEventReader[*ipcpb.ConnectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readConnectEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readConnectEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ConnectEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "connect-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("connect events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list connect events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get connect event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.ConnectEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal connect event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadFileEvents reads file events from S3
func (b *S3Backend) ReadFileEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.FileEvent], error) {
	reader := NewEventReader[*ipcpb.FileEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readFileEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readFileEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.FileEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "file-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("file events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list file events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get file event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.FileEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal file event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadAcceptEvents reads accept events from S3
func (b *S3Backend) ReadAcceptEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.AcceptEvent], error) {
	reader := NewEventReader[*ipcpb.AcceptEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readAcceptEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readAcceptEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.AcceptEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "accept-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("accept events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list accept events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get accept event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.AcceptEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal accept event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadSocketDataEvents reads socket data events from S3
func (b *S3Backend) ReadSocketDataEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.SocketDataEvent], error) {
	reader := NewEventReader[*ipcpb.SocketDataEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readSocketDataEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readSocketDataEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.SocketDataEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "socket-data-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("socket data events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list socket data events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get socket data event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.SocketDataEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal socket data event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadMmapEvents reads mmap events from S3
func (b *S3Backend) ReadMmapEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MmapEvent], error) {
	reader := NewEventReader[*ipcpb.MmapEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMmapEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readMmapEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MmapEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "mmap-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("mmap events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list mmap events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get mmap event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.MmapEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal mmap event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// ReadMprotectEvents reads mprotect events from S3
func (b *S3Backend) ReadMprotectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MprotectEvent], error) {
	reader := NewEventReader[*ipcpb.MprotectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(b.readMprotectEventsFromS3(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (b *S3Backend) readMprotectEventsFromS3(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MprotectEvent) error {
	prefix := b.getS3KeyPrefix(query.NodeID, query.JobUUID, "mprotect-events")

	keys, err := b.listObjects(ctx, prefix)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NoSuchBucket") {
			b.logger.Debug("mprotect events not found in S3", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to list mprotect events from S3: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	count := 0
	skipped := 0

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if query.Limit > 0 && count >= query.Limit {
			return nil
		}

		data, err := b.getObject(ctx, key)
		if err != nil {
			b.logger.Warn("failed to get mprotect event object", "key", key, "error", err)
			continue
		}

		gzReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			b.logger.Warn("failed to create gzip reader", "key", key, "error", err)
			continue
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			default:
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event ipcpb.MprotectEvent
			if err := json.Unmarshal(line, &event); err != nil {
				b.logger.Warn("failed to unmarshal mprotect event", "error", err)
				continue
			}

			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			if query.Limit > 0 && count >= query.Limit {
				gzReader.Close()
				return nil
			}

			select {
			case ch <- &event:
				count++
			case <-ctx.Done():
				gzReader.Close()
				return ctx.Err()
			}
		}

		gzReader.Close()
	}

	return nil
}

// DeleteJob deletes all S3 objects for a job
func (b *S3Backend) DeleteJob(jobID string) error {
	ctx := context.Background()

	// First, flush any buffered data for this job
	b.buffersMu.Lock()
	for key, buf := range b.buffers {
		if strings.HasPrefix(key, jobID+"/") {
			buf.mu.Lock()
			// Discard buffer without flushing
			buf.data.Reset()
			buf.count = 0
			buf.mu.Unlock()
			delete(b.buffers, key)
		}
	}
	b.buffersMu.Unlock()

	// List and delete all objects for this job
	prefix := fmt.Sprintf("%s%s/%s/", b.config.KeyPrefix, b.config.NodeID, jobID)

	var objectsToDelete []types.ObjectIdentifier
	paginator := s3.NewListObjectsV2Paginator(b.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.config.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects for deletion: %w", err)
		}

		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objectsToDelete) == 0 {
		b.logger.Debug("no S3 objects to delete for job", "job_uuid", jobID)
		return nil
	}

	// Delete objects in batches of 1000 (S3 limit)
	for i := 0; i < len(objectsToDelete); i += 1000 {
		end := i + 1000
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}
		batch := objectsToDelete[i:end]

		_, err := b.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(b.config.Bucket),
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete S3 objects: %w", err)
		}
	}

	b.logger.Info("deleted S3 objects for job", "job_uuid", jobID, "count", len(objectsToDelete))
	return nil
}

// Close closes the S3 backend and flushes all pending data
func (b *S3Backend) Close() error {
	// Stop background flush
	b.flushTicker.Stop()
	close(b.flushDone)

	// Flush all remaining buffers
	b.flushAllBuffers(true)

	b.logger.Info("S3 backend closed")
	return nil
}

// Ensure S3Backend implements Backend interface
var _ Backend = (*S3Backend)(nil)
