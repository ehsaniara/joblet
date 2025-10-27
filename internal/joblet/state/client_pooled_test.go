package state

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

func TestPooledClient_Connect(t *testing.T) {
	// Start a mock server
	socketPath := "/tmp/test-pooled-client-connect.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	err := client.Connect()
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}
}

func TestPooledClient_CreateUpdateDelete(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-crud.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	ctx := context.Background()

	// Test Create
	job := &domain.Job{
		Uuid:      "test-job-1",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    "SCHEDULED",
		StartTime: time.Now(),
	}

	err := client.Create(ctx, job)
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	// Test Update
	job.Status = "RUNNING"
	err = client.Update(ctx, job)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	// Test Delete
	err = client.Delete(ctx, job.Uuid)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
}

func TestPooledClient_Ping(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-ping.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	ctx := context.Background()
	err := client.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestPooledClient_Stats(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-stats.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 10, log)
	defer client.Close()

	stats := client.Stats()

	if stats["pool_size"].(int) != 10 {
		t.Errorf("Expected pool_size 10, got %v", stats["pool_size"])
	}

	if stats["total_conns"].(int32) != 0 {
		t.Errorf("Expected 0 total_conns initially, got %v", stats["total_conns"])
	}
}

func TestPooledClient_ConcurrentOperations(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-concurrent.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	const numOps = 50
	errors := make(chan error, numOps)

	ctx := context.Background()

	// Run concurrent creates
	for i := 0; i < numOps; i++ {
		go func(id int) {
			job := &domain.Job{
				Uuid:      fmt.Sprintf("concurrent-job-%d", id),
				Command:   "echo",
				Args:      []string{"test"},
				Status:    "SCHEDULED",
				StartTime: time.Now(),
			}

			err := client.Create(ctx, job)
			if err != nil {
				errors <- fmt.Errorf("create %d failed: %w", id, err)
			} else {
				errors <- nil
			}
		}(i)
	}

	// Check results
	errorCount := 0
	for i := 0; i < numOps; i++ {
		if err := <-errors; err != nil {
			t.Logf("Error: %v", err)
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors out of %d operations", errorCount, numOps)
	}

	// Check stats
	stats := client.Stats()
	t.Logf("Stats after %d operations: %+v", numOps, stats)

	// Should have created some connections
	if stats["creations"].(uint64) == 0 {
		t.Error("Expected at least one connection to be created")
	}

	// Should have processed all acquisitions
	if stats["acquisitions"].(uint64) < uint64(numOps) {
		t.Errorf("Expected at least %d acquisitions, got %v", numOps, stats["acquisitions"])
	}
}

func TestPooledClient_ContextCancellation(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-cancel.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	job := &domain.Job{
		Uuid:      "cancel-test-job",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    "SCHEDULED",
		StartTime: time.Now(),
	}

	err := client.Create(ctx, job)
	if err == nil {
		t.Error("Expected error with cancelled context")
	} else if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestPooledClient_Timeout(t *testing.T) {
	socketPath := "/tmp/test-pooled-client-timeout.sock"
	cleanup := startMockServer(t, socketPath)
	defer cleanup()

	log := logger.WithField("test", "pooled-client")
	client := NewPooledClient(socketPath, 5, log)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout has passed

	job := &domain.Job{
		Uuid:      "timeout-test-job",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    "SCHEDULED",
		StartTime: time.Now(),
	}

	err := client.Create(ctx, job)
	if err == nil {
		t.Error("Expected error with exceeded deadline")
	} else if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected deadline exceeded or timeout error, got: %v", err)
	}
}

// startMockServer starts a simple mock server that responds to state IPC messages
func startMockServer(t *testing.T, socketPath string) func() {
	// Remove existing socket
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Start server in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Listener closed
			}

			// Handle connection in goroutine
			go handleMockConnection(conn)
		}
	}()

	// Cleanup function
	return func() {
		listener.Close()
		os.Remove(socketPath)
	}
}

// handleMockConnection handles a single mock connection
func handleMockConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		// Send success response
		response := Response{
			RequestID: msg.RequestID,
			Success:   true,
		}

		// For get/list operations, include data
		if msg.Operation == "get" {
			response.Job = &domain.Job{
				Uuid:      msg.JobID,
				Status:    "RUNNING",
				StartTime: time.Now(),
			}
		} else if msg.Operation == "list" {
			response.Jobs = []*domain.Job{}
		}

		_ = encoder.Encode(response)
	}
}
