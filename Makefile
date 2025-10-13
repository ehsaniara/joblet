# Simple Joblet Makefile
REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay

# Fix GOPATH if IntelliJ IDEA has set it incorrectly
export GOPATH := $(HOME)/go

.PHONY: all clean deploy test proto help joblet rnx persist

all: proto joblet rnx persist
	@echo "✅ Build complete - all binaries ready"

joblet:
	@echo "Building joblet daemon..."
	@GOOS=linux GOARCH=amd64 go build -o bin/joblet ./cmd/joblet
	@echo "✅ joblet built"

rnx:
	@echo "Building rnx CLI..."
	@GOOS=linux GOARCH=amd64 go build -o bin/rnx ./cmd/rnx
	@echo "✅ rnx built"

persist:
	@echo "Building joblet-persist..."
	@cd persist && GOOS=linux GOARCH=amd64 go build -o ../bin/joblet-persist ./cmd/joblet-persist
	@echo "✅ joblet-persist built"

proto:
	@echo "Generating proto files..."
	@./scripts/generate-proto.sh
	@./scripts/generate-internal-proto.sh
	@echo "Proto generation complete"


clean:
	rm -rf bin/ dist/ api/gen/ internal/proto/gen/

deploy: all
	@echo "Deploying to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	@echo "Copying binaries..."
	@scp bin/joblet bin/rnx bin/joblet-persist $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "Stopping services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service || true'
	@echo "Installing binaries..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp /tmp/joblet/build/* /opt/joblet/bin/ && sudo chmod +x /opt/joblet/bin/*'
	@echo "Starting services..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl start joblet.service'
	@echo "✅ Deployment complete (joblet-persist runs as subprocess)"

test:
	@echo "Running tests..."
	@echo "Testing main module..."
	@go test ./...
	@echo "Testing persist module..."
	@cd persist && go test ./...
	@echo "✅ All tests complete"

help:
	@echo "Joblet Monorepo Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make all       - Build all binaries (joblet, rnx, joblet-persist)"
	@echo "  make joblet    - Build joblet daemon only"
	@echo "  make rnx       - Build rnx CLI only"
	@echo "  make persist   - Build joblet-persist only"
	@echo "  make proto     - Generate proto files"
	@echo "  make clean     - Remove build artifacts"
	@echo "  make test      - Run all tests (both modules)"
	@echo "  make deploy    - Deploy to remote server"
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