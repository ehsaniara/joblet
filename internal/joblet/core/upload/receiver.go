package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Receiver handles receiving streamed uploads inside the isolated job process
type Receiver struct {
	platform     platform.Platform
	logger       *logger.Logger
	workspaceDir string
}

// NewReceiver creates a new upload receiver
func NewReceiver(platform platform.Platform, logger *logger.Logger) *Receiver {
	return &Receiver{
		platform:     platform,
		logger:       logger.WithField("component", "upload-receiver"),
		workspaceDir: "/work", // Inside chroot, this is the workspace
	}
}

// ReceiveLargeFiles receives large files from the named pipe
func (r *Receiver) ReceiveLargeFiles(ctx context.Context, pipePath string) error {
	log := r.logger.WithField("operation", "receive-large-files")
	log.Debug("waiting for large file upload stream", "pipePath", pipePath)

	// Open pipe for reading (this will block until writer opens it)
	// Add timeout to prevent indefinite blocking
	ctx, cancel := context.WithTimeout(ctx, UploadTimeout)
	defer cancel()

	// Open pipe in a goroutine to allow cancellation
	pipeChan := make(chan *os.File, 1)
	errChan := make(chan error, 1)

	go func() {
		pipe, err := os.OpenFile(pipePath, os.O_RDONLY, 0)
		if err != nil {
			errChan <- err
			return
		}
		pipeChan <- pipe
	}()

	var pipe *os.File
	select {
	case pipe = <-pipeChan:
		defer pipe.Close()
	case err := <-errChan:
		return fmt.Errorf("failed to open upload pipe: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for upload stream: %w", ctx.Err())
	}

	log.Debug("upload stream opened, receiving files")

	// Receive files from the stream
	return r.receiveFilesFromStream(ctx, pipe)
}

// receiveFilesFromStream processes the incoming file stream
func (r *Receiver) receiveFilesFromStream(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	// Increase buffer size for large headers
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "FILE:") {
			continue
		}

		// Parse file header: "FILE:path:size:mode:isDirectory"
		parts := strings.Split(line, ":")
		if len(parts) != 5 {
			return fmt.Errorf("invalid file header format: %s", line)
		}

		filePath := parts[1]
		sizeStr := parts[2]
		modeStr := parts[3]
		isDirStr := parts[4]

		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid file size: %s", sizeStr)
		}

		mode, err := strconv.ParseUint(modeStr, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid file mode: %s", modeStr)
		}

		isDirectory := isDirStr == "true"

		r.logger.Debug("receiving file", "path", filePath, "size", size, "isDirectory", isDirectory)

		if err := r.receiveFile(ctx, reader, filePath, size, uint32(mode), isDirectory); err != nil {
			return fmt.Errorf("failed to receive file %s: %w", filePath, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading upload stream: %w", err)
	}

	r.logger.Debug("large file reception completed")
	return nil
}

// receiveFile receives a single file from the stream
func (r *Receiver) receiveFile(ctx context.Context, reader io.Reader, filePath string, size int64, mode uint32, isDirectory bool) error {
	fullPath := filepath.Join(r.workspaceDir, filePath)

	if isDirectory {
		// Create directory
		dirMode := os.FileMode(mode)
		if dirMode == 0 {
			dirMode = 0755
		}
		if err := r.platform.MkdirAll(fullPath, dirMode); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		r.logger.Debug("created directory", "path", filePath)
		return nil
	}

	// Create parent directories
	parentDir := filepath.Dir(fullPath)
	if err := r.platform.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create and write file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy file content with progress monitoring
	written, err := r.copyWithMonitoring(ctx, file, reader, size)
	if err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	if written != size {
		return fmt.Errorf("file size mismatch: expected %d, got %d", size, written)
	}

	// Set file permissions
	if err := file.Chmod(os.FileMode(mode)); err != nil {
		r.logger.Warn("failed to set file permissions", "path", filePath, "mode", mode, "error", err)
	}

	r.logger.Debug("received file", "path", filePath, "size", written)
	return nil
}

// copyWithMonitoring copies data with context cancellation and progress monitoring
func (r *Receiver) copyWithMonitoring(ctx context.Context, writer io.Writer, reader io.Reader, expectedSize int64) (int64, error) {
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for written < expectedSize {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		// Calculate how much to read
		remaining := expectedSize - written
		toRead := int64(len(buf))
		if remaining < toRead {
			toRead = remaining
		}

		// Read chunk
		n, err := reader.Read(buf[:toRead])
		if err != nil {
			if err == io.EOF && written == expectedSize {
				break
			}
			return written, err
		}

		// Write chunk
		wn, err := writer.Write(buf[:n])
		if err != nil {
			return written, err
		}

		written += int64(wn)

		// Progress monitoring (every 1MB)
		if written%(1024*1024) == 0 {
			progress := float64(written) / float64(expectedSize) * 100
			r.logger.Debug("file reception progress", "written", written, "total", expectedSize, "progress", fmt.Sprintf("%.1f%%", progress))
		}
	}

	return written, nil
}

// ProcessSmallFilesFromEnv processes small files from environment variables (fallback)
func (r *Receiver) ProcessSmallFilesFromEnv() error {
	// This is the fallback method using environment variables
	// for compatibility with the existing system

	uploadsData := os.Getenv("JOB_UPLOADS")
	if uploadsData == "" {
		r.logger.Debug("no small files to process from environment")
		return nil
	}

	r.logger.Debug("processing small files from environment", "dataSize", len(uploadsData))

	// Use the existing loadUploadsFromEnv logic
	// This will be called from the jobexec package
	return nil
}
