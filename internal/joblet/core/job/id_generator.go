package job

import (
	"fmt"
	"sync/atomic"
	"time"
)

// IDGenerator generates unique job IDs
type IDGenerator struct {
	counter  int64
	prefix   string
	nodeID   string // For distributed systems
	useNanos bool   // Use nanoseconds for higher precision
}

// NewIDGenerator creates a new ID generator
func NewIDGenerator(prefix, nodeID string) *IDGenerator {
	return &IDGenerator{
		prefix:   prefix,
		nodeID:   nodeID,
		useNanos: false,
	}
}

// Next generates the next job ID
func (g *IDGenerator) Next() string {
	count := atomic.AddInt64(&g.counter, 1)

	//if g.nodeID != "" {
	//	// Distributed format: prefix-nodeID-timestamp-counter
	//	if g.useNanos {
	//		return fmt.Sprintf("%s-%s-%d-%d", g.prefix, g.nodeID, time.Now().UnixNano(), count)
	//	}
	//	return fmt.Sprintf("%s-%s-%d-%d", g.prefix, g.nodeID, time.Now().Unix(), count)
	//}

	// Simple format: prefix-counter
	//return fmt.Sprintf("%s-%d", g.prefix, count)
	return fmt.Sprintf("%d", count)
}

// NextWithTimestamp generates an ID with timestamp
func (g *IDGenerator) NextWithTimestamp() string {
	count := atomic.AddInt64(&g.counter, 1)
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s-%d", g.prefix, timestamp, count)
}

// SetHighPrecision enables nanosecond timestamps
func (g *IDGenerator) SetHighPrecision(enabled bool) {
	g.useNanos = enabled
}

// Reset resets the counter (useful for testing)
func (g *IDGenerator) Reset() {
	atomic.StoreInt64(&g.counter, 0)
}
