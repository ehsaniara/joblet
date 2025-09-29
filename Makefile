# Simple Joblet Makefile
REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay

.PHONY: all clean deploy test

all: proto
	@echo "Building all components..."
	@go mod download
	@./scripts/build-version.sh rnx bin
	@GOOS=linux GOARCH=amd64 ./scripts/build-version.sh joblet bin
	@cd admin/ui && npm install && npm run build
	@echo "Build complete"

proto:
	@echo "Generating proto files..."
	@go generate ./api
	@echo "Proto generation complete"


clean:
	rm -rf bin/ dist/ api/gen/ admin/ui/dist/ admin/ui/node_modules/

deploy: all
	@echo "Deploying to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	@scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service && \
		sudo cp /tmp/joblet/build/* /opt/joblet/ && \
		sudo chmod +x /opt/joblet/* && \
		sudo systemctl start joblet.service'
	@echo "Deployment complete"

test:
	@echo "Running tests..."
	@go test ./...
	@echo "Tests complete"

help:
	@echo "Usage:"
	@echo "  make all     - Build everything (rnx, joblet, admin-ui)"
	@echo "  make proto   - Generate proto files"
	@echo "  make clean   - Remove build artifacts"
	@echo "  make deploy  - Deploy to remote server"
	@echo "  make test    - Run tests"
	@echo ""
	@echo "Proto Version Management:"
	@echo "  Version is managed in go.mod (single source of truth)"
	@echo "  Current version: $(shell go list -m github.com/ehsaniara/joblet-proto 2>/dev/null | awk '{print $$2}' || echo 'not found')"
	@echo ""
	@echo "Configuration:"
	@echo "  REMOTE_HOST=$(REMOTE_HOST)"
	@echo "  REMOTE_USER=$(REMOTE_USER)"