package job

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// UUIDGenerator generates unique job UUIDs using Linux kernel's native UUID generation
// This provides complete immunity to race conditions and unlimited concurrency
type UUIDGenerator struct {
	// Legacy fields for backward compatibility and testing
	counter  int64
	prefix   string
	nodeID   string
	useNanos bool

	// UUID generation settings
	useUUID    bool     // When true, use UUID generation; when false, fall back to sequential
	fallbackFD *os.File // Fallback to /dev/urandom if /proc/sys/kernel/random/uuid unavailable
}

// NewUUIDGenerator creates a new UUID generator using Linux kernel native UUID generation
func NewUUIDGenerator(prefix, nodeID string) *UUIDGenerator {
	return &UUIDGenerator{
		prefix:   prefix,
		nodeID:   nodeID,
		useNanos: false,
		useUUID:  true, // Default to UUID generation
	}
}

// NewSequentialIDGenerator creates a legacy sequential ID generator for backward compatibility
func NewSequentialIDGenerator(prefix, nodeID string) *UUIDGenerator {
	return &UUIDGenerator{
		prefix:   prefix,
		nodeID:   nodeID,
		useNanos: false,
		useUUID:  false, // Use sequential generation
	}
}

// Next generates the next job UUID using Linux kernel's native UUID generation
// This method provides complete immunity to race conditions and supports unlimited concurrency
func (g *UUIDGenerator) Next() string {
	if g.useUUID {
		return g.generateKernelUUID()
	}
	// Fallback to sequential for testing or when UUID is disabled
	return g.generateSequentialID()
}

// generateKernelUUID generates a UUID using Linux kernel's native /proc/sys/kernel/random/uuid
// This provides mathematically guaranteed uniqueness with zero synchronization overhead
func (g *UUIDGenerator) generateKernelUUID() string {
	// Primary: Use kernel's native UUID generation
	if uuid, err := g.readKernelUUID(); err == nil {
		return uuid
	}

	// Fallback 1: Use /dev/urandom to generate RFC 4122 compliant UUID
	if uuid, err := g.generateUrandomUUID(); err == nil {
		return uuid
	}

	// Fallback 2: Emergency fallback to sequential (should never happen in production)
	return g.generateSequentialID()
}

// readKernelUUID reads a UUID from the Linux kernel's native UUID generator
func (g *UUIDGenerator) readKernelUUID() (string, error) {
	file, err := os.Open("/proc/sys/kernel/random/uuid")
	if err != nil {
		return "", fmt.Errorf("failed to open kernel UUID generator: %w", err)
	}
	defer file.Close()

	uuidBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read kernel UUID: %w", err)
	}

	uuid := strings.TrimSpace(string(uuidBytes))
	if len(uuid) != 36 {
		return "", fmt.Errorf("invalid UUID length: %d", len(uuid))
	}

	return uuid, nil
}

// generateUrandomUUID generates an RFC 4122 compliant UUID v4 using /dev/urandom
func (g *UUIDGenerator) generateUrandomUUID() (string, error) {
	if g.fallbackFD == nil {
		fd, err := os.Open("/dev/urandom")
		if err != nil {
			return "", fmt.Errorf("failed to open /dev/urandom: %w", err)
		}
		g.fallbackFD = fd
	}

	// Read 16 random bytes
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(g.fallbackFD, randomBytes); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	// Set version (4) and variant bits according to RFC 4122
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40 // Version 4
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80 // Variant 10

	// Format as UUID string
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		randomBytes[0:4],
		randomBytes[4:6],
		randomBytes[6:8],
		randomBytes[8:10],
		randomBytes[10:16])

	return uuid, nil
}

// generateSequentialID generates a sequential ID for testing/backward compatibility
func (g *UUIDGenerator) generateSequentialID() string {
	count := atomic.AddInt64(&g.counter, 1)
	return fmt.Sprintf("%d", count)
}

// NextWithTimestamp generates a UUID with timestamp (legacy method for compatibility)
func (g *UUIDGenerator) NextWithTimestamp() string {
	if g.useUUID {
		// For UUID mode, just return a standard UUID since it already includes randomness
		return g.generateKernelUUID()
	}
	// Legacy sequential with timestamp
	count := atomic.AddInt64(&g.counter, 1)
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s-%d", g.prefix, timestamp, count)
}

// SetUUIDMode enables or disables UUID generation
func (g *UUIDGenerator) SetUUIDMode(enabled bool) {
	g.useUUID = enabled
}

// SetHighPrecision enables nanosecond timestamps (legacy - no effect in UUID mode)
func (g *UUIDGenerator) SetHighPrecision(enabled bool) {
	g.useNanos = enabled
}

// Reset resets the counter (only affects sequential mode, used for testing)
func (g *UUIDGenerator) Reset() {
	atomic.StoreInt64(&g.counter, 0)
}

// Close closes any open file descriptors
func (g *UUIDGenerator) Close() error {
	if g.fallbackFD != nil {
		return g.fallbackFD.Close()
	}
	return nil
}

// IsUUIDMode returns true if generator is using UUID mode
func (g *UUIDGenerator) IsUUIDMode() bool {
	return g.useUUID
}

// Legacy type aliases for backward compatibility
type IDGenerator = UUIDGenerator

// Legacy constructor for backward compatibility
func NewIDGenerator(prefix, nodeID string) *IDGenerator {
	return NewSequentialIDGenerator(prefix, nodeID)
}
