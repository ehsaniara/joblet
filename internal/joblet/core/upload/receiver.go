package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// Receiver handles incoming file uploads through pipes
type Receiver struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewReceiver creates a new upload receiver
func NewReceiver(platform platform.Platform, logger *logger.Logger) *Receiver {
	return &Receiver{
		platform: platform,
		logger:   logger.WithField("component", "upload-receiver"),
	}
}

// ProcessAllFiles processes all files from the upload pipe
func (r *Receiver) ProcessAllFiles(pipePath string, workspacePath string) error {
	log := r.logger.WithField("operation", "process-all-files")

	if pipePath == "" {
		log.Debug("no upload pipe specified, skipping file processing")
		return nil
	}

	log.Debug("opening upload pipe for reading", "pipePath", pipePath)

	pipe, err := os.OpenFile(pipePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open upload pipe: %w", err)
	}
	defer pipe.Close()

	log.Debug("processing files from pipe", "workspacePath", workspacePath)

	// Process files from pipe using existing logic
	return r.processFilesFromPipe(pipe, workspacePath)
}

// processFilesFromPipe reads and processes files from the pipe
func (r *Receiver) processFilesFromPipe(pipe io.Reader, workspacePath string) error {
	log := r.logger.WithField("operation", "process-pipe-files")
	scanner := bufio.NewScanner(pipe)

	fileCount := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Parse file header: FILE:path:size:mode:isDirectory
		if !strings.HasPrefix(line, "FILE:") {
			continue
		}

		parts := strings.SplitN(line[5:], ":", 4) // Remove "FILE:" prefix and split
		if len(parts) != 4 {
			return fmt.Errorf("invalid file header format: %s", line)
		}

		filePath := parts[0]
		sizeStr := parts[1]
		modeStr := parts[2]
		isDirStr := parts[3]

		// Parse file metadata
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid file size: %s", sizeStr)
		}

		mode, err := strconv.ParseUint(modeStr, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid file mode: %s", modeStr)
		}

		isDirectory, err := strconv.ParseBool(isDirStr)
		if err != nil {
			return fmt.Errorf("invalid directory flag: %s", isDirStr)
		}

		fullPath := filepath.Join(workspacePath, filePath)

		log.Debug("processing file from pipe",
			"path", filePath,
			"size", size,
			"mode", fmt.Sprintf("0%o", mode),
			"isDirectory", isDirectory)

		if isDirectory {
			// Create directory
			fileMode := os.FileMode(mode)
			if fileMode == 0 {
				fileMode = 0755
			}
			if err := r.platform.MkdirAll(fullPath, fileMode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
			log.Debug("created directory", "path", filePath)
		} else {
			// Create parent directory
			parentDir := filepath.Dir(fullPath)
			if err := r.platform.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
			}

			// Create file and copy content
			fileMode := os.FileMode(mode)
			if fileMode == 0 {
				fileMode = 0644
			}

			file, er := r.platform.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
			if er != nil {
				return fmt.Errorf("failed to create file %s: %w", filePath, er)
			}

			// Copy file content with monitoring
			ctx := context.Background()
			written, e := r.copyWithMonitoring(ctx, file, pipe, size)
			file.Close()

			if e != nil {
				return fmt.Errorf("failed to write file content for %s: %w", filePath, e)
			}

			if written != size {
				return fmt.Errorf("size mismatch for file %s: expected %d, wrote %d", filePath, size, written)
			}

			log.Debug("wrote file", "path", filePath, "size", written)
		}

		fileCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from pipe: %w", err)
	}

	log.Debug("file processing completed", "filesProcessed", fileCount)
	return nil
}

// copyWithMonitoring copies data with progress monitoring
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
