# Simple Joblet Makefile
#
# Everything targets this machine by default; pass parameters to override:
#   make deploy                                  # deploy to localhost, native arch
#   make deploy REMOTE_HOST=10.0.0.5             # deploy to a remote host over ssh
#   make deploy REMOTE_HOST=10.0.0.5 GOARCH=amd64  # remote host with a different arch
REMOTE_HOST ?=
REMOTE_USER ?= $(USER)

# Target architecture (native unless specified)
GOARCH ?= $(shell go env GOARCH)

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

.PHONY: all clean deploy fresh-install pre-pr test proto bpf help joblet rnx persist state version

all: joblet rnx persist state
	@echo "✅ Build complete - all binaries ready"

joblet:
	@echo "Building joblet daemon..."
	@GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=joblet" -o bin/joblet ./cmd/joblet
	@echo "✅ joblet built (version: $(VERSION))"

rnx:
	@echo "Building rnx CLI..."
	@GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=rnx" -o bin/rnx ./cmd/rnx
	@echo "✅ rnx built (version: $(VERSION))"

persist:
	@echo "Building persist..."
	@cd persist && GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=persist" -o ../bin/persist ./cmd/persist
	@echo "✅ persist built (version: $(VERSION))"

state:
	@echo "Building state..."
	@cd state && GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS) -X github.com/ehsaniara/joblet/pkg/version.Component=state" -o ../bin/state ./cmd/state
	@echo "✅ state built (version: $(VERSION))"

proto:
	@echo "Generating proto files..."
	@./scripts/generate-proto.sh
	@echo "Proto generation complete"

bpf:
	@echo "Compiling BPF objects (requires clang, llvm, libbpf-dev)..."
	@go generate ./internal/joblet/ebpf/telematics
	@echo "BPF compilation complete"

version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Tag: $(GIT_TAG)"
	@echo "Build Date: $(BUILD_DATE)"

clean:
	rm -rf bin/ dist/ api/gen/ internal/proto/gen/
	rm -f internal/joblet/ebpf/telematics/telematics_*_bpfel.o

deploy: all
ifeq ($(strip $(REMOTE_HOST)),)
	@test -d /opt/joblet/bin || { echo "❌ /opt/joblet/bin not found - install the joblet package first"; exit 1; }
	@echo "Deploying to localhost ($(GOARCH))..."
	@echo "Stopping services..."
	@sudo systemctl stop joblet.service || true
	@echo "Installing binaries..."
	@sudo cp bin/joblet bin/rnx bin/persist bin/state /opt/joblet/bin/ && sudo chmod +x /opt/joblet/bin/*
	@echo "Starting services..."
	@sudo systemctl start joblet.service
	@echo "Waiting for service readiness (persist socket + gRPC)..."
	@ready=0; for i in $$(seq 1 30); do \
		if [ -S /opt/joblet/run/persist-ipc.sock ] && ./bin/rnx job list >/dev/null 2>&1; then ready=1; sleep 1; break; fi; \
		sleep 0.5; \
	done; \
	if [ $$ready -eq 1 ]; then echo "✅ Local deployment complete (persist and state run as subprocesses)"; \
	else echo "⚠️  Service started but not ready after 15s - check: journalctl -u joblet"; exit 1; fi
else
	@echo "Deploying to $(REMOTE_USER)@$(REMOTE_HOST) ($(GOARCH))..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	@echo "Copying binaries..."
	@scp bin/joblet bin/rnx bin/persist bin/state $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "Stopping services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service || true'
	@echo "Installing binaries..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp /tmp/joblet/build/* /opt/joblet/bin/ && sudo chmod +x /opt/joblet/bin/*'
	@echo "Starting services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl start joblet.service'
	@echo "✅ Remote deployment complete (persist and state run as subprocesses)"
endif

fresh-install: all
	@echo "Purging existing joblet installation..."
	@sudo ./scripts/uninstall.sh --purge
	@echo "Building package from local working tree..."
	@./scripts/build-deb.sh $(GOARCH) $(VERSION)
	@echo "Installing package..."
	@sudo DEBIAN_FRONTEND=noninteractive dpkg -i "$$(ls -t joblet_*_$(GOARCH).deb | head -1)"
	@echo "Starting service..."
	@sudo systemctl start joblet.service
	@echo "Waiting for service readiness (persist socket + gRPC)..."
	@ready=0; for i in $$(seq 1 30); do \
		if [ -S /opt/joblet/run/persist-ipc.sock ] && ./bin/rnx job list >/dev/null 2>&1; then ready=1; sleep 1; break; fi; \
		sleep 0.5; \
	done; \
	if [ $$ready -eq 1 ]; then echo "✅ Fresh install complete - brand new setup from local build"; \
	else echo "⚠️  Service started but not ready after 15s - check: journalctl -u joblet"; exit 1; fi

pre-pr:
	@./scripts/pre-pr-check.sh

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
	@echo "  make all            - Build all binaries (joblet, rnx, persist, state)"
	@echo "  make joblet         - Build joblet daemon only"
	@echo "  make rnx            - Build rnx CLI only"
	@echo "  make persist        - Build persist only"
	@echo "  make state          - Build state only"
	@echo "  make proto          - Generate proto files"
	@echo "  make bpf            - Compile BPF objects (needs clang, llvm, libbpf-dev)"
	@echo "  make version        - Show version information"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make test           - Run all tests (all modules)"
	@echo "  make deploy         - Deploy to this host (default) or REMOTE_HOST=ip for remote"
	@echo "  make fresh-install  - Purge install, rebuild local .deb, install from scratch"
	@echo "  make pre-pr         - Full pre-PR check: deploy + e2e + packaged install"
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
	@echo "Deployment (localhost by default; override parametrically):"
	@echo "  make deploy REMOTE_HOST=<ip> REMOTE_USER=<user> GOARCH=<arch>"
