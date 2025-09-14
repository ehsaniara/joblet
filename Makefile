REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/joblet
REMOTE_ARCH ?= amd64

.PHONY: all clean rnx joblet admin-ui deploy config-generate config-remote-generate config-download config-view help setup-remote-passwordless setup-dev service-status live-log test-connection validate-user-namespaces setup-user-namespaces check-kernel-support setup-subuid-subgid test-user-namespace-isolation debug-user-namespaces test-user-namespace-job release release-clean test test-unit test-visual test-automated

all: rnx joblet admin-ui

# Version-aware builds
rnx-versioned:
	./scripts/build-version.sh rnx bin/

joblet-versioned:
	./scripts/build-version.sh joblet bin/

all-versioned:
	./scripts/build-version.sh all bin/

help:
	@echo "Joblet Makefile - Embedded Certificates Version"
	@echo ""
	@echo "Build targets:"
	@echo "  make all               - Build all binaries (rnx, joblet, admin-ui)"
	@echo "  make rnx               - Build RNX CLI for local development"
	@echo "  make joblet            - Build joblet binary for Linux"
	@echo "  make admin-ui          - Build admin UI (requires Node.js)"
	@echo "  make clean             - Remove build artifacts"
	@echo ""
	@echo "Release targets:"
	@echo "  make release           - Create production release package"
	@echo "  make release-clean     - Clean release directory"
	@echo ""
	@echo "Configuration targets (Embedded Certificates):"
	@echo "  make config-generate   - Generate local configs with embedded certs"
	@echo "  make config-remote-generate - Generate configs on remote server"
	@echo "  make config-download   - Download client config from remote"
	@echo "  make config-view       - View embedded certificates in config"
	@echo ""
	@echo "Deployment targets:"
	@echo "  make deploy - Deploy without password (requires sudo setup)"
	@echo ""
	@echo "Quick setup:"
	@echo "  make setup-remote-passwordless - Complete passwordless setup"
	@echo "  make setup-dev         - Development setup with embedded certs"
	@echo ""
	@echo "User Namespace Setup:"
	@echo "  make validate-user-namespaces  - Check user namespace support"
	@echo "  make setup-user-namespaces     - Setup user namespace environment"
	@echo "  make debug-user-namespaces     - Debug user namespace issues"
	@echo "  make test-user-namespace-job   - Test job isolation"
	@echo ""
	@echo "Testing:"
	@echo "  make test              - Run comprehensive test suite (unit + visual + automated)"
	@echo "  make test-unit         - Run Go unit tests only"
	@echo "  make test-visual       - Run visual feature tests only"
	@echo "  make test-automated    - Run automated feature tests only"
	@echo ""
	@echo "Debugging:"
	@echo "  make config-check-remote - Check config status on server"
	@echo "  make service-status    - Check service status"
	@echo "  make test-connection   - Test SSH connection"
	@echo "  make live-log          - View live service logs"
	@echo ""
	@echo "Configuration (override with make target VAR=value):"
	@echo "  REMOTE_HOST = $(REMOTE_HOST)"
	@echo "  REMOTE_USER = $(REMOTE_USER)"
	@echo "  REMOTE_DIR  = $(REMOTE_DIR)"
	@echo ""
	@echo "Examples:"
	@echo "  make deploy REMOTE_HOST=prod.example.com"
	@echo "  make deploy-with-examples         - Deploy with workflow examples for testing"
	@echo "  make config-download"
	@echo "  make setup-remote-passwordless"

rnx:
	@echo "Building RNX CLI with version info..."
	@chmod +x ./scripts/build-version.sh
	./scripts/build-version.sh rnx bin/

admin-server: admin-ui
	@echo "Installing Admin Server dependencies..."
	cd admin/server && npm install
	@echo "✅ Admin Server ready"

joblet:
	@echo "Building Joblet with version info..."
	@chmod +x ./scripts/build-version.sh
	GOOS=linux GOARCH=$(REMOTE_ARCH) ./scripts/build-version.sh joblet bin/

admin-ui:
	@echo "Building Admin UI..."
	@if [ ! -d "admin/ui/node_modules" ]; then \
		echo "Installing Node.js dependencies..."; \
		cd admin/ui && npm install; \
	fi
	@echo "Building React application..."
	cd admin/ui && npm run build
	@echo "✅ Admin UI built to admin/ui/dist/"

deploy: joblet
	@echo "🚀 Passwordless deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service && sudo cp /tmp/joblet/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo systemctl start joblet.service && echo "✅ Deployed successfully"'

deploy-with-examples: joblet
	@echo "🚀 Development deployment with examples to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	scp -r examples $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service && sudo cp /tmp/joblet/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo mkdir -p $(REMOTE_DIR)/examples && sudo cp -r /tmp/joblet/examples/* $(REMOTE_DIR)/examples/ && sudo systemctl start joblet.service && echo "✅ Development deployment with examples completed"'

live-log:
	@echo "📊 Viewing live logs from $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'journalctl -u joblet.service -f'

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf config/
	rm -rf web/ui/dist/
	rm -rf web/ui/node_modules/
	rm -rf internal/rnx/admin/static/

# Testing targets
test: all deploy
	@echo "🧪 Running comprehensive joblet test suite..."
	@echo "Testing Go unit tests + all joblet features (2-step execution, namespaces, isolation, cgroups, logs, networking, filesystem, security boundaries)..."
	@echo ""
	@echo "📋 Phase 1: Unit Tests"
	@go test ./... || (echo "❌ Unit tests failed" && exit 1)
	@echo "✅ Unit tests passed"
	@echo ""
	@echo "📋 Phase 2: Visual Feature Tests"
	@cd tests/e2e && SKIP_BUILD=true ./final_isolation_test.sh
	@echo ""
	@echo "📋 Phase 3: Comprehensive Automated Tests"
	@cd tests/e2e && SKIP_BUILD=true ./test_joblet_principles.sh
	@echo ""
	@echo "🎉 Complete joblet test suite finished!"
	@echo "   ✅ Unit tests: Go code validation"
	@echo "   ✅ Feature tests: Visual verification of core isolation"
	@echo "   ✅ Automated tests: Comprehensive feature validation"
	@echo ""
	@echo "💡 To run individual components:"
	@echo "   go test ./...                              # Unit tests only"
	@echo "   cd tests/e2e && ./final_isolation_test.sh  # Visual tests only"
	@echo "   cd tests/e2e && ./test_joblet_principles.sh # Automated tests only"

# Individual test components (for advanced users)
test-unit:
	@echo "🧪 Running unit tests only..."
	@go test ./... || (echo "❌ Unit tests failed" && exit 1)
	@echo "✅ Unit tests passed"

test-visual: all deploy
	@echo "👁️ Running visual feature tests only..."
	@cd tests/e2e && SKIP_BUILD=true ./final_isolation_test.sh
	@echo "✅ Visual tests completed"

test-automated: all deploy
	@echo "🤖 Running automated feature tests only..."
	@cd tests/e2e && SKIP_BUILD=true ./test_joblet_principles.sh
	@echo "✅ Automated tests completed"

config-generate:
	@echo "🔐 Generating local configuration with embedded certificates..."
	@if [ ! -f ./scripts/certs_gen_embedded.sh ]; then \
		echo "❌ ./scripts/certs_gen_embedded.sh script not found"; \
		exit 1; \
	fi
	@chmod +x ./scripts/certs_gen_embedded.sh
	@JOBLET_SERVER_ADDRESS="localhost" ./scripts/certs_gen_embedded.sh
	@echo "✅ Local configuration generated with embedded certificates:"
	@echo "   Server config: ./config/joblet-config.yml"
	@echo "   Client config: ./config/rnx-config.yml"

config-remote-generate:
	@echo "🔐 Generating configuration on $(REMOTE_USER)@$(REMOTE_HOST) with embedded certificates..."
	@if [ ! -f ./scripts/certs_gen_embedded.sh ]; then \
		echo "❌ ./scripts/certs_gen_embedded.sh script not found"; \
		exit 1; \
	fi
	@echo "📤 Uploading certificate generation script..."
	scp ./scripts/certs_gen_embedded.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@echo "🏗️  Generating configuration with embedded certificates on remote server..."
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		chmod +x /tmp/certs_gen_embedded.sh; \
		sudo JOBLET_SERVER_ADDRESS=$(REMOTE_HOST) /tmp/certs_gen_embedded.sh; \
		echo ""; \
		echo "📋 Configuration files created:"; \
		sudo ls -la /opt/joblet/config/ 2>/dev/null || echo "No configuration found"; \
		rm -f /tmp/certs_gen_embedded.sh'
	@echo "✅ Remote configuration generated with embedded certificates!"

config-download:
	@echo "📥 Downloading client configuration from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p config
	@echo "📥 Downloading rnx-config.yml with embedded certificates..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cat /opt/joblet/config/rnx-config.yml' > config/rnx-config.yml 2>/dev/null || \
		(echo "❌ Failed to download config. Trying with temporary copy..." && \
		ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp /opt/joblet/config/rnx-config.yml /tmp/rnx-config-$${USER}.yml && sudo chmod 644 /tmp/rnx-config-$${USER}.yml' && \
		scp $(REMOTE_USER)@$(REMOTE_HOST):/tmp/rnx-config-$${USER}.yml config/rnx-config.yml && \
		ssh $(REMOTE_USER)@$(REMOTE_HOST) 'rm -f /tmp/rnx-config-$${USER}.yml')
	@chmod 600 config/rnx-config.yml
	@echo "✅ Client configuration downloaded to ./config/rnx-config.yml"
	@echo "💡 Usage: ./bin/rnx --config config/rnx-config.yml list"
	@echo "💡 Or: ./bin/rnx list  (will auto-find config/rnx-config.yml)"

config-view:
	@echo "🔍 Viewing embedded certificates in configuration..."
	@if [ -f config/rnx-config.yml ]; then \
		echo "📋 Client configuration nodes:"; \
		grep -E "^  [a-zA-Z]+:|address:" config/rnx-config.yml | head -20; \
		echo ""; \
		echo "🔐 Embedded certificates found:"; \
		grep -c "BEGIN CERTIFICATE" config/rnx-config.yml | xargs echo "  Certificates:"; \
		grep -c "BEGIN PRIVATE KEY" config/rnx-config.yml | xargs echo "  Private keys:"; \
	else \
		echo "❌ No client configuration found at config/rnx-config.yml"; \
		echo "💡 Run 'make config-download' to download from server"; \
	fi

setup-remote-passwordless: config-remote-generate deploy
	@echo "🎉 Complete passwordless setup finished!"
	@echo "   Server: $(REMOTE_USER)@$(REMOTE_HOST)"
	@echo "   Configuration: /opt/joblet/config/ (with embedded certificates)"
	@echo "   Service: joblet.service"
	@echo ""
	@echo "📥 Next steps:"
	@echo "   make config-download  # Download client configuration"
	@echo "   ./bin/rnx list        # Test connection"
	@echo "   ./bin/rnx run echo 'Hello World'"

setup-dev: config-generate all
	@echo "🎉 Development setup complete!"
	@echo "   Configuration: ./config/ (with embedded certificates)"
	@echo "   Binaries: ./bin/"
	@echo ""
	@echo "🚀 To test locally:"
	@echo "   ./bin/joblet  # Start server (uses config/joblet-config.yml)"
	@echo "   ./bin/rnx list  # Connect as client (uses config/rnx-config.yml)"

config-check-remote:
	@echo "📤 Uploading config check script to $(REMOTE_HOST)..."
	@scp scripts/config-check-remote.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'chmod +x /tmp/config-check-remote.sh && /tmp/config-check-remote.sh && rm /tmp/config-check-remote.sh'

service-status:
	@echo "📊 Checking service status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo systemctl status joblet.service --no-pager"

test-connection:
	@echo "🔍 Testing connection to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '✅ SSH connection successful'"
	@echo "📊 Checking if joblet service exists..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "systemctl list-units --type=service | grep joblet || echo '❌ joblet service not found'"

validate-user-namespaces:
	@echo "📤 Uploading validation script to $(REMOTE_HOST)..."
	@scp scripts/validate-user-namespaces.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'chmod +x /tmp/validate-user-namespaces.sh && /tmp/validate-user-namespaces.sh && rm /tmp/validate-user-namespaces.sh'

setup-user-namespaces:
	@echo "📤 Uploading setup script to $(REMOTE_HOST)..."
	@scp scripts/setup-user-namespaces.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'chmod +x /tmp/setup-user-namespaces.sh && /tmp/setup-user-namespaces.sh && rm /tmp/setup-user-namespaces.sh'

debug-user-namespaces:
	@echo "📤 Uploading debug script to $(REMOTE_HOST)..."
	@scp scripts/debug-user-namespaces.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) 'chmod +x /tmp/debug-user-namespaces.sh && /tmp/debug-user-namespaces.sh && rm /tmp/debug-user-namespaces.sh'

test-user-namespace-job: config-download
	@echo "🧪 Testing job execution with user namespace isolation..."
	@echo "📋 Creating test jobs to verify isolation..."
	./bin/rnx --config config/rnx-config.yml run whoami || echo "❌ Failed to run whoami job"
	sleep 1
	./bin/rnx --config config/rnx-config.yml run id || echo "❌ Failed to run id job"
	sleep 1
	./bin/rnx --config config/rnx-config.yml run ps aux || echo "❌ Failed to run ps job"
	@echo "✅ Test jobs submitted. Check logs to verify each job runs with different UID:"
	@echo "   Expected: Each job should run as different UID (100000+)"
	@echo "   Expected: Jobs should not see each other's processes"
	@echo "💡 View logs with: make live-log"

# Release packaging targets
release: all
	@echo "📦 Creating production release package..."
	@rm -rf release/
	@mkdir -p release/rnx-release/bin
	@mkdir -p release/rnx-release/admin/server
	@mkdir -p release/rnx-release/admin/ui
	
	@echo "📋 Copying binaries..."
	@cp bin/rnx release/rnx-release/bin/
	@cp bin/joblet release/rnx-release/bin/
	@chmod +x release/rnx-release/bin/*
	
	@echo "📋 Copying admin server..."
	@cp admin/server/package.json release/rnx-release/admin/server/
	@cp admin/server/server.js release/rnx-release/admin/server/
	
	@echo "📋 Copying admin UI..."
	@cp -r admin/ui/dist/* release/rnx-release/admin/ui/
	
	@echo "📋 Creating installation script..."
	@cp scripts/install.sh.template release/rnx-release/install.sh
	@chmod +x release/rnx-release/install.sh
	
	@echo "📋 Creating README..."
	@./scripts/release/generate-readme.sh "linux-amd64" "linux" "amd64" release/rnx-release/README.md
	
	@echo "📋 Creating release archive..."
	@cd release && tar -czf rnx-release.tar.gz rnx-release/
	
	@echo "✅ Release package created:"
	@echo "   📦 Archive: release/rnx-release.tar.gz"
	@echo "   📁 Directory: release/rnx-release/"
	@echo "   📄 Size: $$(du -h release/rnx-release.tar.gz | cut -f1)"

release-clean:
	@echo "🧹 Cleaning release directory..."
	@rm -rf release/
	@echo "✅ Release directory cleaned"
