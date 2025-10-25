package adapters

import (
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestWriteToBuffer_PersistEnabled verifies that logs ARE buffered when persist is enabled
func TestWriteToBuffer_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log) // persistClient=nil, stateClient=nil, persistEnabled = true
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-123"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write to buffer
	testData := []byte("test log chunk")
	jobStoreAdapter.WriteToBuffer(jobID, testData)

	// Verify: Data should be in buffer (persist enabled)
	chunks := buffer.ReadAll()
	assert.Equal(t, 1, len(chunks), "Buffer should contain 1 chunk when persist enabled")
	assert.Equal(t, testData, chunks[0], "Buffered data should match written data")
}

// TestWriteToBuffer_PersistDisabled verifies that logs are NOT buffered when persist is disabled
func TestWriteToBuffer_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, false, log) // persistClient=nil, stateClient=nil, persistEnabled = false
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-456"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write to buffer
	testData := []byte("test log chunk")
	jobStoreAdapter.WriteToBuffer(jobID, testData)

	// Verify: Data should NOT be in buffer (persist disabled)
	chunks := buffer.ReadAll()
	assert.Equal(t, 0, len(chunks), "Buffer should be empty when persist disabled (no buffering)")
}

// TestWriteToBuffer_MultipleWrites_PersistEnabled verifies multiple writes are buffered
func TestWriteToBuffer_MultipleWrites_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log) // persistClient=nil, stateClient=nil, persistEnabled = true
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-789"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write multiple chunks
	testData1 := []byte("chunk 1")
	testData2 := []byte("chunk 2")
	testData3 := []byte("chunk 3")

	jobStoreAdapter.WriteToBuffer(jobID, testData1)
	jobStoreAdapter.WriteToBuffer(jobID, testData2)
	jobStoreAdapter.WriteToBuffer(jobID, testData3)

	// Verify: All chunks should be in buffer
	chunks := buffer.ReadAll()
	assert.Equal(t, 3, len(chunks), "Buffer should contain 3 chunks when persist enabled")
	assert.Equal(t, testData1, chunks[0])
	assert.Equal(t, testData2, chunks[1])
	assert.Equal(t, testData3, chunks[2])
}

// TestWriteToBuffer_MultipleWrites_PersistDisabled verifies multiple writes skip buffering
func TestWriteToBuffer_MultipleWrites_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, false, log) // persistClient=nil, stateClient=nil, persistEnabled = false
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-000"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write multiple chunks
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 1"))
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 2"))
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 3"))

	// Verify: Buffer should remain empty (all writes skipped)
	chunks := buffer.ReadAll()
	assert.Equal(t, 0, len(chunks), "Buffer should remain empty when persist disabled (no buffering)")
}
