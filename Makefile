REMOTE_HOST ?= 192.168.1.161
REMOTE_USER ?= jay
REMOTE_DIR ?= /opt/job-worker

.PHONY: all clean cli worker init deploy deploy-safe test help

all: cli worker init

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
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "mkdir -p /tmp/job-worker/build"
	scp bin/job-worker $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	scp bin/job-init $(REMOTE_USER)@$(REMOTE_HOST):/tmp/job-worker/build/
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'sudo systemctl stop job-worker.service && sudo cp /tmp/job-worker/build/* $(REMOTE_DIR)/ && sudo chmod +x $(REMOTE_DIR)/* && sudo systemctl start job-worker.service && echo "Deployed successfully"'

live-log:
	ssh $(REMOTE_USER)@$(REMOTE_HOST) 'journalctl -u job-worker.service -f'

clean:
	@echo "Cleaning..."
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

certs-remote:
	@echo "🔐 Generating certificates on $(REMOTE_USER)@$(REMOTE_HOST)..."
	@echo "🔍 Checking for certificate script..."
	@ls -la ./etc/certs_gen.sh || (echo "❌ ./etc/certs_gen.sh not found"; exit 1)
	@echo "📤 Uploading certificate generation script..."
	scp ./etc/certs_gen.sh $(REMOTE_USER)@$(REMOTE_HOST):/tmp/
	@echo "🏗️  Generating certificates on remote server..."
	@read -s -p "Enter sudo password for $(REMOTE_USER)@$(REMOTE_HOST): " SUDO_PASS; \
	echo ""; \
	ssh $(REMOTE_USER)@$(REMOTE_HOST) "echo '$$SUDO_PASS' | sudo -S bash -c '\
		chmod +x /tmp/certs_gen.sh; \
		/tmp/certs_gen.sh; \
		echo \"\"; \
		echo \"📋 Certificate files created:\"; \
		ls -la /opt/job-worker/certs/ 2>/dev/null || echo \"No certificates found\"; \
		rm -f /tmp/certs_gen.sh'"
	@echo "✅ Remote certificates generated!"

certs-download-admin:
	@echo "📥 Downloading Admin certificates from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p certs-remote
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/ca-cert.pem certs/ca-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-cert.pem certs/client-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/admin-client-key.pem certs/client-key.pem
	@echo "✅ Admin Certificates downloaded to ./certs/"
	@echo "💡 Usage: ./cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem"

certs-download-viewer:
	@echo "📥 Downloading Viewer certificates from $(REMOTE_USER)@$(REMOTE_HOST)..."
	@mkdir -p certs-remote
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/ca-cert.pem certs/ca-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/viewer-client-cert.pem certs/client-cert.pem
	scp $(REMOTE_USER)@$(REMOTE_HOST):/opt/job-worker/certs/viewer-client-key.pem certs/client-key.pem
	@echo "✅ Viewer Certificates downloaded to ./certs/"
	@echo "💡 Usage: ./cli --server $(REMOTE_HOST):50051 --cert certs/client-cert.pem --key certs/client-key.pem"
