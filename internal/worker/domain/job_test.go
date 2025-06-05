package domain

import (
	"testing"
)

func TestJobStateTransitions(t *testing.T) {
	job := &Job{
		Id:      "test-1",
		Command: "echo",
		Args:    []string{"hello"},
		Status:  StatusInitializing,
	}

	// test valid transition: INITIALIZING -> RUNNING
	err := job.MarkAsRunning(1234)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if job.Status != StatusRunning {
		t.Errorf("Expected status RUNNING, got %v", job.Status)
	}

	// test invalid transition: RUNNING -> RUNNING
	err = job.MarkAsRunning(5678)
	if err == nil {
		t.Error("Expected error for invalid state transition")
	}

	// test valid transition: RUNNING -> COMPLETED
	err = job.Complete(0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if job.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %v", job.Status)
	}
	if job.EndTime == nil {
		t.Error("Expected end time to be set")
	}
}

func TestJobDeepCopy(t *testing.T) {
	original := &Job{
		Id:      "test-1",
		Command: "echo",
		Args:    []string{"hello", "world"},
		Status:  StatusRunning,
	}

	cp := original.DeepCopy()

	// change the original
	original.Args[0] = "goodbye"
	original.Status = StatusCompleted

	// check copy is not affected
	if cp.Args[0] != "hello" {
		t.Error("Deep copy failed: args slice was not properly copied")
	}
	if cp.Status != StatusRunning {
		t.Error("Deep copy failed: status was not properly copied")
	}
}
