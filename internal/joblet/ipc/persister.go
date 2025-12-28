package ipc

import (
	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
)

// Persister adapts the IPC Writer to the telemetry.EventPersister interface.
// This allows the telemetry collector to persist eBPF events to storage
// via the IPC pipeline to persist → CloudWatch.
type Persister struct {
	writer *Writer
}

// NewPersister creates a new Persister that wraps the given IPC writer.
func NewPersister(writer *Writer) *Persister {
	return &Persister{writer: writer}
}

// PersistExecEvent converts telemetry ExecData to IPC proto and sends to persist.
func (p *Persister) PersistExecEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.ExecData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.ExecEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		Ppid:      data.PPID,
		Filename:  data.Binary,
		Args:      data.Args,
	}

	return p.writer.WriteExecEvent(event)
}

// PersistConnectEvent converts telemetry ConnectData to IPC proto and sends to persist.
func (p *Persister) PersistConnectEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.ConnectData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.ConnectEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		DstAddr:   data.Address,
		DstPort:   data.Port,
		Protocol:  data.Protocol,
		SrcAddr:   data.LocalAddress,
		SrcPort:   data.LocalPort,
	}

	return p.writer.WriteConnectEvent(event)
}

// PersistMetrics converts telemetry MetricsData to IPC proto and sends to persist.
func (p *Persister) PersistMetrics(jobID string, timestamp int64, sequence uint64, data *telemetry.MetricsData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	metricData := &ipcpb.MetricData{
		CpuUsage:    data.CPUPercent,
		MemoryUsage: data.MemoryBytes,
		GpuUsage:    data.GPUPercent,
		DiskIo: &ipcpb.DiskIO{
			ReadBytes:  data.DiskReadBytes,
			WriteBytes: data.DiskWriteBytes,
		},
		NetworkIo: &ipcpb.NetworkIO{
			RxBytes: data.NetRecvBytes,
			TxBytes: data.NetSentBytes,
		},
	}

	return p.writer.WriteMetric(jobID, timestamp, sequence, metricData)
}

// PersistAcceptEvent converts telemetry AcceptData to IPC proto and sends to persist.
func (p *Persister) PersistAcceptEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.AcceptData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.AcceptEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		SrcAddr:   data.RemoteAddr,
		SrcPort:   data.RemotePort,
		DstPort:   data.LocalPort,
		Protocol:  data.Protocol,
	}

	return p.writer.WriteAcceptEvent(event)
}

// PersistSocketDataEvent converts telemetry SocketDataData to IPC proto and sends to persist.
func (p *Persister) PersistSocketDataEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.SocketDataData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.SocketDataEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		Direction: data.Direction,
		Addr:      data.Address,
		Port:      data.Port,
		Protocol:  data.Protocol,
		Bytes:     data.Bytes,
	}

	return p.writer.WriteSocketDataEvent(event)
}

// PersistMmapEvent converts telemetry MmapData to IPC proto and sends to persist.
func (p *Persister) PersistMmapEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.MmapData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.MmapEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		Addr:      data.Addr,
		Length:    data.Length,
		Prot:      data.Prot,
		Flags:     data.Flags,
	}

	return p.writer.WriteMmapEvent(event)
}

// PersistMprotectEvent converts telemetry MprotectData to IPC proto and sends to persist.
func (p *Persister) PersistMprotectEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.MprotectData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.MprotectEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		Addr:      data.Addr,
		Length:    data.Length,
		Prot:      data.Prot,
	}

	return p.writer.WriteMprotectEvent(event)
}

// PersistFileEvent converts telemetry FileData to IPC proto and sends to persist.
func (p *Persister) PersistFileEvent(jobID string, timestamp int64, sequence uint64, data *telemetry.FileData) error {
	if p.writer == nil {
		return nil // No writer configured
	}

	event := &ipcpb.FileEvent{
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Pid:       data.PID,
		Path:      data.Path,
		Operation: data.Operation,
		Bytes:     data.Bytes,
	}

	return p.writer.WriteFileEvent(event)
}

// Verify Persister implements EventPersister
var _ telemetry.EventPersister = (*Persister)(nil)
