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
	"sync"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/pkg/logger"
)

// LocalBackend implements storage using local filesystem
type LocalBackend struct {
	config  *config.StorageConfig
	logger  *logger.Logger
	baseDir string

	// File handles cache
	logFiles    map[string]*logFile
	metricFiles map[string]*metricFile
	filesMu     sync.RWMutex
}

type logFile struct {
	jobID    string
	stdout   *os.File
	stderr   *os.File
	gzStdout *gzip.Writer
	gzStderr *gzip.Writer
}

type metricFile struct {
	jobID    string
	file     *os.File
	gzWriter *gzip.Writer
}

// NewLocalBackend creates a new local storage backend
func NewLocalBackend(cfg *config.StorageConfig, log *logger.Logger) (*LocalBackend, error) {
	backend := &LocalBackend{
		config:      cfg,
		logger:      log.WithField("backend", "local"),
		baseDir:     cfg.BaseDir,
		logFiles:    make(map[string]*logFile),
		metricFiles: make(map[string]*metricFile),
	}

	// Create base directories
	if err := os.MkdirAll(cfg.Local.Logs.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	if err := os.MkdirAll(cfg.Local.Metrics.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	log.Info("Local storage backend initialized",
		"logsDir", cfg.Local.Logs.Directory,
		"metricsDir", cfg.Local.Metrics.Directory)

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

	// Flush both writers
	if err := lf.gzStdout.Flush(); err != nil {
		return err
	}
	if err := lf.gzStderr.Flush(); err != nil {
		return err
	}
	if err := lf.stdout.Sync(); err != nil {
		return err
	}
	if err := lf.stderr.Sync(); err != nil {
		return err
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

	// Flush and sync
	if err := mf.gzWriter.Flush(); err != nil {
		return err
	}
	if err := mf.file.Sync(); err != nil {
		return err
	}

	return nil
}

// getOrCreateLogFile gets or creates log file handles for a job
func (lb *LocalBackend) getOrCreateLogFile(jobID string) (*logFile, error) {
	if lf, exists := lb.logFiles[jobID]; exists {
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
		jobID:    jobID,
		stdout:   stdout,
		stderr:   stderr,
		gzStdout: gzip.NewWriter(stdout),
		gzStderr: gzip.NewWriter(stderr),
	}

	lb.logFiles[jobID] = lf
	lb.logger.Debug("Created log files", "jobID", jobID)

	return lf, nil
}

// getOrCreateMetricFile gets or creates metric file handle for a job
func (lb *LocalBackend) getOrCreateMetricFile(jobID string) (*metricFile, error) {
	if mf, exists := lb.metricFiles[jobID]; exists {
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
		jobID:    jobID,
		file:     file,
		gzWriter: gzip.NewWriter(file),
	}

	lb.metricFiles[jobID] = mf
	lb.logger.Debug("Created metric file", "jobID", jobID)

	return mf, nil
}

// ReadLogs returns a log reader for streaming logs
func (lb *LocalBackend) ReadLogs(ctx context.Context, query *LogQuery) (*LogReader, error) {
	lb.logger.Debug("ReadLogs called", "jobID", query.JobID, "stream", query.Stream, "limit", query.Limit, "offset", query.Offset)

	// Build log directory path
	logDir := filepath.Join(lb.config.Local.Logs.Directory, query.JobID)

	// Check if directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		lb.logger.Debug("No log directory found", "jobID", query.JobID, "path", logDir)
		return nil, fmt.Errorf("no logs found for job %s", query.JobID)
	}

	// Create reader
	reader := &LogReader{
		Channel: make(chan *ipcpb.LogLine, 100),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	// Start reading in background
	go func() {
		defer close(reader.Channel)
		defer close(reader.Error)
		defer close(reader.Done)

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
				reader.Error <- fmt.Errorf("failed to open log file %s: %w", fileInfo.path, err)
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
				reader.Error <- fmt.Errorf("failed to create gzip reader for %s: %w", fileInfo.path, err)
				return
			}

			scanner := bufio.NewScanner(gzReader)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB initial, 1MB max

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					gzReader.Close()
					file.Close()
					lb.logger.Debug("ReadLogs cancelled", "jobID", query.JobID)
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
					lb.logger.Debug("ReadLogs cancelled while sending", "jobID", query.JobID)
					return
				}
			}

			if err := scanner.Err(); err != nil {
				gzReader.Close()
				file.Close()
				reader.Error <- fmt.Errorf("error reading log file %s: %w", fileInfo.path, err)
				return
			}

			gzReader.Close()
			file.Close()
		}

		lb.logger.Debug("Finished reading logs", "jobID", query.JobID, "count", count, "skipped", skipped)
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
func (lb *LocalBackend) ReadMetrics(ctx context.Context, query *MetricQuery) (*MetricReader, error) {
	lb.logger.Debug("ReadMetrics called", "jobID", query.JobID, "limit", query.Limit, "offset", query.Offset)

	// Build metrics file path
	metricsPath := filepath.Join(lb.config.Local.Metrics.Directory, query.JobID, "metrics.jsonl.gz")

	// Check if file exists
	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		lb.logger.Debug("No metrics file found", "jobID", query.JobID, "path", metricsPath)
		return nil, fmt.Errorf("no metrics found for job %s", query.JobID)
	}

	// Create reader
	reader := &MetricReader{
		Channel: make(chan *ipcpb.Metric, 100),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	// Start reading in background
	go func() {
		defer close(reader.Channel)
		defer close(reader.Error)
		defer close(reader.Done)

		file, err := os.Open(metricsPath)
		if err != nil {
			reader.Error <- fmt.Errorf("failed to open metrics file: %w", err)
			return
		}
		defer file.Close()

		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
				// Empty or incomplete gzip file - this can happen if:
				// 1. The job just started and no metrics written yet
				// 2. The gzip writer hasn't been closed yet (job still running)
				// Just return empty result, no error
				lb.logger.Debug("Empty or incomplete gzip metrics file", "path", metricsPath, "jobID", query.JobID)
				return
			}
			reader.Error <- fmt.Errorf("failed to create gzip reader: %w", err)
			return
		}
		defer gzReader.Close()

		scanner := bufio.NewScanner(gzReader)
		// Increase buffer size for large metric lines
		scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB initial, 1MB max

		count := 0
		skipped := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				lb.logger.Debug("ReadMetrics cancelled", "jobID", query.JobID)
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
				break
			}

			select {
			case reader.Channel <- &metric:
				count++
			case <-ctx.Done():
				lb.logger.Debug("ReadMetrics cancelled while sending", "jobID", query.JobID)
				return
			}
		}

		if err := scanner.Err(); err != nil {
			// If we hit unexpected EOF in the middle of reading, it means the gzip
			// stream is incomplete (e.g., job still running and writer not closed)
			// Return what we read so far instead of erroring
			if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
				lb.logger.Debug("Incomplete gzip stream, returning partial metrics", "jobID", query.JobID, "count", count)
				return
			}
			reader.Error <- fmt.Errorf("error reading metrics: %w", err)
			return
		}

		lb.logger.Debug("Finished reading metrics", "jobID", query.JobID, "count", count, "skipped", skipped)
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

	// Delete directories
	logDir := filepath.Join(lb.config.Local.Logs.Directory, jobID)
	if err := os.RemoveAll(logDir); err != nil {
		return fmt.Errorf("failed to delete log directory: %w", err)
	}

	metricsDir := filepath.Join(lb.config.Local.Metrics.Directory, jobID)
	if err := os.RemoveAll(metricsDir); err != nil {
		return fmt.Errorf("failed to delete metrics directory: %w", err)
	}

	lb.logger.Info("Deleted job data", "jobID", jobID)

	return nil
}

// Close closes the backend and all open files
func (lb *LocalBackend) Close() error {
	lb.filesMu.Lock()
	defer lb.filesMu.Unlock()

	// Close all log files
	for jobID, lf := range lb.logFiles {
		lf.gzStdout.Close()
		lf.gzStderr.Close()
		lf.stdout.Close()
		lf.stderr.Close()
		lb.logger.Debug("Closed log files", "jobID", jobID)
	}

	// Close all metric files
	for jobID, mf := range lb.metricFiles {
		mf.gzWriter.Close()
		mf.file.Close()
		lb.logger.Debug("Closed metric file", "jobID", jobID)
	}

	lb.logger.Info("Local storage backend closed")

	return nil
}
