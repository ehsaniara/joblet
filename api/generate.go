package api

// Generate protocol buffer code from joblet-proto module
// Version is managed centrally in PROTO_VERSION file
// This ensures we use the exact version that includes nodeId, serverIPs, and macAddresses
//go:generate ../scripts/generate-proto.sh
