package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/pkg/runtime"
)

const (
	// DefaultCacheTTL is how long we cache registry.json before re-fetching
	DefaultCacheTTL = 1 * time.Hour

	// RegistryJSONPath is the path to registry.json in a GitHub repository
	RegistryJSONPath = "registry.json"

	// DefaultTimeout is the HTTP timeout for registry operations
	DefaultTimeout = 30 * time.Second
)

// Client handles fetching and caching of runtime registries
type Client struct {
	// httpClient is used for HTTP requests
	httpClient *http.Client

	// cacheTTL is how long to cache registry data
	cacheTTL time.Duration

	// cache stores cached registries by URL
	cache map[string]*CachedRegistry

	// cacheMutex protects the cache map
	cacheMutex sync.RWMutex
}

// NewClient creates a new registry client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		cacheTTL: DefaultCacheTTL,
		cache:    make(map[string]*CachedRegistry),
	}
}

// NewClientWithTTL creates a new registry client with custom cache TTL
func NewClientWithTTL(ttl time.Duration) *Client {
	client := NewClient()
	client.cacheTTL = ttl
	return client
}

// FetchRegistry fetches a registry from a GitHub repository URL
// Example URL: "https://github.com/ehsaniara/joblet-runtimes"
//
// This method:
// 1. Checks cache first (with TTL)
// 2. Fetches from GitHub raw content URL
// 3. Parses JSON
// 4. Caches result
func (c *Client) FetchRegistry(ctx context.Context, repoURL string) (*Registry, error) {
	// Convert GitHub repo URL to raw content URL for registry.json
	registryURL := convertToRawURL(repoURL, RegistryJSONPath)

	// Check cache first
	if cached := c.getCached(registryURL); cached != nil {
		return cached, nil
	}

	// Fetch from URL
	registry, err := c.fetchFromURL(ctx, registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry from %s: %w", registryURL, err)
	}

	// Cache the result
	c.setCached(registryURL, registry)

	return registry, nil
}

// ResolveVersion resolves a runtime spec to a specific version
// If the spec has version "latest", it resolves to the actual latest version
// Returns the resolved spec and the registry entry
func (c *Client) ResolveVersion(ctx context.Context, spec *runtime.RuntimeSpec, repoURL string) (*runtime.RuntimeSpec, *RuntimeEntry, error) {
	// Fetch registry
	registry, err := c.FetchRegistry(ctx, repoURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	// Check if runtime exists
	if !registry.HasRuntime(spec.Name) {
		return nil, nil, fmt.Errorf("runtime %q not found in registry", spec.Name)
	}

	// Resolve version
	version := spec.Version
	if spec.IsLatest() {
		latest := registry.GetLatestVersion(spec.Name)
		if latest == "" {
			return nil, nil, fmt.Errorf("no versions found for runtime %q", spec.Name)
		}
		version = latest
	}

	// Get runtime entry
	entry := registry.GetRuntimeEntry(spec.Name, version)
	if entry == nil {
		availableVersions := registry.ListVersions(spec.Name)
		return nil, nil, fmt.Errorf("version %q not found for runtime %q (available: %v)", version, spec.Name, availableVersions)
	}

	// Create resolved spec
	resolvedSpec := &runtime.RuntimeSpec{
		Name:     spec.Name,
		Version:  version,
		Original: spec.Original,
	}

	return resolvedSpec, entry, nil
}

// GetRuntimeEntry gets a specific runtime entry from the registry
// Returns nil if not found
func (c *Client) GetRuntimeEntry(ctx context.Context, spec *runtime.RuntimeSpec, repoURL string) (*RuntimeEntry, error) {
	_, entry, err := c.ResolveVersion(ctx, spec, repoURL)
	return entry, err
}

// ListRuntimes returns all available runtimes in the registry
func (c *Client) ListRuntimes(ctx context.Context, repoURL string) ([]string, error) {
	registry, err := c.FetchRegistry(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	runtimes := make([]string, 0, len(registry.Runtimes))
	for name := range registry.Runtimes {
		runtimes = append(runtimes, name)
	}

	return runtimes, nil
}

// ListVersions returns all available versions for a runtime
func (c *Client) ListVersions(ctx context.Context, runtimeName, repoURL string) ([]string, error) {
	registry, err := c.FetchRegistry(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	versions := registry.ListVersions(runtimeName)
	if len(versions) == 0 {
		return nil, fmt.Errorf("runtime %q not found in registry", runtimeName)
	}

	return versions, nil
}

// ClearCache clears the entire registry cache
func (c *Client) ClearCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.cache = make(map[string]*CachedRegistry)
}

// fetchFromURL fetches and parses registry.json from a URL
func (c *Client) fetchFromURL(ctx context.Context, url string) (*Registry, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "joblet-runtime-client/1.0")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Read and parse JSON
	var registry Registry
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &registry, nil
}

// getCached retrieves a cached registry if it exists and is not expired
func (c *Client) getCached(url string) *Registry {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	cached, exists := c.cache[url]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(cached.FetchedAt) > c.cacheTTL {
		return nil
	}

	return cached.Registry
}

// setCached stores a registry in the cache
func (c *Client) setCached(url string, registry *Registry) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.cache[url] = &CachedRegistry{
		Registry:  registry,
		FetchedAt: time.Now(),
		SourceURL: url,
	}
}

// convertToRawURL converts a GitHub repository URL to a raw content URL
// Example:
//   - Input: "https://github.com/ehsaniara/joblet-runtimes"
//   - Output: "https://raw.githubusercontent.com/ehsaniara/joblet-runtimes/main/registry.json"
func convertToRawURL(repoURL, filePath string) string {
	// Simple conversion for GitHub URLs
	// This assumes the file is in the main branch

	// Remove trailing slash if present
	if len(repoURL) > 0 && repoURL[len(repoURL)-1] == '/' {
		repoURL = repoURL[:len(repoURL)-1]
	}

	// Convert github.com to raw.githubusercontent.com
	// Example: https://github.com/user/repo -> https://raw.githubusercontent.com/user/repo/main/file

	// Extract parts from URL
	// Expected format: https://github.com/owner/repo
	const githubPrefix = "https://github.com/"
	if len(repoURL) > len(githubPrefix) && repoURL[:len(githubPrefix)] == githubPrefix {
		repoPath := repoURL[len(githubPrefix):]
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", repoPath, filePath)
	}

	// If not a GitHub URL, assume it's already a direct URL
	return repoURL + "/" + filePath
}
