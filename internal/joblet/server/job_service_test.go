package server

import (
	"context"
	"testing"

	pb "joblet/api/gen"
	"joblet/internal/joblet/auth/authfakes"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
)

func TestNewJobServiceServer(t *testing.T) {
	mockAuth := &authfakes.FakeGrpcAuthorization{}
	mockStore := state.New()
	mockJoblet := &MockJoblet{}

	server := NewJobServiceServer(mockAuth, mockStore, mockJoblet)

	if server == nil {
		t.Fatal("NewJobServiceServer returned nil")
	}

	if server.auth != mockAuth {
		t.Error("auth not set correctly")
	}

	if server.jobStore != mockStore {
		t.Error("jobStore not set correctly")
	}

	if server.joblet != mockJoblet {
		t.Error("joblet not set correctly")
	}
}

func TestListJobs_EmptyStore(t *testing.T) {
	mockAuth := &authfakes.FakeGrpcAuthorization{}
	mockStore := state.New()
	mockJoblet := &MockJoblet{}

	server := NewJobServiceServer(mockAuth, mockStore, mockJoblet)

	// Mock successful authorization
	mockAuth.AuthorizedReturns(nil)

	req := &pb.EmptyRequest{}
	resp, err := server.ListJobs(context.Background(), req)

	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if len(resp.Jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestListJobs_WithJobs(t *testing.T) {
	mockAuth := &authfakes.FakeGrpcAuthorization{}
	mockStore := state.New()
	mockJoblet := &MockJoblet{}

	server := NewJobServiceServer(mockAuth, mockStore, mockJoblet)

	// Add a test job to the store
	testJob := &domain.Job{
		Id:      "test-job-1",
		Command: "echo",
		Args:    []string{"hello"},
		Status:  domain.StatusCompleted,
		Limits:  *domain.NewResourceLimits(),
	}
	mockStore.CreateNewJob(testJob)

	// Mock successful authorization
	mockAuth.AuthorizedReturns(nil)

	req := &pb.EmptyRequest{}
	resp, err := server.ListJobs(context.Background(), req)

	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if len(resp.Jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(resp.Jobs))
	}

	if len(resp.Jobs) > 0 {
		job := resp.Jobs[0]
		if job.Id != "test-job-1" {
			t.Errorf("Expected job ID 'test-job-1', got '%s'", job.Id)
		}
		if job.Command != "echo" {
			t.Errorf("Expected command 'echo', got '%s'", job.Command)
		}
	}
}

// MockJoblet implements a minimal Joblet interface for testing
type MockJoblet struct{}

func (m *MockJoblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	return nil, nil
}

func (m *MockJoblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	return nil
}

func (m *MockJoblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	return nil
}
