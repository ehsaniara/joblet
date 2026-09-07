#!/bin/bash

# E2E Test: Install Hygiene
# Verifies the security guarantees of a packaged install: root-only config
# files, loopback-only client config on the server host, client/server
# certificate properties, enforced mTLS, and per-role authorization.

source "$(dirname "$0")/../lib/test_framework.sh"

test_suite_init "Install Hygiene Tests"

CONFIG_DIR="/opt/joblet/config"
USER_CONFIG="$HOME/.rnx/rnx-config.yml"
GRPC_ADDR="127.0.0.1:50051"
CREATED_JOBS=()

# ============================================
# Test Helpers
# ============================================

# Permissions and owner of a file, readable without root since the
# directory is traversable
file_mode_owner() {
    stat -c '%a %U' "$1" 2>/dev/null
}

# Extract the first PEM block of a key ("cert" or "ca") for a node from
# the user config
extract_pem() {
    local node="$1" key="$2"
    awk -v node="  $node:" -v key="    $key: |" '
        $0 == node {n=1}
        n && $0 == key {p=1; next}
        p {sub(/^ +/,""); print; if (/END/) exit}
    ' "$USER_CONFIG"
}

# ============================================
# 1. Config File Permissions
# ============================================

test_section "Config File Permissions"

test_server_configs_root_only() {
    local file mode_owner
    for file in joblet-config.yml rnx-config.yml \
        rnx-config-admin.yml rnx-config-maintainer.yml \
        rnx-config-developer.yml rnx-config-reader.yml; do
        mode_owner=$(file_mode_owner "$CONFIG_DIR/$file")
        if [[ "$mode_owner" != "600 root" ]]; then
            echo "    ✗ $file is '$mode_owner', expected '600 root'"
            return 1
        fi
    done
    echo "    All server config files are 600 root"
    return 0
}
run_test "Server config files are root-only (600)" test_server_configs_root_only

test_user_config_private() {
    if [[ ! -f "$USER_CONFIG" ]]; then
        echo "    ✗ $USER_CONFIG not created by installer"
        return 1
    fi
    local mode_owner
    mode_owner=$(file_mode_owner "$USER_CONFIG")
    if [[ "$mode_owner" != "600 $USER" ]]; then
        echo "    ✗ $USER_CONFIG is '$mode_owner', expected '600 $USER'"
        return 1
    fi
    echo "    $USER_CONFIG is 600 $USER"
    return 0
}
run_test "User client config exists and is private (600)" test_user_config_private

# ============================================
# 2. Same-Host Client Config Shape
# ============================================

test_section "Same-Host Client Config Shape"

test_all_nodes_loopback() {
    local addresses
    addresses=$(grep -E '^\s+address:' "$USER_CONFIG" | grep -cv "\"$GRPC_ADDR\"")
    if [[ "$addresses" -ne 0 ]]; then
        echo "    ✗ $addresses node(s) do not point at $GRPC_ADDR"
        grep -E '^\s+address:' "$USER_CONFIG"
        return 1
    fi
    echo "    Every node connects via $GRPC_ADDR"
    return 0
}
run_test "All role nodes connect via loopback" test_all_nodes_loopback

test_roles_and_single_default() {
    local role
    for role in admin maintainer developer reader; do
        if ! grep -q "^  $role:" "$USER_CONFIG"; then
            echo "    ✗ role node '$role' missing from config"
            return 1
        fi
    done
    local defaults
    defaults=$(grep -c 'isDefault: true' "$USER_CONFIG")
    if [[ "$defaults" -ne 1 ]]; then
        echo "    ✗ expected exactly 1 default node, found $defaults"
        return 1
    fi
    echo "    All four roles present with a single default node"
    return 0
}
run_test "All four role nodes present with one default" test_roles_and_single_default

# ============================================
# 3. Certificate Hygiene
# ============================================

test_section "Certificate Hygiene"

test_client_cert_properties() {
    local cert_file
    cert_file=$(mktemp)
    extract_pem admin cert > "$cert_file"
    local text
    text=$(openssl x509 -in "$cert_file" -noout -text 2>/dev/null)
    rm -f "$cert_file"
    if [[ -z "$text" ]]; then
        echo "    ✗ could not parse admin client certificate from config"
        return 1
    fi
    if ! echo "$text" | grep -q 'Version: 3'; then
        echo "    ✗ client certificate is not X.509 v3"
        return 1
    fi
    if ! echo "$text" | grep -q 'TLS Web Client Authentication'; then
        echo "    ✗ client certificate lacks clientAuth extended key usage"
        return 1
    fi
    if ! echo "$text" | grep -q 'CA:FALSE'; then
        echo "    ✗ client certificate lacks basicConstraints CA:FALSE"
        return 1
    fi
    echo "    Client certificate is v3 with clientAuth EKU and CA:FALSE"
    return 0
}
run_test "Client certificate has proper v3 extensions" test_client_cert_properties

test_server_cert_sans() {
    local cert_file
    cert_file=$(mktemp)
    echo | timeout 10 openssl s_client -connect "$GRPC_ADDR" 2>/dev/null |
        openssl x509 -outform PEM > "$cert_file" 2>/dev/null
    local text
    text=$(openssl x509 -in "$cert_file" -noout -text 2>/dev/null)
    rm -f "$cert_file"
    if [[ -z "$text" ]]; then
        echo "    ✗ could not fetch server certificate from $GRPC_ADDR"
        return 1
    fi
    local san
    for san in 'IP Address:127.0.0.1' 'DNS:localhost'; do
        if ! echo "$text" | grep -q "$san"; then
            echo "    ✗ server certificate SANs missing $san"
            return 1
        fi
    done
    if echo "$text" | grep -q 'IP Address:0.0.0.0'; then
        echo "    ✗ server certificate SANs contain the meaningless 0.0.0.0"
        return 1
    fi
    if ! echo "$text" | grep -q 'TLS Web Server Authentication'; then
        echo "    ✗ server certificate lacks serverAuth extended key usage"
        return 1
    fi
    echo "    Server certificate covers loopback and has serverAuth EKU"
    return 0
}
run_test "Server certificate SANs cover loopback" test_server_cert_sans

test_server_requires_client_cert() {
    local handshake
    handshake=$(echo | timeout 10 openssl s_client -connect "$GRPC_ADDR" 2>/dev/null)
    if ! echo "$handshake" | grep -q 'Acceptable client certificate CA names'; then
        echo "    ✗ server did not request a client certificate (mTLS off?)"
        return 1
    fi
    echo "    Server requests client certificates during the handshake"
    return 0
}
run_test "Server enforces mutual TLS" test_server_requires_client_cert

# ============================================
# 4. Role Authorization Boundaries
# ============================================

test_section "Role Authorization Boundaries"

test_reader_boundaries() {
    if ! "$RNX_BINARY" --node reader job list > /dev/null 2>&1; then
        echo "    ✗ reader could not list jobs"
        return 1
    fi
    local output
    output=$("$RNX_BINARY" --node reader job run echo should-be-denied 2>&1)
    if ! echo "$output" | grep -q 'PermissionDenied'; then
        echo "    ✗ reader was not denied job run:"
        echo "$output" | head -2
        return 1
    fi
    echo "    Reader can list jobs but cannot run them"
    return 0
}
run_test "Reader role is read-only" test_reader_boundaries

test_developer_boundaries() {
    local output job_id
    output=$("$RNX_BINARY" --node developer job run echo developer-can-run 2>&1)
    job_id=$(echo "$output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E '^ID:' | awk '{print $2}')
    if [[ -z "$job_id" ]]; then
        echo "    ✗ developer could not run a job:"
        echo "$output" | head -2
        return 1
    fi
    CREATED_JOBS+=("$job_id")
    output=$("$RNX_BINARY" --node developer volume remove no-such-volume 2>&1)
    if ! echo "$output" | grep -q 'PermissionDenied'; then
        echo "    ✗ developer was not denied volume remove:"
        echo "$output" | head -2
        return 1
    fi
    echo "    Developer can run jobs but cannot remove infrastructure"
    return 0
}
run_test "Developer role runs jobs, no infra removal" test_developer_boundaries

test_admin_loopback_end_to_end() {
    local marker="install-hygiene-$$"
    local output job_id
    output=$("$RNX_BINARY" job run echo "$marker" 2>&1)
    job_id=$(echo "$output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E '^ID:' | awk '{print $2}')
    if [[ -z "$job_id" ]]; then
        echo "    ✗ failed to run job on the default node:"
        echo "$output" | head -2
        return 1
    fi
    CREATED_JOBS+=("$job_id")
    local i
    for i in $(seq 1 30); do
        if "$RNX_BINARY" job log "$job_id" 2>/dev/null | grep -q "$marker"; then
            echo "    Default node ran a job over loopback (output verified after ${i}s)"
            return 0
        fi
        sleep 1
    done
    echo "    ✗ job output not retrievable after 30s"
    return 1
}
run_test "Default node runs a job end-to-end over loopback" test_admin_loopback_end_to_end

# ============================================
# 5. Cleanup
# ============================================

test_section "Cleanup"

test_cleanup_jobs() {
    local job_id
    for job_id in "${CREATED_JOBS[@]}"; do
        "$RNX_BINARY" job delete "$job_id" > /dev/null 2>&1 || true
    done
    echo "    Removed ${#CREATED_JOBS[@]} test job(s)"
    return 0
}
run_test "Test jobs cleaned up" test_cleanup_jobs

# ============================================
# Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Install Hygiene Test Results${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  Total:   $TOTAL_TESTS"
echo -e "  ${GREEN}Passed:  $PASSED_TESTS${NC}"
echo -e "  ${RED}Failed:  $FAILED_TESTS${NC}"
echo -e "  ${YELLOW}Skipped: $SKIPPED_TESTS${NC}"

if [[ $FAILED_TESTS -gt 0 ]]; then
    exit 1
fi
