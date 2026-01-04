package storage

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// LocalBackend implements storage using local filesystem
type LocalBackend struct {
	config *config.StorageConfig
	logger *logger.Logger

	// File handles cache with LRU eviction
	logFiles           map[string]*logFile
	metricFiles        map[string]*metricFile
	execEventFiles     map[string]*execEventFile
	connectEventFiles  map[string]*connectEventFile
	fileEventFiles     map[string]*eventFile
	acceptEventFiles   map[string]*eventFile
	socketDataFiles    map[string]*eventFile
	mmapEventFiles     map[string]*eventFile
	mprotectEventFiles map[string]*eventFile
	filesMu            sync.RWMutex

	// Cache settings
	maxOpenFiles  int
	fileHandleTTL time.Duration

	// Eviction goroutine control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type logFile struct {
	jobID      string
	stdout     *os.File
	stderr     *os.File
	gzStdout   *gzip.Writer
	gzStderr   *gzip.Writer
	lastAccess time.Time
}

type metricFile struct {
	jobID      string
	file       *os.File
	gzWriter   *gzip.Writer
	lastAccess time.Time
}

type execEventFile struct {
	jobID      string
	file       *os.File
	gzWriter   *gzip.Writer
	lastAccess time.Time
}

type connectEventFile struct {
	jobID      string
	file       *os.File
	gzWriter   *gzip.Writer
	lastAccess time.Time
}

// eventFile is a generic event file handle for file, accept, socket_data, mmap, mprotect events
type eventFile struct {
	jobID      string
	file       *os.File
	gzWriter   *gzip.Writer
	lastAccess time.Time
}

// NewLocalBackend creates a new local storage backend
func NewLocalBackend(cfg *config.StorageConfig, log *logger.Logger) (*LocalBackend, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults for cache settings
	maxOpenFiles := cfg.Local.MaxOpenFiles
	if maxOpenFiles <= 0 {
		maxOpenFiles = 1000
	}

	fileHandleTTL := time.Duration(cfg.Local.FileHandleTTL) * time.Second
	if fileHandleTTL <= 0 {
		fileHandleTTL = 300 * time.Second // 5 minutes default
	}

	backend := &LocalBackend{
		config:             cfg,
		logger:             log.WithField("backend", "local"),
		logFiles:           make(map[string]*logFile),
		metricFiles:        make(map[string]*metricFile),
		execEventFiles:     make(map[string]*execEventFile),
		connectEventFiles:  make(map[string]*connectEventFile),
		fileEventFiles:     make(map[string]*eventFile),
		acceptEventFiles:   make(map[string]*eventFile),
		socketDataFiles:    make(map[string]*eventFile),
		mmapEventFiles:     make(map[string]*eventFile),
		mprotectEventFiles: make(map[string]*eventFile),
		maxOpenFiles:       maxOpenFiles,
		fileHandleTTL:      fileHandleTTL,
		ctx:                ctx,
		cancel:             cancel,
	}

	// Create base directories
	if err := os.MkdirAll(cfg.Local.Logs.Directory, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	if err := os.MkdirAll(cfg.Local.Metrics.Directory, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	if err := os.MkdirAll(cfg.Local.Events.Directory, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	// Start background eviction goroutine
	backend.wg.Add(1)
	go backend.evictionLoop()

	log.Info("Local storage backend initialized",
		"logsDir", cfg.Local.Logs.Directory,
		"metricsDir", cfg.Local.Metrics.Directory,
		"eventsDir", cfg.Local.Events.Directory,
		"maxOpenFiles", maxOpenFiles,
		"fileHandleTTL", fileHandleTTL)

	return backend, nil
}

// WriteLogs writes log lines to disk
func (lb *LocalBackend) WriteLogs(jobID string, logs []*ipcpb.LogLine) error {
	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	lf, err := lb.getOrCreateLogFile(jobID)
	if err != nil {
		return err
	}

	for _, log := range logs {
		// Marshal to JSON
		data, err := json.Marshal(log)
		if err != nil {
			return fmt.Errorf("failed to marshal log: %w", err)
		}

		data = append(data, '\n') // JSONL format

		// Write to appropriate stream
		var writer *gzip.Writer
		if log.Stream == ipcpb.StreamType_STREAM_TYPE_STDOUT {
			writer = lf.gzStdout
		} else {
			writer = lf.gzStderr
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("failed to write log: %w", err)
		}
	}

	// Flush gzip streams safely - always recreate to ensure valid state
	if err := lb.flushLogFile(lf); err != nil {
		return err
	}

	return nil
}

// flushLogFile safely closes and recreates gzip writers, ensuring valid state even on partial failure
func (lb *LocalBackend) flushLogFile(lf *logFile) error {
	var stdoutErr, stderrErr, syncErr error

	// Close gzip writers to write trailer (CRC32 + size)
	// This makes logs immediately readable while allowing future appends
	stdoutErr = lf.gzStdout.Close()
	stderrErr = lf.gzStderr.Close()

	// Always recreate gzip writers to ensure valid state for next write
	// Multiple gzip streams in one file is valid and gzip.NewReader handles it
	lf.gzStdout = gzip.NewWriter(lf.stdout)
	lf.gzStderr = gzip.NewWriter(lf.stderr)

	// Check for close errors
	if stdoutErr != nil {
		return fmt.Errorf("failed to close stdout gzip writer: %w", stdoutErr)
	}
	if stderrErr != nil {
		return fmt.Errorf("failed to close stderr gzip writer: %w", stderrErr)
	}

	// Sync file handles to disk
	if syncErr = lf.stdout.Sync(); syncErr != nil {
		return fmt.Errorf("failed to sync stdout: %w", syncErr)
	}
	if syncErr = lf.stderr.Sync(); syncErr != nil {
		return fmt.Errorf("failed to sync stderr: %w", syncErr)
	}

	return nil
}

// WriteMetrics writes metrics to disk
func (lb *LocalBackend) WriteMetrics(jobID string, metrics []*ipcpb.Metric) error {
	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	mf, err := lb.getOrCreateMetricFile(jobID)
	if err != nil {
		return err
	}

	for _, metric := range metrics {
		// Marshal to JSON
		data, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		data = append(data, '\n') // JSONL format

		if _, err := mf.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write metric: %w", err)
		}
	}

	// Flush gzip stream safely - always recreate to ensure valid state
	if err := lb.flushMetricFile(mf); err != nil {
		return err
	}

	return nil
}

// flushMetricFile safely closes and recreates gzip writer, ensuring valid state even on failure
func (lb *LocalBackend) flushMetricFile(mf *metricFile) error {
	// Close gzip writer to write trailer (CRC32 + size)
	closeErr := mf.gzWriter.Close()

	// Always recreate gzip writer to ensure valid state for next write
	mf.gzWriter = gzip.NewWriter(mf.file)

	// Check for close error
	if closeErr != nil {
		return fmt.Errorf("failed to close metric gzip writer: %w", closeErr)
	}

	// Sync file handle to disk
	if err := mf.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync metrics file: %w", err)
	}

	return nil
}

// WriteExecEvents writes process execution events to disk
func (lb *LocalBackend) WriteExecEvents(jobID string, events []*ipcpb.ExecEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateExecEventFile(jobID)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal exec event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write exec event: %w", err)
		}
	}

	// Flush gzip stream safely
	if err := lb.flushExecEventFile(ef); err != nil {
		return err
	}

	return nil
}

// flushExecEventFile safely closes and recreates gzip writer
func (lb *LocalBackend) flushExecEventFile(ef *execEventFile) error {
	closeErr := ef.gzWriter.Close()
	ef.gzWriter = gzip.NewWriter(ef.file)

	if closeErr != nil {
		return fmt.Errorf("failed to close exec events gzip writer: %w", closeErr)
	}
	if err := ef.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync exec events file: %w", err)
	}
	return nil
}

// WriteConnectEvents writes network connection events to disk
func (lb *LocalBackend) WriteConnectEvents(jobID string, events []*ipcpb.ConnectEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	cf, err := lb.getOrCreateConnectEventFile(jobID)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal connect event: %w", err)
		}
		data = append(data, '\n')
		if _, err := cf.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write connect event: %w", err)
		}
	}

	// Flush gzip stream safely
	if err := lb.flushConnectEventFile(cf); err != nil {
		return err
	}

	return nil
}

// flushConnectEventFile safely closes and recreates gzip writer
func (lb *LocalBackend) flushConnectEventFile(cf *connectEventFile) error {
	closeErr := cf.gzWriter.Close()
	cf.gzWriter = gzip.NewWriter(cf.file)

	if closeErr != nil {
		return fmt.Errorf("failed to close connect events gzip writer: %w", closeErr)
	}
	if err := cf.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync connect events file: %w", err)
	}
	return nil
}

// getOrCreateLogFile gets or creates log file handles for a job
func (lb *LocalBackend) getOrCreateLogFile(jobID string) (*logFile, error) {
	if lf, exists := lb.logFiles[jobID]; exists {
		lf.lastAccess = time.Now()
		return lf, nil
	}

	// Create job log directory
	logDir := filepath.Join(lb.config.Local.Logs.Directory, jobID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open stdout file
	stdoutPath := filepath.Join(logDir, "stdout.log.gz")
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout file: %w", err)
	}

	// Open stderr file
	stderrPath := filepath.Join(logDir, "stderr.log.gz")
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		stdout.Close()
		return nil, fmt.Errorf("failed to open stderr file: %w", err)
	}

	lf := &logFile{
		jobID:      jobID,
		stdout:     stdout,
		stderr:     stderr,
		gzStdout:   gzip.NewWriter(stdout),
		gzStderr:   gzip.NewWriter(stderr),
		lastAccess: time.Now(),
	}

	lb.logFiles[jobID] = lf
	lb.logger.Debug("Created log files", "job_uuid", jobID)

	return lf, nil
}

// getOrCreateMetricFile gets or creates metric file handle for a job
func (lb *LocalBackend) getOrCreateMetricFile(jobID string) (*metricFile, error) {
	if mf, exists := lb.metricFiles[jobID]; exists {
		mf.lastAccess = time.Now()
		return mf, nil
	}

	// Create job metrics directory
	metricsDir := filepath.Join(lb.config.Local.Metrics.Directory, jobID)
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	// Open metrics file
	metricsPath := filepath.Join(metricsDir, "metrics.jsonl.gz")
	file, err := os.OpenFile(metricsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics file: %w", err)
	}

	mf := &metricFile{
		jobID:      jobID,
		file:       file,
		gzWriter:   gzip.NewWriter(file),
		lastAccess: time.Now(),
	}

	lb.metricFiles[jobID] = mf
	lb.logger.Debug("Created metric file", "job_uuid", jobID)

	return mf, nil
}

// getOrCreateExecEventFile gets or creates exec event file handle for a job
func (lb *LocalBackend) getOrCreateExecEventFile(jobID string) (*execEventFile, error) {
	if ef, exists := lb.execEventFiles[jobID]; exists {
		ef.lastAccess = time.Now()
		return ef, nil
	}

	// Create job events directory
	eventsDir := filepath.Join(lb.config.Local.Events.Directory, jobID)
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	// Open exec events file
	eventsPath := filepath.Join(eventsDir, "exec_events.jsonl.gz")
	file, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open exec events file: %w", err)
	}

	ef := &execEventFile{
		jobID:      jobID,
		file:       file,
		gzWriter:   gzip.NewWriter(file),
		lastAccess: time.Now(),
	}

	lb.execEventFiles[jobID] = ef
	lb.logger.Debug("Created exec events file", "job_uuid", jobID)

	return ef, nil
}

// getOrCreateConnectEventFile gets or creates connect event file handle for a job
func (lb *LocalBackend) getOrCreateConnectEventFile(jobID string) (*connectEventFile, error) {
	if cf, exists := lb.connectEventFiles[jobID]; exists {
		cf.lastAccess = time.Now()
		return cf, nil
	}

	// Create job events directory
	eventsDir := filepath.Join(lb.config.Local.Events.Directory, jobID)
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	// Open connect events file
	eventsPath := filepath.Join(eventsDir, "connect_events.jsonl.gz")
	file, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open connect events file: %w", err)
	}

	cf := &connectEventFile{
		jobID:      jobID,
		file:       file,
		gzWriter:   gzip.NewWriter(file),
		lastAccess: time.Now(),
	}

	lb.connectEventFiles[jobID] = cf
	lb.logger.Debug("Created connect events file", "job_uuid", jobID)

	return cf, nil
}

// ReadLogs returns a log reader for streaming logs
func (lb *LocalBackend) ReadLogs(ctx context.Context, query *LogQuery) (*EventReader[*ipcpb.LogLine], error) {
	lb.logger.Debug("ReadLogs called", "job_uuid", query.JobUUID, "stream", query.Stream, "limit", query.Limit, "offset", query.Offset)

	// Build log directory path
	logDir := filepath.Join(lb.config.Local.Logs.Directory, query.JobUUID)

	// Check if directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		lb.logger.Debug("No log directory found", "job_uuid", query.JobUUID, "path", logDir)
		return nil, fmt.Errorf("no logs found for job %s", query.JobUUID)
	}

	// Create reader using generic helper
	reader := NewEventReader[*ipcpb.LogLine](100)

	// Start reading in background
	go func() {
		defer reader.Close()

		// Determine which files to read based on stream filter
		var files []struct {
			path   string
			stream ipcpb.StreamType
		}

		if query.Stream == ipcpb.StreamType_STREAM_TYPE_UNSPECIFIED || query.Stream == ipcpb.StreamType_STREAM_TYPE_STDOUT {
			files = append(files, struct {
				path   string
				stream ipcpb.StreamType
			}{
				path:   filepath.Join(logDir, "stdout.log.gz"),
				stream: ipcpb.StreamType_STREAM_TYPE_STDOUT,
			})
		}

		if query.Stream == ipcpb.StreamType_STREAM_TYPE_UNSPECIFIED || query.Stream == ipcpb.StreamType_STREAM_TYPE_STDERR {
			files = append(files, struct {
				path   string
				stream ipcpb.StreamType
			}{
				path:   filepath.Join(logDir, "stderr.log.gz"),
				stream: ipcpb.StreamType_STREAM_TYPE_STDERR,
			})
		}

		count := 0
		skipped := 0

		// Read each file
		for _, fileInfo := range files {
			if _, err := os.Stat(fileInfo.path); os.IsNotExist(err) {
				lb.logger.Debug("Log file not found", "path", fileInfo.path)
				continue
			}

			file, err := os.Open(fileInfo.path)
			if err != nil {
				reader.SendError(fmt.Errorf("failed to open log file %s: %w", fileInfo.path, err))
				return
			}

			gzReader, err := gzip.NewReader(file)
			if err != nil {
				file.Close()
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					// Empty or corrupted gzip file, skip it
					lb.logger.Warn("Empty or corrupted gzip file", "path", fileInfo.path)
					continue
				}
				reader.SendError(fmt.Errorf("failed to create gzip reader for %s: %w", fileInfo.path, err))
				return
			}

			scanner := bufio.NewScanner(gzReader)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB initial, 1MB max

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					gzReader.Close()
					file.Close()
					lb.logger.Debug("ReadLogs cancelled", "job_uuid", query.JobUUID)
					return
				default:
				}

				line := scanner.Bytes()
				if len(line) == 0 {
					continue
				}

				var logLine ipcpb.LogLine
				if err := json.Unmarshal(line, &logLine); err != nil {
					lb.logger.Warn("Failed to unmarshal log line", "error", err, "line", string(line[:min(len(line), 100)]))
					continue
				}

				// Apply time range filter
				if query.StartTime != nil && logLine.Timestamp < *query.StartTime {
					continue
				}
				if query.EndTime != nil && logLine.Timestamp > *query.EndTime {
					continue
				}

				// Apply text filter if specified
				if query.Filter != "" && !contains(string(logLine.Content), query.Filter) {
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
					file.Close()
					return
				}

				select {
				case reader.Channel <- &logLine:
					count++
				case <-ctx.Done():
					gzReader.Close()
					file.Close()
					lb.logger.Debug("ReadLogs cancelled while sending", "job_uuid", query.JobUUID)
					return
				}
			}

			if err := scanner.Err(); err != nil {
				gzReader.Close()
				file.Close()
				reader.SendError(fmt.Errorf("error reading log file %s: %w", fileInfo.path, err))
				return
			}

			gzReader.Close()
			file.Close()
		}

		lb.logger.Debug("Finished reading logs", "job_uuid", query.JobUUID, "count", count, "skipped", skipped)
	}()

	return reader, nil
}

// contains is a simple case-insensitive substring check helper
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexSubstring(s, substr) >= 0)))
}

// indexSubstring finds the index of substr in s (case-sensitive)
func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ReadMetrics returns a metric reader for streaming metrics
func (lb *LocalBackend) ReadMetrics(ctx context.Context, query *MetricQuery) (*EventReader[*ipcpb.Metric], error) {
	lb.logger.Debug("ReadMetrics called", "job_uuid", query.JobUUID, "limit", query.Limit, "offset", query.Offset)

	// Build metrics file path
	metricsPath := filepath.Join(lb.config.Local.Metrics.Directory, query.JobUUID, "metrics.jsonl.gz")

	// Check if file exists
	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		lb.logger.Debug("No metrics file found", "job_uuid", query.JobUUID, "path", metricsPath)
		return nil, fmt.Errorf("no metrics found for job %s", query.JobUUID)
	}

	// Create reader using generic helper
	reader := NewEventReader[*ipcpb.Metric](100)

	// Start reading in background
	go func() {
		defer reader.Close()

		file, err := os.Open(metricsPath)
		if err != nil {
			reader.SendError(fmt.Errorf("failed to open metrics file: %w", err))
			return
		}
		defer file.Close()

		count := 0
		skipped := 0

		// Handle multi-stream gzip files (each WriteMetrics call creates a new gzip stream)
		for {
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				if err == io.EOF {
					// No more gzip streams - we're done
					break
				}
				if errors.Is(err, io.ErrUnexpectedEOF) {
					// Incomplete gzip stream (job still running)
					lb.logger.Debug("Incomplete gzip metrics stream", "path", metricsPath, "job_uuid", query.JobUUID, "count", count)
					break
				}
				reader.SendError(fmt.Errorf("failed to create gzip reader: %w", err))
				return
			}

			scanner := bufio.NewScanner(gzReader)
			// Increase buffer size for large metric lines
			scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB initial, 1MB max

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					lb.logger.Debug("ReadMetrics cancelled", "job_uuid", query.JobUUID)
					gzReader.Close()
					return
				default:
				}

				line := scanner.Bytes()
				if len(line) == 0 {
					continue
				}

				var metric ipcpb.Metric
				if err := json.Unmarshal(line, &metric); err != nil {
					lb.logger.Warn("Failed to unmarshal metric", "error", err, "line", string(line[:min(len(line), 100)]))
					continue
				}

				// Apply time range filter
				if query.StartTime != nil && metric.Timestamp < *query.StartTime {
					continue
				}
				if query.EndTime != nil && metric.Timestamp > *query.EndTime {
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
					lb.logger.Debug("Finished reading metrics (limit reached)", "job_uuid", query.JobUUID, "count", count, "skipped", skipped)
					return
				}

				select {
				case reader.Channel <- &metric:
					count++
				case <-ctx.Done():
					lb.logger.Debug("ReadMetrics cancelled while sending", "job_uuid", query.JobUUID)
					gzReader.Close()
					return
				}
			}

			if err := scanner.Err(); err != nil {
				// Log scanner errors but continue to next stream
				lb.logger.Warn("Scanner error reading metrics", "error", err, "job_uuid", query.JobUUID)
			}

			gzReader.Close()
		}

		lb.logger.Debug("Finished reading metrics", "job_uuid", query.JobUUID, "count", count, "skipped", skipped)
	}()

	return reader, nil
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DeleteJob deletes all data for a job
func (lb *LocalBackend) DeleteJob(jobID string) error {
	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	// Close open files
	if lf, exists := lb.logFiles[jobID]; exists {
		lf.gzStdout.Close()
		lf.gzStderr.Close()
		lf.stdout.Close()
		lf.stderr.Close()
		delete(lb.logFiles, jobID)
	}

	if mf, exists := lb.metricFiles[jobID]; exists {
		mf.gzWriter.Close()
		mf.file.Close()
		delete(lb.metricFiles, jobID)
	}

	if ef, exists := lb.execEventFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.execEventFiles, jobID)
	}

	if cf, exists := lb.connectEventFiles[jobID]; exists {
		cf.gzWriter.Close()
		cf.file.Close()
		delete(lb.connectEventFiles, jobID)
	}

	// Close new eBPF event files
	if ef, exists := lb.fileEventFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.fileEventFiles, jobID)
	}

	if ef, exists := lb.acceptEventFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.acceptEventFiles, jobID)
	}

	if ef, exists := lb.socketDataFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.socketDataFiles, jobID)
	}

	if ef, exists := lb.mmapEventFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.mmapEventFiles, jobID)
	}

	if ef, exists := lb.mprotectEventFiles[jobID]; exists {
		ef.gzWriter.Close()
		ef.file.Close()
		delete(lb.mprotectEventFiles, jobID)
	}

	// Delete directories
	logDir := filepath.Join(lb.config.Local.Logs.Directory, jobID)
	if err := os.RemoveAll(logDir); err != nil {
		return fmt.Errorf("failed to delete log directory: %w", err)
	}

	metricsDir := filepath.Join(lb.config.Local.Metrics.Directory, jobID)
	if err := os.RemoveAll(metricsDir); err != nil {
		return fmt.Errorf("failed to delete metrics directory: %w", err)
	}

	eventsDir := filepath.Join(lb.config.Local.Events.Directory, jobID)
	if err := os.RemoveAll(eventsDir); err != nil {
		return fmt.Errorf("failed to delete events directory: %w", err)
	}

	lb.logger.Info("Deleted job data", "job_uuid", jobID)

	return nil
}

// evictionLoop runs periodically to evict idle file handles
func (lb *LocalBackend) evictionLoop() {
	defer lb.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-lb.ctx.Done():
			lb.logger.Debug("Eviction loop stopped")
			return
		case <-ticker.C:
			lb.evictIdleHandles()
		}
	}
}

// evictIdleHandles evicts file handles that have exceeded TTL or when over max limit
func (lb *LocalBackend) evictIdleHandles() {
	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	now := time.Now()
	evicted := 0

	// Evict log files past TTL
	for jobID, lf := range lb.logFiles {
		if now.Sub(lf.lastAccess) > lb.fileHandleTTL {
			lb.closeLogFile(lf)
			delete(lb.logFiles, jobID)
			evicted++
		}
	}

	// Evict metric files past TTL
	for jobID, mf := range lb.metricFiles {
		if now.Sub(mf.lastAccess) > lb.fileHandleTTL {
			lb.closeMetricFile(mf)
			delete(lb.metricFiles, jobID)
			evicted++
		}
	}

	// Evict exec event files past TTL
	for jobID, ef := range lb.execEventFiles {
		if now.Sub(ef.lastAccess) > lb.fileHandleTTL {
			lb.closeExecEventFile(ef)
			delete(lb.execEventFiles, jobID)
			evicted++
		}
	}

	// Evict connect event files past TTL
	for jobID, cf := range lb.connectEventFiles {
		if now.Sub(cf.lastAccess) > lb.fileHandleTTL {
			lb.closeConnectEventFile(cf)
			delete(lb.connectEventFiles, jobID)
			evicted++
		}
	}

	// Evict generic event files past TTL
	evicted += lb.evictEventFiles(lb.fileEventFiles, now)
	evicted += lb.evictEventFiles(lb.acceptEventFiles, now)
	evicted += lb.evictEventFiles(lb.socketDataFiles, now)
	evicted += lb.evictEventFiles(lb.mmapEventFiles, now)
	evicted += lb.evictEventFiles(lb.mprotectEventFiles, now)

	// If still over max, evict oldest handles using LRU
	totalHandles := lb.countOpenHandles()
	if totalHandles > lb.maxOpenFiles {
		lb.evictLRU(totalHandles - lb.maxOpenFiles)
	}

	if evicted > 0 {
		lb.logger.Debug("Evicted idle file handles", "count", evicted, "remaining", lb.countOpenHandles())
	}
}

// evictEventFiles evicts generic event files past TTL
func (lb *LocalBackend) evictEventFiles(files map[string]*eventFile, now time.Time) int {
	evicted := 0
	for jobID, ef := range files {
		if now.Sub(ef.lastAccess) > lb.fileHandleTTL {
			lb.closeEventFile(ef)
			delete(files, jobID)
			evicted++
		}
	}
	return evicted
}

// countOpenHandles returns total open file handle count
func (lb *LocalBackend) countOpenHandles() int {
	return len(lb.logFiles) + len(lb.metricFiles) + len(lb.execEventFiles) +
		len(lb.connectEventFiles) + len(lb.fileEventFiles) + len(lb.acceptEventFiles) +
		len(lb.socketDataFiles) + len(lb.mmapEventFiles) + len(lb.mprotectEventFiles)
}

// evictLRU evicts the oldest n file handles (LRU policy)
func (lb *LocalBackend) evictLRU(n int) {
	if n <= 0 {
		return
	}

	// Collect all handles with their last access times
	type handleInfo struct {
		fileType   string
		jobID      string
		lastAccess time.Time
	}

	var handles []handleInfo

	for jobID, lf := range lb.logFiles {
		handles = append(handles, handleInfo{"log", jobID, lf.lastAccess})
	}
	for jobID, mf := range lb.metricFiles {
		handles = append(handles, handleInfo{"metric", jobID, mf.lastAccess})
	}
	for jobID, ef := range lb.execEventFiles {
		handles = append(handles, handleInfo{"exec", jobID, ef.lastAccess})
	}
	for jobID, cf := range lb.connectEventFiles {
		handles = append(handles, handleInfo{"connect", jobID, cf.lastAccess})
	}
	for jobID, ef := range lb.fileEventFiles {
		handles = append(handles, handleInfo{"file", jobID, ef.lastAccess})
	}
	for jobID, ef := range lb.acceptEventFiles {
		handles = append(handles, handleInfo{"accept", jobID, ef.lastAccess})
	}
	for jobID, ef := range lb.socketDataFiles {
		handles = append(handles, handleInfo{"socket_data", jobID, ef.lastAccess})
	}
	for jobID, ef := range lb.mmapEventFiles {
		handles = append(handles, handleInfo{"mmap", jobID, ef.lastAccess})
	}
	for jobID, ef := range lb.mprotectEventFiles {
		handles = append(handles, handleInfo{"mprotect", jobID, ef.lastAccess})
	}

	// Sort by last access (oldest first)
	sort.Slice(handles, func(i, j int) bool {
		return handles[i].lastAccess.Before(handles[j].lastAccess)
	})

	// Evict oldest n handles
	evicted := 0
	for i := 0; i < len(handles) && evicted < n; i++ {
		h := handles[i]
		switch h.fileType {
		case "log":
			if lf, exists := lb.logFiles[h.jobID]; exists {
				lb.closeLogFile(lf)
				delete(lb.logFiles, h.jobID)
				evicted++
			}
		case "metric":
			if mf, exists := lb.metricFiles[h.jobID]; exists {
				lb.closeMetricFile(mf)
				delete(lb.metricFiles, h.jobID)
				evicted++
			}
		case "exec":
			if ef, exists := lb.execEventFiles[h.jobID]; exists {
				lb.closeExecEventFile(ef)
				delete(lb.execEventFiles, h.jobID)
				evicted++
			}
		case "connect":
			if cf, exists := lb.connectEventFiles[h.jobID]; exists {
				lb.closeConnectEventFile(cf)
				delete(lb.connectEventFiles, h.jobID)
				evicted++
			}
		case "file":
			if ef, exists := lb.fileEventFiles[h.jobID]; exists {
				lb.closeEventFile(ef)
				delete(lb.fileEventFiles, h.jobID)
				evicted++
			}
		case "accept":
			if ef, exists := lb.acceptEventFiles[h.jobID]; exists {
				lb.closeEventFile(ef)
				delete(lb.acceptEventFiles, h.jobID)
				evicted++
			}
		case "socket_data":
			if ef, exists := lb.socketDataFiles[h.jobID]; exists {
				lb.closeEventFile(ef)
				delete(lb.socketDataFiles, h.jobID)
				evicted++
			}
		case "mmap":
			if ef, exists := lb.mmapEventFiles[h.jobID]; exists {
				lb.closeEventFile(ef)
				delete(lb.mmapEventFiles, h.jobID)
				evicted++
			}
		case "mprotect":
			if ef, exists := lb.mprotectEventFiles[h.jobID]; exists {
				lb.closeEventFile(ef)
				delete(lb.mprotectEventFiles, h.jobID)
				evicted++
			}
		}
	}

	if evicted > 0 {
		lb.logger.Info("LRU evicted file handles", "count", evicted, "target", n)
	}
}

// Helper functions to close file handles properly
func (lb *LocalBackend) closeLogFile(lf *logFile) {
	lf.gzStdout.Close()
	lf.gzStderr.Close()
	lf.stdout.Close()
	lf.stderr.Close()
}

func (lb *LocalBackend) closeMetricFile(mf *metricFile) {
	mf.gzWriter.Close()
	mf.file.Close()
}

func (lb *LocalBackend) closeExecEventFile(ef *execEventFile) {
	ef.gzWriter.Close()
	ef.file.Close()
}

func (lb *LocalBackend) closeConnectEventFile(cf *connectEventFile) {
	cf.gzWriter.Close()
	cf.file.Close()
}

func (lb *LocalBackend) closeEventFile(ef *eventFile) {
	ef.gzWriter.Close()
	ef.file.Close()
}

// Close closes the backend and all open files
func (lb *LocalBackend) Close() error {
	// Stop eviction goroutine
	lb.cancel()
	lb.wg.Wait()

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	// Close all log files
	for jobID, lf := range lb.logFiles {
		lf.gzStdout.Close()
		lf.gzStderr.Close()
		lf.stdout.Close()
		lf.stderr.Close()
		lb.logger.Debug("Closed log files", "job_uuid", jobID)
	}

	// Close all metric files
	for jobID, mf := range lb.metricFiles {
		mf.gzWriter.Close()
		mf.file.Close()
		lb.logger.Debug("Closed metric file", "job_uuid", jobID)
	}

	// Close all exec event files
	for jobID, ef := range lb.execEventFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed exec events file", "job_uuid", jobID)
	}

	// Close all connect event files
	for jobID, cf := range lb.connectEventFiles {
		cf.gzWriter.Close()
		cf.file.Close()
		lb.logger.Debug("Closed connect events file", "job_uuid", jobID)
	}

	// Close all file event files
	for jobID, ef := range lb.fileEventFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed file events file", "job_uuid", jobID)
	}

	// Close all accept event files
	for jobID, ef := range lb.acceptEventFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed accept events file", "job_uuid", jobID)
	}

	// Close all socket data event files
	for jobID, ef := range lb.socketDataFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed socket data events file", "job_uuid", jobID)
	}

	// Close all mmap event files
	for jobID, ef := range lb.mmapEventFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed mmap events file", "job_uuid", jobID)
	}

	// Close all mprotect event files
	for jobID, ef := range lb.mprotectEventFiles {
		ef.gzWriter.Close()
		ef.file.Close()
		lb.logger.Debug("Closed mprotect events file", "job_uuid", jobID)
	}

	lb.logger.Info("Local storage backend closed")

	return nil
}

// ReadExecEvents reads exec events from local storage
func (lb *LocalBackend) ReadExecEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ExecEvent], error) {
	reader := NewEventReader[*ipcpb.ExecEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readExecEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readExecEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ExecEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "exec_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("exec events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open exec events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				// No more gzip streams - we're done
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				// Incomplete gzip stream
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal exec event", "error", err)
				continue
			}

			// Apply time filters
			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			// Skip offset
			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			// Check limit
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

// ReadConnectEvents reads connect events from local storage
func (lb *LocalBackend) ReadConnectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ConnectEvent], error) {
	reader := NewEventReader[*ipcpb.ConnectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readConnectEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readConnectEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.ConnectEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "connect_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("connect events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open connect events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				// No more gzip streams - we're done
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				// Incomplete gzip stream
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal connect event", "error", err)
				continue
			}

			// Apply time filters
			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			// Skip offset
			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			// Check limit
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

// flushGenericEventFile safely closes and recreates gzip writer for generic event files
func (lb *LocalBackend) flushGenericEventFile(ef *eventFile, eventType string) error {
	closeErr := ef.gzWriter.Close()
	ef.gzWriter = gzip.NewWriter(ef.file)

	if closeErr != nil {
		return fmt.Errorf("failed to close %s events gzip writer: %w", eventType, closeErr)
	}
	if err := ef.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync %s events file: %w", eventType, err)
	}
	return nil
}

// WriteFileEvents writes file events to disk
func (lb *LocalBackend) WriteFileEvents(jobID string, events []*ipcpb.FileEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateEventFile(jobID, "file_events.jsonl.gz", lb.fileEventFiles)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal file event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write file event: %w", err)
		}
	}

	return lb.flushGenericEventFile(ef, "file")
}

// WriteAcceptEvents writes accept events to disk
func (lb *LocalBackend) WriteAcceptEvents(jobID string, events []*ipcpb.AcceptEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateEventFile(jobID, "accept_events.jsonl.gz", lb.acceptEventFiles)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal accept event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write accept event: %w", err)
		}
	}

	return lb.flushGenericEventFile(ef, "accept")
}

// WriteSocketDataEvents writes socket data events to disk
func (lb *LocalBackend) WriteSocketDataEvents(jobID string, events []*ipcpb.SocketDataEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateEventFile(jobID, "socket_data_events.jsonl.gz", lb.socketDataFiles)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal socket data event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write socket data event: %w", err)
		}
	}

	return lb.flushGenericEventFile(ef, "socket_data")
}

// WriteMmapEvents writes mmap events to disk
func (lb *LocalBackend) WriteMmapEvents(jobID string, events []*ipcpb.MmapEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateEventFile(jobID, "mmap_events.jsonl.gz", lb.mmapEventFiles)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal mmap event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write mmap event: %w", err)
		}
	}

	return lb.flushGenericEventFile(ef, "mmap")
}

// WriteMprotectEvents writes mprotect events to disk
func (lb *LocalBackend) WriteMprotectEvents(jobID string, events []*ipcpb.MprotectEvent) error {
	if len(events) == 0 {
		return nil
	}

	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	ef, err := lb.getOrCreateEventFile(jobID, "mprotect_events.jsonl.gz", lb.mprotectEventFiles)
	if err != nil {
		return err
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal mprotect event: %w", err)
		}
		data = append(data, '\n')
		if _, err := ef.gzWriter.Write(data); err != nil {
			return fmt.Errorf("failed to write mprotect event: %w", err)
		}
	}

	return lb.flushGenericEventFile(ef, "mprotect")
}

// getOrCreateEventFile gets or creates a generic event file handle for a job
func (lb *LocalBackend) getOrCreateEventFile(jobID, filename string, fileMap map[string]*eventFile) (*eventFile, error) {
	if ef, exists := fileMap[jobID]; exists {
		ef.lastAccess = time.Now()
		return ef, nil
	}

	// Create job events directory
	eventsDir := filepath.Join(lb.config.Local.Events.Directory, jobID)
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	// Open events file
	eventsPath := filepath.Join(eventsDir, filename)
	file, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open events file %s: %w", filename, err)
	}

	ef := &eventFile{
		jobID:      jobID,
		file:       file,
		gzWriter:   gzip.NewWriter(file),
		lastAccess: time.Now(),
	}

	fileMap[jobID] = ef
	lb.logger.Debug("Created events file", "job_uuid", jobID, "file", filename)

	return ef, nil
}

// ReadFileEvents reads file events from local storage
func (lb *LocalBackend) ReadFileEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.FileEvent], error) {
	reader := NewEventReader[*ipcpb.FileEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readFileEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readFileEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.FileEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "file_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("file events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open file events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				// No more gzip streams - we're done
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				// Incomplete gzip stream
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal file event", "error", err)
				continue
			}

			// Apply time filters
			if query.StartTime != nil && event.Timestamp < *query.StartTime {
				continue
			}
			if query.EndTime != nil && event.Timestamp > *query.EndTime {
				continue
			}

			// Skip offset
			if query.Offset > 0 && skipped < query.Offset {
				skipped++
				continue
			}

			// Check limit
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

// ReadAcceptEvents reads accept events from local storage
func (lb *LocalBackend) ReadAcceptEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.AcceptEvent], error) {
	reader := NewEventReader[*ipcpb.AcceptEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readAcceptEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readAcceptEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.AcceptEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "accept_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("accept events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open accept events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal accept event", "error", err)
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

// ReadSocketDataEvents reads socket data events from local storage
func (lb *LocalBackend) ReadSocketDataEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.SocketDataEvent], error) {
	reader := NewEventReader[*ipcpb.SocketDataEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readSocketDataEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readSocketDataEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.SocketDataEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "socket_data_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("socket data events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open socket data events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal socket data event", "error", err)
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

// ReadMmapEvents reads mmap events from local storage
func (lb *LocalBackend) ReadMmapEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MmapEvent], error) {
	reader := NewEventReader[*ipcpb.MmapEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readMmapEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readMmapEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MmapEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "mmap_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("mmap events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open mmap events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal mmap event", "error", err)
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

// ReadMprotectEvents reads mprotect events from local storage
func (lb *LocalBackend) ReadMprotectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MprotectEvent], error) {
	reader := NewEventReader[*ipcpb.MprotectEvent](100)

	go func() {
		defer reader.Close()
		reader.SendError(lb.readMprotectEventsFromFile(ctx, query, reader.Channel))
	}()

	return reader, nil
}

func (lb *LocalBackend) readMprotectEventsFromFile(ctx context.Context, query *TelemetryQuery, ch chan<- *ipcpb.MprotectEvent) error {
	filePath := filepath.Join(lb.config.Local.Events.Directory, query.JobUUID, "mprotect_events.jsonl.gz")

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lb.logger.Debug("mprotect events file not found", "job_uuid", query.JobUUID)
			return nil
		}
		return fmt.Errorf("failed to open mprotect events file: %w", err)
	}
	defer file.Close()

	count := 0
	skipped := 0

	// Handle multi-stream gzip files (each write call creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("failed to create gzip reader: %w", err)
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
				lb.logger.Warn("Failed to unmarshal mprotect event", "error", err)
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
