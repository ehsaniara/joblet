package adapters

import (
	"sync"
)

// SimpleLogBuffer replaces the over-engineered buffer system
// Just stores log chunks for jobs without unnecessary abstractions
type SimpleLogBuffer struct {
	jobID string
	data  [][]byte
	mutex sync.RWMutex
}

// NewSimpleLogBuffer creates a basic log buffer for a job
func NewSimpleLogBuffer(jobID string) *SimpleLogBuffer {
	return &SimpleLogBuffer{
		jobID: jobID,
		data:  make([][]byte, 0),
	}
}

// Write appends log data to the buffer
func (b *SimpleLogBuffer) Write(data []byte) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Make a copy to avoid data races
	chunk := make([]byte, len(data))
	copy(chunk, data)
	b.data = append(b.data, chunk)

	return nil
}

// ReadAll returns all buffered data
func (b *SimpleLogBuffer) ReadAll() [][]byte {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Return copy to prevent external modification
	result := make([][]byte, len(b.data))
	for i, chunk := range b.data {
		result[i] = make([]byte, len(chunk))
		copy(result[i], chunk)
	}
	return result
}

// ReadAfterSkip returns buffered data starting after skipCount items
// This is used to avoid duplicates when persist has already sent the first N items
func (b *SimpleLogBuffer) ReadAfterSkip(skipCount int) [][]byte {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// If skip count is greater than or equal to data length, return empty
	if skipCount >= len(b.data) {
		return [][]byte{}
	}

	// Return items after skipCount
	remaining := b.data[skipCount:]
	result := make([][]byte, len(remaining))
	for i, chunk := range remaining {
		result[i] = make([]byte, len(chunk))
		copy(result[i], chunk)
	}
	return result
}

// Size returns the number of log chunks
func (b *SimpleLogBuffer) Size() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.data)
}

// Clear removes all buffered data
func (b *SimpleLogBuffer) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.data = b.data[:0] // Keep capacity but reset length
}

// SimpleLogManager manages log buffers for all jobs
// Replaces complex buffer factory and manager abstractions
type SimpleLogManager struct {
	buffers map[string]*SimpleLogBuffer
	mutex   sync.RWMutex
}

// NewSimpleLogManager creates a simple log manager
func NewSimpleLogManager() *SimpleLogManager {
	return &SimpleLogManager{
		buffers: make(map[string]*SimpleLogBuffer),
	}
}

// GetBuffer returns the log buffer for a job, creating if needed
func (m *SimpleLogManager) GetBuffer(jobID string) *SimpleLogBuffer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	buffer, exists := m.buffers[jobID]
	if !exists {
		buffer = NewSimpleLogBuffer(jobID)
		m.buffers[jobID] = buffer
	}
	return buffer
}

// RemoveBuffer removes and returns the buffer for a job
func (m *SimpleLogManager) RemoveBuffer(jobID string) *SimpleLogBuffer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	buffer, exists := m.buffers[jobID]
	if exists {
		delete(m.buffers, jobID)
	}
	return buffer
}

// ListBuffers returns all active job IDs with buffers
func (m *SimpleLogManager) ListBuffers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	jobIDs := make([]string, 0, len(m.buffers))
	for jobID := range m.buffers {
		jobIDs = append(jobIDs, jobID)
	}
	return jobIDs
}

// Stats returns simple buffer statistics
func (m *SimpleLogManager) Stats() SimpleLogStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	totalBuffers := len(m.buffers)
	totalChunks := 0

	for _, buffer := range m.buffers {
		totalChunks += buffer.Size()
	}

	return SimpleLogStats{
		ActiveBuffers: totalBuffers,
		TotalChunks:   totalChunks,
	}
}

// SimpleLogStats provides basic buffer statistics
type SimpleLogStats struct {
	ActiveBuffers int
	TotalChunks   int
}
