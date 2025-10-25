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

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Client provides IPC communication with state subprocess
type Client struct {
	socketPath     string
	conn           net.Conn
	mu             sync.Mutex
	reconnectDelay time.Duration
	logger         *logger.Logger
	requestID      uint64 // Accessed atomically, do not access directly
}

// NewClient creates a new state IPC client
func NewClient(socketPath string, logger *logger.Logger) *Client {
	if logger == nil {
		logger = logger.WithField("component", "state-client")
	}

	return &Client{
		socketPath:     socketPath,
		reconnectDelay: 1 * time.Second,
		logger:         logger,
	}
}

// Connect establishes connection to state subprocess
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil // Already connected
	}

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to state socket %s: %w", c.socketPath, err)
	}

	c.conn = conn
	c.logger.Info("connected to state subprocess", "socket", c.socketPath)
	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Create creates a new job state
func (c *Client) Create(ctx context.Context, job *domain.Job) error {
	msg := Message{
		Operation: "create",
		Job:       job,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessage(ctx, msg)
}

// Update updates an existing job state
func (c *Client) Update(ctx context.Context, job *domain.Job) error {
	msg := Message{
		Operation: "update",
		Job:       job,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessage(ctx, msg)
}

// Delete deletes a job state
func (c *Client) Delete(ctx context.Context, jobID string) error {
	msg := Message{
		Operation: "delete",
		JobID:     jobID,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessage(ctx, msg)
}

// Get retrieves a job state
func (c *Client) Get(ctx context.Context, jobID string) (*domain.Job, error) {
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

// List retrieves all job states with optional filter
func (c *Client) List(ctx context.Context, filter *Filter) ([]*domain.Job, error) {
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

// Sync synchronizes bulk job states (for reconciliation)
func (c *Client) Sync(ctx context.Context, jobs []*domain.Job) error {
	msg := Message{
		Operation: "sync",
		Jobs:      jobs,
		RequestID: c.nextRequestID(),
		Timestamp: time.Now().Unix(),
	}

	return c.sendMessage(ctx, msg)
}

// Ping checks if the state service is healthy (lightweight health check)
func (c *Client) Ping(ctx context.Context) error {
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

// sendMessage sends a message without waiting for response (fire-and-forget)
func (c *Client) sendMessage(ctx context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		// Try to reconnect
		if err := c.reconnect(); err != nil {
			c.logger.Warn("failed to reconnect to state service", "error", err)
			return err
		}
	}

	// Encode and send message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		c.logger.Error("failed to write to state socket", "error", err)
		c.conn.Close()
		c.conn = nil
		return err
	}

	return nil
}

// sendMessageWithResponse sends a message and waits for response
func (c *Client) sendMessageWithResponse(ctx context.Context, msg Message) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		// Try to reconnect
		if err := c.reconnect(); err != nil {
			c.logger.Warn("failed to reconnect to state service", "error", err)
			return nil, err
		}
	}

	// Encode and send message
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		c.logger.Error("failed to write to state socket", "error", err)
		c.conn.Close()
		c.conn = nil
		return nil, err
	}

	// Read response
	scanner := bufio.NewScanner(c.conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("connection closed")
	}

	var response Response
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// reconnect attempts to reconnect to the state service
func (c *Client) reconnect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("failed to reconnect to state socket: %w", err)
	}

	c.conn = conn
	c.logger.Info("reconnected to joblet-state subprocess")
	return nil
}

// nextRequestID generates a unique request ID (thread-safe)
func (c *Client) nextRequestID() string {
	id := atomic.AddUint64(&c.requestID, 1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), id)
}

// Message types matching state IPC protocol

type Message struct {
	Operation string        `json:"op"`
	JobID     string        `json:"jobId,omitempty"`
	Job       *domain.Job   `json:"job,omitempty"`
	Jobs      []*domain.Job `json:"jobs,omitempty"`
	Filter    *Filter       `json:"filter,omitempty"`
	RequestID string        `json:"requestId"`
	Timestamp int64         `json:"timestamp"`
}

type Response struct {
	RequestID string        `json:"requestId"`
	Success   bool          `json:"success"`
	Job       *domain.Job   `json:"job,omitempty"`
	Jobs      []*domain.Job `json:"jobs,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type Filter struct {
	Status   string   `json:"status,omitempty"`
	NodeID   string   `json:"nodeId,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	SortBy   string   `json:"sortBy,omitempty"`
	SortDesc bool     `json:"sortDesc,omitempty"`
	Statuses []string `json:"statuses,omitempty"`
}
