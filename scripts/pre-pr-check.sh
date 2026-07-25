#!/bin/bash

# Pre-PR verification pipeline
#
# Run this before opening a PR. It validates the working tree end to end on
# this machine's architecture:
#
#   1. Run unit tests (make test)
#   2. Build and deploy the local changes to the joblet service (make deploy)
#   3. Run the e2e suite against the deployed service
#   4. Uninstall joblet completely (--purge)
#   5. Build a .deb from the working tree and install it, verifying the
#      package pipeline works on this host's architecture and OS
#   6. Smoke-test the packaged install (service up, rnx runs a job)
#
# Must run in a real terminal - several steps use sudo.
#
# Environment:
#   E2E_EXCLUDE   Suites to skip, comma-separated patterns.
#                 Default: "02_,09_" (runtime suite needs runtimes built,
#                 GPU suite needs GPU hardware). Set E2E_EXCLUDE="" to run all.

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH=$(go env GOARCH)
E2E_EXCLUDE="${E2E_EXCLUDE-02_,09_}"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

step() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

fail() {
    echo -e "\n${RED}❌ PRE-PR CHECK FAILED: $1${NC}"
    exit 1
}

if [ ! -t 0 ]; then
    echo "⚠️  No terminal detected - sudo prompts will fail. Run this in a real terminal."
fi

step "1/6 Unit tests"
make -C "$ROOT" test || fail "unit tests"

step "2/6 Deploy local changes ($ARCH)"
make -C "$ROOT" deploy || fail "deploy"

step "3/6 Run e2e suite"
if [ -n "$E2E_EXCLUDE" ]; then
    echo -e "${BLUE}Excluding suites: $E2E_EXCLUDE (set E2E_EXCLUDE=\"\" to run all)${NC}"
    SKIP_DEPLOY=1 "$ROOT/tests/e2e/run_tests.sh" -x "$E2E_EXCLUDE" || fail "e2e suite"
else
    SKIP_DEPLOY=1 "$ROOT/tests/e2e/run_tests.sh" || fail "e2e suite"
fi

step "4/6 Uninstall joblet (purge)"
sudo "$ROOT/scripts/uninstall.sh" --purge || fail "uninstall"

step "5/6 Build and install local .deb ($ARCH)"
VERSION=$(cd "$ROOT" && git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0-dev")
(cd "$ROOT" && ./scripts/build-deb.sh "$ARCH" "$VERSION") || fail "package build"
DEB=$(ls -t "$ROOT"/joblet_*_"$ARCH".deb | head -1)
echo -e "${BLUE}Installing: $DEB${NC}"
sudo DEBIAN_FRONTEND=noninteractive dpkg -i "$DEB" || fail "package install"
sudo systemctl start joblet.service || fail "service start"

step "6/6 Smoke test packaged install"
echo -e "${BLUE}Waiting for service readiness (persist socket + gRPC)...${NC}"
ready=0
for i in $(seq 1 30); do
    if [ -S /opt/joblet/run/persist-ipc.sock ] && rnx job list >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 0.5
done
[ "$ready" -eq 1 ] || fail "service not ready after 15s - check: journalctl -u joblet"
systemctl is-active --quiet joblet.service || fail "service not active"
echo -e "${GREEN}✓ Service active and ready${NC}"

[ -f "$HOME/.rnx/rnx-config.yml" ] || fail "~/.rnx/rnx-config.yml not created by installer"
echo -e "${GREEN}✓ ~/.rnx client config installed${NC}"

JOB_ID=$(rnx job run echo "pre-pr smoke test" 2>&1 | grep '^ID:' | awk '{print $2}')
[ -n "$JOB_ID" ] || fail "could not submit smoke-test job"
# On a fresh install the IPC writer's first persist connection can take a few
# seconds (5s retry interval); buffered logs land right after it connects
logs_ok=0
for i in $(seq 1 10); do
    if rnx job log "$JOB_ID" 2>/dev/null | grep -q "pre-pr smoke test"; then
        logs_ok=1
        break
    fi
    sleep 2
done
[ "$logs_ok" -eq 1 ] || fail "smoke-test job produced no output after 20s"
echo -e "${GREEN}✓ Job execution works (job $JOB_ID)${NC}"

echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ PRE-PR CHECK PASSED${NC}"
echo -e "${GREEN}  Deploy + e2e + clean packaged install verified on $(uname -m)/$(. /etc/os-release && echo "$ID $VERSION_ID")${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
