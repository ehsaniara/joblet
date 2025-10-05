package metrics

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"joblet/internal/joblet/metrics/domain"
	"joblet/pkg/logger"
)

// MetricsDiskReader reads historical metrics from disk files
type MetricsDiskReader struct {
	baseDir string
	logger  *logger.Logger
}

// NewMetricsDiskReader creates a new metrics disk reader
func NewMetricsDiskReader(baseDir string, logger *logger.Logger) *MetricsDiskReader {
	return &MetricsDiskReader{
		baseDir: baseDir,
		logger:  logger,
	}
}

// ReadJobMetrics reads all historical metrics for a job within a time range
// If from/to are zero, reads all metrics for the job
// Supports short UUID prefixes - will search for matching job directories
func (r *MetricsDiskReader) ReadJobMetrics(
	jobID string,
	from time.Time,
	to time.Time,
) ([]*domain.JobMetricsSample, error) {
	// Try to resolve short UUID if needed
	resolvedJobID, err := r.resolveJobID(jobID)
	if err != nil {
		return nil, err
	}

	jobDir := filepath.Join(r.baseDir, resolvedJobID)

	// Check if directory exists
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no metrics found for job %s", resolvedJobID)
	}

	// Find all metrics files for this job
	files, err := r.findMetricsFiles(jobDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find metrics files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no metrics files found for job %s", jobID)
	}

	// Read samples from all files
	var allSamples []*domain.JobMetricsSample
	for _, file := range files {
		samples, err := r.readMetricsFile(file)
		if err != nil {
			r.logger.Warn("failed to read metrics file", "file", file, "error", err)
			continue
		}
		allSamples = append(allSamples, samples...)
	}

	// Filter by time range if specified
	if !from.IsZero() || !to.IsZero() {
		allSamples = r.filterByTimeRange(allSamples, from, to)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(allSamples, func(i, j int) bool {
		return allSamples[i].Timestamp.Before(allSamples[j].Timestamp)
	})

	return allSamples, nil
}

// findMetricsFiles finds all metrics files in a job directory
func (r *MetricsDiskReader) findMetricsFiles(jobDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(jobDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Look for .jsonl or .jsonl.gz files
		if filepath.Ext(path) == ".jsonl" || filepath.Ext(path) == ".gz" {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files by name (which includes timestamp)
	sort.Strings(files)

	return files, nil
}

// readMetricsFile reads all samples from a single metrics file
func (r *MetricsDiskReader) readMetricsFile(filePath string) ([]*domain.JobMetricsSample, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if file is gzipped
	if filepath.Ext(filePath) == ".gz" {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		// Set multistream mode to handle incomplete gzip files
		gzReader.Multistream(false)
		reader = gzReader
	}

	// Read JSON Lines format
	var samples []*domain.JobMetricsSample
	scanner := bufio.NewScanner(reader)

	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		var sample domain.JobMetricsSample
		if err := json.Unmarshal(line, &sample); err != nil {
			r.logger.Warn("failed to unmarshal sample", "file", filePath, "line", lineNum, "error", err)
			continue
		}

		samples = append(samples, &sample)
	}

	if err := scanner.Err(); err != nil {
		// If we got some samples but hit an EOF error, that's okay for incomplete gzip files
		if len(samples) > 0 && (err == io.EOF || err == io.ErrUnexpectedEOF) {
			r.logger.Debug("read metrics file (incomplete)", "file", filePath, "samples", len(samples))
			return samples, nil
		}
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	r.logger.Debug("read metrics file", "file", filePath, "samples", len(samples))
	return samples, nil
}

// resolveJobID resolves a job ID (full UUID or short prefix) to a full UUID
// by searching the metrics directory for matching job folders
func (r *MetricsDiskReader) resolveJobID(jobID string) (string, error) {
	// If it looks like a full UUID (36 chars with dashes), use it as-is
	if len(jobID) == 36 {
		return jobID, nil
	}

	// Search for directories matching the prefix
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read metrics directory: %w", err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), jobID) {
			matches = append(matches, entry.Name())
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no metrics found for job %s", jobID)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous job ID '%s' matches multiple jobs: %v", jobID, matches)
	}

	return matches[0], nil
}

// filterByTimeRange filters samples to only those within the specified time range
func (r *MetricsDiskReader) filterByTimeRange(
	samples []*domain.JobMetricsSample,
	from time.Time,
	to time.Time,
) []*domain.JobMetricsSample {
	var filtered []*domain.JobMetricsSample

	for _, sample := range samples {
		// Skip if before start time
		if !from.IsZero() && sample.Timestamp.Before(from) {
			continue
		}

		// Skip if after end time
		if !to.IsZero() && sample.Timestamp.After(to) {
			continue
		}

		filtered = append(filtered, sample)
	}

	return filtered
}
