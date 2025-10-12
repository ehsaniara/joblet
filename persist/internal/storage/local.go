package storage

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
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

	// Index
	index *jobIndex
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
		index:       newJobIndex(filepath.Join(cfg.BaseDir, "job_index.json")),
	}

	// Create base directories
	if err := os.MkdirAll(cfg.Local.Logs.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	if err := os.MkdirAll(cfg.Local.Metrics.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	// Load index
	if err := backend.index.Load(); err != nil {
		log.Warn("Failed to load job index, starting fresh", "error", err)
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

	// Update index
	lb.index.UpdateJob(jobID, int64(len(logs)), 0)

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

	// Update index
	lb.index.UpdateJob(jobID, 0, int64(len(metrics)))

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
	// TODO: Implement log reading
	return nil, fmt.Errorf("ReadLogs not implemented yet")
}

// ReadMetrics returns a metric reader for streaming metrics
func (lb *LocalBackend) ReadMetrics(ctx context.Context, query *MetricQuery) (*MetricReader, error) {
	// TODO: Implement metrics reading
	return nil, fmt.Errorf("ReadMetrics not implemented yet")
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

	// Remove from index
	lb.index.DeleteJob(jobID)

	lb.logger.Info("Deleted job data", "jobID", jobID)

	return nil
}

// ListJobs lists all jobs matching the filter
func (lb *LocalBackend) ListJobs(filter *JobFilter) ([]string, error) {
	return lb.index.ListJobs(filter)
}

// GetJobInfo returns information about a job
func (lb *LocalBackend) GetJobInfo(jobID string) (*JobInfo, error) {
	return lb.index.GetJobInfo(jobID)
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

	// Save index
	if err := lb.index.Save(); err != nil {
		lb.logger.Error("Failed to save index", "error", err)
	}

	lb.logger.Info("Local storage backend closed")

	return nil
}
