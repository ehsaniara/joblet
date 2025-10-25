package storage

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// TestJobToItem_BasicFields tests the jobToItem conversion function
func TestJobToItem_BasicFields(t *testing.T) {
	job := &domain.Job{
		Uuid:      "test-uuid",
		Status:    domain.JobStatus("RUNNING"),
		Command:   "echo test",
		NodeId:    "node-1",
		StartTime: time.Date(2025, 10, 26, 12, 0, 0, 0, time.UTC),
		ExitCode:  0,
		Pid:       12345,
		Network:   "bridge",
		Runtime:   "python-3.11",
	}

	item := jobToItem(job, 30)

	// Verify required fields
	if v, ok := item["jobId"].(*types.AttributeValueMemberS); !ok || v.Value != "test-uuid" {
		t.Error("expected jobId to be set correctly")
	}
	if v, ok := item["jobStatus"].(*types.AttributeValueMemberS); !ok || v.Value != "RUNNING" {
		t.Error("expected jobStatus to be RUNNING")
	}
	if v, ok := item["command"].(*types.AttributeValueMemberS); !ok || v.Value != "echo test" {
		t.Error("expected command to be set correctly")
	}
	if v, ok := item["nodeId"].(*types.AttributeValueMemberS); !ok || v.Value != "node-1" {
		t.Error("expected nodeId to be set correctly")
	}

	// Verify timestamp
	if v, ok := item["startTime"].(*types.AttributeValueMemberS); !ok {
		t.Error("expected startTime to be set")
	} else {
		parsed, err := time.Parse(time.RFC3339, v.Value)
		if err != nil {
			t.Errorf("failed to parse startTime: %v", err)
		}
		if !parsed.Equal(job.StartTime) {
			t.Errorf("expected startTime %v, got %v", job.StartTime, parsed)
		}
	}

	// Verify optional fields
	if v, ok := item["pid"].(*types.AttributeValueMemberN); !ok || v.Value != "12345" {
		t.Error("expected pid to be 12345")
	}
	if v, ok := item["network"].(*types.AttributeValueMemberS); !ok || v.Value != "bridge" {
		t.Error("expected network to be bridge")
	}
	if v, ok := item["runtime"].(*types.AttributeValueMemberS); !ok || v.Value != "python-3.11" {
		t.Error("expected runtime to be python-3.11")
	}
}

// TestJobToItem_TTL tests TTL attribute generation
func TestJobToItem_TTL(t *testing.T) {
	// Completed job should get TTL
	completedJob := &domain.Job{
		Uuid:      "completed-job",
		Status:    domain.JobStatus("COMPLETED"),
		Command:   "echo done",
		StartTime: time.Now(),
	}

	item := jobToItem(completedJob, 30)

	// Verify TTL is set for completed job
	if _, ok := item["expiresAt"].(*types.AttributeValueMemberN); !ok {
		t.Error("expected expiresAt TTL to be set for completed job")
	}

	// Running job should NOT get TTL
	runningJob := &domain.Job{
		Uuid:      "running-job",
		Status:    domain.JobStatus("RUNNING"),
		Command:   "echo running",
		StartTime: time.Now(),
	}

	item2 := jobToItem(runningJob, 30)

	// Verify TTL is NOT set for running job
	if _, ok := item2["expiresAt"]; ok {
		t.Error("expected expiresAt TTL to NOT be set for running job")
	}

	// TTL disabled (0 days)
	item3 := jobToItem(completedJob, 0)
	if _, ok := item3["expiresAt"]; ok {
		t.Error("expected expiresAt TTL to NOT be set when ttlDays=0")
	}
}

// TestItemToJob tests the itemToJob conversion function
func TestItemToJob(t *testing.T) {
	now := time.Now()
	item := map[string]types.AttributeValue{
		"jobId":         &types.AttributeValueMemberS{Value: "test-job"},
		"jobStatus":     &types.AttributeValueMemberS{Value: "COMPLETED"},
		"command":       &types.AttributeValueMemberS{Value: "echo test"},
		"nodeId":        &types.AttributeValueMemberS{Value: "node-1"},
		"startTime":     &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		"endTime":       &types.AttributeValueMemberS{Value: now.Add(5 * time.Second).Format(time.RFC3339)},
		"exitCode":      &types.AttributeValueMemberN{Value: "0"},
		"pid":           &types.AttributeValueMemberN{Value: "99999"},
		"network":       &types.AttributeValueMemberS{Value: "host"},
		"runtime":       &types.AttributeValueMemberS{Value: "openjdk-21"},
		"scheduledTime": &types.AttributeValueMemberS{Value: now.Add(-1 * time.Minute).Format(time.RFC3339)},
	}

	job, err := itemToJob(item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all fields
	if job.Uuid != "test-job" {
		t.Errorf("expected UUID test-job, got %s", job.Uuid)
	}
	if job.Status != domain.JobStatus("COMPLETED") {
		t.Errorf("expected status COMPLETED, got %s", job.Status)
	}
	if job.Command != "echo test" {
		t.Errorf("expected command 'echo test', got %s", job.Command)
	}
	if job.NodeId != "node-1" {
		t.Errorf("expected nodeId node-1, got %s", job.NodeId)
	}
	if job.ExitCode != 0 {
		t.Errorf("expected exitCode 0, got %d", job.ExitCode)
	}
	if job.Pid != 99999 {
		t.Errorf("expected pid 99999, got %d", job.Pid)
	}
	if job.Network != "host" {
		t.Errorf("expected network host, got %s", job.Network)
	}
	if job.Runtime != "openjdk-21" {
		t.Errorf("expected runtime openjdk-21, got %s", job.Runtime)
	}

	// Verify timestamps
	if job.StartTime.IsZero() {
		t.Error("expected startTime to be set")
	}
	if job.EndTime == nil || job.EndTime.IsZero() {
		t.Error("expected endTime to be set")
	}
	if job.ScheduledTime == nil || job.ScheduledTime.IsZero() {
		t.Error("expected scheduledTime to be set")
	}
}

// TestItemToJob_MinimalFields tests conversion with minimal fields
func TestItemToJob_MinimalFields(t *testing.T) {
	// Test with only required fields
	item := map[string]types.AttributeValue{
		"jobId":     &types.AttributeValueMemberS{Value: "minimal-job"},
		"jobStatus": &types.AttributeValueMemberS{Value: "PENDING"},
		"command":   &types.AttributeValueMemberS{Value: "echo minimal"},
		"nodeId":    &types.AttributeValueMemberS{Value: "node-1"},
	}

	job, err := itemToJob(item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Uuid != "minimal-job" {
		t.Errorf("expected UUID minimal-job, got %s", job.Uuid)
	}
	if job.Status != domain.JobStatus("PENDING") {
		t.Errorf("expected status PENDING, got %s", job.Status)
	}

	// Verify optional fields are zero values
	if job.ExitCode != 0 {
		t.Errorf("expected exitCode 0 for minimal job, got %d", job.ExitCode)
	}
	if job.EndTime != nil {
		t.Error("expected endTime to be nil for minimal job")
	}
}
