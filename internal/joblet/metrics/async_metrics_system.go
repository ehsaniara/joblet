package metrics

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"joblet/internal/joblet/metrics/domain"
	"joblet/pkg/logger"
)

// AsyncMetricsSystem provides rate-decoupled metrics persistence
// Following the same pattern as AsyncLogSystem but optimized for time-series data
type AsyncMetricsSystem struct {
	// Fast producer side (collectors write here)
	metricsQueue chan *domain.JobMetricsSample
	queueSize    int

	// Consumer side (background disk writer)
	diskWriter *MetricsDiskWriter
	diskReader *MetricsDiskReader

	// Configuration
	config *domain.MetricsConfig

	// Metrics and monitoring
	metrics *MetricsSystemMetrics
	logger  *logger.Logger

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MetricsSystemMetrics tracks performance and health
type MetricsSystemMetrics struct {
	QueueUsage        int64 // Current queue size
	QueueCapacity     int64 // Max queue size
	SamplesWritten    int64 // Total samples written
	DroppedSamples    int64 // Samples dropped (overflow)
	TotalBytesWritten int64 // Total bytes processed
	DiskLagSeconds    int64 // How far behind disk writer is
}

// MetricsDiskWriter handles background writing to disk in time-series format
type MetricsDiskWriter struct {
	files      map[string]*metricsFile // jobID -> file
	filesMutex sync.RWMutex
	baseDir    string
	format     string // jsonl, protobuf, or msgpack
	compress   bool
	logger     *logger.Logger
}

// metricsFile represents an open metrics file with metadata
type metricsFile struct {
	file        *os.File
	gzWriter    *gzip.Writer
	jobID       string
	startTime   time.Time
	sampleCount int64
	byteCount   int64
}

// NewAsyncMetricsSystem creates a new rate-decoupled async metrics system
func NewAsyncMetricsSystem(config *domain.MetricsConfig, logger *logger.Logger) *AsyncMetricsSystem {
	ctx, cancel := context.WithCancel(context.Background())

	// Use configuration values or defaults
	queueSize := 1000
	baseDir := "/opt/joblet/metrics"
	format := "jsonl"
	compress := true

	if config != nil && config.Storage.Directory != "" {
		baseDir = config.Storage.Directory
	}

	system := &AsyncMetricsSystem{
		metricsQueue: make(chan *domain.JobMetricsSample, queueSize),
		queueSize:    queueSize,
		config:       config,
		metrics:      &MetricsSystemMetrics{QueueCapacity: int64(queueSize)},
		logger:       logger.WithField("component", "async-metrics-system"),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Create disk writer
	system.diskWriter = &MetricsDiskWriter{
		files:    make(map[string]*metricsFile),
		baseDir:  baseDir,
		format:   format,
		compress: compress,
		logger:   logger.WithField("component", "metrics-disk-writer"),
	}

	// Create disk reader
	system.diskReader = NewMetricsDiskReader(baseDir, logger.WithField("component", "metrics-disk-reader"))

	// Create metrics directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		logger.Error("failed to create metrics directory", "error", err, "dir", baseDir)
	}

	// TODO: Implement retention cleanup - delete metrics files older than config.Storage.Retention.Days
	// This should run periodically (e.g., daily) to clean up old metrics and prevent disk space issues

	// Start background workers
	system.startWorkers()

	return system
}

// WriteMetrics writes a metrics sample asynchronously with zero performance impact on collectors
func (a *AsyncMetricsSystem) WriteMetrics(sample *domain.JobMetricsSample) {
	// Non-blocking write - collector NEVER waits
	select {
	case a.metricsQueue <- sample:
		// Success: queued for async processing
		atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.metricsQueue)))
	default:
		// Queue full: drop oldest (metrics are sampled, loss acceptable)
		atomic.AddInt64(&a.metrics.DroppedSamples, 1)
		a.logger.Warn("metrics queue full, dropping sample", "jobId", sample.JobID)
	}
}

// startWorkers starts background processing goroutines
func (a *AsyncMetricsSystem) startWorkers() {
	// Start disk writer worker
	a.wg.Add(1)
	go a.diskWriterLoop()

	// Start metrics updater
	a.wg.Add(1)
	go a.metricsLoop()
}

// diskWriterLoop processes the metrics queue in batches for optimal disk I/O
func (a *AsyncMetricsSystem) diskWriterLoop() {
	defer a.wg.Done()

	// Use configurable batch size and flush interval
	batchSize := 100
	flushInterval := 5 * time.Second

	batch := make([]*domain.JobMetricsSample, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case sample := <-a.metricsQueue:
			batch = append(batch, sample)
			atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.metricsQueue)))

			// Flush when batch is full
			if len(batch) >= batchSize {
				a.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Periodic flush even if batch not full
			if len(batch) > 0 {
				a.flushBatch(batch)
				batch = batch[:0]
			}

		case <-a.ctx.Done():
			// Flush remaining batch on shutdown
			if len(batch) > 0 {
				a.flushBatch(batch)
			}
			return
		}
	}
}

// flushBatch writes a batch of samples to disk
func (a *AsyncMetricsSystem) flushBatch(batch []*domain.JobMetricsSample) {
	// Group by job for efficient writing
	jobBatches := make(map[string][]*domain.JobMetricsSample)
	for _, sample := range batch {
		jobBatches[sample.JobID] = append(jobBatches[sample.JobID], sample)
	}

	// Write each job's samples
	for jobID, samples := range jobBatches {
		metricsFile := a.diskWriter.getOrCreateMetricsFile(jobID)
		if metricsFile != nil {
			for _, sample := range samples {
				if err := a.diskWriter.writeSample(metricsFile, sample); err != nil {
					a.logger.Warn("failed to write metrics sample", "jobId", jobID, "error", err)
				} else {
					atomic.AddInt64(&a.metrics.SamplesWritten, 1)
				}
			}

			// Flush to ensure durability
			if err := metricsFile.Sync(); err != nil {
				a.logger.Warn("failed to sync metrics file", "jobId", jobID, "error", err)
			}
		}
	}
}

// metricsLoop updates metrics periodically
func (a *AsyncMetricsSystem) metricsLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.updateMetrics()
		case <-a.ctx.Done():
			return
		}
	}
}

// updateMetrics calculates current system metrics
func (a *AsyncMetricsSystem) updateMetrics() {
	queueUsage := float64(len(a.metricsQueue)) / float64(a.queueSize)

	if queueUsage > 0.8 {
		a.logger.Warn("metrics queue usage high", "usage", queueUsage)
	}

	// Update queue metrics
	atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.metricsQueue)))

	// Log metrics periodically
	a.logger.Debug("async metrics system status",
		"queueUsage", atomic.LoadInt64(&a.metrics.QueueUsage),
		"samplesWritten", atomic.LoadInt64(&a.metrics.SamplesWritten),
		"droppedSamples", atomic.LoadInt64(&a.metrics.DroppedSamples),
		"totalBytes", atomic.LoadInt64(&a.metrics.TotalBytesWritten))
}

// GetMetrics returns current system metrics
func (a *AsyncMetricsSystem) GetMetrics() *MetricsSystemMetrics {
	return &MetricsSystemMetrics{
		QueueUsage:        atomic.LoadInt64(&a.metrics.QueueUsage),
		QueueCapacity:     a.metrics.QueueCapacity,
		SamplesWritten:    atomic.LoadInt64(&a.metrics.SamplesWritten),
		DroppedSamples:    atomic.LoadInt64(&a.metrics.DroppedSamples),
		TotalBytesWritten: atomic.LoadInt64(&a.metrics.TotalBytesWritten),
	}
}

// Close shuts down the async metrics system gracefully
func (a *AsyncMetricsSystem) Close() error {
	a.logger.Info("shutting down async metrics system")
	a.cancel()
	a.wg.Wait()

	// Close disk writer
	return a.diskWriter.Close()
}

// DeleteJobMetricsFiles deletes all metrics files for a specific job
func (a *AsyncMetricsSystem) DeleteJobMetricsFiles(jobID string) error {
	return a.diskWriter.DeleteJobMetricsFiles(jobID)
}

// GetReader returns the disk reader for reading historical metrics
func (a *AsyncMetricsSystem) GetReader() *MetricsDiskReader {
	return a.diskReader
}

// MetricsDiskWriter methods

// getOrCreateMetricsFile gets or creates a metrics file for a job
func (dw *MetricsDiskWriter) getOrCreateMetricsFile(jobID string) *metricsFile {
	dw.filesMutex.RLock()
	file, exists := dw.files[jobID]
	dw.filesMutex.RUnlock()

	if exists {
		return file
	}

	dw.filesMutex.Lock()
	defer dw.filesMutex.Unlock()

	// Double-check pattern
	if file, exists := dw.files[jobID]; exists {
		return file
	}

	// Create job metrics directory
	jobDir := filepath.Join(dw.baseDir, jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		dw.logger.Error("failed to create job metrics directory", "jobId", jobID, "error", err)
		return nil
	}

	// Create new metrics file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s.%s", timestamp, dw.format)
	if dw.compress {
		filename += ".gz"
	}

	filePath := filepath.Join(jobDir, filename)
	f, err := os.Create(filePath)
	if err != nil {
		dw.logger.Error("failed to create metrics file", "jobId", jobID, "error", err)
		return nil
	}

	metricsFile := &metricsFile{
		file:      f,
		jobID:     jobID,
		startTime: time.Now(),
	}

	if dw.compress {
		metricsFile.gzWriter = gzip.NewWriter(f)
	}

	dw.files[jobID] = metricsFile
	dw.logger.Info("created metrics file", "jobId", jobID, "path", filePath)

	return metricsFile
}

// writeSample writes a single metrics sample to the file
func (dw *MetricsDiskWriter) writeSample(mf *metricsFile, sample *domain.JobMetricsSample) error {
	// Currently only support JSON Lines format
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("failed to marshal sample: %w", err)
	}

	data = append(data, '\n') // Add newline for JSON Lines format

	var n int
	if mf.gzWriter != nil {
		n, err = mf.gzWriter.Write(data)
	} else {
		n, err = mf.file.Write(data)
	}

	if err != nil {
		return fmt.Errorf("failed to write sample: %w", err)
	}

	mf.sampleCount++
	mf.byteCount += int64(n)

	return nil
}

// Sync flushes the file to disk
func (mf *metricsFile) Sync() error {
	if mf.gzWriter != nil {
		if err := mf.gzWriter.Flush(); err != nil {
			return err
		}
	}
	return mf.file.Sync()
}

// Close closes all metrics files
func (dw *MetricsDiskWriter) Close() error {
	dw.filesMutex.Lock()
	defer dw.filesMutex.Unlock()

	for jobID, mf := range dw.files {
		if mf.gzWriter != nil {
			if err := mf.gzWriter.Close(); err != nil {
				dw.logger.Error("failed to close gzip writer", "jobId", jobID, "error", err)
			}
		}
		if err := mf.file.Close(); err != nil {
			dw.logger.Error("failed to close metrics file", "jobId", jobID, "error", err)
		}

		dw.logger.Info("closed metrics file", "jobId", jobID, "samples", mf.sampleCount, "bytes", mf.byteCount)
	}

	dw.files = make(map[string]*metricsFile)
	return nil
}

// DeleteJobMetricsFiles removes all metrics files for a specific job
func (dw *MetricsDiskWriter) DeleteJobMetricsFiles(jobID string) error {
	dw.filesMutex.Lock()
	defer dw.filesMutex.Unlock()

	// Close the file if it's open
	if mf, exists := dw.files[jobID]; exists {
		if mf.gzWriter != nil {
			_ = mf.gzWriter.Close()
		}
		if err := mf.file.Close(); err != nil {
			dw.logger.Warn("failed to close metrics file before deletion", "jobId", jobID, "error", err)
		}
		delete(dw.files, jobID)
	}

	// Delete the job metrics directory
	jobDir := filepath.Join(dw.baseDir, jobID)
	if err := os.RemoveAll(jobDir); err != nil {
		return fmt.Errorf("failed to delete metrics directory for job %s: %w", jobID, err)
	}

	dw.logger.Info("deleted metrics files", "jobId", jobID, "directory", jobDir)
	return nil
}
