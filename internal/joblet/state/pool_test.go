package state

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// BenchmarkPooledClient_ConcurrentCreates benchmarks concurrent job creation
// This simulates the scenario of 1000 jobs starting simultaneously
func BenchmarkPooledClient_ConcurrentCreates(b *testing.B) {
	benchmarkConcurrentOps(b, 100, 20) // 100 concurrent operations, pool size 20
}

// BenchmarkPooledClient_HighConcurrency benchmarks very high concurrency
func BenchmarkPooledClient_HighConcurrency(b *testing.B) {
	benchmarkConcurrentOps(b, 1000, 20) // 1000 concurrent operations, pool size 20
}

// BenchmarkPooledClient_VaryingPoolSizes benchmarks different pool sizes
func BenchmarkPooledClient_PoolSize5(b *testing.B) {
	benchmarkConcurrentOps(b, 100, 5)
}

func BenchmarkPooledClient_PoolSize10(b *testing.B) {
	benchmarkConcurrentOps(b, 100, 10)
}

func BenchmarkPooledClient_PoolSize20(b *testing.B) {
	benchmarkConcurrentOps(b, 100, 20)
}

func BenchmarkPooledClient_PoolSize50(b *testing.B) {
	benchmarkConcurrentOps(b, 100, 50)
}

// benchmarkConcurrentOps runs concurrent operations against a mock server
func benchmarkConcurrentOps(b *testing.B, concurrency int, poolSize int) {
	// Note: This benchmark requires a running state service
	// For unit testing, you can skip this with: go test -short
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode - requires running state service")
	}

	log := logger.WithField("test", "benchmark")
	socketPath := "/tmp/state-benchmark.sock"

	// For actual benchmarking, you would need to start a mock server
	// This is a simplified version that shows the structure
	pool := NewConnectionPool(socketPath, poolSize, log)
	defer pool.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for j := 0; j < concurrency; j++ {
			go func(id int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// Simulate a create operation
				msg := Message{
					Operation: "create",
					Job: &domain.Job{
						Uuid:   fmt.Sprintf("job-%d-%d", i, id),
						Status: "RUNNING",
					},
					RequestID: fmt.Sprintf("req-%d-%d", i, id),
					Timestamp: time.Now().Unix(),
				}

				conn, err := pool.Get(ctx)
				if err != nil {
					b.Logf("Failed to get connection: %v", err)
					return
				}

				// In real test, would send message here
				_ = msg

				pool.Put(conn)
			}(j)
		}

		wg.Wait()
	}

	b.StopTimer()

	// Report pool statistics
	stats := pool.Stats()
	b.ReportMetric(float64(stats["acquisitions"].(uint64))/float64(b.N), "acquisitions/op")
	b.ReportMetric(float64(stats["creations"].(uint64)), "total_conns")
	b.ReportMetric(float64(stats["errors"].(uint64)), "errors")
	b.ReportMetric(float64(stats["timeouts"].(uint64)), "timeouts")
}

// TestConnectionPool_ConcurrentAccess tests concurrent access to the pool
func TestConnectionPool_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.WithField("test", "pool")
	socketPath := "/tmp/state-test.sock"

	pool := NewConnectionPool(socketPath, 10, log)
	defer pool.Close()

	const concurrency = 100
	const opsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(concurrency)

	errors := make(chan error, concurrency*opsPerGoroutine)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

				conn, err := pool.Get(ctx)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d op %d: failed to get connection: %w", id, j, err)
					cancel()
					continue
				}

				// Simulate work
				time.Sleep(1 * time.Millisecond)

				pool.Put(conn)
				cancel()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Logf("Total errors: %d out of %d operations", errorCount, concurrency*opsPerGoroutine)
	}

	// Report statistics
	stats := pool.Stats()
	t.Logf("Pool stats: %+v", stats)
	t.Logf("Pool size: %d", pool.poolSize)
	t.Logf("Total connections created: %d", stats["creations"])
	t.Logf("Active connections: %d", stats["active_conns"])
	t.Logf("Available connections: %d", stats["available_conns"])
	t.Logf("Total acquisitions: %d", stats["acquisitions"])
	t.Logf("Errors: %d", stats["errors"])
	t.Logf("Timeouts: %d", stats["timeouts"])
}

// TestConnectionPool_Stats tests pool statistics tracking
func TestConnectionPool_Stats(t *testing.T) {
	log := logger.WithField("test", "stats")
	socketPath := "/tmp/state-stats.sock"

	pool := NewConnectionPool(socketPath, 5, log)
	defer pool.Close()

	// Get initial stats
	stats := pool.Stats()
	if stats["pool_size"].(int) != 5 {
		t.Errorf("Expected pool size 5, got %d", stats["pool_size"])
	}

	if stats["total_conns"].(int32) != 0 {
		t.Errorf("Expected 0 total connections initially, got %d", stats["total_conns"])
	}
}

// TestConnectionPool_Lifecycle tests pool creation and cleanup
func TestConnectionPool_Lifecycle(t *testing.T) {
	log := logger.WithField("test", "lifecycle")
	socketPath := "/tmp/state-lifecycle.sock"

	pool := NewConnectionPool(socketPath, 10, log)

	// Check initial state
	if pool.closed.Load() {
		t.Error("Pool should not be closed initially")
	}

	// Close pool
	err := pool.Close()
	if err != nil {
		t.Errorf("Error closing pool: %v", err)
	}

	// Check closed state
	if !pool.closed.Load() {
		t.Error("Pool should be closed after Close()")
	}

	// Try to get connection from closed pool
	ctx := context.Background()
	_, err = pool.Get(ctx)
	if err == nil {
		t.Error("Expected error when getting connection from closed pool")
	}
}
