REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/job-worker

.PHONY: all clean cli worker init deploy-passwordless deploy-safe certs-local certs-remote-passwordless certs-download-admin certs-download-admin-simple certs-download-viewer live-log help setup-remote-passwordless setup-dev check-certs-remote service-status validate-user-namespaces setup-user-namespaces check-kernel-support setup-subuid-subgid test-user-namespace-isolation debug-user-namespaces deploy-with-user-namespaces test-user-namespace-job

all: cli worker init

help:
	@echo "Job Worker Makefile"
	@echo ""
	@echo "Build targets:"
	@echo "  make all               - Build all binaries (cli, worker, init)"
	@echo "  make cli               - Build CLI for local development"
	@echo "  make worker            - Build job-worker binary for Linux"
	@echo "  make init              - Build job-init binary for Linux"
	@echo "  make clean             - Remove build artifacts"
	@echo ""
	@echo "User Namespace Setup:"
	@echo "  make validate-user-namespaces  - Check user namespace support"
	@echo "  make setup-user-namespaces     - Setup user namespace environment"
	@echo "  make debug-user-namespaces     - Debug user namespace issues"
	@echo "  make deploy-with-user-namespaces - Deploy with user namespace validation"
	@echo "  make test-user-namespace-job   - Test job isolation"
	@echo ""
	@echo "Deployment targets:"
	@echo "  make deploy-passwordless - Deploy without password (requires sudo setup)"
	@echo "  make deploy-safe       - Deploy with password prompt (safe)"
	@echo ""
	@echo "Certificate targets:"
	@echo "  make certs-local       - Generate certificates locally (./certs/)"
	@echo "  make certs-remote-passwordless - Generate certificates on remote server (passwordless)"
	@echo "  make certs-download-admin - Download admin client certificates (with sudo)"
	@echo "  make certs-download-admin-simple - Download admin certificates (no sudo)"
	@echo "  make certs-download-viewer - Download viewer client certificates"
	@echo ""
	@echo "Quick setup:"
	@echo "  make setup-remote-passwordless - Complete passwordless setup"
	@echo "  make setup-dev         - Development setup"
	@echo ""
	@echo "Debugging:"
	@echo "  make check-certs-remote - Check certificate status on server"
	@echo "  make examine-certs     - Examine local and remote certificates"
	@echo "  make examine-server-cert - Detailed server certificate examination"
	@echo "  make verify-cert-chain - Verify certificate chain validity"
	@echo "  make test-tls          - Test TLS connection to server"
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
	@echo "  make certs-download-admin-simple"
	@echo "  make setup-remote-passwordless"

cli:
	@echo "Building CLI..."
	GOOS=darwin GOARCH=amd64 go build -o bin/cli ./cmd/cli

worker:
	@echo "Building job-worker..."
	GOOS=linux GOARCH=amd64 go build -o bin/job-worker ./cmd/worker

init:
	@echo "Building job-init..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/job-init ./cmd/job-init

deploy-passwordless: worker init
	@echo "🚀 Passwordless deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/job-worker/build"
	scp bin/job-worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	scp bin/job-init $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop job-worker.service && sudo cp /tmp/job-worker/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo systemctl start job-worker.service && echo "✅ Deployed successfully"'

deploy-safe: worker init
	@echo "🔐 Safe deployment to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/job-worker/build"
	scp bin/job-worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	scp bin/job-init $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	@echo "Files uploaded. Installing with sudo..."
	@read -s -p "Enter sudo password for $(REMOTE_USER)@$(REMOTE_HOST): " SUDO_PASS; \
	echo ""; \
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '$$SUDO_PASS' | sudo -S bash -c '\
		echo \"Stopping service...\"; \
		systemctl stop job-worker.service 2>/dev/null || echo \"Service not running\"; \
		echo \"Installing binaries...\"; \
		cp /tmp/job-worker/build/job-worker $(REMOTE_DIR)/; \
		cp /tmp/job-worker/build/job-init $(REMOTE_DIR)/; \
		chmod +x $(REMOTE_DIR)/job-worker $(REMOTE_DIR)/job-init; \
		echo \"Starting service...\"; \
		systemctl start job-worker.service; \
		echo \"Checking service status...\"; \
		systemctl is-active job-worker.service >/dev/null && echo \"✅ Service started successfully\" || echo \"❌ Service failed to start\"'"

live-log:
	@echo "📊 Viewing live logs from $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'journalctl -u job-worker.service -f'

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/

certs-local:
	@echo "🔐 Generating certificates locally..."
	@if [ ! -f ./etc/certs_gen.sh ]; then \
		echo "❌ ./etc/certs_gen.sh script not found"; \
		exit 1; \
	fi
	@chmod +x ./etc/certs_gen.sh
	@./etc/certs_gen.sh
	@echo "✅ Local certificates generated in ./certs/"

certs-remote-passwordless:
	@echo "🔐 Generating certificates on $(REMOTE_USER)@$(REMOTE_HOST) (passwordless)..."
	@if [ ! -f ./etc/certs_gen.sh ]; then \
		echo "❌ ./etc/certs_gen.sh script not found"; \
		exit 1; \
	fi
	@echo "📤 Uploading certificate generation script..."
	scp ./etc/certs_gen.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@echo "🏗️  Generating certificates on remote server..."
	@echo "⚠️  Note: This requires passwordless sudo to be configured"
	ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		chmod +x /tmp/certs_gen.sh; \
		sudo /tmp/certs_gen.sh; \
		echo ""; \
		echo "📋 Certificate files created:"; \
		sudo ls -la /opt/job-worker/certs/ 2>/dev/null || echo "No certificates found"; \
		rm -f /tmp/certs_gen.sh'
	@echo "✅ Remote certificates generated!"

certs-download-admin:
	@echo "📥 Downloading Admin certificates from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p certs
	@echo "🔧 Fixing certificate permissions on server..."
	ssh -t $(REMOTE_USER)@$(REMOTE_HOST) "sudo chown jay /opt/job-worker/certs/ca-cert.pem /opt/job-worker/certs/admin-client-cert.pem /opt/job-worker/certs/admin-client-key.pem"
	ssh -t $(REMOTE_USER)@$(REMOTE_HOST) "sudo chmod 644 /opt/job-worker/certs/ca-cert.pem /opt/job-worker/certs/admin-client-cert.pem /opt/job-worker/certs/admin-client-key.pem"
	@echo "📥 Downloading certificates..."
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/ca-cert.pem certs/ca-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-cert.pem certs/client-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-key.pem certs/client-key.pem
	@echo "✅ Admin Certificates downloaded to ./certs/"
	@echo "💡 Usage: ./bin/cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem"

certs-download-admin-simple:
	@echo "📥 Simple download of Admin certificates from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p certs
	@echo "📥 Downloading certificates (assuming permissions are correct)..."
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/ca-cert.pem certs/ca-cert.pem || echo "❌ Failed to download ca-cert.pem"
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-cert.pem certs/client-cert.pem || echo "❌ Failed to download admin-client-cert.pem"
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-key.pem certs/client-key.pem || echo "❌ Failed to download admin-client-key.pem"
	@echo "✅ Download attempt completed. Check for any error messages above."
	@echo "💡 Usage: ./bin/cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem"

certs-download-viewer:
	@echo "📥 Downloading Viewer certificates from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p certs
	@echo "🔧 Fixing certificate permissions on server..."
	ssh -t $(REMOTE_USER)@$(REMOTE_HOST) "sudo chown jay /opt/job-worker/certs/ca-cert.pem /opt/job-worker/certs/viewer-client-cert.pem /opt/job-worker/certs/viewer-client-key.pem"
	ssh -t $(REMOTE_USER)@$(REMOTE_HOST) "sudo chmod 644 /opt/job-worker/certs/ca-cert.pem /opt/job-worker/certs/viewer-client-cert.pem /opt/job-worker/certs/viewer-client-key.pem"
	@echo "📥 Downloading certificates..."
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/ca-cert.pem certs/ca-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/viewer-client-cert.pem certs/client-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/viewer-client-key.pem certs/client-key.pem
	@echo "✅ Viewer Certificates downloaded to ./certs/"
	@echo "💡 Usage: ./bin/cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem"

setup-remote-passwordless: certs-remote-passwordless deploy-passwordless
	@echo "🎉 Complete passwordless setup finished!"
	@echo "   Server: $(REMOTE_USER)@$(REMOTE_HOST)"
	@echo "   Certificates: /opt/job-worker/certs/"
	@echo "   Service: job-worker.service"
	@echo ""
	@echo "📥 Next steps:"
	@echo "   make certs-download-admin-simple  # Download admin certificates"
	@echo "   ./bin/cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem run echo 'Hello World'"

setup-dev: certs-local all
	@echo "🎉 Development setup complete!"
	@echo "   Certificates: ./certs/"
	@echo "   Binaries: ./bin/"
	@echo ""
	@echo "🚀 To test locally:"
	@echo "   ./bin/job-worker  # Start server"
	@echo "   ./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem run echo 'Hello World'"

check-certs-remote:
	@echo "🔍 Checking certificate status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📁 Checking directory structure..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/job-worker/ || echo 'Directory /opt/job-worker/ not found'"
	@echo "📋 Checking certificate files..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo ls -la /opt/job-worker/certs/ || echo 'Certificate directory not found'"

service-status:
	@echo "📊 Checking service status on $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo systemctl status job-worker.service --no-pager"

fix-cert-permissions:
	@echo "🔧 Fixing certificate permissions on $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "sudo chown jay /opt/job-worker/certs/*.pem && sudo chmod 644 /opt/job-worker/certs/*.pem"
	@echo "✅ Certificate permissions fixed!"

test-connection:
	@echo "🔍 Testing connection to $(REMOTE_USER)@$(REMOTE_HOST)..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '✅ SSH connection successful'"
	@echo "📊 Checking if job-worker service exists..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "systemctl list-units --type=service | grep job-worker || echo '❌ job-worker service not found'"

examine-certs:
	@echo "🔍 Examining certificates..."
	@echo ""
	@echo "📋 LOCAL CERTIFICATES:"
	@if [ -f certs/ca-cert.pem ]; then \
		echo "✅ Local CA certificate:"; \
		openssl x509 -in certs/ca-cert.pem -noout -subject -issuer -dates; \
		echo ""; \
	else \
		echo "❌ No local CA certificate found"; \
	fi
	@if [ -f certs/client-cert.pem ]; then \
		echo "✅ Local client certificate:"; \
		openssl x509 -in certs/client-cert.pem -noout -subject -issuer -dates; \
		echo "   Client Role (OU): $(openssl x509 -in certs/client-cert.pem -noout -subject | grep -o 'OU=[^/,]*' | cut -d= -f2)"; \
		echo ""; \
	else \
		echo "❌ No local client certificate found"; \
	fi
	@echo "📋 REMOTE SERVER CERTIFICATES:"
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		if [ -f /opt/job-worker/certs/server-cert.pem ]; then \
			echo "✅ Remote server certificate:"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -subject -issuer -dates; \
			echo "   🌐 Subject Alternative Names (SAN):"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -text | grep -A 10 "Subject Alternative Name" | grep -E "(DNS:|IP Address:)" || echo "   ❌ No SAN found"; \
			echo ""; \
		else \
			echo "❌ No remote server certificate found"; \
		fi'

examine-server-cert:
	@echo "🔍 Detailed examination of server certificate on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		if [ -f /opt/job-worker/certs/server-cert.pem ]; then \
			echo "📋 Certificate Subject:"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -subject; \
			echo ""; \
			echo "📅 Certificate Validity:"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -dates; \
			echo ""; \
			echo "🌐 Subject Alternative Names (SAN):"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -text | grep -A 20 "Subject Alternative Name" || echo "   ❌ No SAN extension found"; \
			echo ""; \
			echo "🔑 Certificate Key Usage:"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -text | grep -A 5 "Key Usage" || echo "   ❌ No Key Usage found"; \
			echo ""; \
			echo "🎯 Extended Key Usage:"; \
			openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -text | grep -A 5 "Extended Key Usage" || echo "   ❌ No Extended Key Usage found"; \
		else \
			echo "❌ Server certificate not found at /opt/job-worker/certs/server-cert.pem"; \
		fi'

test-tls:
	@echo "🔐 Testing TLS connection to $(REMOTE_HOST):50051..."
	@echo "📡 Attempting to connect and examine server certificate..."
	@echo | openssl s_client -connect $(REMOTE_HOST):50051 -servername $(REMOTE_HOST) 2>/dev/null | openssl x509 -noout -text | grep -A 20 "Subject Alternative Name" || echo "❌ Failed to connect or no SAN found"

verify-cert-chain:
	@echo "🔗 Verifying certificate chain on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		cd /opt/job-worker/certs && \
		if [ -f ca-cert.pem ] && [ -f server-cert.pem ]; then \
			echo "✅ Verifying server certificate against CA:"; \
			openssl verify -CAfile ca-cert.pem server-cert.pem; \
			echo ""; \
			if [ -f admin-client-cert.pem ]; then \
				echo "✅ Verifying admin client certificate against CA:"; \
				openssl verify -CAfile ca-cert.pem admin-client-cert.pem; \
			fi; \
			if [ -f viewer-client-cert.pem ]; then \
				echo "✅ Verifying viewer client certificate against CA:"; \
				openssl verify -CAfile ca-cert.pem viewer-client-cert.pem; \
			fi; \
		else \
			echo "❌ Missing certificates for verification"; \
		fi'

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
		echo "📋 Checking job-worker user configuration..."; \
		if ! grep -q "job-worker:" /etc/subuid; then \
			echo "❌ job-worker not configured in /etc/subuid"; \
			exit 1; \
		fi; \
		if ! grep -q "job-worker:" /etc/subgid; then \
			echo "❌ job-worker not configured in /etc/subgid"; \
			exit 1; \
		fi; \
		echo "✅ All user namespace requirements validated successfully!"'

setup-user-namespaces:
	@echo "🚀 Setting up user namespace environment on $(REMOTE_HOST)..."
	@ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		echo "📋 Creating job-worker user if not exists..."; \
		if ! id job-worker >/dev/null 2>&1; then \
			echo "Creating job-worker user..."; \
			sudo useradd -r -s /bin/false job-worker; \
			echo "✅ job-worker user created"; \
		else \
			echo "✅ job-worker user already exists"; \
		fi; \
		echo "📋 Creating subuid/subgid files if needed..."; \
		sudo touch /etc/subuid /etc/subgid; \
		echo "📋 Setting up subuid/subgid ranges..."; \
		if ! grep -q "^job-worker:" /etc/subuid 2>/dev/null; then \
			echo "job-worker:100000:6553600" | sudo tee -a /etc/subuid; \
			echo "✅ Added subuid entry for job-worker"; \
		else \
			echo "✅ subuid entry already exists for job-worker"; \
		fi; \
		if ! grep -q "^job-worker:" /etc/subgid 2>/dev/null; then \
			echo "job-worker:100000:6553600" | sudo tee -a /etc/subgid; \
			echo "✅ Added subgid entry for job-worker"; \
		else \
			echo "✅ subgid entry already exists for job-worker"; \
		fi; \
		echo "📋 Setting up cgroup permissions..."; \
		sudo mkdir -p /sys/fs/cgroup; \
		sudo chown job-worker:job-worker /sys/fs/cgroup 2>/dev/null || echo "Note: Could not change cgroup ownership (may be read-only)"; \
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
		id job-worker 2>/dev/null || echo "  job-worker user not found"; \
		echo "📋 Service status:"; \
		sudo systemctl status job-worker.service --no-pager --lines=5 2>/dev/null || echo "  Service not found"'

deploy-with-user-namespaces: worker init
	@echo "🚀 Deploying with user namespace validation to $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "📋 Validating remote user namespace support..."
	@$(MAKE) validate-user-namespaces || (echo "❌ User namespace validation failed. Running setup..." && $(MAKE) setup-user-namespaces && $(MAKE) validate-user-namespaces)
	@echo "📤 Uploading binaries..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/job-worker/build"
	scp bin/job-worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	scp bin/job-init $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	@echo "🔧 Installing with user namespace support..."
	ssh $(REMOTE_USER)@$(REMOTE_HOST) '\
		sudo systemctl stop job-worker.service 2>/dev/null || echo "Service not running"; \
		sudo cp /tmp/job-worker/build/* $(REMOTE_DIR)/; \
		sudo chmod +x $(REMOTE_DIR)/*; \
		sudo chown job-worker:job-worker $(REMOTE_DIR)/*; \
		echo "Starting service..."; \
		sudo systemctl start job-worker.service; \
		echo "Checking service status..."; \
		sleep 2; \
		if sudo systemctl is-active job-worker.service >/dev/null; then \
			echo "✅ Service started successfully with user namespace support"; \
		else \
			echo "❌ Service failed to start. Checking logs..."; \
			sudo journalctl -u job-worker.service --no-pager --lines=10; \
		fi'

test-user-namespace-job: certs-download-admin-simple
	@echo "🧪 Testing job execution with user namespace isolation..."
	@echo "📋 Creating test jobs to verify isolation..."
	./bin/cli --server $(REMOTE_HOST):50051 run whoami || echo "❌ Failed to run whoami job"
	sleep 1
	./bin/cli --server $(REMOTE_HOST):50051 run id || echo "❌ Failed to run id job"
	sleep 1
	./bin/cli --server $(REMOTE_HOST):50051 run "ps aux | head -10" || echo "❌ Failed to run ps job"
	@echo "✅ Test jobs submitted. Check logs to verify each job runs with different UID:"
	@echo "   Expected: Each job should run as different UID (100000+)"
	@echo "   Expected: Jobs should not see each other's processes"
	@echo "💡 View logs with: make live-log"