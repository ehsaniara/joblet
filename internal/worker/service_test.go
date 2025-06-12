package worker

//
//import (
//	"context"
//	"job-worker/internal/worker/interfaces"
//	"testing"
//	"time"
//
//	"job-worker/internal/worker/domain"
//	"job-worker/internal/worker/interfaces/interfacesfakes"
//	_ "job-worker/pkg/logger"
//
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//)
//
//func TestNew(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	assert.NotNil(t, w)
//	_, ok := w.(interfaces.JobWorker)
//	assert.True(t, ok)
//}
//
//func TestStartJob_Basic(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	job, err := w.StartJob(ctx, "echo", []string{"hello", "world"}, 50, 100, 1000)
//
//	require.NoError(t, err)
//	assert.NotNil(t, job)
//	assert.Equal(t, "echo", job.Command)
//	assert.Equal(t, []string{"hello", "world"}, job.Args)
//	assert.Equal(t, int32(50), job.Limits.MaxCPU)
//	assert.Equal(t, int32(100), job.Limits.MaxMemory)
//	assert.Equal(t, int32(1000), job.Limits.MaxIOBPS)
//	assert.Equal(t, domain.StatusRunning, job.Status)
//	assert.NotZero(t, job.Pid)
//
//	// Verify store interactions
//	assert.Equal(t, 1, fakeStore.CreateNewJobCallCount())
//	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
//}
//
//func TestStartJob_DefaultValues(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	// Pass 0 values to test defaults
//	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 0, 0, 0)
//
//	require.NoError(t, err)
//	assert.NotZero(t, job.Limits.MaxCPU)    // Should get default
//	assert.NotZero(t, job.Limits.MaxMemory) // Should get default
//	assert.NotZero(t, job.Limits.MaxIOBPS)  // Should get default
//}
//
//func TestStopJob_Basic(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	// Create a mock running job
//	runningJob := &domain.Job{
//		Id:     "test-job",
//		Status: domain.StatusRunning,
//		Pid:    123,
//	}
//	fakeStore.GetJobReturns(runningJob, true)
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	err := w.StopJob(ctx, "test-job")
//
//	assert.NoError(t, err)
//	assert.Equal(t, 1, fakeStore.UpdateJobCallCount())
//}
//
//func TestStopJob_JobNotFound(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	// Job doesn't exist
//	fakeStore.GetJobReturns(nil, false)
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	err := w.StopJob(ctx, "nonexistent-job")
//
//	assert.Error(t, err)
//	assert.Contains(t, err.Error(), "job not found: nonexistent-job")
//}
//
//func TestStopJob_JobNotRunning(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	// Job exists but is not running
//	completedJob := &domain.Job{
//		Id:     "test-job",
//		Status: domain.StatusCompleted,
//		Pid:    123,
//	}
//	fakeStore.GetJobReturns(completedJob, true)
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	err := w.StopJob(ctx, "test-job")
//
//	assert.Error(t, err)
//	assert.Contains(t, err.Error(), "job is not running")
//}
//
//func TestStartJob_ContextCancelled(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	ctx, cancel := context.WithCancel(context.Background())
//	cancel() // Cancel immediately
//
//	job, err := w.StartJob(ctx, "echo", []string{"hello"}, 50, 100, 1000)
//
//	assert.Error(t, err)
//	assert.Nil(t, job)
//	assert.Equal(t, context.Canceled, err)
//}
//
//func TestStopJob_ContextCancelled(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	ctx, cancel := context.WithCancel(context.Background())
//	cancel() // Cancel immediately
//
//	err := w.StopJob(ctx, "test-job")
//
//	assert.Error(t, err)
//	assert.Equal(t, context.Canceled, err)
//}
//
//// Test platform-specific behavior
//func TestWorker_PlatformBehavior(t *testing.T) {
//	fakeStore := &interfacesfakes.FakeStore{}
//
//	w := New(fakeStore)
//
//	ctx := context.Background()
//	job, err := w.StartJob(ctx, "ls", []string{"-la"}, 25, 512, 1000)
//
//	require.NoError(t, err)
//	assert.NotNil(t, job)
//
//	// Job should be in running state
//	assert.Equal(t, domain.StatusRunning, job.Status)
//
//	// Should have a PID assigned
//	assert.NotZero(t, job.Pid)
//
//	// Give some time for any background processing
//	time.Sleep(100 * time.Millisecond)
//
//	// Try to stop the job
//	err = w.StopJob(ctx, job.Id)
//	assert.NoError(t, err)
//}
