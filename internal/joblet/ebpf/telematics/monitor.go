//go:build linux && (amd64 || arm64)

// Package telematics provides eBPF-based activity tracking for jobs.
// It monitors process execution and network connections for job processes,
// filtering by cgroup ID to only capture events from monitored jobs.
package telematics

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	"github.com/ehsaniara/joblet/pkg/logger"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror" -target amd64,arm64 telematics ./bpf/telematics.c

// EventTypeConfig controls which eBPF event types are enabled
type EventTypeConfig struct {
	Exec       bool
	Connect    bool
	Accept     bool
	Mmap       bool
	Mprotect   bool
	File       bool
	SocketData bool
}

// DefaultEventTypeConfig returns a config with all event types enabled
func DefaultEventTypeConfig() EventTypeConfig {
	return EventTypeConfig{
		Exec:       true,
		Connect:    true,
		Accept:     true,
		Mmap:       true,
		Mprotect:   true,
		File:       true,
		SocketData: true,
	}
}

// Monitor provides eBPF-based telematics into job activity.
// It tracks process execution (execve) and network connections (connect)
// for processes running in monitored cgroups.
type Monitor struct {
	collector   *telemetry.Collector
	logger      *logger.Logger
	eventConfig EventTypeConfig

	objs  *telematicsObjects
	links []link.Link

	// Map of job IDs to their cgroup IDs
	jobs      map[string]uint64
	jobsMutex sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// For graceful shutdown
	running bool
	mu      sync.Mutex
}

// ExecEvent represents a process execution event from eBPF
type ExecEvent struct {
	Timestamp uint64
	CgroupID  uint64
	PID       uint32
	PPID      uint32
	UID       uint32
	Comm      [16]byte
	Filename  [256]byte
	RetVal    int32
}

// ConnectEvent represents a network connection event from eBPF
type ConnectEvent struct {
	Timestamp uint64
	CgroupID  uint64
	PID       uint32
	Port      uint16
	Family    uint16
	Protocol  uint8
	Pad       [3]byte
	Addr      [16]byte // IPv4 uses first 4 bytes, IPv6 uses all 16
}

// AcceptEvent represents an incoming connection accept event from eBPF
type AcceptEvent struct {
	Timestamp  uint64
	CgroupID   uint64
	PID        uint32
	LocalPort  uint16
	RemotePort uint16
	Family     uint16
	Pad        [2]byte
	RemoteAddr [16]byte
}

// SocketDataEvent represents a sendto/recvfrom event from eBPF
type SocketDataEvent struct {
	Timestamp uint64
	CgroupID  uint64
	PID       uint32
	Port      uint16
	Family    uint16
	Direction uint8 // 0 = send, 1 = recv
	Protocol  uint8
	Pad       [2]byte
	Bytes     uint64
	Addr      [16]byte
}

// MmapEvent represents a memory mapping event from eBPF
type MmapEvent struct {
	Timestamp uint64
	CgroupID  uint64
	PID       uint32
	Prot      uint32
	Flags     uint32
	Pad       uint32
	Addr      uint64
	Length    uint64
}

// MprotectEvent represents a memory protection change event from eBPF
type MprotectEvent struct {
	Timestamp uint64
	CgroupID  uint64
	PID       uint32
	Prot      uint32
	Addr      uint64
	Length    uint64
}

// NewMonitor creates a new eBPF telematics monitor.
// The collector is used to emit telemetry events.
func NewMonitor(collector *telemetry.Collector, log *logger.Logger) *Monitor {
	return NewMonitorWithConfig(collector, log, DefaultEventTypeConfig())
}

// NewMonitorWithConfig creates a new eBPF telematics monitor with custom event type configuration.
// Use this to selectively enable/disable high-volume event types for performance tuning.
func NewMonitorWithConfig(collector *telemetry.Collector, log *logger.Logger, eventConfig EventTypeConfig) *Monitor {
	if log == nil {
		log = logger.New()
	}
	return &Monitor{
		collector:   collector,
		logger:      log.WithField("component", "ebpf-telematics"),
		jobs:        make(map[string]uint64),
		eventConfig: eventConfig,
	}
}

// Start loads and attaches the eBPF programs.
// Call this once when joblet starts.
func (m *Monitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return errors.New("monitor already running")
	}

	m.logger.Info("starting eBPF telematics monitor")

	// Remove MEMLOCK rlimit for eBPF maps on kernels < 5.11
	// This is required on older kernels that have memlock restrictions
	if err := rlimit.RemoveMemlock(); err != nil {
		m.logger.Warn("failed to remove memlock rlimit, eBPF may fail on older kernels", "error", err)
	}

	// Load pre-compiled eBPF objects
	m.objs = &telematicsObjects{}
	if err := loadTelematicsObjects(m.objs, nil); err != nil {
		return fmt.Errorf("failed to load eBPF objects: %w", err)
	}

	// Attach tracepoints based on configuration
	goroutineCount := 0

	// execve tracepoint (always attach if enabled)
	if m.eventConfig.Exec {
		execLink, err := link.Tracepoint("syscalls", "sys_enter_execve", m.objs.TracepointSyscallsSysEnterExecve, nil)
		if err != nil {
			m.objs.Close()
			return fmt.Errorf("failed to attach execve tracepoint: %w", err)
		}
		m.links = append(m.links, execLink)
		goroutineCount++
	} else {
		m.logger.Info("exec event type disabled by configuration")
	}

	// connect tracepoint
	if m.eventConfig.Connect {
		connectLink, err := link.Tracepoint("syscalls", "sys_enter_connect", m.objs.TracepointSyscallsSysEnterConnect, nil)
		if err != nil {
			m.cleanup()
			return fmt.Errorf("failed to attach connect tracepoint: %w", err)
		}
		m.links = append(m.links, connectLink)
		goroutineCount++
	} else {
		m.logger.Info("connect event type disabled by configuration")
	}

	// accept4 tracepoint (for incoming connections)
	if m.eventConfig.Accept {
		accept4Link, err := link.Tracepoint("syscalls", "sys_exit_accept4", m.objs.TracepointSyscallsSysExitAccept4, nil)
		if err != nil {
			m.logger.Warn("failed to attach accept4 tracepoint", "error", err)
		} else {
			m.links = append(m.links, accept4Link)
			goroutineCount++
		}
	} else {
		m.logger.Info("accept event type disabled by configuration")
	}

	// sendto/recvfrom tracepoints (socket data)
	if m.eventConfig.SocketData {
		sendtoLink, err := link.Tracepoint("syscalls", "sys_enter_sendto", m.objs.TracepointSyscallsSysEnterSendto, nil)
		if err != nil {
			m.logger.Warn("failed to attach sendto tracepoint", "error", err)
		} else {
			m.links = append(m.links, sendtoLink)
		}

		recvfromLink, err := link.Tracepoint("syscalls", "sys_enter_recvfrom", m.objs.TracepointSyscallsSysEnterRecvfrom, nil)
		if err != nil {
			m.logger.Warn("failed to attach recvfrom tracepoint", "error", err)
		} else {
			m.links = append(m.links, recvfromLink)
		}
		goroutineCount++
	} else {
		m.logger.Info("socket_data event type disabled by configuration (high volume)")
	}

	// mmap tracepoint
	if m.eventConfig.Mmap {
		mmapLink, err := link.Tracepoint("syscalls", "sys_enter_mmap", m.objs.TracepointSyscallsSysEnterMmap, nil)
		if err != nil {
			m.logger.Warn("failed to attach mmap tracepoint", "error", err)
		} else {
			m.links = append(m.links, mmapLink)
			goroutineCount++
		}
	} else {
		m.logger.Info("mmap event type disabled by configuration (high volume)")
	}

	// mprotect tracepoint
	if m.eventConfig.Mprotect {
		mprotectLink, err := link.Tracepoint("syscalls", "sys_enter_mprotect", m.objs.TracepointSyscallsSysEnterMprotect, nil)
		if err != nil {
			m.logger.Warn("failed to attach mprotect tracepoint", "error", err)
		} else {
			m.links = append(m.links, mprotectLink)
			goroutineCount++
		}
	} else {
		m.logger.Info("mprotect event type disabled by configuration")
	}

	// Create ring buffer readers
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// Start event processing goroutines based on configuration
	m.wg.Add(goroutineCount)
	if m.eventConfig.Exec {
		go m.processExecEvents()
	}
	if m.eventConfig.Connect {
		go m.processConnectEvents()
	}
	if m.eventConfig.Accept {
		go m.processAcceptEvents()
	}
	if m.eventConfig.SocketData {
		go m.processSocketDataEvents()
	}
	if m.eventConfig.Mmap {
		go m.processMmapEvents()
	}
	if m.eventConfig.Mprotect {
		go m.processMprotectEvents()
	}

	m.running = true
	m.logger.Info("eBPF telematics monitor started successfully")

	return nil
}

// Stop detaches eBPF programs and stops event processing.
func (m *Monitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("stopping eBPF telematics monitor")

	// Signal goroutines to stop
	m.cancel()

	// Wait for goroutines to finish
	m.wg.Wait()

	// Cleanup eBPF resources
	m.cleanup()

	m.running = false
	m.logger.Info("eBPF telematics monitor stopped")

	return nil
}

func (m *Monitor) cleanup() {
	for _, l := range m.links {
		l.Close()
	}
	m.links = nil

	if m.objs != nil {
		m.objs.Close()
		m.objs = nil
	}
}

// AddJob starts monitoring a job by its cgroup ID.
// The cgroupID is the cgroup v2 ID (from /sys/fs/cgroup).
func (m *Monitor) AddJob(jobID string, cgroupID uint64) error {
	m.jobsMutex.Lock()
	defer m.jobsMutex.Unlock()

	if m.objs == nil {
		return errors.New("monitor not started")
	}

	// Add to monitored cgroups map
	marker := uint8(1)
	if err := m.objs.MonitoredCgroups.Put(cgroupID, marker); err != nil {
		return fmt.Errorf("failed to add cgroup to monitor: %w", err)
	}

	m.jobs[jobID] = cgroupID
	m.logger.Info("added job to eBPF monitor", "jobId", jobID, "cgroupId", cgroupID)

	return nil
}

// RemoveJob stops monitoring a job.
func (m *Monitor) RemoveJob(jobID string) error {
	m.jobsMutex.Lock()
	defer m.jobsMutex.Unlock()

	cgroupID, exists := m.jobs[jobID]
	if !exists {
		return nil // Job not being monitored
	}

	if m.objs != nil {
		if err := m.objs.MonitoredCgroups.Delete(cgroupID); err != nil {
			// Log but don't fail - the job is being removed anyway
			m.logger.Warn("failed to remove cgroup from monitor", "jobId", jobID, "error", err)
		}
	}

	delete(m.jobs, jobID)
	m.logger.Debug("removed job from eBPF monitor", "jobId", jobID)

	return nil
}

// processExecEvents reads exec events from the ring buffer and emits telemetry
func (m *Monitor) processExecEvents() {
	defer m.wg.Done()

	reader, err := ringbuf.NewReader(m.objs.ExecEvents)
	if err != nil {
		m.logger.Error("failed to create exec events reader", "error", err)
		return
	}
	defer reader.Close()

	m.logger.Info("exec events reader started, waiting for events")

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading exec event", "error", err)
			continue
		}

		var event ExecEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse exec event", "error", err)
			continue
		}

		// Log raw event for debugging
		m.logger.Debug("received exec event from eBPF", "cgroupId", event.CgroupID, "pid", event.PID, "filename", nullTerminatedString(event.Filename[:]))

		// Find job ID for this cgroup
		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			m.logger.Debug("exec event from unknown cgroup, skipping", "cgroupId", event.CgroupID)
			continue // Unknown cgroup, skip
		}

		// Emit telemetry event
		if m.collector != nil {
			m.collector.EmitExec(jobID, &telemetry.ExecData{
				PID:      event.PID,
				PPID:     event.PPID,
				Binary:   nullTerminatedString(event.Filename[:]),
				ExitCode: event.RetVal,
			})
		}
	}
}

// processConnectEvents reads connect events from the ring buffer and emits telemetry
func (m *Monitor) processConnectEvents() {
	defer m.wg.Done()

	reader, err := ringbuf.NewReader(m.objs.ConnectEvents)
	if err != nil {
		m.logger.Error("failed to create connect events reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading connect event", "error", err)
			continue
		}

		var event ConnectEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse connect event", "error", err)
			continue
		}

		// Find job ID for this cgroup
		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			continue // Unknown cgroup, skip
		}

		// Parse address
		var addr string
		if event.Family == 2 { // AF_INET
			addr = net.IP(event.Addr[:4]).String()
		} else { // AF_INET6
			addr = net.IP(event.Addr[:]).String()
		}

		// Parse protocol
		protocol := "tcp"
		if event.Protocol == 17 { // IPPROTO_UDP
			protocol = "udp"
		}

		// Port is in network byte order (big endian), convert to host byte order
		port := (event.Port >> 8) | (event.Port << 8)

		// Emit telemetry event
		if m.collector != nil {
			m.collector.EmitConnect(jobID, &telemetry.ConnectData{
				PID:      event.PID,
				Address:  addr,
				Port:     uint32(port),
				Protocol: protocol,
			})
		}
	}
}

// findJobByCgroup returns the job ID for a given cgroup ID
func (m *Monitor) findJobByCgroup(cgroupID uint64) string {
	m.jobsMutex.RLock()
	defer m.jobsMutex.RUnlock()

	for jobID, cid := range m.jobs {
		if cid == cgroupID {
			return jobID
		}
	}
	return ""
}

// nullTerminatedString converts a null-terminated byte slice to a string
func nullTerminatedString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// processAcceptEvents reads accept events from the ring buffer and emits telemetry
func (m *Monitor) processAcceptEvents() {
	defer m.wg.Done()

	if m.objs.AcceptEvents == nil {
		m.logger.Debug("accept events ring buffer not available")
		return
	}

	reader, err := ringbuf.NewReader(m.objs.AcceptEvents)
	if err != nil {
		m.logger.Error("failed to create accept events reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading accept event", "error", err)
			continue
		}

		var event AcceptEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse accept event", "error", err)
			continue
		}

		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			continue
		}

		// Parse address
		var addr string
		if event.Family == 2 {
			addr = net.IP(event.RemoteAddr[:4]).String()
		} else {
			addr = net.IP(event.RemoteAddr[:]).String()
		}

		port := binary.BigEndian.Uint16([]byte{byte(event.RemotePort >> 8), byte(event.RemotePort)})

		if m.collector != nil {
			m.collector.EmitAccept(jobID, &telemetry.AcceptData{
				PID:        event.PID,
				RemoteAddr: addr,
				RemotePort: uint32(port),
				LocalPort:  uint32(event.LocalPort),
				Protocol:   "tcp",
			})
		}
	}
}

// processSocketDataEvents reads sendto/recvfrom events from the ring buffer
func (m *Monitor) processSocketDataEvents() {
	defer m.wg.Done()

	if m.objs.SocketDataEvents == nil {
		m.logger.Debug("socket data events ring buffer not available")
		return
	}

	reader, err := ringbuf.NewReader(m.objs.SocketDataEvents)
	if err != nil {
		m.logger.Error("failed to create socket data events reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading socket data event", "error", err)
			continue
		}

		var event SocketDataEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse socket data event", "error", err)
			continue
		}

		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			continue
		}

		// Parse address
		var addr string
		if event.Family == 2 {
			addr = net.IP(event.Addr[:4]).String()
		} else if event.Family == 10 {
			addr = net.IP(event.Addr[:]).String()
		}

		direction := "send"
		if event.Direction == 1 {
			direction = "recv"
		}

		protocol := "tcp"
		if event.Protocol == 17 {
			protocol = "udp"
		}

		// Port is in network byte order (big endian), convert to host byte order
		port := (event.Port >> 8) | (event.Port << 8)

		if m.collector != nil {
			m.collector.EmitSocketData(jobID, &telemetry.SocketDataData{
				PID:       event.PID,
				Direction: direction,
				Address:   addr,
				Port:      uint32(port),
				Protocol:  protocol,
				Bytes:     int64(event.Bytes),
			})
		}
	}
}

// processMmapEvents reads mmap events from the ring buffer
func (m *Monitor) processMmapEvents() {
	defer m.wg.Done()

	if m.objs.MmapEvents == nil {
		m.logger.Debug("mmap events ring buffer not available")
		return
	}

	reader, err := ringbuf.NewReader(m.objs.MmapEvents)
	if err != nil {
		m.logger.Error("failed to create mmap events reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading mmap event", "error", err)
			continue
		}

		var event MmapEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse mmap event", "error", err)
			continue
		}

		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			continue
		}

		if m.collector != nil {
			m.collector.EmitMmap(jobID, &telemetry.MmapData{
				PID:    event.PID,
				Addr:   event.Addr,
				Length: event.Length,
				Prot:   event.Prot,
				Flags:  event.Flags,
			})
		}
	}
}

// processMprotectEvents reads mprotect events from the ring buffer
func (m *Monitor) processMprotectEvents() {
	defer m.wg.Done()

	if m.objs.MprotectEvents == nil {
		m.logger.Debug("mprotect events ring buffer not available")
		return
	}

	reader, err := ringbuf.NewReader(m.objs.MprotectEvents)
	if err != nil {
		m.logger.Error("failed to create mprotect events reader", "error", err)
		return
	}
	defer reader.Close()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			m.logger.Warn("error reading mprotect event", "error", err)
			continue
		}

		var event MprotectEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			m.logger.Warn("failed to parse mprotect event", "error", err)
			continue
		}

		jobID := m.findJobByCgroup(event.CgroupID)
		if jobID == "" {
			continue
		}

		if m.collector != nil {
			m.collector.EmitMprotect(jobID, &telemetry.MprotectData{
				PID:    event.PID,
				Addr:   event.Addr,
				Length: event.Length,
				Prot:   event.Prot,
			})
		}
	}
}

// IsSupported checks if eBPF telematics is supported on this system.
// Returns nil if supported, otherwise returns an error describing why not.
func IsSupported() error {
	// Check if cgroup v2 is available
	if !IsCgroupV2() {
		return errors.New("cgroup v2 (unified hierarchy) is required for eBPF telematics")
	}

	// Check if /sys/kernel/tracing exists (for tracepoints)
	if _, err := os.Stat("/sys/kernel/tracing"); os.IsNotExist(err) {
		// Try alternative path
		if _, err := os.Stat("/sys/kernel/debug/tracing"); os.IsNotExist(err) {
			return errors.New("kernel tracing is not available")
		}
	}

	return nil
}

// GetStats returns current monitoring statistics
func (m *Monitor) GetStats() MonitorStats {
	m.jobsMutex.RLock()
	defer m.jobsMutex.RUnlock()

	return MonitorStats{
		Running:       m.running,
		JobsMonitored: len(m.jobs),
		StartTime:     time.Now(), // Would track actual start time
	}
}

// MonitorStats contains monitoring statistics
type MonitorStats struct {
	Running       bool
	JobsMonitored int
	StartTime     time.Time
}
