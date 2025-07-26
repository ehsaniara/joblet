package network

import "net"

// JobAllocation represents a job's network allocation
type JobAllocation struct {
	JobID    string
	Network  string
	IP       net.IP
	Hostname string
	VethHost string
	VethPeer string
}

// NetworkConfig represents persistent network configuration
type NetworkConfig struct {
	CIDR   string `json:"cidr"`
	Bridge string `json:"bridge"`
}

// NetworkInfo represents network information for listing
type NetworkInfo struct {
	Name     string
	CIDR     string
	Bridge   string
	JobCount int
}

// NetworkLimits defines bandwidth limits for a job
type NetworkLimits struct {
	IngressBPS int64 // Incoming bandwidth in bytes per second
	EgressBPS  int64 // Outgoing bandwidth in bytes per second
	BurstSize  int   // Burst size in KB (optional)
}

// BandwidthStats holds bandwidth usage statistics
type BandwidthStats struct {
	Interface       string
	BytesSent       uint64
	BytesReceived   uint64
	PacketsSent     uint64
	PacketsReceived uint64
}
