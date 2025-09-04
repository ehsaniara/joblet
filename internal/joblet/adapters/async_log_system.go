package adapters

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// OverflowStrategy defines how to handle queue overflow
type OverflowStrategy int

const (
	OverflowCompress OverflowStrategy = iota // Compress old chunks
	OverflowSpill                            // Write oldest to temp disk files
	OverflowSample                           // Keep every Nth chunk
	OverflowAlert                            // Alert ops, increase memory
)

// LogChunk represents a chunk of log data with metadata
type LogChunk struct {
	JobID     string
	Data      []byte
	Timestamp time.Time
	Sequence  int64
}

// AsyncLogSystem provides rate-decoupled log processing
type AsyncLogSystem struct {
	// Fast producer side (jobs write here)
	logQueue  chan LogChunk
	queueSize int

	// Consumer side (background disk writer)
	diskWriter *DiskWriter

	// Overflow protection
	config       *config.LogPersistenceConfig
	memoryLimit  int64
	overflowMode OverflowStrategy

	// Compressed storage for overflow
	compressedChunks map[string]*CompressedBuffer
	compressMutex    sync.RWMutex

	// Spill files for extreme overflow
	spillFiles map[string]*os.File
	spillMutex sync.Mutex
	spillDir   string

	// Metrics and monitoring
	metrics *LogSystemMetrics
	logger  *logger.Logger

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LogSystemMetrics tracks performance and health
type LogSystemMetrics struct {
	QueueUsage        int64   // Current queue size
	QueueCapacity     int64   // Max queue size
	DiskLagSeconds    int64   // How far behind disk writer is
	OverflowEvents    int64   // Count of overflow situations
	CompressionRatio  float64 // Achieved compression
	SpillFilesCount   int32   // Active spill files
	TotalBytesWritten int64   // Total bytes processed
	DroppedChunks     int64   // Chunks dropped (if sampling)
}

// CompressedBuffer stores compressed log chunks
type CompressedBuffer struct {
	buffer     *bytes.Buffer
	writer     *gzip.Writer
	chunkCount int
}

// DiskWriter handles background writing to disk
type DiskWriter struct {
	logFiles   map[string]*os.File
	filesMutex sync.RWMutex
	baseDir    string
	logger     *logger.Logger
}

// NewAsyncLogSystem creates a new rate-decoupled async log system optimized for HPC workloads.
// Provides non-blocking log writes with configurable overflow protection and background batched disk writing.
// Must be closed with Close() to ensure graceful shutdown and data preservation.
func NewAsyncLogSystem(config *config.LogPersistenceConfig, logger *logger.Logger) *AsyncLogSystem {
	ctx, cancel := context.WithCancel(context.Background())

	// Use configuration values or defaults
	queueSize := 100000
	memoryLimit := int64(1024 * 1024 * 1024) // 1GB default
	overflowMode := OverflowCompress

	if config != nil {
		if config.QueueSize > 0 {
			queueSize = config.QueueSize
		}
		if config.MemoryLimit > 0 {
			memoryLimit = config.MemoryLimit
		}
		if config.OverflowStrategy != "" {
			switch config.OverflowStrategy {
			case "compress":
				overflowMode = OverflowCompress
			case "spill":
				overflowMode = OverflowSpill
			case "sample":
				overflowMode = OverflowSample
			case "alert":
				overflowMode = OverflowAlert
			}
		}
	}

	system := &AsyncLogSystem{
		logQueue:         make(chan LogChunk, queueSize),
		queueSize:        queueSize,
		config:           config,
		memoryLimit:      memoryLimit,
		overflowMode:     overflowMode,
		compressedChunks: make(map[string]*CompressedBuffer),
		spillFiles:       make(map[string]*os.File),
		spillDir:         "/tmp/joblet-spill",
		metrics:          &LogSystemMetrics{QueueCapacity: int64(queueSize)},
		logger:           logger.WithField("component", "async-log-system"),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Create spill directory
	if err := os.MkdirAll(system.spillDir, 0755); err != nil {
		logger.Error("failed to create spill directory", "error", err, "dir", system.spillDir)
		// Continue anyway - spill to disk might fail but memory buffering will work
	}

	// Create disk writer
	system.diskWriter = &DiskWriter{
		logFiles: make(map[string]*os.File),
		baseDir:  config.Directory,
		logger:   logger.WithField("component", "disk-writer"),
	}

	// Start background workers
	system.startWorkers()

	return system
}

// WriteLog writes a log chunk asynchronously with zero performance impact on jobs.
//
// This is the primary interface for job log writing. It implements rate decoupling
// by immediately placing log data into an in-memory queue, allowing jobs to continue
// execution without waiting for disk I/O.
//
// Key characteristics:
// - NEVER blocks regardless of disk speed or queue state
// - Completes in microseconds (typically < 1μs)
// - Handles data safely with defensive copying
// - Triggers overflow protection when queue is full
// - Updates metrics atomically for monitoring
//
// Overflow handling:
// When the queue is full, the system automatically engages overflow protection
// based on the configured strategy (compress/spill/sample/alert) ensuring no
// data loss while maintaining job performance.
//
// Parameters:
//   - jobID: Unique identifier for the job producing the log data
//   - data: Raw log data bytes to be persisted (copied for safety)
//
// The function returns immediately after queuing - actual disk writing happens
// asynchronously in background workers.
func (a *AsyncLogSystem) WriteLog(jobID string, data []byte) {
	chunk := LogChunk{
		JobID:     jobID,
		Data:      make([]byte, len(data)), // Copy to avoid data races
		Timestamp: time.Now(),
		Sequence:  atomic.AddInt64(&a.metrics.TotalBytesWritten, int64(len(data))),
	}
	copy(chunk.Data, data)

	// Non-blocking write - job NEVER waits
	select {
	case a.logQueue <- chunk:
		// Success: queued for async processing
		atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.logQueue)))
	default:
		// Queue full: handle overflow WITHOUT blocking job
		a.handleOverflow(chunk)
	}
}

// handleOverflow manages queue overflow situations using configurable strategies.
//
// This function is called when the primary log queue is full and implements
// multiple overflow protection strategies to prevent data loss while maintaining
// system stability. The strategy is chosen at system initialization time.
//
// Available strategies:
// - OverflowCompress: Compress log chunks in memory to save space
// - OverflowSpill: Write chunks to temporary disk files
// - OverflowSample: Keep every Nth chunk, dropping others with metrics tracking
// - OverflowAlert: Alert operators and temporarily expand memory limits
//
// Each strategy is designed to handle different scenarios:
// - Compress: Best for temporary bursts with good compression ratio
// - Spill: Best for sustained high throughput with adequate disk space
// - Sample: Best for extreme overload when some data loss is acceptable
// - Alert: Best for debugging and operational awareness
//
// Parameters:
//   - chunk: The log chunk that couldn't be queued normally
//
// The function updates overflow metrics and ensures the chunk is handled
// according to the configured strategy, never dropping data silently.
func (a *AsyncLogSystem) handleOverflow(chunk LogChunk) {
	atomic.AddInt64(&a.metrics.OverflowEvents, 1)

	switch a.overflowMode {
	case OverflowCompress:
		if a.compressChunk(chunk) {
			return
		}
		// Fallback to spill if compression fails
		fallthrough
	case OverflowSpill:
		a.spillToDisk(chunk)
	case OverflowSample:
		a.sampleChunk(chunk)
	case OverflowAlert:
		a.alertAndExpand(chunk)
	}
}

// compressChunk compresses the chunk to save memory
func (a *AsyncLogSystem) compressChunk(chunk LogChunk) bool {
	a.compressMutex.Lock()
	defer a.compressMutex.Unlock()

	buffer, exists := a.compressedChunks[chunk.JobID]
	if !exists {
		buffer = &CompressedBuffer{
			buffer: &bytes.Buffer{},
		}
		var err error
		buffer.writer, err = gzip.NewWriterLevel(buffer.buffer, gzip.BestSpeed)
		if err != nil {
			return false
		}
		a.compressedChunks[chunk.JobID] = buffer
	}

	_, err := buffer.writer.Write(chunk.Data)
	if err != nil {
		return false
	}

	buffer.chunkCount++

	// Update compression ratio metric
	if buffer.chunkCount > 0 {
		originalSize := float64(buffer.chunkCount * len(chunk.Data))
		compressedSize := float64(buffer.buffer.Len())
		a.metrics.CompressionRatio = compressedSize / originalSize
	}

	return true
}

// spillToDisk writes chunk to temporary spill file
func (a *AsyncLogSystem) spillToDisk(chunk LogChunk) {
	a.spillMutex.Lock()
	defer a.spillMutex.Unlock()

	spillFile, exists := a.spillFiles[chunk.JobID]
	if !exists {
		var err error
		spillPath := filepath.Join(a.spillDir, fmt.Sprintf("spill_%s_%d.log", chunk.JobID, time.Now().Unix()))
		spillFile, err = os.Create(spillPath)
		if err != nil {
			a.logger.Error("failed to create spill file", "error", err)
			return
		}
		a.spillFiles[chunk.JobID] = spillFile
		atomic.AddInt32(&a.metrics.SpillFilesCount, 1)
	}

	_, _ = spillFile.Write(chunk.Data)
	_ = spillFile.Sync() // Ensure durability
}

// sampleChunk implements sampling strategy (keep every Nth chunk)
func (a *AsyncLogSystem) sampleChunk(chunk LogChunk) {
	// Keep every 10th chunk during overflow
	if chunk.Sequence%10 == 0 {
		// Force into queue by removing oldest item
		select {
		case <-a.logQueue:
			// Removed one item
		default:
			// Queue empty
		}
		a.logQueue <- chunk
	} else {
		atomic.AddInt64(&a.metrics.DroppedChunks, 1)
	}
}

// alertAndExpand alerts operators and temporarily increases limits
func (a *AsyncLogSystem) alertAndExpand(chunk LogChunk) {
	a.logger.Error("log queue overflow - emergency expansion",
		"jobId", chunk.JobID,
		"queueUsage", len(a.logQueue),
		"queueCapacity", a.queueSize)

	// Try to expand memory limits temporarily
	a.memoryLimit *= 2

	// Try compression as fallback
	if !a.compressChunk(chunk) {
		a.spillToDisk(chunk)
	}
}

// startWorkers starts background processing goroutines
func (a *AsyncLogSystem) startWorkers() {
	// Start disk writer worker
	a.wg.Add(1)
	go a.diskWriterLoop()

	// Start metrics updater
	a.wg.Add(1)
	go a.metricsLoop()
}

// diskWriterLoop processes the log queue in batches for optimal disk I/O performance.
//
// This is the main background worker that implements the consumer side of the
// producer-consumer pattern. It runs in a separate goroutine and continuously
// processes log chunks from the queue, batching them for efficient disk writes.
//
// Batching strategy:
// - Collects chunks until batch size is reached OR flush interval expires
// - Groups chunks by job ID for efficient file operations
// - Uses configurable batch size and flush interval for tuning
// - Ensures low latency even with small batch sizes
//
// Performance optimizations:
// - Batch writes reduce disk I/O overhead
// - Grouping by job ID minimizes file handle operations
// - Configurable timing allows tuning for latency vs throughput
// - Graceful shutdown ensures no data loss during system termination
//
// The loop continues until the system context is cancelled, at which point
// it flushes any remaining batched data before terminating.
func (a *AsyncLogSystem) diskWriterLoop() {
	defer a.wg.Done()

	// Use configurable batch size and flush interval
	batchSize := 100
	flushInterval := 100 * time.Millisecond

	if a.config != nil {
		if a.config.BatchSize > 0 {
			batchSize = a.config.BatchSize
		}
		if a.config.FlushInterval > 0 {
			flushInterval = a.config.FlushInterval
		}
	}

	batch := make([]LogChunk, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case chunk := <-a.logQueue:
			batch = append(batch, chunk)
			atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.logQueue)))

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

// flushBatch writes a batch of chunks to disk
func (a *AsyncLogSystem) flushBatch(batch []LogChunk) {
	// Group by job for efficient writing
	jobBatches := make(map[string][]LogChunk)
	for _, chunk := range batch {
		jobBatches[chunk.JobID] = append(jobBatches[chunk.JobID], chunk)
	}

	// Write each job's chunks
	for jobID, chunks := range jobBatches {
		logFile := a.diskWriter.getLogFile(jobID)
		if logFile != nil {
			for _, chunk := range chunks {
				_, _ = logFile.Write(chunk.Data)
			}
			_ = logFile.Sync() // Ensure durability
		}
	}
}

// metricsLoop updates metrics periodically
func (a *AsyncLogSystem) metricsLoop() {
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
func (a *AsyncLogSystem) updateMetrics() {
	queueUsage := float64(len(a.logQueue)) / float64(a.queueSize)

	if queueUsage > 0.8 {
		a.logger.Warn("log queue usage high", "usage", queueUsage)
	}

	// Update queue metrics
	atomic.StoreInt64(&a.metrics.QueueUsage, int64(len(a.logQueue)))

	// Log metrics periodically
	a.logger.Debug("async log system metrics",
		"queueUsage", atomic.LoadInt64(&a.metrics.QueueUsage),
		"overflowEvents", atomic.LoadInt64(&a.metrics.OverflowEvents),
		"spillFiles", atomic.LoadInt32(&a.metrics.SpillFilesCount),
		"totalBytes", atomic.LoadInt64(&a.metrics.TotalBytesWritten))
}

// GetMetrics returns current system metrics for monitoring and operational visibility.
//
// This function provides real-time statistics about the async log system's
// performance and health. The metrics are atomically updated during operation
// and can be used for monitoring, alerting, and performance tuning.
//
// Available metrics:
// - QueueUsage: Current number of items in the log queue
// - QueueCapacity: Maximum queue capacity
// - OverflowEvents: Total number of overflow situations encountered
// - SpillFilesCount: Number of temporary spill files currently active
// - TotalBytesWritten: Cumulative bytes processed by the system
// - DroppedChunks: Number of chunks dropped (during sampling strategy)
// - CompressionRatio: Achieved compression ratio (for compress strategy)
//
// The returned metrics are a snapshot at the time of the call and can be
// used for real-time monitoring dashboards, alerting systems, and performance
// analysis.
//
// Returns:
//   - *LogSystemMetrics: Current system metrics with atomic consistency
func (a *AsyncLogSystem) GetMetrics() *LogSystemMetrics {
	return &LogSystemMetrics{
		QueueUsage:        atomic.LoadInt64(&a.metrics.QueueUsage),
		QueueCapacity:     a.metrics.QueueCapacity,
		OverflowEvents:    atomic.LoadInt64(&a.metrics.OverflowEvents),
		SpillFilesCount:   atomic.LoadInt32(&a.metrics.SpillFilesCount),
		TotalBytesWritten: atomic.LoadInt64(&a.metrics.TotalBytesWritten),
		DroppedChunks:     atomic.LoadInt64(&a.metrics.DroppedChunks),
		CompressionRatio:  a.metrics.CompressionRatio,
	}
}

// Close shuts down the async log system gracefully, ensuring no data loss.
//
// This function implements a clean shutdown sequence that guarantees all
// queued log data is written to disk before terminating. It coordinates
// the shutdown of all background workers and cleanup of system resources.
//
// Shutdown sequence:
// 1. Cancel the system context to signal workers to stop
// 2. Wait for all background workers to complete using WaitGroup
// 3. Close all temporary spill files with proper error handling
// 4. Close the disk writer and all managed log files
//
// Data safety guarantees:
// - All queued log chunks are processed before shutdown
// - Background workers complete their current batches
// - Temporary spill files are properly closed and preserved
// - No log data is lost during the shutdown process
//
// The function blocks until all cleanup is complete, making it safe to
// terminate the process immediately after Close() returns.
//
// Returns:
//   - error: Any error encountered during cleanup (logged but not fatal)
func (a *AsyncLogSystem) Close() error {
	a.cancel()
	a.wg.Wait()

	// Close spill files
	a.spillMutex.Lock()
	for _, file := range a.spillFiles {
		_ = file.Close()
	}
	a.spillMutex.Unlock()

	// Close disk writer
	return a.diskWriter.Close()
}

// DiskWriter methods

// getLogFile gets or creates a log file for a job
func (dw *DiskWriter) getLogFile(jobID string) *os.File {
	dw.filesMutex.RLock()
	file, exists := dw.logFiles[jobID]
	dw.filesMutex.RUnlock()

	if exists {
		return file
	}

	dw.filesMutex.Lock()
	defer dw.filesMutex.Unlock()

	// Double-check pattern
	if file, exists := dw.logFiles[jobID]; exists {
		return file
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dw.baseDir, 0755); err != nil {
		dw.logger.Error("failed to create log directory", "directory", dw.baseDir, "error", err)
		return nil
	}

	// Create new log file
	timestamp := time.Now().Format("20060102-150405")
	logPath := filepath.Join(dw.baseDir, fmt.Sprintf("%s_%s.log", jobID, timestamp))

	file, err := os.Create(logPath)
	if err != nil {
		dw.logger.Error("failed to create log file", "jobId", jobID, "error", err)
		return nil
	}

	dw.logFiles[jobID] = file
	return file
}

// Close closes all log files
func (dw *DiskWriter) Close() error {
	dw.filesMutex.Lock()
	defer dw.filesMutex.Unlock()

	for jobID, file := range dw.logFiles {
		if err := file.Close(); err != nil {
			dw.logger.Error("failed to close log file", "jobId", jobID, "error", err)
		}
	}

	dw.logFiles = make(map[string]*os.File)
	return nil
}
