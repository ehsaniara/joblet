REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/worker

.PHONY: all clean cli worker deploy-passwordless deploy-safe config-generate config-remote-generate config-download config-view help setup-remote-passwordless setup-dev service-status live-log test-connection validate-user-namespaces setup-user-namespaces check-kernel-support setup-subuid-subgid test-user-namespace-isolation debug-user-namespaces deploy-with-user-namespaces test-user-namespace-job

all: cli worker

help:
	@echo "Worker Makefile - Embedded Certificates Version"
	@echo ""
	@echo "Build targets:"
	@echo "  make all               - Build all binaries (cli, worker)"
	@echo "  make cli               - Build CLI for local development"
	@echo "  make worker            - Build worker binary for Linux"
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

cli:
	@echo "Building CLI..."
	GOOS=darwin GOARCH=amd64 go build -o bin/cli ./cmd/cli

worker:
	@echo "Building worker..."
	GOOS=linux GOARCH=amd64 go build -o bin/worker ./cmd/worker

deploy-passwordless: worker
	@echo "🚀 Passwordless deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/worker/build"
	scp bin/worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/worker/build/
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop worker.service && sudo cp /tmp/worker/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo systemctl start worker.service && echo "✅ Deployed successfully"'

deploy-safe: worker
	@echo "🔐 Safe deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/worker/build"
	scp bin/worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/worker/build/
	@echo "Files uploaded. Installing with sudo..."
	@read -s -p "Enter sudo password for $(REMOTE_USER)@$(REMOTE_HOST): " SUDO_PASS; \
	echo ""; \
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '$$SUDO_PASS' | sudo -S bash -c '\
		echo \"Stopping service...\"; \
		systemctl stop worker.service 2>/dev/null || echo \"Service not running\"; \
		echo \"Installing binaries...\"; \
		cp /tmp/worker/build/worker $(REMOTE_DIR)/; \
		chmod +x $(REMOTE_DIR)/worker; \
		echo \"Starting service...\"; \
		systemctl start worker.service; \
		echo \"Checking service status...\"; \
		systemctl is-active worker.service >/dev/null && echo \"✅ Service started successfully\" || echo \"❌ Service failed to start\"'"

live-log:
	@echo "📊 Viewing live logs from $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'journalctl -u worker.service -f'

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
	@WORKER_SERVER_ADDRESS="localhost" ./scripts/certs_gen_embedded.sh
	@echo "✅ Local configuration generated with embedded certificates:"
	@echo "   Server config: ./config/server-config.yml"
	@echo "   Client config: ./config/client-config.yml"

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
		sudo WORKER_SERVER_ADDRESS=$(REMOTE_HOST) /tmp/certs_gen_embedded.sh; \
		echo ""; \
		echo "📋 Configuration files created:"; \
		sudo ls -la /opt/worker/config/ 2>/dev/null || echo "No configuration found"; \
		rm -f /tmp/certs_gen_embedded.sh'
	@echo "✅ Remote configuration generated with embedded certificates!"

config-download:
	@echo "📥 Downloading client configuration from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p config
	@echo "📥 Downloading client-config.yml with embedded certificates..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cat /opt/worker/config/client-config.yml' > config/client-config.yml 2>/dev/null || \
		(echo "❌ Failed to download config. Trying with temporary copy..." && \
		ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp /opt/worker/config/client-config.yml /tmp/client-config-$${USER}.yml && sudo chmod 644 /tmp/client-config-$${USER}.yml' && \
		scp $(REMOTE_USER)@$(REMOTE_HOST):/tmp/client-config-$${USER}.yml config/client-config.yml && \
		ssh $(REMOTE_USER)@$(REMOTE_HOST) 'rm -f /tmp/client-config-$${USER}.yml')
	@chmod 600 config/client-config.yml
	@echo "✅ Client configuration downloaded to ./config/client-config.yml"
	@echo "💡 Usage: ./bin/cli --config config/client-config.yml list"
	@echo "💡 Or: ./bin/cli list  (will auto-find config/client-config.yml)"

config-view:
	@echo "🔍 Viewing embedded certificates in configuration..."
	@if [ -f config/client-config.yml ]; then \
		echo "📋 Client configuration nodes:"; \
		grep -E "^  [a-zA-Z]+:|address:" config/client-config.yml | head -20; \
		echo ""; \
		echo "🔐 Embedded certificates found:"; \
		grep -c "BEGIN CERTIFICATE" config/client-config.yml | xargs echo "  Certificates:"; \
		grep -c "BEGIN PRIVATE KEY" config/client-config.yml | xargs echo "  Private keys:"; \
	else \
		echo "❌ No client configuration found at config/client-config.yml"; \
		echo "💡 Run 'make config-download' to download from server"; \
	fi

setup-remote-passwordless: config-remote-generate deploy-passwordless
	@echo "🎉 Complete passwordless setup finished!"
	@echo "   Server: $(REMOTE_USER)@$(REMOTE_HOST)"
	@echo "   Configuration: /opt/worker/config/ (with embedded certificates)"
	@echo "   Service: worker.service"
	@echo ""
	@echo "📥 Next steps:"
	@echo "   make config-download  # Download client configuration"
	@echo "   ./bin/cli list        # Test connection"
	@echo "   ./bin/cli run echo 'Hello World'"

setup-dev: config-generate all
	@echo "🎉 Development setup complete!"
	@echo "   Configuration: ./config/ (with embedded certificates)"
	@echo "   Binaries: ./bin/"
	@echo ""
	@echo "🚀 To test locally:"
	@echo "   ./bin/worker  # Start server (uses config/server-config.yml)"
	@echo "   ./bin/cli list  # Connect as client (uses config/client-config.yml)"

config-check-remote:
	@echo "🔍 Checking configuration status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📁 Checking directory structure..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/worker/ || echo 'Directory /opt/worker/ not found'"
	@echo "📋 Checking configuration files..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/worker/config/ || echo 'Configuration directory not found'"
	@echo "🔐 Checking embedded certificates in server config..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo grep -c 'BEGIN CERTIFICATE' /opt/worker/config/server-config.yml 2>/dev/null | xargs echo 'Certificates found:' || echo 'No embedded certificates found'"

service-status:
	@echo "📊 Checking service status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo systemctl status worker.service --no-pager"

test-connection:
	@echo "🔍 Testing connection to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '✅ SSH connection successful'"
	@echo "📊 Checking if worker service exists..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "systemctl list-units --type=service | grep worker || echo '❌ worker service not found'"

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
		echo "📋 Checking worker user configuration..."; \
		if ! grep -q "worker:" /etc/subuid; then \
			echo "❌ worker not configured in /etc/subuid"; \
			exit 1; \
		fi; \
		if ! grep -q "worker:" /etc/subgid; then \
			echo "❌ worker not configured in /etc/subgid"; \
			exit 1; \
		fi; \
		echo "✅ All user namespace requirements validated successfully!"'

setup-user-namespaces:
	@echo "🚀 Setting up user namespace environment on $(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		echo "📋 Creating worker user if not exists..."; \
		if ! id worker >/dev/null 2>&1; then \
			echo "Creating worker user..."; \
			sudo useradd -r -s /bin/false worker; \
			echo "✅ worker user created"; \
		else \
			echo "✅ worker user already exists"; \
		fi; \
		echo "📋 Creating subuid/subgid files if needed..."; \
		sudo touch /etc/subuid /etc/subgid; \
		echo "📋 Setting up subuid/subgid ranges..."; \
		if ! grep -q "^worker:" /etc/subuid 2>/dev/null; then \
			echo "worker:100000:6553600" | sudo tee -a /etc/subuid; \
			echo "✅ Added subuid entry for worker"; \
		else \
			echo "✅ subuid entry already exists for worker"; \
		fi; \
		if ! grep -q "^worker:" /etc/subgid 2>/dev/null; then \
			echo "worker:100000:6553600" | sudo tee -a /etc/subgid; \
			echo "✅ Added subgid entry for worker"; \
		else \
			echo "✅ subgid entry already exists for worker"; \
		fi; \
		echo "📋 Setting up cgroup permissions..."; \
		sudo mkdir -p /sys/fs/cgroup; \
		sudo chown worker:worker /sys/fs/cgroup 2>/dev/null || echo "Note: Could not change cgroup ownership (may be read-only)"; \
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
		echo "📋 Job-worker user info:"; \
		id worker 2>/dev/null || echo "  worker user not found"; \
		echo "📋 Service status:"; \
		sudo systemctl status worker.service --no-pager --lines=5 2>/dev/null || echo "  Service not found"'

deploy-with-user-namespaces: worker
	@echo "🚀 Deploying with user namespace validation to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📋 Validating remote user namespace support..."
	@$(MAKE) validate-user-namespaces || (echo "❌ User namespace validation failed. Running setup..." && $(MAKE) setup-user-namespaces && $(MAKE) validate-user-namespaces)
	@echo "📤 Uploading binaries..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/worker/build"
	scp bin/worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/worker/build/
	@echo "🔧 Installing with user namespace support..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		sudo systemctl stop worker.service 2>/dev/null || echo "Service not running"; \
		sudo cp /tmp/worker/build/* $(REMOTE_DIR)/; \
		sudo chmod +x $(REMOTE_DIR)/*; \
		sudo chown worker:worker $(REMOTE_DIR)/*; \
		echo "Starting service..."; \
		sudo systemctl start worker.service; \
		echo "Checking service status..."; \
		sleep 2; \
		if sudo systemctl is-active worker.service >/dev/null; then \
			echo "✅ Service started successfully with user namespace support"; \
		else \
			echo "❌ Service failed to start. Checking logs..."; \
			sudo journalctl -u worker.service --no-pager --lines=10; \
		fi'

test-user-namespace-job: config-download
	@echo "🧪 Testing job execution with user namespace isolation..."
	@echo "📋 Creating test jobs to verify isolation..."
	./bin/cli --config config/client-config.yml run whoami || echo "❌ Failed to run whoami job"
	sleep 1
	./bin/cli --config config/client-config.yml run id || echo "❌ Failed to run id job"
	sleep 1
	./bin/cli --config config/client-config.yml run ps aux || echo "❌ Failed to run ps job"
	@echo "✅ Test jobs submitted. Check logs to verify each job runs with different UID:"
	@echo "   Expected: Each job should run as different UID (100000+)"
	@echo "   Expected: Jobs should not see each other's processes"
	@echo "💡 View logs with: make live-log"

# Migration helper targets
migrate-check:
	@echo "🔍 Checking for old certificate files..."
	@if ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls /opt/worker/certs/ 2>/dev/null" > /dev/null; then \
		echo "⚠️  Old certificate directory found at /opt/worker/certs/"; \
		echo "💡 Run 'make migrate-to-embedded' to migrate to embedded certificates"; \
	else \
		echo "✅ No old certificate directory found"; \
	fi

migrate-to-embedded:
	@echo "🔄 Migrating from file-based to embedded certificates..."
	@echo "📦 Backing up old certificates..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo cp -r /opt/worker/certs /opt/worker/certs.backup 2>/dev/null || echo "No old certs to backup"'
	@echo "🔐 Generating new configuration with embedded certificates..."
	@$(MAKE) config-remote-generate
	@echo "🧹 Cleaning up old certificate directory..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo rm -rf /opt/worker/certs'
	@echo "✅ Migration completed! Old certs backed up to /opt/worker/certs.backup"
	@echo "💡 Download new client config with: make config-download"