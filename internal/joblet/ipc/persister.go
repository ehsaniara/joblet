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
		JobId:     jobID,
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
		JobId:     jobID,
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

// Verify Persister implements EventPersister
var _ telemetry.EventPersister = (*Persister)(nil)
