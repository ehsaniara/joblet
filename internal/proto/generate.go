// Package proto contains internal protocol buffer definitions for joblet IPC
//
// This package defines internal protos that are NOT part of the public API:
// - ipc.proto: Binary IPC between joblet-core and persist subprocess
//
// PersistService moved to joblet-proto (github.com/ehsaniara/joblet-proto/v2/gen/persist).
//
// To regenerate proto files:
//
//	go generate ./internal/proto    # or: make proto
//
// Generation uses protoc-gen-go and protoc-gen-go-grpc at the versions pinned
// in go.mod (tool directives), so output is identical across developers
// regardless of which plugins they have installed globally. `protoc` itself
// must still be on PATH.
package proto

//go:generate ../../scripts/generate-proto.sh
