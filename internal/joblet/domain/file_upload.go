package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FileUpload represents a file to be uploaded to the job workspace with streaming support
type FileUpload struct {
	Path        string // Relative path in job workspace
	Content     []byte // File content (for small files < 1MB)
	Mode        uint32 // Unix file permissions
	IsDirectory bool   // True if this represents a directory
	Size        int64  // Total file size (for large files)
	IsLarge     bool   // True if file should be streamed (> 1MB)
}

// UploadSession represents a file upload session with streaming capabilities
type UploadSession struct {
	JobID       string
	TotalFiles  int
	TotalSize   int64
	PipePath    string       // Named pipe for streaming large files
	SmallFiles  []FileUpload // Files < 1MB (embedded)
	LargeFiles  []FileUpload // Files >= 1MB (streamed)
	MemoryLimit int64        // Cgroup memory limit for chunking
	ChunkSize   int          // Optimal chunk size based on memory
}

// ValidateUpload ensures upload is safe and within limits
func (us *UploadSession) ValidateUpload() error {
	// Security validations
	for _, file := range append(us.SmallFiles, us.LargeFiles...) {
		if err := validateFilePath(file.Path); err != nil {
			return fmt.Errorf("invalid file path %s: %w", file.Path, err)
		}
	}

	// Memory safety validations
	if us.MemoryLimit > 0 {
		// Ensure chunk size doesn't exceed 10% of memory limit
		maxChunkSize := int(us.MemoryLimit / 10)
		if us.ChunkSize > maxChunkSize {
			us.ChunkSize = maxChunkSize
		}

		// Minimum chunk size for performance
		if us.ChunkSize < 4096 {
			us.ChunkSize = 4096
		}
	}

	return nil
}

// OptimizeForMemory calculates optimal chunking strategy based on cgroup limits
func (us *UploadSession) OptimizeForMemory(memoryLimitMB int32) {
	us.MemoryLimit = int64(memoryLimitMB) * 1024 * 1024

	// Use 5% of memory limit for chunk size, but cap at 1MB
	us.ChunkSize = int(us.MemoryLimit / 20)
	if us.ChunkSize > 1024*1024 {
		us.ChunkSize = 1024 * 1024
	}
	if us.ChunkSize < 4096 {
		us.ChunkSize = 4096
	}
}

// GetWorkspacePath returns the absolute workspace path for the job
func (us *UploadSession) GetWorkspacePath() string {
	return fmt.Sprintf("/opt/joblet/jobs/%s/work", us.JobID)
}

// validateFilePath ensures file paths are safe
func validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	// Normalize path
	cleaned := filepath.Clean(path)

	// Security checks
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal not allowed")
	}

	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("absolute paths not allowed")
	}

	// Check for dangerous paths
	dangerous := []string{
		"proc", "sys", "dev", "etc", "bin", "sbin", "usr/bin", "usr/sbin",
	}

	for _, danger := range dangerous {
		if strings.HasPrefix(cleaned, danger+"/") || cleaned == danger {
			return fmt.Errorf("access to system directory not allowed")
		}
	}

	return nil
}
