#!/bin/bash

# Pre-PR verification pipeline
#
# Run this before opening a PR. It validates the working tree end to end on
# this machine's architecture:
#
#   1. Run unit tests (make test)
#   2. Run the e2e suite via tests/e2e/run_tests.sh, which purges any
#      existing install, builds and installs a .deb from the working tree,
#      and runs every suite against that clean packaged install
#
# Must run in a real terminal - several steps use sudo.
#
# Environment:
#   E2E_EXCLUDE   Suites to skip, comma-separated patterns (default: none;
#                 GPU suites enable simulation themselves and need the sudo
#                 credential the install step already cached).

set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH=$(go env GOARCH)
E2E_EXCLUDE="${E2E_EXCLUDE-}"

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

step "1/2 Unit tests"
make -C "$ROOT" test || fail "unit tests"

step "2/2 E2E suite on a clean packaged install ($ARCH)"
if [ -n "$E2E_EXCLUDE" ]; then
    echo -e "${BLUE}Excluding suites: $E2E_EXCLUDE (set E2E_EXCLUDE=\"\" to run all)${NC}"
    "$ROOT/tests/e2e/run_tests.sh" -x "$E2E_EXCLUDE" || fail "e2e suite"
else
    "$ROOT/tests/e2e/run_tests.sh" || fail "e2e suite"
fi

echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✅ PRE-PR CHECK PASSED${NC}"
echo -e "${GREEN}  Unit tests + e2e on a clean packaged install verified on $(uname -m)/$(. /etc/os-release && echo "$ID $VERSION_ID")${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
