package server

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	pb "worker/api/gen"
	"worker/internal/worker/auth"
	"worker/internal/worker/auth/authfakes"
	"worker/internal/worker/domain"
	"worker/internal/worker/interfaces/interfacesfakes"
)

func TestJobServiceServer_RunJob_Success(t *testing.T) {
	// Setup mocks using counterfeiter
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mock behavior
	fakeAuth.AuthorizedReturns(nil) // Allow operation

	expectedJob := &domain.Job{
		Id:      "test-job-1",
		Command: "echo",
		Args:    []string{"hello"},
		Limits: domain.ResourceLimits{
			MaxCPU:    100,
			MaxMemory: 512,
			MaxIOBPS:  1000,
		},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
	}

	fakeWorker.StartJobReturns(expectedJob, nil)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.RunJobReq{
		Command:   "echo",
		Args:      []string{"hello"},
		MaxCPU:    100,
		MaxMemory: 512,
		MaxIOBPS:  1000,
	}

	resp, err := server.RunJob(context.Background(), req)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Id != expectedJob.Id {
		t.Errorf("Expected job ID %v, got %v", expectedJob.Id, resp.Id)
	}
	if resp.Command != req.Command {
		t.Errorf("Expected command %v, got %v", req.Command, resp.Command)
	}

	// Verify mock interactions
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}

	ctx, operation := fakeAuth.AuthorizedArgsForCall(0)
	if ctx == nil {
		t.Error("Expected context to be passed to Authorized")
	}
	if operation != auth.RunJobOp {
		t.Errorf("Expected operation %v, got %v", auth.RunJobOp, operation)
	}

	if fakeWorker.StartJobCallCount() != 1 {
		t.Errorf("Expected 1 call to StartJob, got %d", fakeWorker.StartJobCallCount())
	}

	startCtx, command, args, maxCPU, maxMemory, maxIOBPS := fakeWorker.StartJobArgsForCall(0)
	if startCtx == nil {
		t.Error("Expected context to be passed to StartJob")
	}
	if command != req.Command {
		t.Errorf("Expected command %v, got %v", req.Command, command)
	}
	if len(args) != len(req.Args) || args[0] != req.Args[0] {
		t.Errorf("Expected args %v, got %v", req.Args, args)
	}
	if maxCPU != req.MaxCPU {
		t.Errorf("Expected maxCPU %v, got %v", req.MaxCPU, maxCPU)
	}
	if maxMemory != req.MaxMemory {
		t.Errorf("Expected maxMemory %v, got %v", req.MaxMemory, maxMemory)
	}
	if maxIOBPS != req.MaxIOBPS {
		t.Errorf("Expected maxIOBPS %v, got %v", req.MaxIOBPS, maxIOBPS)
	}
}

func TestJobServiceServer_RunJob_AuthorizationFailure(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure auth to fail
	authError := status.Error(codes.PermissionDenied, "not authorized")
	fakeAuth.AuthorizedReturns(authError)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.RunJobReq{
		Command: "echo",
		Args:    []string{"hello"},
	}

	_, err := server.RunJob(context.Background(), req)

	if err == nil {
		t.Fatal("Expected authorization error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("Expected PermissionDenied, got %v", st.Code())
	}

	// Verify auth was called but worker was not
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}
	if fakeWorker.StartJobCallCount() != 0 {
		t.Errorf("Expected 0 calls to StartJob after auth failure, got %d", fakeWorker.StartJobCallCount())
	}
}

func TestJobServiceServer_RunJob_WorkerFailure(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil) // Allow operation
	workerError := errors.New("worker failed to start job")
	fakeWorker.StartJobReturns(nil, workerError)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.RunJobReq{
		Command: "echo",
		Args:    []string{"hello"},
	}

	_, err := server.RunJob(context.Background(), req)

	if err == nil {
		t.Fatal("Expected worker error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal error, got %v", st.Code())
	}

	// Verify both auth and worker were called
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}
	if fakeWorker.StartJobCallCount() != 1 {
		t.Errorf("Expected 1 call to StartJob, got %d", fakeWorker.StartJobCallCount())
	}
}

func TestJobServiceServer_GetJobStatus_Success(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)

	testJob := &domain.Job{
		Id:        "test-job-1",
		Command:   "echo",
		Args:      []string{"hello"},
		Status:    domain.StatusCompleted,
		ExitCode:  0,
		StartTime: time.Now(),
	}

	fakeStore.GetJobReturns(testJob, true)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.GetJobStatusReq{Id: "test-job-1"}

	resp, err := server.GetJobStatus(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Id != testJob.Id {
		t.Errorf("Expected ID %v, got %v", testJob.Id, resp.Id)
	}
	if resp.Status != string(testJob.Status) {
		t.Errorf("Expected status %v, got %v", string(testJob.Status), resp.Status)
	}
	if resp.ExitCode != testJob.ExitCode {
		t.Errorf("Expected exit code %v, got %v", testJob.ExitCode, resp.ExitCode)
	}

	// Verify mock interactions
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}

	_, operation := fakeAuth.AuthorizedArgsForCall(0)
	if operation != auth.GetJobOp {
		t.Errorf("Expected operation %v, got %v", auth.GetJobOp, operation)
	}

	if fakeStore.GetJobCallCount() != 1 {
		t.Errorf("Expected 1 call to GetJob, got %d", fakeStore.GetJobCallCount())
	}

	jobId := fakeStore.GetJobArgsForCall(0)
	if jobId != req.Id {
		t.Errorf("Expected job ID %v, got %v", req.Id, jobId)
	}
}

func TestJobServiceServer_GetJobStatus_NotFound(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)
	fakeStore.GetJobReturns(nil, false) // Job not found

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.GetJobStatusReq{Id: "non-existent"}

	_, err := server.GetJobStatus(context.Background(), req)

	if err == nil {
		t.Fatal("Expected not found error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("Expected NotFound, got %v", st.Code())
	}

	// Verify store was called
	if fakeStore.GetJobCallCount() != 1 {
		t.Errorf("Expected 1 call to GetJob, got %d", fakeStore.GetJobCallCount())
	}
}

func TestJobServiceServer_StopJob_Success(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)
	fakeWorker.StopJobReturns(nil) // Stop succeeds

	stoppedJob := &domain.Job{
		Id:        "test-job-1",
		Command:   "sleep",
		Status:    domain.StatusStopped,
		ExitCode:  -1,
		StartTime: time.Now(),
	}

	fakeStore.GetJobReturns(stoppedJob, true)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.StopJobReq{Id: "test-job-1"}

	resp, err := server.StopJob(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Id != stoppedJob.Id {
		t.Errorf("Expected ID %v, got %v", stoppedJob.Id, resp.Id)
	}
	if resp.Status != string(stoppedJob.Status) {
		t.Errorf("Expected status %v, got %v", string(stoppedJob.Status), resp.Status)
	}

	// Verify mock interactions
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}

	_, operation := fakeAuth.AuthorizedArgsForCall(0)
	if operation != auth.StopJobOp {
		t.Errorf("Expected operation %v, got %v", auth.StopJobOp, operation)
	}

	if fakeWorker.StopJobCallCount() != 1 {
		t.Errorf("Expected 1 call to StopJob, got %d", fakeWorker.StopJobCallCount())
	}

	stopCtx, jobId := fakeWorker.StopJobArgsForCall(0)
	if stopCtx == nil {
		t.Error("Expected context to be passed to StopJob")
	}
	if jobId != req.Id {
		t.Errorf("Expected job ID %v, got %v", req.Id, jobId)
	}

	if fakeStore.GetJobCallCount() != 1 {
		t.Errorf("Expected 1 call to GetJob, got %d", fakeStore.GetJobCallCount())
	}
}

func TestJobServiceServer_StopJob_WorkerError(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)
	stopError := errors.New("failed to stop job")
	fakeWorker.StopJobReturns(stopError)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	req := &pb.StopJobReq{Id: "test-job-1"}

	_, err := server.StopJob(context.Background(), req)

	if err == nil {
		t.Fatal("Expected stop error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal error, got %v", st.Code())
	}

	// Verify worker was called
	if fakeWorker.StopJobCallCount() != 1 {
		t.Errorf("Expected 1 call to StopJob, got %d", fakeWorker.StopJobCallCount())
	}
}

func TestJobServiceServer_ListJobs_Success(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)

	testJobs := []*domain.Job{
		{
			Id:        "job-1",
			Command:   "echo",
			Status:    domain.StatusRunning,
			StartTime: time.Now(),
		},
		{
			Id:        "job-2",
			Command:   "ls",
			Status:    domain.StatusCompleted,
			StartTime: time.Now(),
		},
	}

	fakeStore.ListJobsReturns(testJobs)

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	resp, err := server.ListJobs(context.Background(), &pb.EmptyRequest{})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(resp.Jobs) != len(testJobs) {
		t.Errorf("Expected %d jobs, got %d", len(testJobs), len(resp.Jobs))
	}

	// Verify job IDs are present
	jobIds := make(map[string]bool)
	for _, job := range resp.Jobs {
		jobIds[job.Id] = true
	}

	for _, expectedJob := range testJobs {
		if !jobIds[expectedJob.Id] {
			t.Errorf("Expected job %v to be in list", expectedJob.Id)
		}
	}

	// Verify mock interactions
	if fakeAuth.AuthorizedCallCount() != 1 {
		t.Errorf("Expected 1 call to Authorized, got %d", fakeAuth.AuthorizedCallCount())
	}

	_, operation := fakeAuth.AuthorizedArgsForCall(0)
	if operation != auth.ListJobsOp {
		t.Errorf("Expected operation %v, got %v", auth.ListJobsOp, operation)
	}

	if fakeStore.ListJobsCallCount() != 1 {
		t.Errorf("Expected 1 call to ListJobs, got %d", fakeStore.ListJobsCallCount())
	}
}

func TestJobServiceServer_ListJobs_Empty(t *testing.T) {
	fakeAuth := &authfakes.FakeGrpcAuthorization{}
	fakeStore := &interfacesfakes.FakeStore{}
	fakeWorker := &interfacesfakes.FakeJobWorker{}

	// Configure mocks
	fakeAuth.AuthorizedReturns(nil)
	fakeStore.ListJobsReturns([]*domain.Job{}) // Empty list

	server := NewJobServiceServer(fakeAuth, fakeStore, fakeWorker)

	resp, err := server.ListJobs(context.Background(), &pb.EmptyRequest{})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(resp.Jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(resp.Jobs))
	}

	if fakeStore.ListJobsCallCount() != 1 {
		t.Errorf("Expected 1 call to ListJobs, got %d", fakeStore.ListJobsCallCount())
	}
}
