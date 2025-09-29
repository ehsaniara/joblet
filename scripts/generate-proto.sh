#!/bin/bash
# Generate protocol buffer code from joblet-proto Go module
# This ensures we use the exact version that includes nodeId, serverIPs, and macAddresses

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GEN_DIR="${PROJECT_ROOT}/api/gen"

# Extract proto version from go.mod (single source of truth)
cd "${PROJECT_ROOT}"
PROTO_VERSION=$(go list -m github.com/ehsaniara/joblet-proto | awk '{print $2}')
if [ -z "${PROTO_VERSION}" ]; then
    echo "❌ Error: Could not extract joblet-proto version from go.mod"
    echo "Make sure github.com/ehsaniara/joblet-proto is in go.mod"
    exit 1
fi

PROTO_MODULE="github.com/ehsaniara/joblet-proto@${PROTO_VERSION}"

echo "Generating protobuf code from ${PROTO_MODULE}..."

# Ensure we're in the project root
cd "${PROJECT_ROOT}"

# Download the specific proto version directly (ignoring go.mod version)
echo "Downloading proto module ${PROTO_MODULE}..."
go mod download "${PROTO_MODULE}"

# Get the module cache path
GOMODCACHE=$(go env GOMODCACHE)
PROTO_PATH="${GOMODCACHE}/${PROTO_MODULE}"

# Verify the proto module exists
if [ ! -d "${PROTO_PATH}" ]; then
    echo "❌ Error: Proto module not found at ${PROTO_PATH}"
    echo "Please run 'go mod download' first"
    exit 1
fi

# Verify proto file exists
PROTO_FILE="${PROTO_PATH}/proto/joblet.proto"
if [ ! -f "${PROTO_FILE}" ]; then
    echo "❌ Error: Proto file not found at ${PROTO_FILE}"
    exit 1
fi

# Check if protoc is available
if ! command -v protoc &> /dev/null; then
    echo "❌ Error: protoc is not installed or not in PATH"
    echo "Please install protobuf compiler: https://grpc.io/docs/protoc-installation/"
    exit 1
fi

# Clean and create output directory
echo "Cleaning output directory..."
rm -rf "${GEN_DIR}"
mkdir -p "${GEN_DIR}"

# Generate Go code
echo "Generating Go protobuf code..."
protoc \
    --proto_path="${PROTO_PATH}/proto" \
    --go_out="${GEN_DIR}" \
    --go_opt=paths=source_relative \
    --go-grpc_out="${GEN_DIR}" \
    --go-grpc_opt=paths=source_relative \
    "${PROTO_FILE}"

# Verify generation succeeded
if [ ! -f "${GEN_DIR}/joblet.pb.go" ]; then
    echo "❌ Error: Proto generation failed - joblet.pb.go not found"
    exit 1
fi

if [ ! -f "${GEN_DIR}/joblet_grpc.pb.go" ]; then
    echo "❌ Error: Proto generation failed - joblet_grpc.pb.go not found"
    exit 1
fi

echo "Protocol buffer generation complete from ${PROTO_VERSION}"
echo "Generated files:"
ls -la "${GEN_DIR}"/*.go