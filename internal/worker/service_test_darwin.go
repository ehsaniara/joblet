//go:build darwin

package worker

import (
	"context"
	"runtime"
	"testing"
	"time"

	"job-worker/internal/worker/interfaces/interfacesfakes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker_DarwinMockBehavior(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin-specific test")
	}

	fakeStore := &interfacesfakes.FakeStore{}

	w := New(fakeStore)

	ctx := context.Background()
	job, err := w.StartJob(ctx, "echo", []string{"hello from macOS"}, 50, 100, 1000)

	require.NoError(t, err)
	assert.NotNil(t, job)
	assert.Contains(t, job.Command, "echo")
	assert.Equal(t, []string{"hello from macOS"}, job.Args)

	// On macOS, we should get a mock PID
	assert.True(t, job.Pid >= 12345, "Should get mock PID >= 12345")

	// Job should complete automatically after a few seconds (simulated)
	time.Sleep(6 * time.Second)

	// Check if job was updated to completed status
	updateCallCount := fakeStore.UpdateJobCallCount()
	assert.GreaterOrEqual(t, updateCallCount, 2, "Should have at least 2 updates (running + completed)")
}

func TestWorker_DarwinResourceDefaults(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	w := New(fakeStore)

	ctx := context.Background()
	job, err := w.StartJob(ctx, "test-command", []string{}, 0, 0, 0)

	require.NoError(t, err)

	// Should apply default resource limits even on macOS
	assert.NotZero(t, job.Limits.MaxCPU)
	assert.NotZero(t, job.Limits.MaxMemory)
	assert.NotZero(t, job.Limits.MaxIOBPS)
}

func TestWorker_DarwinStopJob(t *testing.T) {
	fakeStore := &interfacesfakes.FakeStore{}

	w := New(fakeStore)

	// Start a job first
	ctx := context.Background()
	job, err := w.StartJob(ctx, "long-running-command", []string{}, 25, 256, 500)
	require.NoError(t, err)

	// Stop the job
	err = w.StopJob(ctx, job.Id)
	assert.NoError(t, err)

	// Verify the job was updated
	assert.GreaterOrEqual(t, fakeStore.UpdateJobCallCount(), 2)
}
