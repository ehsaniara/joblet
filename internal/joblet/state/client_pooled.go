package state

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Default client configuration values
const (
	DefaultMaxRetries     = 3
	DefaultRetryBaseDelay = 100 * time.Millisecond
	DefaultRetryMaxDelay  = 2 * time.Second
	DefaultConnectTimeout = 5 * time.Second
)

// ClientConfig contains configuration options for the pooled client
type ClientConfig struct {
	// PoolConfig contains connection pool configuration
	PoolConfig PoolConfig
	// MaxRetries is the maximum number of retry attempts for transient failures
	MaxRetries int
	// RetryBaseDelay is the initial delay between retries (doubles with each attempt)
	RetryBaseDelay time.Duration
	// RetryMaxDelay is the maximum delay between retries
	RetryMaxDelay time.Duration
	// ConnectTimeout is the timeout for initial connection test
	ConnectTimeout time.Duration
}

// DefaultClientConfig returns the default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		PoolConfig:     DefaultPoolConfig(),
		MaxRetries:     DefaultMaxRetries,
		RetryBaseDelay: DefaultRetryBaseDelay,
		RetryMaxDelay:  DefaultRetryMaxDelay,
		ConnectTimeout: DefaultConnectTimeout,
	}
}

// withDefaults fills in zero values with defaults
func (c ClientConfig) withDefaults() ClientConfig {
	c.PoolConfig = c.PoolConfig.withDefaults()
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = DefaultRetryBaseDelay
	}
	if c.RetryMaxDelay <= 0 {
		c.RetryMaxDelay = DefaultRetryMaxDelay
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = DefaultConnectTimeout
	}
	return c
}

// PooledClient provides high-performance IPC communication with state subprocess
// using a connection pool to eliminate global mutex bottleneck
type PooledClient struct {
	pool      *ConnectionPool
	config    ClientConfig
	logger    *logger.Logger
	requestID uint64 // Accessed atomically
}

// NewPooledClient creates a new pooled state IPC client with default configuration
func NewPooledClient(socketPath string, poolSize int, logger *logger.Logger) *PooledClient {
	cfg := DefaultClientConfig()
	if poolSize > 0 {
		cfg.PoolConfig.PoolSize = poolSize
	}
	return NewPooledClientWithConfig(socketPath, cfg, logger)
}

// NewPooledClientWithConfig creates a new pooled state IPC client with custom configuration
func NewPooledClientWithConfig(socketPath string, config ClientConfig, logger *logger.Logger) *PooledClient {
	config = config.withDefaults()

	if logger == nil {
		logger = logger.WithField("component", "state-client-pooled")
	}

	pool := NewConnectionPoolWithConfig(socketPath, config.PoolConfig, logger)

	return &PooledClient{
		pool:   pool,
		config: config,
		logger: logger,
	}
}

// Connect performs initial connection test (optional for pooled client)
func (c *PooledClient) Connect() error {
	// For pooled client, we just test that we can get a connection
	ctx, cancel := context.WithTimeout(context.Background(), c.config.ConnectTimeout)
	defer cancel()

	conn, err := c.pool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %w", err)
	}

	// Return immediately
	c.pool.Put(conn)
	c.logger.Info("pooled client connected", "pool_size", c.config.PoolConfig.PoolSize)
	return nil
}

// Close closes the connection pool
func (c *PooledClient) Close() error {
	return c.pool.Close()
}

// Create creates a new job state (fire-and-forget with acknowledgment)
func (c *PooledClient) Create(ctx context.Context, job *domain.Job) error {
	msg := Message{
		Operation: "create",
		Job:       job,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessageFireAndForget(ctx, msg)
}

// Update updates an existing job state (fire-and-forget with acknowledgment)
func (c *PooledClient) Update(ctx context.Context, job *domain.Job) error {
	msg := Message{
		Operation: "update",
		Job:       job,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessageFireAndForget(ctx, msg)
}

// Delete deletes a job state (fire-and-forget with acknowledgment)
func (c *PooledClient) Delete(ctx context.Context, jobID string) error {
	msg := Message{
		Operation: "delete",
		JobUUID:   jobID,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessageFireAndForget(ctx, msg)
}

// Get retrieves a job state (synchronous with response)
func (c *PooledClient) Get(ctx context.Context, jobID string) (*domain.Job, error) {
	msg := Message{
		Operation: "get",
		JobUUID:   jobID,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	response, err := c.sendMessageWithResponse(ctx, msg)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("get failed: %s", response.Error)
	}

	return response.Job, nil
}

// List retrieves all job states with optional filter (synchronous with response)
func (c *PooledClient) List(ctx context.Context, filter *Filter) ([]*domain.Job, error) {
	msg := Message{
		Operation: "list",
		Filter:    filter,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	response, err := c.sendMessageWithResponse(ctx, msg)
	if err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("list failed: %s", response.Error)
	}

	return response.Jobs, nil
}

// Sync synchronizes bulk job states (fire-and-forget with acknowledgment)
func (c *PooledClient) Sync(ctx context.Context, jobs []*domain.Job) error {
	msg := Message{
		Operation: "sync",
		Jobs:      jobs,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessageFireAndForget(ctx, msg)
}

// Ping checks if the state service is healthy (lightweight health check)
func (c *PooledClient) Ping(ctx context.Context) error {
	msg := Message{
		Operation: "ping",
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	response, err := c.sendMessageWithResponse(ctx, msg)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("ping failed: %s", response.Error)
	}

	return nil
}

// Stats returns connection pool statistics
func (c *PooledClient) Stats() map[string]interface{} {
	return c.pool.Stats()
}

// sendMessageFireAndForget sends a message and waits for acknowledgment with retry support
// This ensures the message was received, but doesn't wait for full processing
func (c *PooledClient) sendMessageFireAndForget(ctx context.Context, msg Message) error {
	var lastErr error
	delay := c.config.RetryBaseDelay

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w (last error: %v)", ctx.Err(), lastErr)
			case <-time.After(delay):
				// Double delay for next attempt, cap at max
				delay *= 2
				if delay > c.config.RetryMaxDelay {
					delay = c.config.RetryMaxDelay
				}
			}
			c.logger.Debug("retrying fire-and-forget operation",
				"attempt", attempt+1,
				"max_retries", c.config.MaxRetries,
				"operation", msg.Operation)
		}

		err := c.trySendMessage(ctx, msg)
		if err == nil {
			return nil
		}

		lastErr = err
		// Only retry on connection errors, not on logical failures
		if !isRetryableError(err) {
			return err
		}
	}

	c.logger.Error("fire-and-forget operation failed after retries",
		"operation", msg.Operation,
		"max_retries", c.config.MaxRetries,
		"error", lastErr)

	return fmt.Errorf("operation failed after %d retries: %w", c.config.MaxRetries, lastErr)
}

// trySendMessage attempts to send a message once
func (c *PooledClient) trySendMessage(ctx context.Context, msg Message) error {
	// Get connection from pool
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}

	// Send message and get acknowledgment
	response, err := c.pool.sendMessageWithResponse(ctx, conn, msg)

	if err != nil {
		// Connection is broken, remove from pool
		c.pool.Remove(conn)
		return err
	}

	// Return connection to pool
	c.pool.Put(conn)

	// Check if operation succeeded
	if response != nil && !response.Success {
		return fmt.Errorf("operation failed: %s", response.Error)
	}

	return nil
}

// isRetryableError checks if an error is transient and worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Retry on connection/network errors, not on logical errors
	return contains(errStr, "connection") ||
		contains(errStr, "timeout") ||
		contains(errStr, "dial") ||
		contains(errStr, "EOF") ||
		contains(errStr, "reset") ||
		contains(errStr, "broken pipe")
}

// contains checks if s contains substr (simple helper to avoid strings import)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// sendMessageWithResponse sends a message and waits for full response
func (c *PooledClient) sendMessageWithResponse(ctx context.Context, msg Message) (*Response, error) {
	// Get connection from pool
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	// Send message and get response
	response, err := c.pool.sendMessageWithResponse(ctx, conn, msg)

	if err != nil {
		// Connection is broken, remove from pool
		c.pool.Remove(conn)
		return nil, err
	}

	// Return connection to pool
	c.pool.Put(conn)

	return response, nil
}

// nextRequestID generates a unique request ID (thread-safe)
func (c *PooledClient) nextRequestID() string {
	id := atomic.AddUint64(&c.requestID, 1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), id)
}
