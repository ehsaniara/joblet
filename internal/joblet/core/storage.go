package core

import (
	"joblet/internal/joblet/adapters"
)

// Define type aliases to avoid importing concrete adapters directly
// This allows for interface-based dependency injection

// JobStore is an alias for the job storage interface
type JobStore = adapters.JobStorer

// NetworkStore is an alias for the network storage interface (types are identical)
type NetworkStore = adapters.NetworkStorer

// VolumeStore is an alias for the volume storage interface
type VolumeStore = adapters.VolumeStorer
