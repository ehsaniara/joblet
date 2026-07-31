#!/usr/bin/env bash
# Regenerate protobuf code using the plugin versions pinned in go.mod.
# This avoids per-developer drift: everyone gets identical .pb.go output
# regardless of which protoc-gen-go they have installed globally.
#
# Requirements on PATH: protoc (system package), go.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$(mktemp -d)"
trap 'rm -rf "$BIN_DIR"' EXIT

# Build the pinned plugins from go.mod tool directives.
(cd "$REPO_ROOT" && go build -o "$BIN_DIR/protoc-gen-go" google.golang.org/protobuf/cmd/protoc-gen-go)
(cd "$REPO_ROOT" && go build -o "$BIN_DIR/protoc-gen-go-grpc" google.golang.org/grpc/cmd/protoc-gen-go-grpc)

cd "$REPO_ROOT/internal/proto"
mkdir -p gen/ipc

protoc \
  --plugin=protoc-gen-go="$BIN_DIR/protoc-gen-go" \
  --proto_path=. \
  --go_out=gen/ipc \
  --go_opt=paths=source_relative \
  ipc.proto
