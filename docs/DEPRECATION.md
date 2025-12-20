# Deprecation Timeline and Migration Guide

This document outlines deprecated features, their replacement alternatives, and removal timelines for the Joblet
project.

## Overview

As Joblet evolves, certain features become obsolete or are replaced by better alternatives. This document helps users
migrate away from deprecated functionality before it's removed in future major versions.

## Deprecation Policy

- **Current Version (v5.0.0)**: All previously deprecated features have been removed
- **Clean Codebase**: No backward compatibility code remains
- **Breaking Changes**: v5.0.0 is a major release with breaking changes (see V5_CLEANUP_SUMMARY.md)

---

## Removed Features (v5.0.0)

All features listed below have been **REMOVED** in v5.0.0. See migration guide at the end.

### 1. JobServiceServer

**Status**: ✅ **ACTIVE** in v5.0.0

**Current Location**: `internal/joblet/server/job_service.go`

**Description**:
The `JobServiceServer` handles all job-related gRPC operations including job submission, status queries, and lifecycle
management.

**Current Status**:

- ✅ Implements `JobServiceServer` interface from joblet-proto
- ✅ Used in production (see `internal/joblet/server/grpc_server.go`)
- ✅ Handles individual job execution

**Impact**: None - This is the active implementation

---

### 2. Legacy Job Status Constants

**Status**: ✅ **REMOVED** in v5.0.0

**Removed Constants**:

```go
// REMOVED - No longer available
const (
    JobStatusRunning   = StatusRunning    // ❌ REMOVED
    JobStatusCompleted = StatusCompleted  // ❌ REMOVED
    JobStatusFailed    = StatusFailed     // ❌ REMOVED
    JobStatusScheduled = StatusScheduled  // ❌ REMOVED
    JobStatusStopping  = StatusStopping   // ❌ REMOVED
)
```

**Migration** (Required for v5.0.0):

#### Before (v4.x):

```go
if job.Status == domain.JobStatusRunning {
    // ...
}
```

#### After (v5.0.0):

```go
if job.Status == domain.StatusRunning {
    // ...
}
```

**Impact**: Low - Simple find/replace migration (breaking change)

---

### 3. Sequential ID Generator

**Status**: 🟡 **LEGACY** - Superseded by UUID generation

**Current Location**: `internal/joblet/core/job/id_generator.go:36-43`

**Deprecated Function**:

```go
// NewSequentialIDGenerator creates a legacy sequential ID generator
func NewSequentialIDGenerator(prefix, nodeID string) *UUIDGenerator
```

**Deprecated Methods**:

```go
func (g *UUIDGenerator) NextWithTimestamp() string
func (g *UUIDGenerator) SetHighPrecision(enabled bool)
```

**Reason for Deprecation**:
UUID generation using Linux kernel's native UUID provides:

- Complete immunity to race conditions
- Unlimited concurrency support
- Better distributed system compatibility
- RFC 4122 compliance

**Migration Path**:

#### Before (Deprecated):

```go
// OLD: Sequential ID generation
generator := job.NewSequentialIDGenerator("job", "node1")
jobID := generator.NextWithTimestamp()
```

#### After (Current):

```go
// NEW: UUID generation (default)
generator := job.NewUUIDGenerator("job", "node1")
jobID := generator.Next()
```

**Removal Plan**:

1. **v4.7.3** (Current): Legacy methods maintained for tests
2. **v5.0.0**: Remove sequential generation methods

**Impact**: Low - UUID generation is default, sequential only used in tests

---

### 4. Runtime Init Path Resolution

**Status**: 🔴 **REMOVED** - Functionality moved

**Current Location**: `internal/joblet/core/execution/environment_service.go:167-170`

**Deprecated Method**:

```go
// GetRuntimeInitPath is deprecated - runtime functionality handled by filesystem isolator
func (es *EnvironmentService) GetRuntimeInitPath(ctx context.Context, runtimeSpec string) (string, error) {
    return "", fmt.Errorf("runtime init path resolution is deprecated - handled by filesystem isolator")
}
```

**Migration Path**:

Runtime init path resolution is now handled automatically by the filesystem isolator. No manual path resolution needed.

#### Before (Deprecated):

```go
initPath, err := envService.GetRuntimeInitPath(ctx, "python-3.11")
```

#### After (Current):

```go
// Runtime paths are resolved automatically by filesystem isolator
// No manual intervention needed
```

**Removal Plan**:

1. **v4.7.3** (Current): Method returns error immediately
2. **v5.0.0**: Remove method entirely

**Impact**: None - Already non-functional, filesystem isolator handles this

---

### 5. Separate Secret Environment Flag

**Status**: ✅ **REMOVED** in v5.0.0

**Removed CLI Flag**:

```bash
# OLD - No longer supported
rnx job run --env="PUBLIC_VAR=value" --secret-env="API_KEY=secret" app
```

**Migration** (Required for v5.0.0):

Use single `--env` flag with naming conventions for automatic secret detection.

#### After (v5.0.0 - Required):

```bash
# NEW - Use single --env flag
rnx job run --env="PUBLIC_VAR=value" --env="API_KEY=secret" app
# API_KEY auto-detected as secret by _KEY suffix
```

**Secret Detection** (New in v5.0.0):
Secrets are automatically detected by naming convention:

- `SECRET_*` prefix (e.g., `SECRET_DATABASE_PASSWORD`)
- `*_TOKEN` suffix (e.g., `GITHUB_TOKEN`)
- `*_KEY` suffix (e.g., `API_KEY`)
- `*_PASSWORD` suffix (e.g., `DATABASE_PASSWORD`)
- `*_SECRET` suffix (e.g., `OAUTH_SECRET`)

**Impact**: Medium - Breaking change, CLI usage must be updated

---

## Migration Timeline

### v5.0.0 (Released: 2025-10-13)

**Status**: ✅ **RELEASED**

Breaking Changes Applied:

- ✅ Removed legacy `JobStatus*` constants
- ✅ Removed sequential ID generator methods
- ✅ Removed `GetRuntimeInitPath` method
- ✅ Removed `--secret-env` CLI flag
- ✅ Removed network ready FD fallback (`NETWORK_READY_FD`)
- ✅ Removed legacy Job struct fields (`StartedAt`, `CompletedAt` aliases)
- ✅ Removed workflow orchestration (moved to separate project per ADR-013)
- ✅ Added automatic secret detection by naming convention

Migration Support:

- ✅ Complete migration guide in V5_CLEANUP_SUMMARY.md
- ✅ All replacements documented with examples

---

## How to Migrate to v5.0.0

### 1. Audit Your Code

```bash
# Search for removed constants (will cause compile errors)
grep -r "JobStatusRunning\|JobStatusCompleted\|JobStatusFailed" .

# Search for removed ID generator (will cause compile errors)
grep -r "NewSequentialIDGenerator\|NextWithTimestamp" .

# Search for removed runtime method (will cause compile errors)
grep -r "GetRuntimeInitPath" .
```

### 2. Update Code

Manual updates:

- Replace `JobStatus*` → `Status*` in Go code
- Replace `NewSequentialIDGenerator` → `NewUUIDGenerator`
- Replace `--secret-env` → `--env` with naming conventions for secrets
- Update `NETWORK_READY_FD` → `NETWORK_READY_FILE` in deployment scripts

### 3. Test Changes

```bash
# Run full test suite
go test ./...

# Run E2E tests
./tests/e2e/run_tests.sh
```

### 4. Update Deployment

```bash
# If using NETWORK_READY_FD, switch to NETWORK_READY_FILE
# OLD:
export NETWORK_READY_FD=3

# NEW:
export NETWORK_READY_FILE=/tmp/network-ready
```

---

## Additional Resources

- [V5 Cleanup Summary](../V5_CLEANUP_SUMMARY.md) - Complete v5.0.0 changes and migration guide
- [V5 Deployment Status](../V5_DEPLOYMENT_STATUS.md) - Deployment verification and testing
- [API Documentation](./API.md) - Current API reference
- [GitHub Issues](https://github.com/ehsaniara/joblet/issues) - Report migration problems
- [Changelog](../CHANGELOG.md) - Version-specific changes

---

## Questions or Issues?

If you encounter problems migrating to v5.0.0:

1. Check [V5_CLEANUP_SUMMARY.md](../V5_CLEANUP_SUMMARY.md) for detailed migration guide
2. Review this document for all removed features
3. Review the [API Documentation](./API.md)
4. Search [existing GitHub issues](https://github.com/ehsaniara/joblet/issues)
5. Open a new issue with:
    - Source version (e.g., v4.7.3)
    - Target version (v5.0.0)
    - Specific feature causing issues
    - Error messages
    - Attempted workaround

---

**Last Updated**: 2025-10-13
**Document Version**: 2.0
**Joblet Version**: v5.0.0
