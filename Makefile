REMOTE_HOST ?= 192.168.100.94
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/joblet
REMOTE_ARCH ?= arm64

.PHONY: all clean rnx joblet deploy-passwordless deploy-safe config-generate config-remote-generate config-download config-view help setup-remote-passwordless setup-dev service-status live-log test-connection validate-user-namespaces setup-user-namespaces check-kernel-support setup-subuid-subgid test-user-namespace-isolation debug-user-namespaces deploy-with-user-namespaces test-user-namespace-job

all: rnx joblet

help:
	@echo "Joblet Makefile - Embedded Certificates Version"
	@echo ""
	@echo "Build targets:"
	@echo "  make all               - Build all binaries (rnx, joblet)"
	@echo "  make rnx               - Build RNX CLI for local development"
	@echo "  make joblet            - Build joblet binary for Linux"
	@echo "  make clean             - Remove build artifacts"
	@echo ""
	@echo "Configuration targets (Embedded Certificates):"
	@echo "  make config-generate   - Generate local configs with embedded certs"
	@echo "  make config-remote-generate - Generate configs on remote server"
	@echo "  make config-download   - Download client config from remote"
	@echo "  make config-view       - View embedded certificates in config"
	@echo ""
	@echo "Deployment targets:"
	@echo "  make deploy-passwordless - Deploy without password (requires sudo setup)"
	@echo "  make deploy-safe       - Deploy with password prompt (safe)"
	@echo ""
	@echo "Quick setup:"
	@echo "  make setup-remote-passwordless - Complete passwordless setup"
	@echo "  make setup-dev         - Development setup with embedded certs"
	@echo ""
	@echo "User Namespace Setup:"
	@echo "  make validate-user-namespaces  - Check user namespace support"
	@echo "  make setup-user-namespaces     - Setup user namespace environment"
	@echo "  make debug-user-namespaces     - Debug user namespace issues"
	@echo "  make deploy-with-user-namespaces - Deploy with user namespace validation"
	@echo "  make test-user-namespace-job   - Test job isolation"
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
	@echo "  make deploy-passwordless REMOTE_HOST=prod.example.com"
	@echo "  make config-download"
	@echo "  make setup-remote-passwordless"

rnx:
	@echo "Building RNX CLI..."
	GOOS=darwin GOARCH=$(REMOTE_ARCH) go build -o bin/rnx ./cmd/rnx

joblet:
	@echo "Building Joblet..."
	GOOS=linux GOARCH=$(REMOTE_ARCH) go build -o bin/joblet ./cmd/joblet

deploy-passwordless: joblet
	@echo "🚀 Passwordless deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop joblet.service && sudo cp /tmp/joblet/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo systemctl start joblet.service && echo "✅ Deployed successfully"'

deploy-safe: joblet
	@echo "🔐 Safe deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "Files uploaded. Installing with sudo..."
	@read -s -p "Enter sudo password for $(REMOTE_USER)@$(REMOTE_HOST): " SUDO_PASS; \
	echo ""; \
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '$$SUDO_PASS' | sudo -S bash -c '\
		echo \"Stopping service...\"; \
		systemctl stop joblet.service 2>/dev/null || echo \"Service not running\"; \
		echo \"Installing binaries...\"; \
		cp /tmp/joblet/build/joblet $(REMOTE_DIR)/; \
		chmod +x $(REMOTE_DIR)/joblet; \
		echo \"Starting service...\"; \
		systemctl start joblet.service; \
		echo \"Checking service status...\"; \
		systemctl is-active joblet.service >/dev/null && echo \"✅ Service started successfully\" || echo \"❌ Service failed to start\"'"

live-log:
	@echo "📊 Viewing live logs from $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'journalctl -u joblet.service -f'

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf config/

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

setup-remote-passwordless: config-remote-generate deploy-passwordless
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
	@echo "🔍 Checking configuration status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📁 Checking directory structure..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/joblet/ || echo 'Directory /opt/joblet/ not found'"
	@echo "📋 Checking configuration files..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/joblet/config/ || echo 'Configuration directory not found'"
	@echo "🔐 Checking embedded certificates in server config..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo grep -c 'BEGIN CERTIFICATE' /opt/joblet/config/joblet-config.yml 2>/dev/null | xargs echo 'Certificates found:' || echo 'No embedded certificates found'"

service-status:
	@echo "📊 Checking service status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo systemctl status joblet.service --no-pager"

test-connection:
	@echo "🔍 Testing connection to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '✅ SSH connection successful'"
	@echo "📊 Checking if joblet service exists..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "systemctl list-units --type=service | grep joblet || echo '❌ joblet service not found'"

validate-user-namespaces:
	@echo "🔍 Validating user namespace support on $(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		echo "📋 Checking kernel support..."; \
		if [ ! -f /proc/self/ns/user ]; then \
			echo "❌ User namespaces not supported by kernel"; \
			exit 1; \
		else \
			echo "✅ User namespace kernel support detected"; \
		fi; \
		echo "📋 Checking user namespace limits..."; \
		if [ -f /proc/sys/user/max_user_namespaces ]; then \
			MAX_NS=$$(cat /proc/sys/user/max_user_namespaces); \
			if [ "$$MAX_NS" = "0" ]; then \
				echo "❌ User namespaces disabled (max_user_namespaces=0)"; \
				exit 1; \
			else \
				echo "✅ User namespaces enabled (max: $$MAX_NS)"; \
			fi; \
		fi; \
		echo "📋 Checking cgroup namespace support..."; \
		if [ ! -f /proc/self/ns/cgroup ]; then \
			echo "❌ Cgroup namespaces not supported by kernel"; \
			exit 1; \
		else \
			echo "✅ Cgroup namespace kernel support detected"; \
		fi; \
		echo "📋 Checking cgroups v2..."; \
		if [ ! -f /sys/fs/cgroup/cgroup.controllers ]; then \
			echo "❌ Cgroups v2 not available"; \
			exit 1; \
		else \
			echo "✅ Cgroups v2 detected"; \
		fi; \
		echo "📋 Checking subuid/subgid files..."; \
		if [ ! -f /etc/subuid ]; then \
			echo "❌ /etc/subuid not found"; \
			exit 1; \
		fi; \
		if [ ! -f /etc/subgid ]; then \
			echo "❌ /etc/subgid not found"; \
			exit 1; \
		fi; \
		echo "📋 Checking joblet user configuration..."; \
		if ! grep -q "joblet:" /etc/subuid; then \
			echo "❌ joblet not configured in /etc/subuid"; \
			exit 1; \
		fi; \
		if ! grep -q "joblet:" /etc/subgid; then \
			echo "❌ joblet not configured in /etc/subgid"; \
			exit 1; \
		fi; \
		echo "✅ All user namespace requirements validated successfully!"'

setup-user-namespaces:
	@echo "🚀 Setting up user namespace environment on $(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		echo "📋 Creating joblet user if not exists..."; \
		if ! id joblet >/dev/null 2>&1; then \
			echo "Creating joblet user..."; \
			sudo useradd -r -s /bin/false joblet; \
			echo "✅ joblet user created"; \
		else \
			echo "✅ joblet user already exists"; \
		fi; \
		echo "📋 Creating subuid/subgid files if needed..."; \
		sudo touch /etc/subuid /etc/subgid; \
		echo "📋 Setting up subuid/subgid ranges..."; \
		if ! grep -q "^joblet:" /etc/subuid 2>/dev/null; then \
			echo "joblet:100000:6553600" | sudo tee -a /etc/subuid; \
			echo "✅ Added subuid entry for joblet"; \
		else \
			echo "✅ subuid entry already exists for joblet"; \
		fi; \
		if ! grep -q "^joblet:" /etc/subgid 2>/dev/null; then \
			echo "joblet:100000:6553600" | sudo tee -a /etc/subgid; \
			echo "✅ Added subgid entry for joblet"; \
		else \
			echo "✅ subgid entry already exists for joblet"; \
		fi; \
		echo "📋 Setting up cgroup permissions..."; \
		sudo mkdir -p /sys/fs/cgroup; \
		sudo chown joblet:joblet /sys/fs/cgroup 2>/dev/null || echo "Note: Could not change cgroup ownership (may be read-only)"; \
		echo "✅ User namespace environment setup completed!"'

debug-user-namespaces:
	@echo "🔍 Debugging user namespace configuration on $(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		echo "📋 Kernel configuration:"; \
		echo "  /proc/sys/user/max_user_namespaces: $$(cat /proc/sys/user/max_user_namespaces 2>/dev/null || echo \"not found\")"; \
		echo "  /proc/sys/kernel/unprivileged_userns_clone: $$(cat /proc/sys/kernel/unprivileged_userns_clone 2>/dev/null || echo \"not found\")"; \
		echo "📋 SubUID/SubGID configuration:"; \
		echo "  /etc/subuid entries:"; \
		cat /etc/subuid 2>/dev/null || echo "  File not found"; \
		echo "  /etc/subgid entries:"; \
		cat /etc/subgid 2>/dev/null || echo "  File not found"; \
		echo "📋 Joblet user info:"; \
		id joblet 2>/dev/null || echo "  joblet user not found"; \
		echo "📋 Service status:"; \
		sudo systemctl status joblet.service --no-pager --lines=5 2>/dev/null || echo "  Service not found"'

deploy-with-user-namespaces: joblet
	@echo "🚀 Deploying with user namespace validation to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📋 Validating remote user namespace support..."
	@$(MAKE) validate-user-namespaces || (echo "❌ User namespace validation failed. Running setup..." && $(MAKE) setup-user-namespaces && $(MAKE) validate-user-namespaces)
	@echo "📤 Uploading binaries..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/joblet/build"
	scp bin/joblet $(REMOTE_USER)@$(REMOTE_HOST):/tmp/joblet/build/
	@echo "🔧 Installing with user namespace support..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		sudo systemctl stop joblet.service 2>/dev/null || echo "Service not running"; \
		sudo cp /tmp/joblet/build/* $(REMOTE_DIR)/; \
		sudo chmod +x $(REMOTE_DIR)/*; \
		sudo chown joblet:joblet $(REMOTE_DIR)/*; \
		echo "Starting service..."; \
		sudo systemctl start joblet.service; \
		echo "Checking service status..."; \
		sleep 2; \
		if sudo systemctl is-active joblet.service >/dev/null; then \
			echo "✅ Service started successfully with user namespace support"; \
		else \
			echo "❌ Service failed to start. Checking logs..."; \
			sudo journalctl -u joblet.service --no-pager --lines=10; \
		fi'

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
