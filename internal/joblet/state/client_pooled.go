package state

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// PooledClient provides high-performance IPC communication with state subprocess
// using a connection pool to eliminate global mutex bottleneck
type PooledClient struct {
	pool      *ConnectionPool
	logger    *logger.Logger
	requestID uint64 // Accessed atomically
}

// NewPooledClient creates a new pooled state IPC client
func NewPooledClient(socketPath string, poolSize int, logger *logger.Logger) *PooledClient {
	if logger == nil {
		logger = logger.WithField("component", "state-client-pooled")
	}

	if poolSize <= 0 {
		poolSize = defaultPoolSize
	}

	pool := NewConnectionPool(socketPath, poolSize, logger)

	return &PooledClient{
		pool:   pool,
		logger: logger,
	}
}

// Connect performs initial connection test (optional for pooled client)
func (c *PooledClient) Connect() error {
	// For pooled client, we just test that we can get a connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := c.pool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %w", err)
	}

	// Return immediately
	c.pool.Put(conn)
	c.logger.Info("pooled client connected", "pool_size", c.pool.poolSize)
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
		JobID:     jobID,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessageFireAndForget(ctx, msg)
}

// Get retrieves a job state (synchronous with response)
func (c *PooledClient) Get(ctx context.Context, jobID string) (*domain.Job, error) {
	msg := Message{
		Operation: "get",
		JobID:     jobID,
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

// sendMessageFireAndForget sends a message and waits for acknowledgment
// This ensures the message was received, but doesn't wait for full processing
func (c *PooledClient) sendMessageFireAndForget(ctx context.Context, msg Message) error {
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
