package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VolumeType represents the type of volume storage
type VolumeType string

const (
	VolumeTypeFilesystem VolumeType = "filesystem" // Directory-based persistent storage
	VolumeTypeMemory     VolumeType = "memory"     // tmpfs-based temporary storage
)

// Volume represents a persistent storage volume that can be mounted into jobs
type Volume struct {
	Name        string     // Unique volume identifier
	Type        VolumeType // Storage backend type
	Size        string     // Size limit (e.g., "1GB", "500MB")
	SizeBytes   int64      // Parsed size in bytes
	Path        string     // Host filesystem path where volume is stored
	CreatedTime time.Time  // When the volume was created
	JobCount    int32      // Number of jobs currently using this volume
}

// ParseSize converts human-readable size to bytes
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, errors.New("size cannot be empty")
	}

	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// Extract numeric part and unit
	var numStr string
	var unit string

	for i, char := range sizeStr {
		if char >= '0' && char <= '9' || char == '.' {
			numStr += string(char)
		} else {
			unit = sizeStr[i:]
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	// Parse the numeric value
	size, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value in size: %s", numStr)
	}

	// Convert based on unit
	var multiplier int64
	switch unit {
	case "B", "":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported size unit: %s", unit)
	}

	return int64(size * float64(multiplier)), nil
}

// FormatSize converts bytes to human-readable format
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// NewVolume creates a new volume with the given parameters
func NewVolume(name, sizeStr string, volumeType VolumeType) (*Volume, error) {
	if name == "" {
		return nil, errors.New("volume name cannot be empty")
	}

	if !IsValidVolumeName(name) {
		return nil, fmt.Errorf("invalid volume name: %s (must contain only alphanumeric characters, hyphens, and underscores)", name)
	}

	sizeBytes, err := ParseSize(sizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}

	if sizeBytes <= 0 {
		return nil, errors.New("volume size must be positive")
	}

	// Validate volume type
	if volumeType != VolumeTypeFilesystem && volumeType != VolumeTypeMemory {
		return nil, fmt.Errorf("invalid volume type: %s", volumeType)
	}

	return &Volume{
		Name:        name,
		Type:        volumeType,
		Size:        sizeStr,
		SizeBytes:   sizeBytes,
		CreatedTime: time.Now(),
		JobCount:    0,
	}, nil
}

// IsValidVolumeName checks if a volume name is valid
func IsValidVolumeName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	// Must start and end with alphanumeric
	first := rune(name[0])
	last := rune(name[len(name)-1])

	return ((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) &&
		((last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9'))
}

// IncrementJobCount increases the job count when a job starts using this volume
func (v *Volume) IncrementJobCount() {
	v.JobCount++
}

// DecrementJobCount decreases the job count when a job stops using this volume
func (v *Volume) DecrementJobCount() {
	if v.JobCount > 0 {
		v.JobCount--
	}
}

// IsInUse returns true if any jobs are currently using this volume
func (v *Volume) IsInUse() bool {
	return v.JobCount > 0
}

// GetMountPath returns the path where this volume should be mounted inside the job container
func (v *Volume) GetMountPath() string {
	return fmt.Sprintf("/volumes/%s", v.Name)
}

// DeepCopy creates a deep copy of the volume
func (v *Volume) DeepCopy() *Volume {
	if v == nil {
		return nil
	}

	return &Volume{
		Name:        v.Name,
		Type:        v.Type,
		Size:        v.Size,
		SizeBytes:   v.SizeBytes,
		Path:        v.Path,
		CreatedTime: v.CreatedTime,
		JobCount:    v.JobCount,
	}
}
