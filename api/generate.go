package api

// Generate protocol buffer code from joblet-proto module
// Version is managed in go.mod (single source of truth)
// This ensures we use the exact version that includes nodeId, serverIPs, and macAddresses
//go:generate ../scripts/generate-proto.sh
