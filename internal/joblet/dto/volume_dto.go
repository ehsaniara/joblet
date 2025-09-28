package dto

import "time"

// VolumeDTO represents a volume for data transfer between layers
type VolumeDTO struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`       // "filesystem" or "memory"
	Size        string    `json:"size"`       // Readable size (e.g., "1GB")
	SizeBytes   int64     `json:"size_bytes"` // Size in bytes
	Path        string    `json:"path"`       // Host filesystem path
	CreatedTime time.Time `json:"created_time"`
	JobCount    int32     `json:"job_count"`  // Number of jobs currently using this volume
	MountPath   string    `json:"mount_path"` // Path where volume is mounted in jobs
	InUse       bool      `json:"in_use"`     // Whether any jobs are currently using this volume
}

// VolumeListItemDTO represents a volume in list responses
type VolumeListItemDTO struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        string `json:"size"`
	JobCount    int32  `json:"job_count"`
	CreatedTime string `json:"created_time"` // ISO 8601 format
	InUse       bool   `json:"in_use"`
}

// CreateVolumeRequestDTO for creating new volumes
type CreateVolumeRequestDTO struct {
	Name string `json:"name"`
	Type string `json:"type"` // "filesystem" or "memory"
	Size string `json:"size"` // Readable size (e.g., "1GB", "500MB")
}

// DeleteVolumeRequestDTO for deleting volumes
type DeleteVolumeRequestDTO struct {
	Name  string `json:"name"`
	Force bool   `json:"force,omitempty"` // Force delete even if jobs are using it
}

// VolumeUsageDTO represents volume usage statistics
type VolumeUsageDTO struct {
	Name           string  `json:"name"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// VolumeMountDTO represents how a volume is mounted in a job
type VolumeMountDTO struct {
	VolumeName string `json:"volume_name"`
	MountPath  string `json:"mount_path"`
	ReadOnly   bool   `json:"read_only,omitempty"`
}
