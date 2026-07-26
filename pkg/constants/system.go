package constants

// System constants shared across the codebase, kept here as a single source of truth

const (
	// Memory and storage units
	BinaryUnit = 1024 // Base unit for binary calculations (KB, MB, GB)

	// System page and sector sizes
	DefaultPageSize   = 4096 // Standard memory page size on most systems
	DefaultSectorSize = 512  // Standard disk sector size

	// Network and timing constants
	DefaultPollInterval = 50 // Default polling interval in milliseconds
	DefaultTimeout      = 10 // Default timeout in seconds
	NetworkReadyTimeout = 10 // Timeout for network ready signals in seconds

	// Buffer and chunk sizes
	DefaultChunkSize = 32 * 1024         // 32KB default chunk size for file operations
	MaxUploadSize    = 100 * 1024 * 1024 // 100MB max upload size

	// Network defaults
	DefaultNetworkMTU   = 1500 // Standard Ethernet MTU
	DefaultNetworkSpeed = 1000 // Default network speed in Mbps
)

// File permissions and modes
const (
	DefaultFileMode = 0644 // Standard file permission for created files
	DefaultDirMode  = 0755 // Standard directory permission
)

// Monitoring and metrics
const (
	MetricsCollectionInterval = 1000 // Metrics collection interval in milliseconds
	StatsBufferSize           = 100  // Buffer size for statistics collection
)
