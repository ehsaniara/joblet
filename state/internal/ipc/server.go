package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/state/internal/storage"
)

// Server handles IPC communication via Unix socket
type Server struct {
	socketPath  string
	backend     storage.Backend
	listener    net.Listener
	mu          sync.Mutex
	connections map[string]*connection
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// connection represents a single client connection
type connection struct {
	id   string
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

// NewServer creates a new IPC server
func NewServer(socketPath string, backend storage.Backend) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		socketPath:  socketPath,
		backend:     backend,
		connections: make(map[string]*connection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins listening for IPC connections
func (s *Server) Start() error {
	// Remove existing socket file
	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix listener: %w", err)
	}

	s.listener = listener

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.cancel()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all connections
	s.mu.Lock()
	for _, conn := range s.connections {
		conn.conn.Close()
	}
	s.mu.Unlock()

	// Wait for all goroutines
	s.wg.Wait()

	// Remove socket file
	return os.RemoveAll(s.socketPath)
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				continue
			}
		}

		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(netConn net.Conn) {
	defer s.wg.Done()
	defer netConn.Close()

	connID := fmt.Sprintf("conn-%d", time.Now().UnixNano())
	conn := &connection{
		id:   connID,
		conn: netConn,
		enc:  json.NewEncoder(netConn),
		dec:  json.NewDecoder(netConn),
	}

	// Register connection
	s.mu.Lock()
	s.connections[connID] = conn
	s.mu.Unlock()

	// Unregister on exit
	defer func() {
		s.mu.Lock()
		delete(s.connections, connID)
		s.mu.Unlock()
	}()

	// Read and process messages
	scanner := bufio.NewScanner(netConn)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 1MB initial, 10MB max

	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			s.sendError(conn, "", "INVALID_JSON", err.Error())
			continue
		}

		// Process message
		response := s.processMessage(msg)
		if err := conn.enc.Encode(response); err != nil {
			break
		}
	}
}

func (s *Server) processMessage(msg Message) *Response {
	ctx := context.Background()

	switch msg.Operation {
	case OpCreate:
		return s.handleCreate(ctx, msg)
	case OpUpdate:
		return s.handleUpdate(ctx, msg)
	case OpDelete:
		return s.handleDelete(ctx, msg)
	case OpGet:
		return s.handleGet(ctx, msg)
	case OpList:
		return s.handleList(ctx, msg)
	case OpSync:
		return s.handleSync(ctx, msg)
	case OpPing:
		return s.handlePing(ctx, msg)
	default:
		return &Response{
			RequestID: msg.RequestID,
			Success:   false,
			Error:     "unknown operation: " + string(msg.Operation),
		}
	}
}

func (s *Server) handleCreate(ctx context.Context, msg Message) *Response {
	if msg.Job == nil {
		return s.makeError(msg.RequestID, "CREATE_ERROR", "job is required")
	}

	if err := s.backend.Create(ctx, msg.Job); err != nil {
		return s.makeError(msg.RequestID, "CREATE_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
		Job:       msg.Job,
	}
}

func (s *Server) handleUpdate(ctx context.Context, msg Message) *Response {
	if msg.Job == nil {
		return s.makeError(msg.RequestID, "UPDATE_ERROR", "job is required")
	}

	if err := s.backend.Update(ctx, msg.Job); err != nil {
		return s.makeError(msg.RequestID, "UPDATE_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
		Job:       msg.Job,
	}
}

func (s *Server) handleDelete(ctx context.Context, msg Message) *Response {
	if msg.JobID == "" {
		return s.makeError(msg.RequestID, "DELETE_ERROR", "jobID is required")
	}

	if err := s.backend.Delete(ctx, msg.JobID); err != nil {
		return s.makeError(msg.RequestID, "DELETE_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
	}
}

func (s *Server) handleGet(ctx context.Context, msg Message) *Response {
	if msg.JobID == "" {
		return s.makeError(msg.RequestID, "GET_ERROR", "jobID is required")
	}

	job, err := s.backend.Get(ctx, msg.JobID)
	if err != nil {
		return s.makeError(msg.RequestID, "GET_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
		Job:       job,
	}
}

func (s *Server) handleList(ctx context.Context, msg Message) *Response {
	jobs, err := s.backend.List(ctx, msg.Filter)
	if err != nil {
		return s.makeError(msg.RequestID, "LIST_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
		Jobs:      jobs,
	}
}

func (s *Server) handleSync(ctx context.Context, msg Message) *Response {
	if msg.Jobs == nil || len(msg.Jobs) == 0 {
		return s.makeError(msg.RequestID, "SYNC_ERROR", "jobs array is required")
	}

	if err := s.backend.Sync(ctx, msg.Jobs); err != nil {
		return s.makeError(msg.RequestID, "SYNC_ERROR", err.Error())
	}

	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
	}
}

// handlePing implements lightweight health check (no backend query)
func (s *Server) handlePing(ctx context.Context, msg Message) *Response {
	return &Response{
		RequestID: msg.RequestID,
		Success:   true,
	}
}

func (s *Server) makeError(requestID, code, message string) *Response {
	return &Response{
		RequestID: requestID,
		Success:   false,
		Error:     code + ": " + message,
	}
}

func (s *Server) sendError(conn *connection, requestID, code, message string) {
	response := s.makeError(requestID, code, message)
	_ = conn.enc.Encode(response)
}

// Message types

type Operation string

const (
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
	OpGet    Operation = "get"
	OpList   Operation = "list"
	OpSync   Operation = "sync"
	OpPing   Operation = "ping"
)

// Message represents an IPC request message
type Message struct {
	Operation Operation       `json:"op"`
	JobID     string          `json:"jobId,omitempty"`
	Job       *domain.Job     `json:"job,omitempty"`
	Jobs      []*domain.Job   `json:"jobs,omitempty"`
	Filter    *storage.Filter `json:"filter,omitempty"`
	RequestID string          `json:"requestId"`
	Timestamp int64           `json:"timestamp"`
}

// Response represents an IPC response message
type Response struct {
	RequestID string        `json:"requestId"`
	Success   bool          `json:"success"`
	Job       *domain.Job   `json:"job,omitempty"`
	Jobs      []*domain.Job `json:"jobs,omitempty"`
	Error     string        `json:"error,omitempty"`
}
