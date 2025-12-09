package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleLogBuffer_Write(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	err := buffer.Write([]byte("line 1\n"))
	require.NoError(t, err)

	err = buffer.Write([]byte("line 2\n"))
	require.NoError(t, err)

	assert.Equal(t, 2, buffer.Size())
}

func TestSimpleLogBuffer_ReadAll(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))
	_ = buffer.Write([]byte("line 3\n"))

	data := buffer.ReadAll()
	require.Len(t, data, 3)
	assert.Equal(t, "line 1\n", string(data[0]))
	assert.Equal(t, "line 2\n", string(data[1]))
	assert.Equal(t, "line 3\n", string(data[2]))
}

func TestSimpleLogBuffer_ReadAll_ReturnsCopy(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("original"))

	data := buffer.ReadAll()
	require.Len(t, data, 1)

	// Modify the returned data
	data[0][0] = 'X'

	// Original should be unchanged
	newData := buffer.ReadAll()
	assert.Equal(t, "original", string(newData[0]))
}

func TestSimpleLogBuffer_ReadAfterSkip(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	// Add 5 log chunks
	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))
	_ = buffer.Write([]byte("line 3\n"))
	_ = buffer.Write([]byte("line 4\n"))
	_ = buffer.Write([]byte("line 5\n"))

	// Skip first 2 items (simulating persist already sent them)
	data := buffer.ReadAfterSkip(2)
	require.Len(t, data, 3)
	assert.Equal(t, "line 3\n", string(data[0]))
	assert.Equal(t, "line 4\n", string(data[1]))
	assert.Equal(t, "line 5\n", string(data[2]))
}

func TestSimpleLogBuffer_ReadAfterSkip_SkipAll(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))
	_ = buffer.Write([]byte("line 3\n"))

	// Skip all items
	data := buffer.ReadAfterSkip(3)
	assert.Len(t, data, 0)
}

func TestSimpleLogBuffer_ReadAfterSkip_SkipMoreThanAvailable(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))

	// Skip more than available
	data := buffer.ReadAfterSkip(10)
	assert.Len(t, data, 0)
}

func TestSimpleLogBuffer_ReadAfterSkip_SkipZero(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))

	// Skip 0 should return all
	data := buffer.ReadAfterSkip(0)
	require.Len(t, data, 2)
	assert.Equal(t, "line 1\n", string(data[0]))
	assert.Equal(t, "line 2\n", string(data[1]))
}

func TestSimpleLogBuffer_ReadAfterSkip_EmptyBuffer(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	data := buffer.ReadAfterSkip(0)
	assert.Len(t, data, 0)

	data = buffer.ReadAfterSkip(5)
	assert.Len(t, data, 0)
}

func TestSimpleLogBuffer_ReadAfterSkip_ReturnsCopy(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))
	_ = buffer.Write([]byte("line 3\n"))

	data := buffer.ReadAfterSkip(1)
	require.Len(t, data, 2)

	// Modify the returned data
	data[0][0] = 'X'

	// Original should be unchanged
	newData := buffer.ReadAfterSkip(1)
	assert.Equal(t, "line 2\n", string(newData[0]))
}

func TestSimpleLogBuffer_Clear(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	_ = buffer.Write([]byte("line 1\n"))
	_ = buffer.Write([]byte("line 2\n"))
	assert.Equal(t, 2, buffer.Size())

	buffer.Clear()
	assert.Equal(t, 0, buffer.Size())

	data := buffer.ReadAll()
	assert.Len(t, data, 0)
}

func TestSimpleLogBuffer_Size(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	assert.Equal(t, 0, buffer.Size())

	_ = buffer.Write([]byte("line 1\n"))
	assert.Equal(t, 1, buffer.Size())

	_ = buffer.Write([]byte("line 2\n"))
	assert.Equal(t, 2, buffer.Size())
}

func TestSimpleLogManager_GetBuffer(t *testing.T) {
	manager := NewSimpleLogManager()

	// Get buffer for a job (creates it)
	buffer1 := manager.GetBuffer("job-1")
	require.NotNil(t, buffer1)

	// Get same buffer again
	buffer2 := manager.GetBuffer("job-1")
	assert.Equal(t, buffer1, buffer2)

	// Get different buffer
	buffer3 := manager.GetBuffer("job-2")
	require.NotNil(t, buffer3)
	assert.NotEqual(t, buffer1, buffer3)
}

func TestSimpleLogManager_RemoveBuffer(t *testing.T) {
	manager := NewSimpleLogManager()

	// Create and write to buffer
	buffer := manager.GetBuffer("job-1")
	_ = buffer.Write([]byte("test data\n"))

	// Remove buffer
	removed := manager.RemoveBuffer("job-1")
	require.NotNil(t, removed)
	assert.Equal(t, 1, removed.Size())

	// Getting buffer again should create a new one
	newBuffer := manager.GetBuffer("job-1")
	assert.Equal(t, 0, newBuffer.Size())
}

func TestSimpleLogManager_RemoveBuffer_NotExists(t *testing.T) {
	manager := NewSimpleLogManager()

	removed := manager.RemoveBuffer("nonexistent")
	assert.Nil(t, removed)
}

func TestSimpleLogManager_ListBuffers(t *testing.T) {
	manager := NewSimpleLogManager()

	// Initially empty
	list := manager.ListBuffers()
	assert.Len(t, list, 0)

	// Add some buffers
	manager.GetBuffer("job-1")
	manager.GetBuffer("job-2")
	manager.GetBuffer("job-3")

	list = manager.ListBuffers()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "job-1")
	assert.Contains(t, list, "job-2")
	assert.Contains(t, list, "job-3")
}

func TestSimpleLogManager_Stats(t *testing.T) {
	manager := NewSimpleLogManager()

	// Initially empty
	stats := manager.Stats()
	assert.Equal(t, 0, stats.ActiveBuffers)
	assert.Equal(t, 0, stats.TotalChunks)

	// Add buffers with data
	buffer1 := manager.GetBuffer("job-1")
	_ = buffer1.Write([]byte("line 1\n"))
	_ = buffer1.Write([]byte("line 2\n"))

	buffer2 := manager.GetBuffer("job-2")
	_ = buffer2.Write([]byte("line 1\n"))

	stats = manager.Stats()
	assert.Equal(t, 2, stats.ActiveBuffers)
	assert.Equal(t, 3, stats.TotalChunks)
}

// Test the gap prevention scenario:
// 1. Persist writes first N lines to disk
// 2. Buffer has all N+M lines
// 3. ReadAfterSkip(N) returns only lines M+1 to N+M
func TestSimpleLogBuffer_GapPreventionScenario(t *testing.T) {
	buffer := NewSimpleLogBuffer("test-job")

	// Simulate job producing 100 log lines
	for i := 1; i <= 100; i++ {
		_ = buffer.Write([]byte("log line\n"))
	}

	// Persist has already sent first 60 lines
	persistCount := 60

	// Get remaining lines from buffer (lines 61-100)
	remaining := buffer.ReadAfterSkip(persistCount)

	// Should have exactly 40 lines (100 - 60)
	assert.Len(t, remaining, 40)
}
