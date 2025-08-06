package buffer

import (
	"context"
	"io"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Buffer provides streaming buffer capabilities with real-time notifications.
// Supports writing data, reading complete contents, and subscribing to new data.
//
//counterfeiter:generate . Buffer
type Buffer interface {
	// Write appends data to the buffer.
	// Returns the number of bytes written and any error.
	Write(data []byte) (int, error)

	// Read returns the complete buffer contents.
	// Returns a copy to prevent external mutations.
	Read() ([]byte, error)

	// ReadFrom reads data from the provided reader into the buffer.
	ReadFrom(r io.Reader) (int64, error)

	// Subscribe creates a subscription to new data written to the buffer.
	// Returns a channel for new data chunks, an unsubscribe function, and any error.
	Subscribe(ctx context.Context) (<-chan []byte, func(), error)

	// Len returns the current buffer size in bytes.
	Len() int

	// Cap returns the buffer capacity in bytes (0 = unlimited).
	Cap() int

	// Clear removes all data from the buffer.
	Clear() error

	// Close gracefully shuts down the buffer and releases resources.
	Close() error

	// IsClosed returns true if the buffer has been closed.
	IsClosed() bool
}

// RingBuffer provides a fixed-size circular buffer that overwrites old data.
// Useful for log streaming where you want to keep recent data only.
//
//counterfeiter:generate . RingBuffer
type RingBuffer interface {
	Buffer

	// SetCapacity changes the buffer capacity.
	// May truncate existing data if new capacity is smaller.
	SetCapacity(capacity int) error

	// GetCapacity returns the current buffer capacity.
	GetCapacity() int

	// IsWrapped returns true if the buffer has wrapped around.
	IsWrapped() bool

	// GetOldestData returns the oldest data in the buffer.
	GetOldestData() ([]byte, error)

	// GetNewestData returns the most recent data in the buffer.
	GetNewestData() ([]byte, error)
}

// PersistentBuffer provides a buffer that can persist data to storage.
// Useful for job logs that need to survive process restarts.
//
//counterfeiter:generate . PersistentBuffer
type PersistentBuffer interface {
	Buffer

	// Sync forces any buffered writes to be flushed to storage.
	Sync() error

	// GetPath returns the file path where data is stored.
	GetPath() string

	// GetSize returns the current size of the persistent storage.
	GetSize() (int64, error)

	// Truncate reduces the buffer to the specified size.
	Truncate(size int64) error

	// Rotate creates a new storage file and closes the current one.
	// Useful for log rotation.
	Rotate() error
}

// BufferManager manages multiple buffers with lifecycle operations.
// Provides factory methods for creating different buffer types.
//
//counterfeiter:generate . BufferManager
type BufferManager interface {
	// CreateBuffer creates a new in-memory buffer with the given ID.
	CreateBuffer(ctx context.Context, id string, config BufferConfig) (Buffer, error)

	// CreateRingBuffer creates a new ring buffer with the given ID and capacity.
	CreateRingBuffer(ctx context.Context, id string, capacity int, config BufferConfig) (RingBuffer, error)

	// CreatePersistentBuffer creates a new persistent buffer with the given ID and path.
	CreatePersistentBuffer(ctx context.Context, id string, path string, config BufferConfig) (PersistentBuffer, error)

	// GetBuffer retrieves an existing buffer by ID.
	GetBuffer(id string) (Buffer, bool)

	// ListBuffers returns all active buffer IDs.
	ListBuffers() []string

	// RemoveBuffer removes a buffer and cleans up its resources.
	RemoveBuffer(id string) error

	// Close shuts down all buffers and releases resources.
	Close() error

	// Stats returns statistics about buffer usage.
	Stats() *BufferManagerStats
}

// BufferConfig contains configuration options for buffers.
type BufferConfig struct {
	// Type specifies which buffer implementation to use.
	Type string `yaml:"type" json:"type"` // memory, ring, persistent

	// InitialCapacity for the buffer in bytes.
	InitialCapacity int `yaml:"initial_capacity" json:"initial_capacity"`

	// MaxCapacity limits buffer growth (0 = unlimited).
	MaxCapacity int `yaml:"max_capacity" json:"max_capacity"`

	// FlushInterval for periodic flushing to storage.
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`

	// EnableCompression compresses buffer contents.
	EnableCompression bool `yaml:"enable_compression" json:"enable_compression"`

	// MaxSubscribers limits the number of subscribers per buffer.
	MaxSubscribers int `yaml:"max_subscribers" json:"max_subscribers"`

	// SubscriberBufferSize for subscription channels.
	SubscriberBufferSize int `yaml:"subscriber_buffer_size" json:"subscriber_buffer_size"`

	// EnableMetrics tracks buffer usage statistics.
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics"`

	// Persistent buffer specific options
	Persistent *PersistentBufferConfig `yaml:"persistent,omitempty" json:"persistent,omitempty"`

	// Ring buffer specific options
	Ring *RingBufferConfig `yaml:"ring,omitempty" json:"ring,omitempty"`
}

// PersistentBufferConfig contains options specific to persistent buffers.
type PersistentBufferConfig struct {
	// Directory where buffer files are stored.
	Directory string `yaml:"directory" json:"directory"`

	// FileMode for created files.
	FileMode string `yaml:"file_mode" json:"file_mode"`

	// SyncInterval for forcing writes to disk.
	SyncInterval time.Duration `yaml:"sync_interval" json:"sync_interval"`

	// EnableRotation enables automatic file rotation.
	EnableRotation bool `yaml:"enable_rotation" json:"enable_rotation"`

	// RotationSize triggers rotation when file reaches this size.
	RotationSize int64 `yaml:"rotation_size" json:"rotation_size"`

	// MaxFiles limits the number of rotated files to keep.
	MaxFiles int `yaml:"max_files" json:"max_files"`
}

// RingBufferConfig contains options specific to ring buffers.
type RingBufferConfig struct {
	// OverwritePolicy determines behavior when buffer is full.
	OverwritePolicy string `yaml:"overwrite_policy" json:"overwrite_policy"` // oldest, newest, error

	// WrapNotification enables notifications when buffer wraps.
	WrapNotification bool `yaml:"wrap_notification" json:"wrap_notification"`
}

// BufferStats provides statistics about a single buffer.
type BufferStats struct {
	// ID of the buffer.
	ID string

	// Type of the buffer (memory, ring, persistent).
	Type string

	// Size is the current buffer size in bytes.
	Size int

	// Capacity is the buffer capacity in bytes.
	Capacity int

	// WriteCount is the total number of write operations.
	WriteCount int64

	// BytesWritten is the total bytes written to the buffer.
	BytesWritten int64

	// ReadCount is the total number of read operations.
	ReadCount int64

	// BytesRead is the total bytes read from the buffer.
	BytesRead int64

	// SubscriberCount is the current number of active subscribers.
	SubscriberCount int

	// CreatedAt is when the buffer was created.
	CreatedAt time.Time

	// LastWriteTime is when data was last written.
	LastWriteTime *time.Time

	// LastReadTime is when data was last read.
	LastReadTime *time.Time

	// IsClosed indicates if the buffer is closed.
	IsClosed bool
}

// BufferManagerStats provides statistics about the buffer manager.
type BufferManagerStats struct {
	// ActiveBuffers is the current number of active buffers.
	ActiveBuffers int

	// TotalBuffersCreated is the cumulative number of buffers created.
	TotalBuffersCreated int64

	// TotalBytesWritten across all buffers.
	TotalBytesWritten int64

	// TotalBytesRead across all buffers.
	TotalBytesRead int64

	// MemoryUsage is the estimated memory usage in bytes.
	MemoryUsage int64

	// BufferStats contains per-buffer statistics.
	BufferStats map[string]*BufferStats
}
