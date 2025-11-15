package state

import (
	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// Message types matching state IPC protocol

type Message struct {
	Operation string        `json:"op"`
	JobID     string        `json:"jobId,omitempty"`
	Job       *domain.Job   `json:"job,omitempty"`
	Jobs      []*domain.Job `json:"jobs,omitempty"`
	Filter    *Filter       `json:"filter,omitempty"`
	RequestID string        `json:"requestId"`
	Timestamp int64         `json:"timestamp"`
}

type Response struct {
	RequestID string        `json:"requestId"`
	Success   bool          `json:"success"`
	Job       *domain.Job   `json:"job,omitempty"`
	Jobs      []*domain.Job `json:"jobs,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type Filter struct {
	Status   string   `json:"status,omitempty"`
	NodeID   string   `json:"nodeId,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	SortBy   string   `json:"sortBy,omitempty"`
	SortDesc bool     `json:"sortDesc,omitempty"`
	Statuses []string `json:"statuses,omitempty"` // Multiple statuses (OR condition)
}
