# Joblet End-to-End Test Suite

Comprehensive end-to-end test suite for Joblet's isolation, networking, runtime management, and functionality with
team-configurable parameters.

## Quick Start

```bash
# 🚀 RECOMMENDED: Full confidence testing (builds + deploys + tests)
./run_tests.sh

# 🎯 Specific test suites with full build/deploy
./run_tests.sh -t isolation    # Core isolation tests
./run_tests.sh -t runtime      # Runtime management tests  
./run_tests.sh -t network      # Network configuration tests
./run_tests.sh -t schedule     # Job scheduling tests
./run_tests.sh -t volume       # Volume management tests
./run_tests.sh -t workflow     # Workflow dependency tests

# ⚡ Development/debugging (skip build - use cautiously)
./run_tests.sh --no-build      # Skip build for rapid test iteration

# 📋 List available tests
./run_tests.sh --list

# 🔧 Individual test suites (for debugging only)
./tests/01_isolation_test.sh
./tests/02_runtime_test.sh
# ... etc
```

## Configuration

Set environment variables to customize for your team:

```bash
# Connection
export RNX_HOST=server.company.com     # Default: localhost
export RNX_USER=username               # Default: current user
export RNX_CONFIG=path/to/config.yml   # Default: config/rnx-config.yml

# Testing
export TEST_NETWORK_CIDR_BASE=10.150   # Default: 10.200 (avoid IP conflicts)
export TEST_RUNTIME=ubuntu:latest      # Default: python:3.11-ml
export TEST_VERBOSE=true               # Default: false (enable detailed output)
export TEST_SAVE_LOGS=true             # Default: false (save logs to files)
```

### Team Examples

```bash
# Developer setup
export RNX_HOST=localhost TEST_VERBOSE=true TEST_SAVE_LOGS=true

# Production validation  
export RNX_HOST=prod.company.com RNX_USER=readonly TEST_PARALLEL_JOBS=1

# CI/CD pipeline
export RNX_HOST=${{secrets.TEST_SERVER}} TEST_SAVE_LOGS=true TEST_LOG_DIR=./logs

# Environment files
echo 'export RNX_HOST=dev-server.internal' > config/team-dev.env
source config/team-dev.env && ./run_all_tests.sh
```

## Test Categories

| Test                  | Command                              | What it tests                                          |
|-----------------------|--------------------------------------|--------------------------------------------------------|
| **Core Isolation**    | `./test_joblet_principles.sh`        | PID namespaces, filesystem isolation, resource limits  |
| **Network Isolation** | `./network/run-all-network-tests.sh` | Bridge/isolated/none networks, inter-job communication |
| **Volume Management** | `./volume/run-all-volume-tests.sh`   | Volume creation, mounting, persistence, data sharing   |
| **Logging**           | `./test_logging_integrity.sh`        | Log streaming, buffer management                       |
| **Visual Validation** | `./final_isolation_test.sh`          | Interactive isolation verification                     |

## Network Tests

Located in `network/` directory:

- **Bridge Network** (172.20.0.x/16): Jobs can communicate with each other
- **Isolated Network**: Complete network isolation, no external access
- **None Network**: Total network isolation, minimal setup
- **Inter-Job Communication**: TCP connectivity testing between jobs

Individual tests: `./network/test-*.sh`

## Volume Tests

Located in `volume/` directory:

- **Volume Creation**: Filesystem and memory volume types with size specifications
- **Volume Mounting**: Single and multiple volume mounting in jobs
- **Data Persistence**: Data persistence across job runs and restarts
- **Inter-Job Sharing**: Data sharing between different jobs via volumes
- **Size Management**: Volume size limits, usage monitoring, performance testing

Individual tests: `./volume/test-*.sh`

## Expected Results

- **Isolation**: Jobs see only own processes, isolated filesystem, enforced limits
- **Networking**: Bridge jobs communicate, isolated jobs properly separated
- **Integration**: All jobs complete, real-time logs, automatic cleanup

## Configuration Validation

```bash
source test-config.sh    # Auto-validates on load
show_config              # Display current settings
validate_config          # Manual validation
```

## Troubleshooting

| Issue              | Solution                                             |
|--------------------|------------------------------------------------------|
| Connection refused | Check `RNX_HOST` and `RNX_CONFIG` settings           |
| Network conflicts  | Use `TEST_NETWORK_CIDR_BASE=10.150`                  |
| Runtime not found  | Check `./bin/rnx runtime list`, set `TEST_RUNTIME`   |
| Permission denied  | Ensure user namespaces enabled, check sudo setup     |
| Jobs hanging       | Check `journalctl -u joblet --since "5 minutes ago"` |

```bash
# Debug commands
./bin/rnx job list                           # Test connection
TEST_VERBOSE=true ./test_joblet_principles.sh  # Detailed output
systemctl status joblet                  # Service status
```

## Development

### Adding Tests

1. Create script in appropriate directory (`network/`, `volume/`, etc.)
2. Source configuration: `source test-config.sh`
3. Use helper functions: `run_rnx`, `wait_for_job_completion`, `cleanup_test_resources`
4. Make executable: `chmod +x new-test.sh`

### Configuration Functions

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-config.sh"

run_rnx job list                          # Configurable rnx command
wait_for_job_completion "$workflow"   # Built-in timeout handling
cleanup_test_resources               # Auto cleanup
log_test_result "Test" "PASSED"      # Consistent logging
```

## File Structure

```
tests/e2e/
├── README.md                 # This guide
├── test-config.sh           # Global configuration
├── run_all_tests.sh         # Main test runner  
├── test_*.sh               # Core tests
├── network/                # Network-specific tests
│   ├── run-all-network-tests.sh
│   ├── test-*.sh          # Individual network tests
│   └── *.yaml            # Test workflows
└── volume/                # Volume tests (future)
```

**Prerequisites**: Joblet server running, user namespaces enabled, RNX client configured

For complete environment variable reference: `source test-config.sh --help`