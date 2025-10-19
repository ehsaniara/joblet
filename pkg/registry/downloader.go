package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadProgress represents download progress information
type DownloadProgress struct {
	// BytesDownloaded is the number of bytes downloaded so far
	BytesDownloaded int64

	// TotalBytes is the total size to download (0 if unknown)
	TotalBytes int64

	// Percentage is the download percentage (0-100, -1 if unknown)
	Percentage int
}

// ProgressCallback is called periodically during download
type ProgressCallback func(progress DownloadProgress)

// Downloader handles downloading and verifying runtime packages
type Downloader struct {
	httpClient *http.Client
}

// NewDownloader creates a new downloader
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 0, // No timeout for downloads (may be large files)
		},
	}
}

// DownloadAndVerify downloads a file from a URL and verifies its checksum
//
// Parameters:
//   - ctx: Context for cancellation
//   - url: Download URL
//   - expectedChecksum: Expected checksum in format "sha256:abc123..."
//   - destPath: Destination file path
//   - progressCallback: Optional callback for progress updates (can be nil)
//
// Returns:
//   - error if download fails or checksum doesn't match
func (d *Downloader) DownloadAndVerify(
	ctx context.Context,
	url string,
	expectedChecksum string,
	destPath string,
	progressCallback ProgressCallback,
) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create temporary file for download
	tempPath := destPath + ".tmp"
	defer os.Remove(tempPath) // Clean up temp file on error

	// Download to temporary file
	if err := d.download(ctx, url, tempPath, progressCallback); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum
	if err := verifyChecksum(tempPath, expectedChecksum); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// Move to final destination
	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}

	return nil
}

// download downloads a file from a URL to a destination path
func (d *Downloader) download(
	ctx context.Context,
	url string,
	destPath string,
	progressCallback ProgressCallback,
) error {
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "joblet-runtime-downloader/1.0")

	// Execute request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Get content length for progress tracking
	contentLength := resp.ContentLength

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Create progress reader if callback provided
	var reader io.Reader = resp.Body
	if progressCallback != nil {
		reader = &progressReader{
			reader:           resp.Body,
			totalBytes:       contentLength,
			progressCallback: progressCallback,
		}
	}

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// verifyChecksum verifies a file's SHA256 checksum
func verifyChecksum(filePath, expectedChecksum string) error {
	// Parse expected checksum
	// Format: "sha256:abc123..." or just "abc123..."
	expectedHash := strings.TrimPrefix(expectedChecksum, "sha256:")
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))

	// Calculate actual checksum
	actualHash, err := calculateSHA256(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Compare checksums
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// calculateSHA256 calculates the SHA256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// progressReader wraps an io.Reader and reports progress via callback
type progressReader struct {
	reader           io.Reader
	bytesRead        int64
	totalBytes       int64
	progressCallback ProgressCallback
	lastReported     int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.bytesRead += int64(n)

	// Report progress every 1MB or at completion
	if pr.progressCallback != nil {
		if pr.bytesRead-pr.lastReported >= 1024*1024 || err == io.EOF {
			progress := DownloadProgress{
				BytesDownloaded: pr.bytesRead,
				TotalBytes:      pr.totalBytes,
				Percentage:      -1,
			}

			if pr.totalBytes > 0 {
				progress.Percentage = int((pr.bytesRead * 100) / pr.totalBytes)
			}

			pr.progressCallback(progress)
			pr.lastReported = pr.bytesRead
		}
	}

	return n, err
}

// CalculateChecksum is a utility function to calculate SHA256 checksum
// This is exported for use in tests and utilities
func CalculateChecksum(filePath string) (string, error) {
	hash, err := calculateSHA256(filePath)
	if err != nil {
		return "", err
	}
	return "sha256:" + hash, nil
}
