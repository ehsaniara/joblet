package job

import (
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestUUIDGeneratorBasic(t *testing.T) {
	generator := NewUUIDGenerator("", "")

	uuid := generator.Next()

	// Verify UUID format (36 characters with hyphens at positions 8, 13, 18, 23)
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		t.Errorf("Generated UUID does not match expected format: %s", uuid)
	}

	if len(uuid) != 36 {
		t.Errorf("Expected UUID length 36, got %d", len(uuid))
	}
}

func TestUUIDGeneratorUniqueness(t *testing.T) {
	generator := NewUUIDGenerator("", "")

	// Generate multiple UUIDs and verify they are unique
	uuids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		uuid := generator.Next()
		if uuids[uuid] {
			t.Errorf("Duplicate UUID generated: %s", uuid)
		}
		uuids[uuid] = true
	}
}

func TestUUIDGeneratorConcurrency(t *testing.T) {
	generator := NewUUIDGenerator("", "")

	const goroutines = 100
	const uuidsPerGoroutine = 100

	uuids := make(chan string, goroutines*uuidsPerGoroutine)
	var wg sync.WaitGroup

	// Launch multiple goroutines generating UUIDs concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < uuidsPerGoroutine; j++ {
				uuid := generator.Next()
				uuids <- uuid
			}
		}()
	}

	wg.Wait()
	close(uuids)

	// Collect all UUIDs and verify uniqueness
	uniqueUUIDs := make(map[string]bool)
	totalCount := 0

	for uuid := range uuids {
		if uniqueUUIDs[uuid] {
			t.Errorf("Duplicate UUID found in concurrent test: %s", uuid)
		}
		uniqueUUIDs[uuid] = true
		totalCount++
	}

	expectedCount := goroutines * uuidsPerGoroutine
	if totalCount != expectedCount {
		t.Errorf("Expected %d UUIDs, got %d", expectedCount, totalCount)
	}
}

func TestUUIDGeneratorPerformance(t *testing.T) {
	generator := NewUUIDGenerator("", "")

	const iterations = 10000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		generator.Next()
	}

	duration := time.Since(start)
	avgTime := duration / iterations

	// Should be much faster than 100µs per UUID
	if avgTime > 100*time.Microsecond {
		t.Logf("Warning: UUID generation seems slow: %v per UUID", avgTime)
	}

	t.Logf("Generated %d UUIDs in %v (avg: %v per UUID)", iterations, duration, avgTime)
}

func TestGeneratorCleanup(t *testing.T) {
	generator := NewUUIDGenerator("", "")

	// Generate a UUID to potentially open file descriptors
	generator.Next()

	// Should close without error
	err := generator.Close()
	if err != nil {
		t.Errorf("Unexpected error closing generator: %v", err)
	}
}

func BenchmarkUUIDGeneration(b *testing.B) {
	generator := NewUUIDGenerator("", "")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		generator.Next()
	}
}

func BenchmarkConcurrentUUIDGeneration(b *testing.B) {
	generator := NewUUIDGenerator("", "")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			generator.Next()
		}
	})
}
