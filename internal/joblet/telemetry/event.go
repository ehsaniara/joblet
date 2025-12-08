// Package telemetry provides unified telemetry collection for jobs.
// It combines metrics (from cgroups v2) and activity events (from eBPF)
// into a single telemetry pipeline as described in ADR-014.
package telemetry

import (
	"time"
)

// EventType represents the type of telemetry event
type EventType string

const (
	// EventTypeMetrics represents periodic metrics from cgroups v2
	EventTypeMetrics EventType = "metrics"
	// EventTypeExec represents process execution events from eBPF
	EventTypeExec EventType = "exec"
	// EventTypeConnect represents network connection events from eBPF
	EventTypeConnect EventType = "connect"
	// EventTypeFile represents file access events from eBPF
	EventTypeFile EventType = "file"
	// EventTypeAccept represents incoming connection accept events from eBPF
	EventTypeAccept EventType = "accept"
	// EventTypeSocketData represents sendto/recvfrom events from eBPF
	EventTypeSocketData EventType = "socket_data"
	// EventTypeMmap represents memory mapping events from eBPF
	EventTypeMmap EventType = "mmap"
	// EventTypeMprotect represents memory protection change events from eBPF
	EventTypeMprotect EventType = "mprotect"
)

// Event represents a unified telemetry event that can contain
// either metrics data or activity data (exec, connect, file).
type Event struct {
	Timestamp time.Time
	JobID     string
	Type      EventType
	Data      interface{} // One of: *MetricsData, *ExecData, *ConnectData, *FileData
}

// MetricsData contains resource metrics collected from cgroups v2.
// This is collected periodically (typically every 1-5 seconds).
type MetricsData struct {
	CPUPercent     float64
	MemoryBytes    int64
	MemoryLimit    int64
	DiskReadBytes  int64
	DiskWriteBytes int64
	NetRecvBytes   int64
	NetSentBytes   int64
	GPUPercent     float64 // 0 if no GPU allocated
	GPUMemoryBytes int64   // 0 if no GPU allocated
}

// ExecData contains process execution event data from eBPF execve tracing.
type ExecData struct {
	PID      uint32
	PPID     uint32
	Binary   string
	Args     []string
	ExitCode int32 // Only set on process exit event
}

// ConnectData contains network connection event data from eBPF connect tracing.
type ConnectData struct {
	PID          uint32
	Address      string // Remote IP address (IPv4 or IPv6)
	Port         uint32
	Protocol     string // "tcp" or "udp"
	LocalAddress string // Optional local address
	LocalPort    uint32 // Optional local port
}

// FileData contains file access event data from eBPF file tracing.
type FileData struct {
	PID       uint32
	Path      string
	Operation string // "read", "write", "create", "delete"
	Bytes     int64  // Bytes read/written if applicable
}

// AcceptData contains incoming connection accept event data from eBPF.
type AcceptData struct {
	PID        uint32
	RemoteAddr string // Remote (client) IP address
	RemotePort uint32 // Remote (client) port
	LocalPort  uint32 // Local listening port
	Protocol   string // "tcp"
}

// SocketDataData contains sendto/recvfrom event data from eBPF.
type SocketDataData struct {
	PID       uint32
	Direction string // "send" or "recv"
	Address   string // Remote IP address
	Port      uint32 // Remote port
	Protocol  string // "tcp" or "udp"
	Bytes     int64  // Bytes transferred
}

// MmapData contains memory mapping event data from eBPF.
type MmapData struct {
	PID    uint32
	Addr   uint64 // Memory address
	Length uint64 // Mapping length
	Prot   uint32 // Protection flags (PROT_READ, PROT_WRITE, PROT_EXEC)
	Flags  uint32 // Mapping flags (MAP_SHARED, MAP_PRIVATE, MAP_ANONYMOUS)
}

// MprotectData contains memory protection change event data from eBPF.
type MprotectData struct {
	PID    uint32
	Addr   uint64 // Memory address
	Length uint64 // Region length
	Prot   uint32 // New protection flags
}

// NewMetricsEvent creates a new metrics telemetry event.
func NewMetricsEvent(jobID string, data *MetricsData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeMetrics,
		Data:      data,
	}
}

// NewExecEvent creates a new process execution telemetry event.
func NewExecEvent(jobID string, data *ExecData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeExec,
		Data:      data,
	}
}

// NewConnectEvent creates a new network connection telemetry event.
func NewConnectEvent(jobID string, data *ConnectData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeConnect,
		Data:      data,
	}
}

// NewFileEvent creates a new file access telemetry event.
func NewFileEvent(jobID string, data *FileData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeFile,
		Data:      data,
	}
}

// NewAcceptEvent creates a new incoming connection accept telemetry event.
func NewAcceptEvent(jobID string, data *AcceptData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeAccept,
		Data:      data,
	}
}

// NewSocketDataEvent creates a new sendto/recvfrom telemetry event.
func NewSocketDataEvent(jobID string, data *SocketDataData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeSocketData,
		Data:      data,
	}
}

// NewMmapEvent creates a new memory mapping telemetry event.
func NewMmapEvent(jobID string, data *MmapData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeMmap,
		Data:      data,
	}
}

// NewMprotectEvent creates a new memory protection change telemetry event.
func NewMprotectEvent(jobID string, data *MprotectData) *Event {
	return &Event{
		Timestamp: time.Now(),
		JobID:     jobID,
		Type:      EventTypeMprotect,
		Data:      data,
	}
}

// EventFilter defines which event types to include when streaming telemetry.
type EventFilter struct {
	Types []EventType
}

// Matches returns true if the event matches the filter.
// An empty filter matches all events.
func (f *EventFilter) Matches(e *Event) bool {
	if len(f.Types) == 0 {
		return true
	}
	for _, t := range f.Types {
		if e.Type == t {
			return true
		}
	}
	return false
}

// ParseEventTypes converts string type names to EventType values.
func ParseEventTypes(types []string) []EventType {
	if len(types) == 0 {
		return nil
	}
	result := make([]EventType, 0, len(types))
	for _, t := range types {
		switch t {
		case "metrics":
			result = append(result, EventTypeMetrics)
		case "exec":
			result = append(result, EventTypeExec)
		case "connect":
			result = append(result, EventTypeConnect)
		case "file":
			result = append(result, EventTypeFile)
		case "accept":
			result = append(result, EventTypeAccept)
		case "socket_data":
			result = append(result, EventTypeSocketData)
		case "mmap":
			result = append(result, EventTypeMmap)
		case "mprotect":
			result = append(result, EventTypeMprotect)
		}
	}
	return result
}
