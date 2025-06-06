REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/job-worker

.PHONY: all clean cli worker init deploy-passwordless deploy-safe certs-local certs-remote-passwordless certs-download-admin certs-download-admin-simple certs-download-viewer live-log help setup-remote-passwordless setup-dev check-certs-remote service-status

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
	@echo "   ./bin/cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem create echo 'Hello World'"

setup-dev: certs-local all
	@echo "🎉 Development setup complete!"
	@echo "   Certificates: ./certs/"
	@echo "   Binaries: ./bin/"
	@echo ""
	@echo "🚀 To test locally:"
	@echo "   ./bin/job-worker  # Start server"
	@echo "   ./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo 'Hello World'"

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