package values

import (
	"fmt"
	"strconv"
	"strings"
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

// ParseBandwidth parses a string like "10MB/s", "1GB/s", "1024KB/s"
func ParseBandwidth(s string) (Bandwidth, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return Bandwidth{}, nil
	}

	// Remove /s suffix if present
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSuffix(s, "/S")

	// Find where the number ends and unit begins
	var numEnd int
	for i, r := range s {
		if !strings.ContainsRune("0123456789.", r) {
			numEnd = i
			break
		}
	}

	if numEnd == 0 {
		return Bandwidth{}, fmt.Errorf("invalid bandwidth format: %s", s)
	}

	numStr := s[:numEnd]
	unitStr := strings.ToUpper(strings.TrimSpace(s[numEnd:]))

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return Bandwidth{}, fmt.Errorf("invalid bandwidth number: %s", numStr)
	}

	var bytesPerSec int64
	switch unitStr {
	case "B", "BPS", "":
		bytesPerSec = int64(value)
	case "K", "KB", "KBPS":
		bytesPerSec = int64(value * 1024)
	case "M", "MB", "MBPS":
		bytesPerSec = int64(value * 1024 * 1024)
	case "G", "GB", "GBPS":
		bytesPerSec = int64(value * 1024 * 1024 * 1024)
	default:
		return Bandwidth{}, fmt.Errorf("unknown bandwidth unit: %s", unitStr)
	}

	return NewBandwidth(bytesPerSec)
}

// BytesPerSecond returns the bandwidth in bytes per second
func (b Bandwidth) BytesPerSecond() int64 {
	return b.bytesPerSecond
}

// KilobytesPerSecond returns the bandwidth in KB/s
func (b Bandwidth) KilobytesPerSecond() float64 {
	return float64(b.bytesPerSecond) / 1024
}

// MegabytesPerSecond returns the bandwidth in MB/s
func (b Bandwidth) MegabytesPerSecond() float64 {
	return float64(b.bytesPerSecond) / (1024 * 1024)
}

// IsUnlimited returns true if no limit is set
func (b Bandwidth) IsUnlimited() bool {
	return b.bytesPerSecond == 0
}

// String returns a human-readable string
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

// CalculateDuration calculates how long it takes to transfer bytes at this bandwidth
func (b Bandwidth) CalculateDuration(bytes int64) (float64, error) {
	if b.bytesPerSecond == 0 {
		return 0, fmt.Errorf("cannot calculate duration with unlimited bandwidth")
	}
	return float64(bytes) / float64(b.bytesPerSecond), nil
}
