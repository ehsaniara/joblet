package telemetry_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	"github.com/ehsaniara/joblet/internal/joblet/telemetry/telemetryfakes"
)

func TestNewCollector(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		c := telemetry.NewCollector(0)
		if c == nil {
			t.Error("expected non-nil collector")
		}
	})

	t.Run("custom buffer size", func(t *testing.T) {
		c := telemetry.NewCollector(500)
		if c == nil {
			t.Error("expected non-nil collector")
		}
	})
}

func TestCollector_Emit(t *testing.T) {
	c := telemetry.NewCollector(10)

	t.Run("emit nil event", func(t *testing.T) {
		c.Emit(nil) // Should not panic
	})

	t.Run("emit event with empty job ID", func(t *testing.T) {
		c.Emit(&telemetry.Event{JobUUID: ""}) // Should not panic, event ignored
	})

	t.Run("emit valid event", func(t *testing.T) {
		jobID := "test-job-1"
		c.Emit(&telemetry.Event{
			JobUUID:   jobID,
			Type:      telemetry.EventTypeMetrics,
			Timestamp: time.Now(),
			Data:      &telemetry.MetricsData{CPUPercent: 50.0},
		})

		events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})
}

func TestCollector_EmitExec_WithPersister(t *testing.T) {
	c := telemetry.NewCollector(10)
	persister := &telemetryfakes.FakeEventPersister{}
	c.SetPersister(persister)

	jobID := "test-job-exec"
	data := &telemetry.ExecData{
		PID:    1234,
		PPID:   1,
		Binary: "/bin/bash",
		Args:   []string{"-c", "echo hello"},
	}

	c.EmitExec(jobID, data)

	// Check event was buffered
	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 1 {
		t.Errorf("expected 1 buffered event, got %d", len(events))
	}
	if events[0].Type != telemetry.EventTypeExec {
		t.Errorf("expected event type exec, got %s", events[0].Type)
	}

	// Check event was persisted
	if persister.PersistExecEventCallCount() != 1 {
		t.Errorf("expected 1 persisted event, got %d", persister.PersistExecEventCallCount())
	}
	persistedJobID, _, persistedSequence, persistedData := persister.PersistExecEventArgsForCall(0)
	if persistedJobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, persistedJobID)
	}
	if persistedData.PID != 1234 {
		t.Errorf("expected PID 1234, got %d", persistedData.PID)
	}
	if persistedSequence != 1 {
		t.Errorf("expected sequence 1, got %d", persistedSequence)
	}
}

func TestCollector_EmitConnect_WithPersister(t *testing.T) {
	c := telemetry.NewCollector(10)
	persister := &telemetryfakes.FakeEventPersister{}
	c.SetPersister(persister)

	jobID := "test-job-connect"
	data := &telemetry.ConnectData{
		PID:      5678,
		Address:  "8.8.8.8",
		Port:     443,
		Protocol: "tcp",
	}

	c.EmitConnect(jobID, data)

	// Check event was buffered
	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 1 {
		t.Errorf("expected 1 buffered event, got %d", len(events))
	}
	if events[0].Type != telemetry.EventTypeConnect {
		t.Errorf("expected event type connect, got %s", events[0].Type)
	}

	// Check event was persisted
	if persister.PersistConnectEventCallCount() != 1 {
		t.Errorf("expected 1 persisted event, got %d", persister.PersistConnectEventCallCount())
	}
	persistedJobID, _, _, persistedData := persister.PersistConnectEventArgsForCall(0)
	if persistedJobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, persistedJobID)
	}
	if persistedData.Address != "8.8.8.8" {
		t.Errorf("expected address 8.8.8.8, got %s", persistedData.Address)
	}
}

func TestCollector_EmitWithoutPersister(t *testing.T) {
	c := telemetry.NewCollector(10)
	// No persister set

	jobID := "test-job-no-persister"
	c.EmitExec(jobID, &telemetry.ExecData{PID: 1})
	c.EmitConnect(jobID, &telemetry.ConnectData{PID: 2})

	// Events should still be buffered
	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 2 {
		t.Errorf("expected 2 buffered events, got %d", len(events))
	}
}

func TestCollector_SequenceIncrement(t *testing.T) {
	c := telemetry.NewCollector(10)
	persister := &telemetryfakes.FakeEventPersister{}
	c.SetPersister(persister)

	jobID := "test-job-seq"

	// Emit multiple events
	for i := 0; i < 5; i++ {
		c.EmitExec(jobID, &telemetry.ExecData{PID: uint32(i)})
	}

	callCount := persister.PersistExecEventCallCount()
	if callCount != 5 {
		t.Fatalf("expected 5 persisted events, got %d", callCount)
	}

	// Check sequences are incrementing
	for i := 0; i < callCount; i++ {
		_, _, sequence, _ := persister.PersistExecEventArgsForCall(i)
		expectedSeq := uint64(i + 1)
		if sequence != expectedSeq {
			t.Errorf("event %d: expected sequence %d, got %d", i, expectedSeq, sequence)
		}
	}
}

func TestCollector_BufferOverflow(t *testing.T) {
	c := telemetry.NewCollector(3) // Small buffer

	jobID := "test-job-overflow"
	for i := 0; i < 5; i++ {
		c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: float64(i)})
	}

	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 3 {
		t.Errorf("expected 3 events (buffer limit), got %d", len(events))
	}

	// Should have the last 3 events (2, 3, 4)
	for i, e := range events {
		data := e.Data.(*telemetry.MetricsData)
		expected := float64(i + 2)
		if data.CPUPercent != expected {
			t.Errorf("event %d: expected CPU %f, got %f", i, expected, data.CPUPercent)
		}
	}
}

func TestCollector_Stream(t *testing.T) {
	c := telemetry.NewCollector(10)

	jobID := "test-job-stream"
	// Pre-buffer some events
	c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: 10})
	c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: 20})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var received []*telemetry.Event
	var mu sync.Mutex

	// Start streaming in goroutine
	done := make(chan error)
	go func() {
		err := c.Stream(ctx, jobID, nil, func(e *telemetry.Event) error {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
			return nil
		})
		done <- err
	}()

	// Give time for buffered events to be sent
	time.Sleep(10 * time.Millisecond)

	// Emit a new event
	c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: 30})

	// Wait for stream to end
	<-done

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 events, got %d", count)
	}
}

func TestCollector_ClearJob(t *testing.T) {
	c := telemetry.NewCollector(10)

	jobID := "test-job-clear"
	c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: 50})

	if c.JobCount() != 1 {
		t.Errorf("expected 1 job, got %d", c.JobCount())
	}

	c.ClearJob(jobID)

	if c.JobCount() != 0 {
		t.Errorf("expected 0 jobs after clear, got %d", c.JobCount())
	}

	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if events != nil {
		t.Errorf("expected nil events after clear, got %v", events)
	}
}

func TestCollector_EventFilter(t *testing.T) {
	c := telemetry.NewCollector(10)

	jobID := "test-job-filter"
	c.EmitMetrics(jobID, &telemetry.MetricsData{CPUPercent: 50})
	c.EmitExec(jobID, &telemetry.ExecData{PID: 1})
	c.EmitConnect(jobID, &telemetry.ConnectData{PID: 2})

	t.Run("filter exec only", func(t *testing.T) {
		filter := &telemetry.EventFilter{Types: []telemetry.EventType{telemetry.EventTypeExec}}
		events := c.GetBufferedEvents(jobID, filter, time.Time{}, time.Time{}, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 exec event, got %d", len(events))
		}
		if events[0].Type != telemetry.EventTypeExec {
			t.Errorf("expected exec type, got %s", events[0].Type)
		}
	})

	t.Run("filter multiple types", func(t *testing.T) {
		filter := &telemetry.EventFilter{Types: []telemetry.EventType{telemetry.EventTypeExec, telemetry.EventTypeConnect}}
		events := c.GetBufferedEvents(jobID, filter, time.Time{}, time.Time{}, 0)
		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
	})

	t.Run("no filter", func(t *testing.T) {
		events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})
}

func TestEventFilter_Matches(t *testing.T) {
	execEvent := &telemetry.Event{Type: telemetry.EventTypeExec}
	connectEvent := &telemetry.Event{Type: telemetry.EventTypeConnect}
	metricsEvent := &telemetry.Event{Type: telemetry.EventTypeMetrics}

	t.Run("empty filter matches all", func(t *testing.T) {
		filter := &telemetry.EventFilter{}
		if !filter.Matches(execEvent) {
			t.Error("empty filter should match exec")
		}
		if !filter.Matches(connectEvent) {
			t.Error("empty filter should match connect")
		}
	})

	t.Run("single type filter", func(t *testing.T) {
		filter := &telemetry.EventFilter{Types: []telemetry.EventType{telemetry.EventTypeExec}}
		if !filter.Matches(execEvent) {
			t.Error("filter should match exec")
		}
		if filter.Matches(connectEvent) {
			t.Error("filter should not match connect")
		}
	})

	t.Run("multi type filter", func(t *testing.T) {
		filter := &telemetry.EventFilter{Types: []telemetry.EventType{telemetry.EventTypeExec, telemetry.EventTypeMetrics}}
		if !filter.Matches(execEvent) {
			t.Error("filter should match exec")
		}
		if !filter.Matches(metricsEvent) {
			t.Error("filter should match metrics")
		}
		if filter.Matches(connectEvent) {
			t.Error("filter should not match connect")
		}
	})
}

func TestParseEventTypes(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := telemetry.ParseEventTypes(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("valid types", func(t *testing.T) {
		result := telemetry.ParseEventTypes([]string{"exec", "connect", "metrics", "file"})
		if len(result) != 4 {
			t.Errorf("expected 4 types, got %d", len(result))
		}
	})

	t.Run("invalid types ignored", func(t *testing.T) {
		result := telemetry.ParseEventTypes([]string{"exec", "invalid", "connect"})
		if len(result) != 2 {
			t.Errorf("expected 2 types, got %d", len(result))
		}
	})
}
