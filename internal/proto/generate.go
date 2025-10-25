// Package proto contains internal protocol buffer definitions for joblet IPC
//
// This package defines internal protos that are NOT part of the public API:
// - ipc.proto: Binary IPC between joblet-core and persist subprocess
// - persist.proto: gRPC service for querying historical logs/metrics
//
// To regenerate proto files:
//
//	go generate ./internal/proto
//	make proto
package proto

// Generate IPC protobuf (used for joblet-core <-> persist communication)
//go:generate mkdir -p gen/ipc
//go:generate protoc --proto_path=. --go_out=gen/ipc --go_opt=paths=source_relative ipc.proto

// Generate Persist protobuf (used for persist gRPC service API)
//go:generate mkdir -p gen/persist
//go:generate protoc --proto_path=. --go_out=gen/persist --go-grpc_out=gen/persist --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative persist.proto
