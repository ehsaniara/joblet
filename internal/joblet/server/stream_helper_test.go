package server

import (
	"context"
	"errors"
	"testing"

	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDetermineJobState(t *testing.T) {
	tests := []struct {
		name        string
		exists      bool
		isCompleted bool
		expected    JobState
	}{
		{
			name:        "job not found",
			exists:      false,
			isCompleted: false,
			expected:    JobStateNotFound,
		},
		{
			name:        "job not found but marked completed",
			exists:      false,
			isCompleted: true,
			expected:    JobStateNotFound,
		},
		{
			name:        "job running",
			exists:      true,
			isCompleted: false,
			expected:    JobStateRunning,
		},
		{
			name:        "job completed",
			exists:      true,
			isCompleted: true,
			expected:    JobStateCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineJobState(tt.exists, tt.isCompleted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStreamWithHistory_CompletedJob(t *testing.T) {
	log := logger.WithField("test", "completed-job")

	historicalCount := 0
	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			historicalCount++
			return 10, nil
		},
		QueryPersistOnly: func() (int, error) {
			t.Fatal("QueryPersistOnly should not be called for completed job")
			return 0, nil
		},
		StreamLive: func() error {
			t.Fatal("StreamLive should not be called for completed job")
			return nil
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateCompleted)
	require.NoError(t, err)
	assert.Equal(t, 1, historicalCount, "SendHistorical should be called once")
}

func TestStreamWithHistory_CompletedJob_HistoricalError(t *testing.T) {
	log := logger.WithField("test", "completed-job-error")

	expectedErr := errors.New("historical data error")
	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			return 0, expectedErr
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateCompleted)
	assert.ErrorIs(t, err, expectedErr)
}

func TestStreamWithHistory_NotFoundJob_WithPersistData(t *testing.T) {
	log := logger.WithField("test", "not-found-job")

	persistCount := 0
	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			t.Fatal("SendHistorical should not be called for not found job")
			return 0, nil
		},
		QueryPersistOnly: func() (int, error) {
			persistCount++
			return 5, nil // Found 5 events in persist
		},
		StreamLive: func() error {
			t.Fatal("StreamLive should not be called for not found job")
			return nil
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateNotFound)
	require.NoError(t, err)
	assert.Equal(t, 1, persistCount, "QueryPersistOnly should be called once")
}

func TestStreamWithHistory_NotFoundJob_NoPersistData(t *testing.T) {
	log := logger.WithField("test", "not-found-no-data")

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		QueryPersistOnly: func() (int, error) {
			return 0, nil // No events found
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateNotFound)
	require.Error(t, err)

	// Should return NotFound error
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "test-job-123")
}

func TestStreamWithHistory_NotFoundJob_NoPersistHandler(t *testing.T) {
	log := logger.WithField("test", "not-found-no-handler")

	cfg := StreamConfig{
		JobUUID:          "test-job-123",
		Logger:           log,
		QueryPersistOnly: nil, // No persist handler configured
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateNotFound)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestStreamWithHistory_RunningJob(t *testing.T) {
	log := logger.WithField("test", "running-job")

	historicalCalled := false
	liveCalled := false

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			historicalCalled = true
			return 10, nil
		},
		QueryPersistOnly: func() (int, error) {
			t.Fatal("QueryPersistOnly should not be called for running job")
			return 0, nil
		},
		StreamLive: func() error {
			liveCalled = true
			return nil
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateRunning)
	require.NoError(t, err)
	assert.True(t, historicalCalled, "SendHistorical should be called")
	assert.True(t, liveCalled, "StreamLive should be called")
}

func TestStreamWithHistory_RunningJob_HistoricalFailsContinuesToLive(t *testing.T) {
	log := logger.WithField("test", "running-job-historical-fails")

	liveCalled := false

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			return 0, errors.New("historical error")
		},
		StreamLive: func() error {
			liveCalled = true
			return nil
		},
	}

	// Even if historical fails, live streaming should continue
	err := StreamWithHistory(context.Background(), cfg, JobStateRunning)
	require.NoError(t, err)
	assert.True(t, liveCalled, "StreamLive should be called even if historical fails")
}

func TestStreamWithHistory_RunningJob_NoLiveHandler(t *testing.T) {
	log := logger.WithField("test", "running-job-no-live")

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			return 10, nil
		},
		StreamLive: nil, // No live handler
	}

	// Should not error, just skip live streaming
	err := StreamWithHistory(context.Background(), cfg, JobStateRunning)
	require.NoError(t, err)
}

func TestStreamWithHistory_RunningJob_LiveError(t *testing.T) {
	log := logger.WithField("test", "running-job-live-error")

	expectedErr := errors.New("live stream error")
	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			return 10, nil
		},
		StreamLive: func() error {
			return expectedErr
		},
	}

	err := StreamWithHistory(context.Background(), cfg, JobStateRunning)
	assert.ErrorIs(t, err, expectedErr)
}

func TestStreamWithHistory_UnknownState(t *testing.T) {
	log := logger.WithField("test", "unknown-state")

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
	}

	// Use an invalid state value
	err := StreamWithHistory(context.Background(), cfg, JobState(99))
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "unknown job state")
}

func TestJobStateConstants(t *testing.T) {
	// Verify the state constants have distinct values
	assert.NotEqual(t, JobStateRunning, JobStateCompleted)
	assert.NotEqual(t, JobStateRunning, JobStateNotFound)
	assert.NotEqual(t, JobStateCompleted, JobStateNotFound)

	// Verify the expected values (iota starts at 0)
	assert.Equal(t, JobState(0), JobStateRunning)
	assert.Equal(t, JobState(1), JobStateCompleted)
	assert.Equal(t, JobState(2), JobStateNotFound)
}

func TestStreamWithHistory_ContextCancellation(t *testing.T) {
	log := logger.WithField("test", "context-cancellation")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := StreamConfig{
		JobUUID: "test-job-123",
		Logger:  log,
		SendHistorical: func() (int, error) {
			return 0, ctx.Err()
		},
	}

	err := StreamWithHistory(ctx, cfg, JobStateCompleted)
	assert.ErrorIs(t, err, context.Canceled)
}
