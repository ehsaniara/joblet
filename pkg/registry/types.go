package registry

import (
	"time"
)

// Registry represents the structure of registry.json from a runtime registry
type Registry struct {
	// Version is the registry format version (currently "1")
	Version string `json:"version"`

	// UpdatedAt is when the registry was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Runtimes is a map of runtime names to their available versions
	// Example: "python-3.11-ml" -> {"1.0.0": {...}, "1.0.1": {...}}
	Runtimes map[string]map[string]*RuntimeEntry `json:"runtimes"`
}

// RuntimeEntry represents a specific version of a runtime in the registry
type RuntimeEntry struct {
	// Version is the semantic version (e.g., "1.0.0")
	Version string `json:"version"`

	// DownloadURL is the direct download URL for the runtime package
	DownloadURL string `json:"download_url"`

	// Checksum is the SHA256 checksum for verification
	// Format: "sha256:abc123..."
	Checksum string `json:"checksum"`

	// Size is the package size in bytes
	Size int64 `json:"size"`

	// Platforms is the list of supported platforms
	// Example: ["ubuntu-amd64", "ubuntu-arm64", "rhel-amd64"]
	Platforms []string `json:"platforms"`

	// Description is optional runtime description
	Description string `json:"description,omitempty"`
}

// RegistryConfig represents configuration for a single registry source
type RegistryConfig struct {
	// Name is a friendly name for this registry (e.g., "official", "company")
	Name string `yaml:"name"`

	// URL is the base URL for the registry
	// For GitHub: "https://github.com/ehsaniara/joblet-runtimes"
	URL string `yaml:"url"`

	// Enabled determines if this registry should be checked
	Enabled bool `yaml:"enabled"`

	// Priority determines the order in which registries are checked (higher = first)
	Priority int `yaml:"priority,omitempty"`
}

// CachedRegistry represents a registry with cache metadata
type CachedRegistry struct {
	// Registry is the actual registry data
	Registry *Registry

	// FetchedAt is when this registry was last fetched
	FetchedAt time.Time

	// SourceURL is where this registry came from
	SourceURL string
}

// GetLatestVersion returns the latest version for a given runtime
// Returns empty string if runtime not found
func (r *Registry) GetLatestVersion(runtimeName string) string {
	versions, exists := r.Runtimes[runtimeName]
	if !exists || len(versions) == 0 {
		return ""
	}

	// Find the latest version by comparing semantic versions
	// For now, we'll use a simple approach: latest by UpdatedAt or highest version string
	var latestVersion string
	for version := range versions {
		if latestVersion == "" || version > latestVersion {
			latestVersion = version
		}
	}

	return latestVersion
}

// GetRuntimeEntry returns a specific runtime entry
// Returns nil if runtime or version not found
func (r *Registry) GetRuntimeEntry(runtimeName, version string) *RuntimeEntry {
	versions, exists := r.Runtimes[runtimeName]
	if !exists {
		return nil
	}

	entry, exists := versions[version]
	if !exists {
		return nil
	}

	return entry
}

// ListVersions returns all available versions for a runtime
// Returns empty slice if runtime not found
func (r *Registry) ListVersions(runtimeName string) []string {
	versions, exists := r.Runtimes[runtimeName]
	if !exists {
		return []string{}
	}

	result := make([]string, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}

	return result
}

// HasRuntime returns true if the registry contains the given runtime
func (r *Registry) HasRuntime(runtimeName string) bool {
	_, exists := r.Runtimes[runtimeName]
	return exists
}

// SupportsPlatform checks if a runtime version supports a given platform
func (re *RuntimeEntry) SupportsPlatform(platform string) bool {
	for _, p := range re.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}
