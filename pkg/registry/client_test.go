package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/pkg/runtime"
)

// mockRegistry creates a mock registry for testing
func mockRegistry() *Registry {
	return &Registry{
		Version:   "1",
		UpdatedAt: time.Now(),
		Runtimes: map[string]map[string]*RuntimeEntry{
			"python-3.11-ml": {
				"1.0.0": {
					Version:     "1.0.0",
					DownloadURL: "https://example.com/python-3.11-ml-1.0.0.tar.gz",
					Checksum:    "sha256:abc123",
					Size:        1024,
					Platforms:   []string{"ubuntu-amd64", "ubuntu-arm64"},
					Description: "Python ML runtime v1.0.0",
				},
				"1.0.1": {
					Version:     "1.0.1",
					DownloadURL: "https://example.com/python-3.11-ml-1.0.1.tar.gz",
					Checksum:    "sha256:def456",
					Size:        2048,
					Platforms:   []string{"ubuntu-amd64", "ubuntu-arm64"},
					Description: "Python ML runtime v1.0.1",
				},
			},
			"openjdk-21": {
				"1.0.0": {
					Version:     "1.0.0",
					DownloadURL: "https://example.com/openjdk-21-1.0.0.tar.gz",
					Checksum:    "sha256:ghi789",
					Size:        4096,
					Platforms:   []string{"ubuntu-amd64"},
					Description: "OpenJDK 21 runtime",
				},
			},
		},
	}
}

func TestConvertToRawURL(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		filePath string
		want     string
	}{
		{
			name:     "github url",
			repoURL:  "https://github.com/ehsaniara/joblet-runtimes",
			filePath: "registry.json",
			want:     "https://raw.githubusercontent.com/ehsaniara/joblet-runtimes/main/registry.json",
		},
		{
			name:     "github url with trailing slash",
			repoURL:  "https://github.com/ehsaniara/joblet-runtimes/",
			filePath: "registry.json",
			want:     "https://raw.githubusercontent.com/ehsaniara/joblet-runtimes/main/registry.json",
		},
		{
			name:     "non-github url",
			repoURL:  "https://example.com/registry",
			filePath: "registry.json",
			want:     "https://example.com/registry/registry.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToRawURL(tt.repoURL, tt.filePath)
			if got != tt.want {
				t.Errorf("convertToRawURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClient_FetchRegistry(t *testing.T) {
	// Create mock registry
	mockReg := mockRegistry()

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept header to be application/json")
		}

		// Return mock registry
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	// Create client
	client := NewClient()

	// Fetch registry
	ctx := context.Background()
	registry, err := client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("FetchRegistry() error = %v", err)
	}

	// Verify registry
	if registry.Version != "1" {
		t.Errorf("Expected version 1, got %s", registry.Version)
	}

	if len(registry.Runtimes) != 2 {
		t.Errorf("Expected 2 runtimes, got %d", len(registry.Runtimes))
	}

	if !registry.HasRuntime("python-3.11-ml") {
		t.Errorf("Expected registry to have python-3.11-ml")
	}
}

func TestClient_FetchRegistry_HTTPError(t *testing.T) {
	// Create test HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	// Create client
	client := NewClient()

	// Fetch registry (should fail)
	ctx := context.Background()
	_, err := client.FetchRegistry(ctx, server.URL)
	if err == nil {
		t.Fatal("Expected error for 404 response, got nil")
	}
}

func TestClient_FetchRegistry_Caching(t *testing.T) {
	// Track number of HTTP requests
	requestCount := 0

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		mockReg := mockRegistry()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	// Create client with long TTL
	client := NewClientWithTTL(1 * time.Hour)
	ctx := context.Background()

	// First fetch - should hit server
	_, err := client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("First FetchRegistry() error = %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request after first fetch, got %d", requestCount)
	}

	// Second fetch - should use cache
	_, err = client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("Second FetchRegistry() error = %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected still 1 request after cached fetch, got %d", requestCount)
	}

	// Clear cache and fetch again - should hit server
	client.ClearCache()
	_, err = client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("Third FetchRegistry() error = %v", err)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests after cache clear, got %d", requestCount)
	}
}

func TestClient_FetchRegistry_CacheExpiration(t *testing.T) {
	// Track number of HTTP requests
	requestCount := 0

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		mockReg := mockRegistry()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	// Create client with very short TTL
	client := NewClientWithTTL(100 * time.Millisecond)
	ctx := context.Background()

	// First fetch
	_, err := client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("First FetchRegistry() error = %v", err)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Fetch again - should hit server due to expiration
	_, err = client.FetchRegistry(ctx, server.URL)
	if err != nil {
		t.Fatalf("Second FetchRegistry() error = %v", err)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests after expiration, got %d", requestCount)
	}
}

func TestClient_ResolveVersion(t *testing.T) {
	// Create mock registry
	mockReg := mockRegistry()

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	tests := []struct {
		name            string
		inputSpec       string
		wantVersion     string
		wantDownloadURL string
		wantErr         bool
	}{
		{
			name:            "resolve latest to 1.0.1",
			inputSpec:       "python-3.11-ml@latest",
			wantVersion:     "1.0.1", // Should resolve to highest version
			wantDownloadURL: "https://example.com/python-3.11-ml-1.0.1.tar.gz",
			wantErr:         false,
		},
		{
			name:            "resolve implicit latest",
			inputSpec:       "python-3.11-ml",
			wantVersion:     "1.0.1",
			wantDownloadURL: "https://example.com/python-3.11-ml-1.0.1.tar.gz",
			wantErr:         false,
		},
		{
			name:            "specific version 1.0.0",
			inputSpec:       "python-3.11-ml@1.0.0",
			wantVersion:     "1.0.0",
			wantDownloadURL: "https://example.com/python-3.11-ml-1.0.0.tar.gz",
			wantErr:         false,
		},
		{
			name:      "runtime not found",
			inputSpec: "nonexistent@1.0.0",
			wantErr:   true,
		},
		{
			name:      "version not found",
			inputSpec: "python-3.11-ml@999.0.0",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input spec
			spec, err := runtime.ParseRuntimeSpec(tt.inputSpec)
			if err != nil {
				t.Fatalf("ParseRuntimeSpec() error = %v", err)
			}

			// Resolve version
			resolvedSpec, entry, err := client.ResolveVersion(ctx, spec, server.URL)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveVersion() error = %v", err)
			}

			if resolvedSpec.Version != tt.wantVersion {
				t.Errorf("Expected version %s, got %s", tt.wantVersion, resolvedSpec.Version)
			}

			if entry.DownloadURL != tt.wantDownloadURL {
				t.Errorf("Expected download URL %s, got %s", tt.wantDownloadURL, entry.DownloadURL)
			}
		})
	}
}

func TestClient_ListRuntimes(t *testing.T) {
	// Create mock registry
	mockReg := mockRegistry()

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	runtimes, err := client.ListRuntimes(ctx, server.URL)
	if err != nil {
		t.Fatalf("ListRuntimes() error = %v", err)
	}

	if len(runtimes) != 2 {
		t.Errorf("Expected 2 runtimes, got %d", len(runtimes))
	}

	// Check that both runtimes are present (order doesn't matter)
	hasML := false
	hasJDK := false
	for _, rt := range runtimes {
		if rt == "python-3.11-ml" {
			hasML = true
		}
		if rt == "openjdk-21" {
			hasJDK = true
		}
	}

	if !hasML {
		t.Error("Expected python-3.11-ml in runtime list")
	}
	if !hasJDK {
		t.Error("Expected openjdk-21 in runtime list")
	}
}

func TestClient_ListVersions(t *testing.T) {
	// Create mock registry
	mockReg := mockRegistry()

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockReg)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// List versions for python-3.11-ml
	versions, err := client.ListVersions(ctx, "python-3.11-ml", server.URL)
	if err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}

	// List versions for non-existent runtime
	_, err = client.ListVersions(ctx, "nonexistent", server.URL)
	if err == nil {
		t.Fatal("Expected error for non-existent runtime, got nil")
	}
}
