package state

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ehsaniara/joblet/pkg/logger"
)

// Default configuration values
const (
	DefaultPoolSize             = 20
	DefaultReadTimeout          = 10 * time.Second
	DefaultDialTimeout          = 5 * time.Second
	DefaultMaxIdleTime          = 30 * time.Second
	DefaultHealthCheckTimeout   = 500 * time.Millisecond
	DefaultShutdownTimeout      = 5 * time.Second
	DefaultShutdownPollInterval = 100 * time.Millisecond
)

// PoolConfig contains configuration options for the connection pool
type PoolConfig struct {
	// PoolSize is the maximum number of connections in the pool
	PoolSize int
	// ReadTimeout is the timeout for read operations
	ReadTimeout time.Duration
	// DialTimeout is the timeout for establishing new connections
	DialTimeout time.Duration
	// MaxIdleTime is the maximum time a connection can be idle before health check
	MaxIdleTime time.Duration
	// HealthCheckTimeout is the timeout for connection health checks
	HealthCheckTimeout time.Duration
	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout time.Duration
	// ShutdownPollInterval is how often to check for in-use connections during shutdown
	ShutdownPollInterval time.Duration
}

// DefaultPoolConfig returns the default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		PoolSize:             DefaultPoolSize,
		ReadTimeout:          DefaultReadTimeout,
		DialTimeout:          DefaultDialTimeout,
		MaxIdleTime:          DefaultMaxIdleTime,
		HealthCheckTimeout:   DefaultHealthCheckTimeout,
		ShutdownTimeout:      DefaultShutdownTimeout,
		ShutdownPollInterval: DefaultShutdownPollInterval,
	}
}

// withDefaults fills in zero values with defaults
func (c PoolConfig) withDefaults() PoolConfig {
	if c.PoolSize <= 0 {
		c.PoolSize = DefaultPoolSize
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeout
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = DefaultDialTimeout
	}
	if c.MaxIdleTime <= 0 {
		c.MaxIdleTime = DefaultMaxIdleTime
	}
	if c.HealthCheckTimeout <= 0 {
		c.HealthCheckTimeout = DefaultHealthCheckTimeout
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}
	if c.ShutdownPollInterval <= 0 {
		c.ShutdownPollInterval = DefaultShutdownPollInterval
	}
	return c
}

// pooledConn represents a single pooled connection
type pooledConn struct {
	conn     net.Conn
	mu       sync.Mutex
	lastUsed time.Time
	inUse    bool
	id       uint64 // Unique identifier for tracking
}

// ConnectionPool manages a pool of connections to the state service
type ConnectionPool struct {
	socketPath  string
	config      PoolConfig
	pool        chan *pooledConn
	logger      *logger.Logger
	closed      atomic.Bool
	activeConns atomic.Int32
	totalConns  atomic.Int32

	// Track in-use connections for graceful shutdown
	inUseConns   map[uint64]*pooledConn
	inUseConnsMu sync.Mutex
	nextConnID   atomic.Uint64

	// Buffer pool for scanner allocations
	bufferPool sync.Pool

	// Metrics
	acquisitions atomic.Uint64
	creations    atomic.Uint64
	errors       atomic.Uint64
	timeouts     atomic.Uint64
	healthChecks atomic.Uint64
	staleConns   atomic.Uint64
}

// NewConnectionPool creates a new connection pool with default configuration
func NewConnectionPool(socketPath string, poolSize int, logger *logger.Logger) *ConnectionPool {
	cfg := DefaultPoolConfig()
	if poolSize > 0 {
		cfg.PoolSize = poolSize
	}
	return NewConnectionPoolWithConfig(socketPath, cfg, logger)
}

// NewConnectionPoolWithConfig creates a new connection pool with custom configuration
func NewConnectionPoolWithConfig(socketPath string, config PoolConfig, logger *logger.Logger) *ConnectionPool {
	config = config.withDefaults()

	if logger == nil {
		logger = logger.WithField("component", "state-pool")
	}

	pool := &ConnectionPool{
		socketPath: socketPath,
		config:     config,
		pool:       make(chan *pooledConn, config.PoolSize),
		logger:     logger,
		inUseConns: make(map[uint64]*pooledConn),
		bufferPool: sync.Pool{
			New: func() interface{} {
				// Allocate 1MB buffer with 10MB max
				buf := make([]byte, 1024*1024)
				return &buf
			},
		},
	}

	return pool
}

// Get acquires a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (*pooledConn, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("connection pool is closed")
	}

	p.acquisitions.Add(1)

	// Check context cancellation first to avoid race condition
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	select {
	case conn := <-p.pool:
		// Check if connection needs health validation
		conn.mu.Lock()
		idleTime := time.Since(conn.lastUsed)
		conn.mu.Unlock()

		if idleTime > p.config.MaxIdleTime {
			// Connection has been idle too long, validate it
			if !p.isConnectionHealthy(conn) {
				p.staleConns.Add(1)
				conn.close()
				p.totalConns.Add(-1)
				// Try to get another connection by recursing
				return p.Get(ctx)
			}
		}

		// Reuse existing connection
		conn.mu.Lock()
		conn.inUse = true
		conn.lastUsed = time.Now()
		conn.mu.Unlock()

		// Track as in-use
		p.inUseConnsMu.Lock()
		p.inUseConns[conn.id] = conn
		p.inUseConnsMu.Unlock()

		p.activeConns.Add(1)
		return conn, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// Pool is empty, try to create new connection if under limit
		// Use atomic increment first to reserve a slot, preventing race conditions
		// where multiple goroutines could exceed the pool size
		newCount := p.totalConns.Add(1)
		if newCount <= int32(p.config.PoolSize) {
			conn, err := p.createConnectionWithoutIncrement(ctx)
			if err != nil {
				// Rollback the increment on failure
				p.totalConns.Add(-1)
				return nil, err
			}
			return conn, nil
		}
		// We exceeded the limit, rollback and wait for available connection
		p.totalConns.Add(-1)

		// Wait for available connection
		select {
		case conn := <-p.pool:
			// Check if connection needs health validation
			conn.mu.Lock()
			idleTime := time.Since(conn.lastUsed)
			conn.mu.Unlock()

			if idleTime > p.config.MaxIdleTime {
				// Connection has been idle too long, validate it
				if !p.isConnectionHealthy(conn) {
					p.staleConns.Add(1)
					conn.close()
					p.totalConns.Add(-1)
					// Try to get another connection by recursing
					return p.Get(ctx)
				}
			}

			conn.mu.Lock()
			conn.inUse = true
			conn.lastUsed = time.Now()
			conn.mu.Unlock()

			// Track as in-use
			p.inUseConnsMu.Lock()
			p.inUseConns[conn.id] = conn
			p.inUseConnsMu.Unlock()

			p.activeConns.Add(1)
			return conn, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// isConnectionHealthy checks if a connection is still alive using a zero-byte read
func (p *ConnectionPool) isConnectionHealthy(conn *pooledConn) bool {
	p.healthChecks.Add(1)

	// Set a short deadline for health check
	if err := conn.conn.SetReadDeadline(time.Now().Add(p.config.HealthCheckTimeout)); err != nil {
		return false
	}

	// Try a zero-byte read to check if connection is still alive
	// This will return immediately if connection is open, or error if closed
	one := make([]byte, 1)
	_ = conn.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	_, err := conn.conn.Read(one)

	// Reset deadline
	_ = conn.conn.SetReadDeadline(time.Time{})

	if err != nil {
		// Check if it's a timeout (expected for healthy connection)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true // Timeout means connection is still alive
		}
		// Any other error means connection is broken
		return false
	}

	// If we actually read data, that's unexpected - connection might be in bad state
	// but we'll let it pass and handle any issues during actual operation
	return true
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn *pooledConn) {
	if conn == nil {
		return
	}

	p.activeConns.Add(-1)

	// Untrack from in-use connections
	p.inUseConnsMu.Lock()
	delete(p.inUseConns, conn.id)
	p.inUseConnsMu.Unlock()

	conn.mu.Lock()
	conn.inUse = false
	conn.mu.Unlock()

	if p.closed.Load() {
		conn.close()
		p.totalConns.Add(-1)
		return
	}

	// Try to return to pool, drop if full
	select {
	case p.pool <- conn:
		// Successfully returned to pool
	default:
		// Pool is full, close this connection
		conn.close()
		p.totalConns.Add(-1)
	}
}

// Remove removes a connection from the pool (used for broken connections)
func (p *ConnectionPool) Remove(conn *pooledConn) {
	if conn == nil {
		return
	}

	// Untrack from in-use connections
	p.inUseConnsMu.Lock()
	delete(p.inUseConns, conn.id)
	p.inUseConnsMu.Unlock()

	p.activeConns.Add(-1)
	conn.close()
	p.totalConns.Add(-1)
}

// createConnectionWithoutIncrement creates a new connection without incrementing totalConns
// Used when the caller has already reserved a slot via atomic increment
func (p *ConnectionPool) createConnectionWithoutIncrement(ctx context.Context) (*pooledConn, error) {
	// Use dial timeout
	dialCtx, cancel := context.WithTimeout(ctx, p.config.DialTimeout)
	defer cancel()

	var d net.Dialer
	netConn, err := d.DialContext(dialCtx, "unix", p.socketPath)
	if err != nil {
		p.errors.Add(1)
		return nil, fmt.Errorf("failed to dial state socket: %w", err)
	}

	connID := p.nextConnID.Add(1)
	conn := &pooledConn{
		conn:     netConn,
		lastUsed: time.Now(),
		inUse:    true,
		id:       connID,
	}

	// Track as in-use
	p.inUseConnsMu.Lock()
	p.inUseConns[connID] = conn
	p.inUseConnsMu.Unlock()

	p.activeConns.Add(1)
	p.creations.Add(1)

	p.logger.Debug("created new state connection",
		"total", p.totalConns.Load(),
		"active", p.activeConns.Load(),
		"conn_id", connID)

	return conn, nil
}

// Close closes all connections in the pool with graceful shutdown
func (p *ConnectionPool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Close the channel to prevent new connections from being added
	close(p.pool)

	// Close all pooled (idle) connections
	for conn := range p.pool {
		conn.close()
		p.totalConns.Add(-1)
	}

	// Wait for in-use connections with timeout
	deadline := time.Now().Add(p.config.ShutdownTimeout)

	for time.Now().Before(deadline) {
		p.inUseConnsMu.Lock()
		inUseCount := len(p.inUseConns)
		p.inUseConnsMu.Unlock()

		if inUseCount == 0 {
			break
		}

		p.logger.Debug("waiting for in-use connections to be released",
			"in_use_count", inUseCount,
			"remaining_time", time.Until(deadline))

		time.Sleep(p.config.ShutdownPollInterval)
	}

	// Force close any remaining in-use connections after timeout
	p.inUseConnsMu.Lock()
	remainingCount := len(p.inUseConns)
	if remainingCount > 0 {
		p.logger.Warn("force closing in-use connections after timeout",
			"count", remainingCount)
		for id, conn := range p.inUseConns {
			conn.close()
			p.totalConns.Add(-1)
			p.activeConns.Add(-1)
			delete(p.inUseConns, id)
		}
	}
	p.inUseConnsMu.Unlock()

	p.logger.Info("connection pool closed",
		"total_acquisitions", p.acquisitions.Load(),
		"total_creations", p.creations.Load(),
		"total_errors", p.errors.Load(),
		"total_timeouts", p.timeouts.Load(),
		"health_checks", p.healthChecks.Load(),
		"stale_conns", p.staleConns.Load(),
		"force_closed", remainingCount)

	return nil
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"pool_size":       p.config.PoolSize,
		"total_conns":     p.totalConns.Load(),
		"active_conns":    p.activeConns.Load(),
		"available_conns": len(p.pool),
		"acquisitions":    p.acquisitions.Load(),
		"health_checks":   p.healthChecks.Load(),
		"stale_conns":     p.staleConns.Load(),
		"creations":       p.creations.Load(),
		"errors":          p.errors.Load(),
		"timeouts":        p.timeouts.Load(),
	}
}

// sendMessageWithResponse sends a message and waits for response
func (p *ConnectionPool) sendMessageWithResponse(ctx context.Context, conn *pooledConn, msg Message) (*Response, error) {
	// Encode message
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	data = append(data, '\n')

	// Set write deadline
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.conn.SetWriteDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set write deadline: %w", err)
		}
	}

	// Write message
	if _, err := conn.conn.Write(data); err != nil {
		p.errors.Add(1)
		return nil, fmt.Errorf("failed to write to state socket: %w", err)
	}

	// Reset write deadline
	_ = conn.conn.SetWriteDeadline(time.Time{})

	// Set read deadline (use context or default timeout)
	readDeadline := time.Now().Add(p.config.ReadTimeout)
	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Before(readDeadline) {
			readDeadline = deadline
		}
	}

	if err := conn.conn.SetReadDeadline(readDeadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response using pooled buffer to avoid repeated allocations
	bufPtr := p.bufferPool.Get().(*[]byte)
	defer p.bufferPool.Put(bufPtr)

	scanner := bufio.NewScanner(conn.conn)
	scanner.Buffer(*bufPtr, 10*1024*1024)

	if !scanner.Scan() {
		// Reset deadline before returning
		_ = conn.conn.SetReadDeadline(time.Time{})

		if err := scanner.Err(); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				p.timeouts.Add(1)
				return nil, fmt.Errorf("read timeout after %v: %w", p.config.ReadTimeout, err)
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("connection closed")
	}

	// Reset deadline
	_ = conn.conn.SetReadDeadline(time.Time{})

	// Decode response
	var response Response
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// close closes the underlying connection
func (c *pooledConn) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
