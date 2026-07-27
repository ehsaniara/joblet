package values

import (
	"fmt"
)

// Bandwidth represents I/O bandwidth with unit conversions
type Bandwidth struct {
	bytesPerSecond int64
}

// BandwidthUnit represents different bandwidth units
type BandwidthUnit string

const (
	BytesPerSec     BandwidthUnit = "B/s"
	KilobytesPerSec BandwidthUnit = "KB/s"
	MegabytesPerSec BandwidthUnit = "MB/s"
	GigabytesPerSec BandwidthUnit = "GB/s"
)

// NewBandwidth creates a new bandwidth from bytes per second
func NewBandwidth(bytesPerSec int64) (Bandwidth, error) {
	if bytesPerSec < 0 {
		return Bandwidth{}, fmt.Errorf("bandwidth cannot be negative: %d", bytesPerSec)
	}
	return Bandwidth{bytesPerSecond: bytesPerSec}, nil
}

// BytesPerSecond returns the bandwidth in bytes per second
func (b Bandwidth) BytesPerSecond() int64 {
	return b.bytesPerSecond
}

// IsUnlimited returns true if no limit is set
func (b Bandwidth) IsUnlimited() bool {
	return b.bytesPerSecond == 0
}

// String returns a readable string
func (b Bandwidth) String() string {
	if b.bytesPerSecond == 0 {
		return "unlimited"
	}

	const unit = 1024
	if b.bytesPerSecond < unit {
		return fmt.Sprintf("%d B/s", b.bytesPerSecond)
	}

	div, exp := int64(unit), 0
	for n := b.bytesPerSecond / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB/s", float64(b.bytesPerSecond)/float64(div), "KMGTPE"[exp])
}

// Validate checks if the bandwidth is within acceptable bounds
func (b Bandwidth) Validate(min, max Bandwidth) error {
	if b.bytesPerSecond < min.bytesPerSecond {
		return fmt.Errorf("bandwidth %s is below minimum %s", b, min)
	}
	if max.bytesPerSecond > 0 && b.bytesPerSecond > max.bytesPerSecond {
		return fmt.Errorf("bandwidth %s exceeds maximum %s", b, max)
	}
	return nil
}
