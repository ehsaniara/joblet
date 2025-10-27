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

const (
	// Default pool size - tuned for high concurrency
	defaultPoolSize = 20

	// Connection timeout for reads
	defaultReadTimeout = 10 * time.Second

	// Dial timeout for new connections
	defaultDialTimeout = 5 * time.Second
)

// pooledConn represents a single pooled connection
type pooledConn struct {
	conn     net.Conn
	mu       sync.Mutex
	lastUsed time.Time
	inUse    bool
}

// ConnectionPool manages a pool of connections to the state service
type ConnectionPool struct {
	socketPath  string
	pool        chan *pooledConn
	poolSize    int
	readTimeout time.Duration
	dialTimeout time.Duration
	logger      *logger.Logger
	closed      atomic.Bool
	activeConns atomic.Int32
	totalConns  atomic.Int32

	// Metrics
	acquisitions atomic.Uint64
	creations    atomic.Uint64
	errors       atomic.Uint64
	timeouts     atomic.Uint64
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(socketPath string, poolSize int, logger *logger.Logger) *ConnectionPool {
	if poolSize <= 0 {
		poolSize = defaultPoolSize
	}

	if logger == nil {
		logger = logger.WithField("component", "state-pool")
	}

	pool := &ConnectionPool{
		socketPath:  socketPath,
		pool:        make(chan *pooledConn, poolSize),
		poolSize:    poolSize,
		readTimeout: defaultReadTimeout,
		dialTimeout: defaultDialTimeout,
		logger:      logger,
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
		// Reuse existing connection
		conn.mu.Lock()
		conn.inUse = true
		conn.lastUsed = time.Now()
		conn.mu.Unlock()
		p.activeConns.Add(1)
		return conn, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// Pool is empty, create new connection if under limit
		if p.totalConns.Load() < int32(p.poolSize) {
			return p.createConnection(ctx)
		}

		// Wait for available connection
		select {
		case conn := <-p.pool:
			conn.mu.Lock()
			conn.inUse = true
			conn.lastUsed = time.Now()
			conn.mu.Unlock()
			p.activeConns.Add(1)
			return conn, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn *pooledConn) {
	if conn == nil {
		return
	}

	p.activeConns.Add(-1)

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

	p.activeConns.Add(-1)
	conn.close()
	p.totalConns.Add(-1)
}

// createConnection creates a new connection
func (p *ConnectionPool) createConnection(ctx context.Context) (*pooledConn, error) {
	// Use dial timeout
	dialCtx, cancel := context.WithTimeout(ctx, p.dialTimeout)
	defer cancel()

	var d net.Dialer
	netConn, err := d.DialContext(dialCtx, "unix", p.socketPath)
	if err != nil {
		p.errors.Add(1)
		return nil, fmt.Errorf("failed to dial state socket: %w", err)
	}

	conn := &pooledConn{
		conn:     netConn,
		lastUsed: time.Now(),
		inUse:    true,
	}

	p.totalConns.Add(1)
	p.activeConns.Add(1)
	p.creations.Add(1)

	p.logger.Debug("created new state connection",
		"total", p.totalConns.Load(),
		"active", p.activeConns.Load())

	return conn, nil
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	close(p.pool)

	// Close all pooled connections
	for conn := range p.pool {
		conn.close()
		p.totalConns.Add(-1)
	}

	p.logger.Info("connection pool closed",
		"total_acquisitions", p.acquisitions.Load(),
		"total_creations", p.creations.Load(),
		"total_errors", p.errors.Load(),
		"total_timeouts", p.timeouts.Load())

	return nil
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"pool_size":       p.poolSize,
		"total_conns":     p.totalConns.Load(),
		"active_conns":    p.activeConns.Load(),
		"available_conns": len(p.pool),
		"acquisitions":    p.acquisitions.Load(),
		"creations":       p.creations.Load(),
		"errors":          p.errors.Load(),
		"timeouts":        p.timeouts.Load(),
	}
}

// sendMessage sends a message on a pooled connection without waiting for response
// nolint:unused // Reserved for future fire-and-forget operations
func (p *ConnectionPool) sendMessage(ctx context.Context, conn *pooledConn, msg Message) error {
	// Encode message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	data = append(data, '\n')

	// Set write deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
	}

	// Write message
	if _, err := conn.conn.Write(data); err != nil {
		p.errors.Add(1)
		return fmt.Errorf("failed to write to state socket: %w", err)
	}

	// Reset deadline
	_ = conn.conn.SetWriteDeadline(time.Time{})

	return nil
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
	readDeadline := time.Now().Add(p.readTimeout)
	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Before(readDeadline) {
			readDeadline = deadline
		}
	}

	if err := conn.conn.SetReadDeadline(readDeadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(conn.conn)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	if !scanner.Scan() {
		// Reset deadline before returning
		_ = conn.conn.SetReadDeadline(time.Time{})

		if err := scanner.Err(); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				p.timeouts.Add(1)
				return nil, fmt.Errorf("read timeout after %v: %w", p.readTimeout, err)
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
