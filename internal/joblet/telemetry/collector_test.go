package telemetry

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockPersister is a test mock for EventPersister
type mockPersister struct {
	mu               sync.Mutex
	execEvents       []execEventCall
	connectEvents    []connectEventCall
	metricsEvents    []metricsEventCall
	acceptEvents     []acceptEventCall
	socketDataEvents []socketDataEventCall
	mmapEvents       []mmapEventCall
	mprotectEvents   []mprotectEventCall
	fileEvents       []fileEventCall
	execErr          error
	connectErr       error
	metricsErr       error
}

type execEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *ExecData
}

type connectEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *ConnectData
}

type metricsEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *MetricsData
}

type acceptEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *AcceptData
}

type socketDataEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *SocketDataData
}

type mmapEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *MmapData
}

type mprotectEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *MprotectData
}

type fileEventCall struct {
	jobID     string
	timestamp int64
	sequence  uint64
	data      *FileData
}

func (m *mockPersister) PersistExecEvent(jobID string, timestamp int64, sequence uint64, data *ExecData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execEvents = append(m.execEvents, execEventCall{jobID, timestamp, sequence, data})
	return m.execErr
}

func (m *mockPersister) PersistConnectEvent(jobID string, timestamp int64, sequence uint64, data *ConnectData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectEvents = append(m.connectEvents, connectEventCall{jobID, timestamp, sequence, data})
	return m.connectErr
}

func (m *mockPersister) PersistMetrics(jobID string, timestamp int64, sequence uint64, data *MetricsData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsEvents = append(m.metricsEvents, metricsEventCall{jobID, timestamp, sequence, data})
	return m.metricsErr
}

func (m *mockPersister) PersistAcceptEvent(jobID string, timestamp int64, sequence uint64, data *AcceptData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acceptEvents = append(m.acceptEvents, acceptEventCall{jobID, timestamp, sequence, data})
	return nil
}

func (m *mockPersister) PersistSocketDataEvent(jobID string, timestamp int64, sequence uint64, data *SocketDataData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.socketDataEvents = append(m.socketDataEvents, socketDataEventCall{jobID, timestamp, sequence, data})
	return nil
}

func (m *mockPersister) PersistMmapEvent(jobID string, timestamp int64, sequence uint64, data *MmapData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mmapEvents = append(m.mmapEvents, mmapEventCall{jobID, timestamp, sequence, data})
	return nil
}

func (m *mockPersister) PersistMprotectEvent(jobID string, timestamp int64, sequence uint64, data *MprotectData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mprotectEvents = append(m.mprotectEvents, mprotectEventCall{jobID, timestamp, sequence, data})
	return nil
}

func (m *mockPersister) PersistFileEvent(jobID string, timestamp int64, sequence uint64, data *FileData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileEvents = append(m.fileEvents, fileEventCall{jobID, timestamp, sequence, data})
	return nil
}

func (m *mockPersister) getExecEvents() []execEventCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]execEventCall{}, m.execEvents...)
}

func (m *mockPersister) getConnectEvents() []connectEventCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]connectEventCall{}, m.connectEvents...)
}

func TestNewCollector(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		c := NewCollector(0)
		if c.bufferSize != 100000 {
			t.Errorf("expected default buffer size 100000, got %d", c.bufferSize)
		}
	})

	t.Run("custom buffer size", func(t *testing.T) {
		c := NewCollector(500)
		if c.bufferSize != 500 {
			t.Errorf("expected buffer size 500, got %d", c.bufferSize)
		}
	})
}

func TestCollector_Emit(t *testing.T) {
	c := NewCollector(10)

	t.Run("emit nil event", func(t *testing.T) {
		c.Emit(nil) // Should not panic
	})

	t.Run("emit event with empty job ID", func(t *testing.T) {
		c.Emit(&Event{JobUUID: ""}) // Should not panic, event ignored
	})

	t.Run("emit valid event", func(t *testing.T) {
		jobID := "test-job-1"
		c.Emit(&Event{
			JobUUID:   jobID,
			Type:      EventTypeMetrics,
			Timestamp: time.Now(),
			Data:      &MetricsData{CPUPercent: 50.0},
		})

		events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})
}

func TestCollector_EmitExec_WithPersister(t *testing.T) {
	c := NewCollector(10)
	persister := &mockPersister{}
	c.SetPersister(persister)

	jobID := "test-job-exec"
	data := &ExecData{
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
	if events[0].Type != EventTypeExec {
		t.Errorf("expected event type exec, got %s", events[0].Type)
	}

	// Check event was persisted
	persisted := persister.getExecEvents()
	if len(persisted) != 1 {
		t.Errorf("expected 1 persisted event, got %d", len(persisted))
	}
	if persisted[0].jobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, persisted[0].jobID)
	}
	if persisted[0].data.PID != 1234 {
		t.Errorf("expected PID 1234, got %d", persisted[0].data.PID)
	}
	if persisted[0].sequence != 1 {
		t.Errorf("expected sequence 1, got %d", persisted[0].sequence)
	}
}

func TestCollector_EmitConnect_WithPersister(t *testing.T) {
	c := NewCollector(10)
	persister := &mockPersister{}
	c.SetPersister(persister)

	jobID := "test-job-connect"
	data := &ConnectData{
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
	if events[0].Type != EventTypeConnect {
		t.Errorf("expected event type connect, got %s", events[0].Type)
	}

	// Check event was persisted
	persisted := persister.getConnectEvents()
	if len(persisted) != 1 {
		t.Errorf("expected 1 persisted event, got %d", len(persisted))
	}
	if persisted[0].jobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, persisted[0].jobID)
	}
	if persisted[0].data.Address != "8.8.8.8" {
		t.Errorf("expected address 8.8.8.8, got %s", persisted[0].data.Address)
	}
}

func TestCollector_EmitWithoutPersister(t *testing.T) {
	c := NewCollector(10)
	// No persister set

	jobID := "test-job-no-persister"
	c.EmitExec(jobID, &ExecData{PID: 1})
	c.EmitConnect(jobID, &ConnectData{PID: 2})

	// Events should still be buffered
	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 2 {
		t.Errorf("expected 2 buffered events, got %d", len(events))
	}
}

func TestCollector_SequenceIncrement(t *testing.T) {
	c := NewCollector(10)
	persister := &mockPersister{}
	c.SetPersister(persister)

	jobID := "test-job-seq"

	// Emit multiple events
	for i := 0; i < 5; i++ {
		c.EmitExec(jobID, &ExecData{PID: uint32(i)})
	}

	persisted := persister.getExecEvents()
	if len(persisted) != 5 {
		t.Fatalf("expected 5 persisted events, got %d", len(persisted))
	}

	// Check sequences are incrementing
	for i, e := range persisted {
		expectedSeq := uint64(i + 1)
		if e.sequence != expectedSeq {
			t.Errorf("event %d: expected sequence %d, got %d", i, expectedSeq, e.sequence)
		}
	}
}

func TestCollector_BufferOverflow(t *testing.T) {
	c := NewCollector(3) // Small buffer

	jobID := "test-job-overflow"
	for i := 0; i < 5; i++ {
		c.EmitMetrics(jobID, &MetricsData{CPUPercent: float64(i)})
	}

	events := c.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 0)
	if len(events) != 3 {
		t.Errorf("expected 3 events (buffer limit), got %d", len(events))
	}

	// Should have the last 3 events (2, 3, 4)
	for i, e := range events {
		data := e.Data.(*MetricsData)
		expected := float64(i + 2)
		if data.CPUPercent != expected {
			t.Errorf("event %d: expected CPU %f, got %f", i, expected, data.CPUPercent)
		}
	}
}

func TestCollector_Stream(t *testing.T) {
	c := NewCollector(10)

	jobID := "test-job-stream"
	// Pre-buffer some events
	c.EmitMetrics(jobID, &MetricsData{CPUPercent: 10})
	c.EmitMetrics(jobID, &MetricsData{CPUPercent: 20})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var received []*Event
	var mu sync.Mutex

	// Start streaming in goroutine
	done := make(chan error)
	go func() {
		err := c.Stream(ctx, jobID, nil, func(e *Event) error {
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
	c.EmitMetrics(jobID, &MetricsData{CPUPercent: 30})

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
	c := NewCollector(10)

	jobID := "test-job-clear"
	c.EmitMetrics(jobID, &MetricsData{CPUPercent: 50})

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
	c := NewCollector(10)

	jobID := "test-job-filter"
	c.EmitMetrics(jobID, &MetricsData{CPUPercent: 50})
	c.EmitExec(jobID, &ExecData{PID: 1})
	c.EmitConnect(jobID, &ConnectData{PID: 2})

	t.Run("filter exec only", func(t *testing.T) {
		filter := &EventFilter{Types: []EventType{EventTypeExec}}
		events := c.GetBufferedEvents(jobID, filter, time.Time{}, time.Time{}, 0)
		if len(events) != 1 {
			t.Errorf("expected 1 exec event, got %d", len(events))
		}
		if events[0].Type != EventTypeExec {
			t.Errorf("expected exec type, got %s", events[0].Type)
		}
	})

	t.Run("filter multiple types", func(t *testing.T) {
		filter := &EventFilter{Types: []EventType{EventTypeExec, EventTypeConnect}}
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
	execEvent := &Event{Type: EventTypeExec}
	connectEvent := &Event{Type: EventTypeConnect}
	metricsEvent := &Event{Type: EventTypeMetrics}

	t.Run("empty filter matches all", func(t *testing.T) {
		filter := &EventFilter{}
		if !filter.Matches(execEvent) {
			t.Error("empty filter should match exec")
		}
		if !filter.Matches(connectEvent) {
			t.Error("empty filter should match connect")
		}
	})

	t.Run("single type filter", func(t *testing.T) {
		filter := &EventFilter{Types: []EventType{EventTypeExec}}
		if !filter.Matches(execEvent) {
			t.Error("filter should match exec")
		}
		if filter.Matches(connectEvent) {
			t.Error("filter should not match connect")
		}
	})

	t.Run("multi type filter", func(t *testing.T) {
		filter := &EventFilter{Types: []EventType{EventTypeExec, EventTypeMetrics}}
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
		result := ParseEventTypes(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("valid types", func(t *testing.T) {
		result := ParseEventTypes([]string{"exec", "connect", "metrics", "file"})
		if len(result) != 4 {
			t.Errorf("expected 4 types, got %d", len(result))
		}
	})

	t.Run("invalid types ignored", func(t *testing.T) {
		result := ParseEventTypes([]string{"exec", "invalid", "connect"})
		if len(result) != 2 {
			t.Errorf("expected 2 types, got %d", len(result))
		}
	})
}
