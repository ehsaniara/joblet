package ipc

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/state/internal/storage"
	"github.com/ehsaniara/joblet/state/internal/storage/storagefakes"
)

func TestServer_Start(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-ipc-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)

	err := server.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Verify socket exists
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect to socket: %v", err)
	}
	conn.Close()
}

func TestServer_CreateOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-create-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)

	err := server.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Prepare test job
	testJob := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo test",
		Status:  domain.JobStatus("PENDING"),
	}

	// Send create message
	msg := Message{
		Operation: OpCreate,
		Job:       testJob,
		RequestID: "req-001",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	_, err = conn.Write(data)
	if err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var response Response
	err = decoder.Decode(&response)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response
	if !response.Success {
		t.Errorf("expected success=true, got success=false, error=%s", response.Error)
	}

	if response.RequestID != "req-001" {
		t.Errorf("expected requestID=req-001, got %s", response.RequestID)
	}

	// Verify backend was called
	if backend.CreateCallCount() != 1 {
		t.Errorf("expected Create to be called once, got %d calls", backend.CreateCallCount())
	}

	_, job := backend.CreateArgsForCall(0)
	if job.Uuid != "test-job-123" {
		t.Errorf("expected job UUID=test-job-123, got %s", job.Uuid)
	}
}

func TestServer_UpdateOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-update-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)

	err := server.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	testJob := &domain.Job{
		Uuid:    "test-job-456",
		Command: "echo updated",
		Status:  domain.JobStatus("RUNNING"),
	}

	msg := Message{
		Operation: OpUpdate,
		Job:       testJob,
		RequestID: "req-002",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if !response.Success {
		t.Errorf("expected success, got error: %s", response.Error)
	}

	if backend.UpdateCallCount() != 1 {
		t.Errorf("expected Update to be called once, got %d calls", backend.UpdateCallCount())
	}
}

func TestServer_DeleteOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-delete-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	msg := Message{
		Operation: OpDelete,
		JobID:     "job-to-delete",
		RequestID: "req-003",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if !response.Success {
		t.Errorf("expected success, got error: %s", response.Error)
	}

	if backend.DeleteCallCount() != 1 {
		t.Errorf("expected Delete to be called once, got %d calls", backend.DeleteCallCount())
	}

	_, jobID := backend.DeleteArgsForCall(0)
	if jobID != "job-to-delete" {
		t.Errorf("expected jobID=job-to-delete, got %s", jobID)
	}
}

func TestServer_GetOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-get-" + time.Now().Format("20060102150405") + ".sock"

	// Setup backend to return a job
	returnJob := &domain.Job{
		Uuid:    "retrieved-job",
		Command: "echo retrieved",
		Status:  domain.JobStatus("COMPLETED"),
	}
	backend.GetReturns(returnJob, nil)

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	msg := Message{
		Operation: OpGet,
		JobID:     "retrieved-job",
		RequestID: "req-004",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if !response.Success {
		t.Errorf("expected success, got error: %s", response.Error)
	}

	if response.Job == nil {
		t.Fatal("expected job in response, got nil")
	}

	if response.Job.Uuid != "retrieved-job" {
		t.Errorf("expected job UUID=retrieved-job, got %s", response.Job.Uuid)
	}
}

func TestServer_ListOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-list-" + time.Now().Format("20060102150405") + ".sock"

	// Setup backend to return jobs
	returnJobs := []*domain.Job{
		{Uuid: "job-1", Status: domain.JobStatus("PENDING")},
		{Uuid: "job-2", Status: domain.JobStatus("RUNNING")},
	}
	backend.ListReturns(returnJobs, nil)

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	msg := Message{
		Operation: OpList,
		Filter: &storage.Filter{
			Status: "RUNNING",
			Limit:  10,
		},
		RequestID: "req-005",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if !response.Success {
		t.Errorf("expected success, got error: %s", response.Error)
	}

	if len(response.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(response.Jobs))
	}

	// Verify filter was passed
	if backend.ListCallCount() != 1 {
		t.Errorf("expected List to be called once, got %d calls", backend.ListCallCount())
	}

	_, filter := backend.ListArgsForCall(0)
	if filter.Status != "RUNNING" {
		t.Errorf("expected filter status=RUNNING, got %s", filter.Status)
	}
}

func TestServer_SyncOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-sync-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	syncJobs := []*domain.Job{
		{Uuid: "sync-job-1", Status: domain.JobStatus("PENDING")},
		{Uuid: "sync-job-2", Status: domain.JobStatus("RUNNING")},
		{Uuid: "sync-job-3", Status: domain.JobStatus("COMPLETED")},
	}

	msg := Message{
		Operation: OpSync,
		Jobs:      syncJobs,
		RequestID: "req-006",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if !response.Success {
		t.Errorf("expected success, got error: %s", response.Error)
	}

	if backend.SyncCallCount() != 1 {
		t.Errorf("expected Sync to be called once, got %d calls", backend.SyncCallCount())
	}

	_, jobs := backend.SyncArgsForCall(0)
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs to sync, got %d", len(jobs))
	}
}

func TestServer_InvalidOperation(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-invalid-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	msg := Message{
		Operation: "invalid-op",
		RequestID: "req-007",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if response.Success {
		t.Error("expected failure for invalid operation")
	}

	if response.Error == "" {
		t.Error("expected error message for invalid operation")
	}
}

func TestServer_MissingJobInCreate(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-no-job-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	msg := Message{
		Operation: OpCreate,
		Job:       nil, // Missing job
		RequestID: "req-008",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if response.Success {
		t.Error("expected failure when job is nil")
	}

	// Backend should not have been called
	if backend.CreateCallCount() != 0 {
		t.Error("expected Create to not be called when job is nil")
	}
}

func TestServer_BackendError(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-backend-err-" + time.Now().Format("20060102150405") + ".sock"

	// Setup backend to return error
	backend.CreateReturns(storage.ErrJobAlreadyExists)

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	testJob := &domain.Job{
		Uuid:    "duplicate-job",
		Command: "echo test",
		Status:  domain.JobStatus("PENDING"),
	}

	msg := Message{
		Operation: OpCreate,
		Job:       testJob,
		RequestID: "req-009",
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	var response Response
	json.NewDecoder(conn).Decode(&response)

	if response.Success {
		t.Error("expected failure when backend returns error")
	}

	if response.Error == "" {
		t.Error("expected error message in response")
	}
}

func TestServer_MultipleConnections(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-multi-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()
	defer server.Stop()

	// Create multiple connections
	conn1, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect 2: %v", err)
	}
	defer conn2.Close()

	// Send messages from both connections
	testJob1 := &domain.Job{Uuid: "job-1", Status: domain.JobStatus("PENDING")}
	testJob2 := &domain.Job{Uuid: "job-2", Status: domain.JobStatus("RUNNING")}

	msg1 := Message{Operation: OpCreate, Job: testJob1, RequestID: "req-1", Timestamp: time.Now().Unix()}
	msg2 := Message{Operation: OpCreate, Job: testJob2, RequestID: "req-2", Timestamp: time.Now().Unix()}

	data1, _ := json.Marshal(msg1)
	data2, _ := json.Marshal(msg2)

	conn1.Write(append(data1, '\n'))
	conn2.Write(append(data2, '\n'))

	var resp1, resp2 Response
	json.NewDecoder(conn1).Decode(&resp1)
	json.NewDecoder(conn2).Decode(&resp2)

	if !resp1.Success || !resp2.Success {
		t.Error("expected both responses to succeed")
	}

	// Both should have been processed
	if backend.CreateCallCount() != 2 {
		t.Errorf("expected 2 Create calls, got %d", backend.CreateCallCount())
	}
}

func TestServer_StopGraceful(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	socketPath := "/tmp/test-state-stop-" + time.Now().Format("20060102150405") + ".sock"

	server := NewServer(socketPath, backend)
	server.Start()

	// Connect to server
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Close connection before stopping server
	conn.Close()

	// Stop server (should close connections gracefully)
	err = server.Stop()
	if err != nil {
		t.Errorf("unexpected error during stop: %v", err)
	}

	// Socket file should be removed
	_, err = net.Dial("unix", socketPath)
	if err == nil {
		t.Error("expected socket to be removed after stop")
	}
}
