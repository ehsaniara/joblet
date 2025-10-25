# Simple Joblet Makefile
REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay

# Fix GOPATH if IntelliJ IDEA has set it incorrectly
export GOPATH := $(HOME)/go

# Version information
# Priority: 1. VERSION env var, 2. VERSION file, 3. git tags, 4. fallback to "dev"
#
# Usage:
#   make version                    # Show current version (from git tags)
#   VERSION=v1.2.3 make all         # Build with custom version (CI/CD)
#   echo "v1.2.3" > VERSION && make # Use VERSION file (no git required)
#
# Note: End users do NOT need git - version is embedded in binary at build time
VERSION ?= $(shell [ -f VERSION ] && cat VERSION || git describe --tags --exact-match 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Ldflags for version injection
LDFLAGS := -s -w \
	-X github.com/ehsaniara/joblet/pkg/version.Version=$(VERSION) \
	-X github.com/ehsaniara/joblet/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/ehsaniara/joblet/pkg/version.GitTag=$(GIT_TAG) \
	-X github.com/ehsaniara/joblet/pkg/version.BuildDate=$(BUILD_DATE)

.PHONY: all clean deploy test proto help joblet rnx persist state version

all: proto joblet rnx persist state
	@echo "✅ Build complete - all binaries ready"

joblet:
	@echo "Building joblet daemon..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=joblet" -o bin/joblet ./cmd/joblet
	@echo "✅ joblet built (version: $(VERSION))"

rnx:
	@echo "Building rnx CLI..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=rnx" -o bin/rnx ./cmd/rnx
	@echo "✅ rnx built (version: $(VERSION))"

persist:
	@echo "Building persist..."
	@cd persist && GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=persist" -o ../bin/persist ./cmd/persist
	@echo "✅ persist built (version: $(VERSION))"

state:
	@echo "Building state..."
	@cd state && GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=state" -o ../bin/state ./cmd/state
	@echo "✅ state built (version: $(VERSION))"

proto:
	@echo "Generating proto files..."
	@./scripts/generate-proto.sh
	@go generate ./internal/proto
	@echo "Proto generation complete"

version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Tag: $(GIT_TAG)"
	@echo "Build Date: $(BUILD_DATE)"

clean:
	rm -rf bin/ dist/ api/gen/ internal/proto/gen/

deploy: all
	@echo "Deploying to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	@echo "Copying binaries..."
	@scp bin/joblet bin/rnx bin/persist bin/state $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "Stopping services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service || true'
	@echo "Installing binaries..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp /tmp/joblet/build/* /opt/joblet/bin/ && sudo chmod +x /opt/joblet/bin/*'
	@echo "Starting services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl start joblet.service'
	@echo "✅ Deployment complete (persist and state run as subprocesses)"

test:
	@echo "Running tests..."
	@echo "Testing main module..."
	@JOBLET_TEST_MODE=true go test ./...
	@echo "Testing persist module..."
	@cd persist && JOBLET_TEST_MODE=true go test ./...
	@echo "Testing state module..."
	@cd state && JOBLET_TEST_MODE=true go test ./...
	@echo "✅ All tests complete"

help:
	@echo "Joblet Monorepo Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make all       - Build all binaries (joblet, rnx, persist, state)"
	@echo "  make joblet    - Build joblet daemon only"
	@echo "  make rnx       - Build rnx CLI only"
	@echo "  make persist   - Build persist only"
	@echo "  make state     - Build state only"
	@echo "  make proto     - Generate proto files"
	@echo "  make version   - Show version information"
	@echo "  make clean     - Remove build artifacts"
	@echo "  make test      - Run all tests (all modules)"
	@echo "  make deploy    - Deploy to remote server"
	@echo ""
	@echo "Version Information:"
	@echo "  Version:    $(VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo ""
	@echo "Modules:"
	@echo "  Main:    github.com/ehsaniara/joblet"
	@echo "  Persist: github.com/ehsaniara/joblet/persist"
	@echo ""
	@echo "Proto Version:"
	@echo "  $(shell go list -m github.com/ehsaniara/joblet-proto 2>/dev/null | awk '{print $$2}' || echo 'not found')"
	@echo ""
	@echo "Deployment:"
	@echo "  REMOTE_HOST=$(REMOTE_HOST)"
	@echo "  REMOTE_USER=$(REMOTE_USER)"
