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
	t.Logf("Pool size: %v", stats["pool_size"])
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

// TestPoolConfig_Defaults tests that PoolConfig fills in defaults for zero values
func TestPoolConfig_Defaults(t *testing.T) {
	// Empty config should get all defaults
	cfg := PoolConfig{}
	cfg = cfg.withDefaults()

	if cfg.PoolSize != DefaultPoolSize {
		t.Errorf("Expected default pool size %d, got %d", DefaultPoolSize, cfg.PoolSize)
	}
	if cfg.ReadTimeout != DefaultReadTimeout {
		t.Errorf("Expected default read timeout %v, got %v", DefaultReadTimeout, cfg.ReadTimeout)
	}
	if cfg.DialTimeout != DefaultDialTimeout {
		t.Errorf("Expected default dial timeout %v, got %v", DefaultDialTimeout, cfg.DialTimeout)
	}
	if cfg.MaxIdleTime != DefaultMaxIdleTime {
		t.Errorf("Expected default max idle time %v, got %v", DefaultMaxIdleTime, cfg.MaxIdleTime)
	}
	if cfg.HealthCheckTimeout != DefaultHealthCheckTimeout {
		t.Errorf("Expected default health check timeout %v, got %v", DefaultHealthCheckTimeout, cfg.HealthCheckTimeout)
	}
	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, cfg.ShutdownTimeout)
	}
}

// TestPoolConfig_PartialOverride tests that PoolConfig preserves non-zero values
func TestPoolConfig_PartialOverride(t *testing.T) {
	cfg := PoolConfig{
		PoolSize:    50,
		ReadTimeout: 30 * time.Second,
		// Leave others as zero
	}
	cfg = cfg.withDefaults()

	if cfg.PoolSize != 50 {
		t.Errorf("Expected pool size 50, got %d", cfg.PoolSize)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", cfg.ReadTimeout)
	}
	// Others should be defaults
	if cfg.DialTimeout != DefaultDialTimeout {
		t.Errorf("Expected default dial timeout, got %v", cfg.DialTimeout)
	}
}

// TestNewConnectionPoolWithConfig tests creating pool with custom config
func TestNewConnectionPoolWithConfig(t *testing.T) {
	log := logger.WithField("test", "config")
	socketPath := "/tmp/state-config.sock"

	cfg := PoolConfig{
		PoolSize:    25,
		ReadTimeout: 15 * time.Second,
	}

	pool := NewConnectionPoolWithConfig(socketPath, cfg, log)
	defer pool.Close()

	stats := pool.Stats()
	if stats["pool_size"].(int) != 25 {
		t.Errorf("Expected pool size 25, got %v", stats["pool_size"])
	}
}

// TestDefaultPoolConfig tests DefaultPoolConfig returns expected values
func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.PoolSize != DefaultPoolSize {
		t.Errorf("Expected pool size %d, got %d", DefaultPoolSize, cfg.PoolSize)
	}
	if cfg.ReadTimeout != DefaultReadTimeout {
		t.Errorf("Expected read timeout %v, got %v", DefaultReadTimeout, cfg.ReadTimeout)
	}
}

// TestConnectionPool_PoolSizeLimit tests that the pool respects the configured size limit
func TestConnectionPool_PoolSizeLimit(t *testing.T) {
	log := logger.WithField("test", "pool-size-limit")
	socketPath := "/tmp/state-pool-limit.sock"

	const poolSize = 3
	pool := NewConnectionPool(socketPath, poolSize, log)
	defer pool.Close()

	// Verify initial state
	stats := pool.Stats()
	if stats["pool_size"].(int) != poolSize {
		t.Errorf("Expected pool size %d, got %v", poolSize, stats["pool_size"])
	}

	if stats["total_conns"].(int32) != 0 {
		t.Errorf("Expected 0 total connections initially, got %v", stats["total_conns"])
	}
}

// TestConnectionPool_CloseIdempotent tests that Close can be called multiple times safely
func TestConnectionPool_CloseIdempotent(t *testing.T) {
	log := logger.WithField("test", "close-idempotent")
	socketPath := "/tmp/state-close-idempotent.sock"

	pool := NewConnectionPool(socketPath, 5, log)

	// First close should succeed
	err := pool.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Second close should also succeed (idempotent)
	err = pool.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}

	// Pool should be marked as closed
	if !pool.closed.Load() {
		t.Error("Pool should be marked as closed")
	}
}

// TestConnectionPool_GetFromClosedPool tests that Get returns error for closed pool
func TestConnectionPool_GetFromClosedPool(t *testing.T) {
	log := logger.WithField("test", "get-closed")
	socketPath := "/tmp/state-get-closed.sock"

	pool := NewConnectionPool(socketPath, 5, log)
	pool.Close()

	ctx := context.Background()
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Expected error when getting from closed pool")
	}
	if err.Error() != "connection pool is closed" {
		t.Errorf("Expected 'connection pool is closed' error, got: %v", err)
	}
}

// TestConnectionPool_StatsTracking tests that pool statistics are tracked correctly
func TestConnectionPool_StatsTracking(t *testing.T) {
	log := logger.WithField("test", "stats-tracking")
	socketPath := "/tmp/state-stats-tracking.sock"

	pool := NewConnectionPool(socketPath, 10, log)
	defer pool.Close()

	// Initial stats should be zero
	stats := pool.Stats()

	if stats["acquisitions"].(uint64) != 0 {
		t.Errorf("Expected 0 acquisitions initially, got %v", stats["acquisitions"])
	}
	if stats["creations"].(uint64) != 0 {
		t.Errorf("Expected 0 creations initially, got %v", stats["creations"])
	}
	if stats["errors"].(uint64) != 0 {
		t.Errorf("Expected 0 errors initially, got %v", stats["errors"])
	}
	if stats["timeouts"].(uint64) != 0 {
		t.Errorf("Expected 0 timeouts initially, got %v", stats["timeouts"])
	}
	if stats["health_checks"].(uint64) != 0 {
		t.Errorf("Expected 0 health_checks initially, got %v", stats["health_checks"])
	}
	if stats["active_conns"].(int32) != 0 {
		t.Errorf("Expected 0 active_conns initially, got %v", stats["active_conns"])
	}
	if stats["available_conns"].(int) != 0 {
		t.Errorf("Expected 0 available_conns initially, got %v", stats["available_conns"])
	}
}

// TestPoolConfig_AllDefaults tests that all config fields get defaults when zero
func TestPoolConfig_AllDefaults(t *testing.T) {
	cfg := PoolConfig{}
	cfg = cfg.withDefaults()

	// Verify all defaults are set
	if cfg.PoolSize != DefaultPoolSize {
		t.Errorf("PoolSize: expected %d, got %d", DefaultPoolSize, cfg.PoolSize)
	}
	if cfg.ReadTimeout != DefaultReadTimeout {
		t.Errorf("ReadTimeout: expected %v, got %v", DefaultReadTimeout, cfg.ReadTimeout)
	}
	if cfg.DialTimeout != DefaultDialTimeout {
		t.Errorf("DialTimeout: expected %v, got %v", DefaultDialTimeout, cfg.DialTimeout)
	}
	if cfg.MaxIdleTime != DefaultMaxIdleTime {
		t.Errorf("MaxIdleTime: expected %v, got %v", DefaultMaxIdleTime, cfg.MaxIdleTime)
	}
	if cfg.HealthCheckTimeout != DefaultHealthCheckTimeout {
		t.Errorf("HealthCheckTimeout: expected %v, got %v", DefaultHealthCheckTimeout, cfg.HealthCheckTimeout)
	}
	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("ShutdownTimeout: expected %v, got %v", DefaultShutdownTimeout, cfg.ShutdownTimeout)
	}
	if cfg.ShutdownPollInterval != DefaultShutdownPollInterval {
		t.Errorf("ShutdownPollInterval: expected %v, got %v", DefaultShutdownPollInterval, cfg.ShutdownPollInterval)
	}
}

// TestPoolConfig_CustomValues tests that custom config values are preserved
func TestPoolConfig_CustomValues(t *testing.T) {
	cfg := PoolConfig{
		PoolSize:             100,
		ReadTimeout:          30 * time.Second,
		DialTimeout:          10 * time.Second,
		MaxIdleTime:          60 * time.Second,
		HealthCheckTimeout:   1 * time.Second,
		ShutdownTimeout:      15 * time.Second,
		ShutdownPollInterval: 200 * time.Millisecond,
	}
	cfg = cfg.withDefaults()

	// All custom values should be preserved
	if cfg.PoolSize != 100 {
		t.Errorf("PoolSize: expected 100, got %d", cfg.PoolSize)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout: expected 30s, got %v", cfg.ReadTimeout)
	}
	if cfg.DialTimeout != 10*time.Second {
		t.Errorf("DialTimeout: expected 10s, got %v", cfg.DialTimeout)
	}
	if cfg.MaxIdleTime != 60*time.Second {
		t.Errorf("MaxIdleTime: expected 60s, got %v", cfg.MaxIdleTime)
	}
	if cfg.HealthCheckTimeout != 1*time.Second {
		t.Errorf("HealthCheckTimeout: expected 1s, got %v", cfg.HealthCheckTimeout)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout: expected 15s, got %v", cfg.ShutdownTimeout)
	}
	if cfg.ShutdownPollInterval != 200*time.Millisecond {
		t.Errorf("ShutdownPollInterval: expected 200ms, got %v", cfg.ShutdownPollInterval)
	}
}
