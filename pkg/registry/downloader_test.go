package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloader_DownloadAndVerify_Success(t *testing.T) {
	// Create test data
	testData := []byte("This is test runtime package data for testing download and verification.")

	// Calculate checksum
	hasher := sha256.New()
	hasher.Write(testData)
	expectedChecksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "downloader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test-runtime.tar.gz")

	// Download and verify
	downloader := NewDownloader()
	ctx := context.Background()

	progressCalled := false
	progressCallback := func(progress DownloadProgress) {
		progressCalled = true
		t.Logf("Progress: %d bytes downloaded", progress.BytesDownloaded)
	}

	err = downloader.DownloadAndVerify(ctx, server.URL, expectedChecksum, destPath, progressCallback)
	if err != nil {
		t.Fatalf("DownloadAndVerify() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Downloaded file does not exist")
	}

	// Verify file content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("Downloaded content mismatch: got %q, want %q", string(content), string(testData))
	}

	// Note: Progress callback might not be called for small files
	t.Logf("Progress callback called: %v", progressCalled)
}

func TestDownloader_DownloadAndVerify_ChecksumMismatch(t *testing.T) {
	// Create test data
	testData := []byte("Test data")

	// Wrong checksum
	wrongChecksum := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "downloader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test-runtime.tar.gz")

	// Download and verify (should fail)
	downloader := NewDownloader()
	ctx := context.Background()

	err = downloader.DownloadAndVerify(ctx, server.URL, wrongChecksum, destPath, nil)
	if err == nil {
		t.Fatal("Expected checksum verification to fail, but it succeeded")
	}

	if !contains(err.Error(), "checksum") {
		t.Errorf("Expected error to mention checksum, got: %v", err)
	}

	// Verify temp file was cleaned up
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("Temporary file should have been cleaned up after checksum failure")
	}
}

func TestDownloader_DownloadAndVerify_HTTPError(t *testing.T) {
	// Create test HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "downloader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test-runtime.tar.gz")

	// Download and verify (should fail)
	downloader := NewDownloader()
	ctx := context.Background()

	err = downloader.DownloadAndVerify(ctx, server.URL, "sha256:abc123", destPath, nil)
	if err == nil {
		t.Fatal("Expected download to fail with HTTP error, but it succeeded")
	}

	if !contains(err.Error(), "404") {
		t.Errorf("Expected error to mention 404, got: %v", err)
	}
}

func TestDownloader_DownloadAndVerify_WithProgress(t *testing.T) {
	// Create larger test data to ensure progress callback is triggered
	testData := make([]byte, 5*1024*1024) // 5 MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Calculate checksum
	hasher := sha256.New()
	hasher.Write(testData)
	expectedChecksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Length", "5242880") // 5 MB
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "downloader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	destPath := filepath.Join(tmpDir, "test-runtime.tar.gz")

	// Download and verify with progress tracking
	downloader := NewDownloader()
	ctx := context.Background()

	progressUpdates := 0
	var lastProgress DownloadProgress

	progressCallback := func(progress DownloadProgress) {
		progressUpdates++
		lastProgress = progress
		t.Logf("Progress update #%d: %d/%d bytes (%.1f%%)",
			progressUpdates,
			progress.BytesDownloaded,
			progress.TotalBytes,
			float64(progress.Percentage))
	}

	err = downloader.DownloadAndVerify(ctx, server.URL, expectedChecksum, destPath, progressCallback)
	if err != nil {
		t.Fatalf("DownloadAndVerify() error = %v", err)
	}

	// Should have received progress updates
	if progressUpdates == 0 {
		t.Error("Expected progress callback to be called, but it wasn't")
	}

	// Last progress should show completion
	if lastProgress.BytesDownloaded != int64(len(testData)) {
		t.Errorf("Expected final bytes downloaded to be %d, got %d",
			len(testData), lastProgress.BytesDownloaded)
	}

	if lastProgress.Percentage != 100 {
		t.Errorf("Expected final percentage to be 100, got %d", lastProgress.Percentage)
	}

	t.Logf("Total progress updates: %d", progressUpdates)
}

func TestCalculateSHA256(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "checksum-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Hello, World!")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate checksum using our function
	hash, err := calculateSHA256(testFile)
	if err != nil {
		t.Fatalf("calculateSHA256() error = %v", err)
	}

	// Calculate expected checksum
	hasher := sha256.New()
	hasher.Write(testData)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	if hash != expectedHash {
		t.Errorf("calculateSHA256() = %s, want %s", hash, expectedHash)
	}
}

func TestVerifyChecksum_Success(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "verify-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Test data for checksum verification")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate checksum
	hasher := sha256.New()
	hasher.Write(testData)
	expectedChecksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	// Verify checksum
	err = verifyChecksum(testFile, expectedChecksum)
	if err != nil {
		t.Errorf("verifyChecksum() error = %v, want nil", err)
	}
}

func TestVerifyChecksum_WithoutPrefix(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "verify-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Test data")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate checksum without prefix
	hasher := sha256.New()
	hasher.Write(testData)
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil)) // No "sha256:" prefix

	// Verify checksum (should work without prefix too)
	err = verifyChecksum(testFile, expectedChecksum)
	if err != nil {
		t.Errorf("verifyChecksum() error = %v, want nil", err)
	}
}

func TestVerifyChecksum_Failure(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "verify-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Some data")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wrong checksum
	wrongChecksum := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	// Verify checksum (should fail)
	err = verifyChecksum(testFile, wrongChecksum)
	if err == nil {
		t.Error("verifyChecksum() should have failed with wrong checksum")
	}

	if !contains(err.Error(), "mismatch") {
		t.Errorf("Expected error to mention mismatch, got: %v", err)
	}
}

func TestCalculateChecksum(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "calc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Test data for CalculateChecksum")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Calculate checksum
	checksum, err := CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum() error = %v", err)
	}

	// Should have sha256: prefix
	if !contains(checksum, "sha256:") {
		t.Errorf("CalculateChecksum() should return checksum with sha256: prefix, got %s", checksum)
	}

	// Verify it matches expected
	hasher := sha256.New()
	hasher.Write(testData)
	expectedChecksum := "sha256:" + hex.EncodeToString(hasher.Sum(nil))

	if checksum != expectedChecksum {
		t.Errorf("CalculateChecksum() = %s, want %s", checksum, expectedChecksum)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
