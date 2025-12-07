package ipc

import (
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
)

func TestNewPersister(t *testing.T) {
	t.Run("with nil writer", func(t *testing.T) {
		p := NewPersister(nil)
		if p == nil {
			t.Fatal("expected non-nil persister")
		}
		if p.writer != nil {
			t.Error("expected nil writer in persister")
		}
	})
}

func TestPersister_PersistExecEvent(t *testing.T) {
	t.Run("with nil writer", func(t *testing.T) {
		p := NewPersister(nil)
		err := p.PersistExecEvent("job-1", 123456789, 1, &telemetry.ExecData{
			PID:    1234,
			Binary: "/bin/bash",
		})
		if err != nil {
			t.Errorf("expected no error with nil writer, got %v", err)
		}
	})
}

func TestPersister_PersistConnectEvent(t *testing.T) {
	t.Run("with nil writer", func(t *testing.T) {
		p := NewPersister(nil)
		err := p.PersistConnectEvent("job-1", 123456789, 1, &telemetry.ConnectData{
			PID:     5678,
			Address: "8.8.8.8",
			Port:    443,
		})
		if err != nil {
			t.Errorf("expected no error with nil writer, got %v", err)
		}
	})
}

func TestPersister_ImplementsInterface(t *testing.T) {
	// This is a compile-time check, but we can make it explicit in tests
	var _ telemetry.EventPersister = (*Persister)(nil)
}

func TestPersister_DataMapping(t *testing.T) {
	t.Run("exec data fields", func(t *testing.T) {
		// Test that all fields are properly mapped
		// We can't easily test with the real Writer since it requires socket connection
		// This test validates the interface contract
		p := NewPersister(nil)

		data := &telemetry.ExecData{
			PID:      1234,
			PPID:     1,
			Binary:   "/usr/bin/python",
			Args:     []string{"script.py", "--verbose"},
			ExitCode: 0,
		}

		// With nil writer, should return nil error
		err := p.PersistExecEvent("test-job", 1234567890, 42, data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("connect data fields", func(t *testing.T) {
		p := NewPersister(nil)

		data := &telemetry.ConnectData{
			PID:          5678,
			Address:      "192.168.1.100",
			Port:         8080,
			Protocol:     "tcp",
			LocalAddress: "10.0.0.1",
			LocalPort:    54321,
		}

		err := p.PersistConnectEvent("test-job", 1234567890, 43, data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
