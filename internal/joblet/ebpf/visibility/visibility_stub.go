//go:build !linux || (!amd64 && !arm64)

// Stub implementation for non-Linux or when eBPF is not available

package visibility

import (
	"errors"

	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	"github.com/ehsaniara/joblet/pkg/logger"
)

var ErrNotSupported = errors.New("eBPF visibility not supported on this platform")

// Monitor is a stub for non-Linux platforms
type Monitor struct{}

func NewMonitor(collector *telemetry.Collector, log *logger.Logger) *Monitor {
	return &Monitor{}
}

func (m *Monitor) Start() error {
	return ErrNotSupported
}

func (m *Monitor) Stop() error {
	return nil
}

func (m *Monitor) AddJob(jobID string, cgroupID uint64) error {
	return ErrNotSupported
}

func (m *Monitor) RemoveJob(jobID string) error {
	return nil
}

func (m *Monitor) GetStats() MonitorStats {
	return MonitorStats{}
}

func IsSupported() error {
	return ErrNotSupported
}
