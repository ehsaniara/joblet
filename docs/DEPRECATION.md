# Deprecation Timeline and Migration Guide

This document outlines deprecated features, their replacement alternatives, and removal timelines for the Joblet project.

## Overview

As Joblet evolves, certain features become obsolete or are replaced by better alternatives. This document helps users migrate away from deprecated functionality before it's removed in future major versions.

## Deprecation Policy

- **Current Version (v4.7.3)**: Features are marked as deprecated but remain functional
- **Next Major Version (v5.0.0)**: Deprecated features will be removed with migration guide
- **Grace Period**: All deprecated features have been documented with migration paths

---

## Deprecated Features

### 1. JobServiceServer (High Priority)

**Status**: 🔴 **DEPRECATED** - Will be removed in v5.0.0

**Current Location**: `internal/joblet/server/job_service.go`

**Reason for Deprecation**:
The original `JobServiceServer` has been superseded by `WorkflowServiceServer`, which implements a unified architecture where all jobs (individual and workflow) are handled through a single service.

**Migration Path**:

#### Before (Deprecated):
```go
// OLD: Using JobServiceServer
jobService := server.NewJobServiceServer(auth, jobStore, metricsStore, joblet)
pb.RegisterJobServiceServer(grpcServer, jobService)
```

#### After (Current):
```go
// NEW: Using WorkflowServiceServer
workflowManager := workflow.NewWorkflowManager()
jobService := server.NewWorkflowServiceServer(
    auth,
    jobStore,
    metricsStore,
    joblet,
    workflowManager,
    volumeManager,
    runtimeResolver,
)
pb.RegisterJobServiceServer(grpcServer, jobService)
```

**Current Status**:
- ✅ WorkflowServiceServer fully implements JobServiceServer interface
- ✅ Already used in production (see `internal/joblet/server/grpc_server.go:81`)
- ⚠️ Still marked as `JobServiceServer` type but uses workflow implementation
- ❌ Old implementation file still exists for reference

**Removal Plan**:
1. **v4.7.3** (Current): File marked as deprecated, workflow implementation active
2. **v5.0.0**: Remove `internal/joblet/server/job_service.go` entirely

**Impact**: Low - Already migrated to WorkflowServiceServer internally

---

### 2. Legacy Job Status Constants

**Status**: 🟡 **DEPRECATED** - Backward compatible aliases

**Current Location**: `internal/joblet/domain/job.go:26-33`

**Deprecated Constants**:
```go
const (
    JobStatusRunning   = StatusRunning    // Use StatusRunning
    JobStatusCompleted = StatusCompleted  // Use StatusCompleted
    JobStatusFailed    = StatusFailed     // Use StatusFailed
    JobStatusScheduled = StatusScheduled  // Use StatusScheduled
    JobStatusStopping  = StatusStopping   // Use StatusStopping
)
```

**Migration Path**:

#### Before (Deprecated):
```go
if job.Status == domain.JobStatusRunning {
    // ...
}
```

#### After (Current):
```go
if job.Status == domain.StatusRunning {
    // ...
}
```

**Removal Plan**:
1. **v4.7.3** (Current): Both forms work (aliases maintained)
2. **v5.0.0**: Remove `JobStatus*` aliases, keep only `Status*` constants

**Impact**: Low - Simple find/replace migration

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

### 5. Workflow-Level Environment Variables

**Status**: 🟡 **DEPRECATED** - Use job-level environment variables

**Current Location**: `internal/joblet/workflow/types/yaml.go:27-31`

**Deprecated Fields**:
```yaml
# workflow.yaml (OLD - Deprecated)
version: "1.0"
environment:              # ❌ Deprecated
  GLOBAL_VAR: "value"
secret_environment:       # ❌ Deprecated
  API_KEY: "secret"
jobs:
  my-job:
    command: python3
    args: [script.py]
```

**Migration Path**:

Define environment variables directly in each job specification.

#### After (Current):
```yaml
# workflow.yaml (NEW - Current)
version: "1.0"
jobs:
  my-job:
    command: python3
    args: [script.py]
    environment:          # ✅ Job-level environment
      GLOBAL_VAR: "value"
      SECRET_API_KEY: "secret"  # Use naming conventions for secrets
```

**Best Practices**:
- Use job-level `environment` field for all variables
- Use naming conventions for secrets (e.g., `SECRET_*`, `*_TOKEN`, `*_KEY`)
- No separate `secret_environment` field needed

**Removal Plan**:
1. **v4.7.3** (Current): Fields ignored if present, no effect
2. **v5.0.0**: Remove fields from YAML schema entirely

**Impact**: Low - Already ignored, job-level environment is standard

---

### 6. Job-Level SecretEnvironment Field

**Status**: 🟡 **DEPRECATED** - Merged into environment field

**Current Location**: `internal/joblet/workflow/types/yaml.go:57-59`

**Deprecated Field**:
```yaml
# Job specification (OLD - Deprecated)
jobs:
  my-job:
    command: python3
    environment:
      NORMAL_VAR: "value"
    secret_environment:    # ❌ Deprecated
      API_KEY: "secret"
```

**Migration Path**:

Merge all variables into a single `environment` field with naming conventions.

#### After (Current):
```yaml
# Job specification (NEW - Current)
jobs:
  my-job:
    command: python3
    environment:
      NORMAL_VAR: "value"
      SECRET_API_KEY: "secret"  # Use naming convention
```

**Naming Conventions for Secrets**:
- Prefix: `SECRET_*` (e.g., `SECRET_DATABASE_PASSWORD`)
- Suffix: `*_TOKEN` (e.g., `GITHUB_TOKEN`)
- Suffix: `*_KEY` (e.g., `API_KEY`)
- Suffix: `*_SECRET` (e.g., `OAUTH_SECRET`)

**Removal Plan**:
1. **v4.7.3** (Current): Field parsed but merged into environment
2. **v5.0.0**: Remove `secret_environment` from JobSpec

**Impact**: Low - Already merged internally, simple YAML update

---

## Migration Timeline

### Current Version (v4.7.3)
- All deprecated features are marked but remain functional
- Backward compatibility is maintained
- No breaking changes
- Migration documentation provided

### Next Major Version (v5.0.0)
**Target: Q1-Q2 2026**

Breaking Changes:
- Remove `internal/joblet/server/job_service.go`
- Remove legacy `JobStatus*` constants
- Remove sequential ID generator methods
- Remove `GetRuntimeInitPath` method
- Remove workflow-level environment fields from YAML schema
- Remove `secret_environment` field from JobSpec

Migration Support:
- Complete migration guide in this document
- Automated migration scripts provided
- All replacements documented with examples

---

## How to Prepare for v5.0.0

### 1. Audit Your Code
```bash
# Search for deprecated constants
grep -r "JobStatusRunning\|JobStatusCompleted\|JobStatusFailed" .

# Search for deprecated ID generator
grep -r "NewSequentialIDGenerator\|NextWithTimestamp" .

# Search for deprecated runtime method
grep -r "GetRuntimeInitPath" .
```

### 2. Audit Your Workflows
```bash
# Search for deprecated YAML fields
grep -r "secret_environment" workflows/

# Check workflow-level environment (should be empty)
grep -A5 "^environment:" workflows/*.yaml
```

### 3. Update Code

Use automated migration tool:
```bash
# Run migration script (available in v5.0.0 release)
./scripts/migrate-to-v5.sh --dry-run
./scripts/migrate-to-v5.sh --apply
```

Or manual updates:
- Replace `JobStatus*` → `Status*`
- Replace `NewSequentialIDGenerator` → `NewUUIDGenerator`
- Move workflow-level environment → job-level environment
- Merge `secret_environment` → `environment`

### 4. Test Changes
```bash
# Run full test suite
go test ./...

# Run E2E tests
./tests/e2e/run_tests.sh

# Verify workflows
rnx workflow validate workflows/*.yaml
```

---

## Additional Resources

- [API Documentation](./API.md) - Current API reference
- [Migration Scripts](../scripts/migrate-to-v5.sh) - Automated migration tools (coming in v5.0.0)
- [GitHub Issues](https://github.com/ehsaniara/joblet/issues) - Report migration problems
- [Changelog](../CHANGELOG.md) - Version-specific changes

---

## Questions or Issues?

If you encounter problems migrating away from deprecated features:

1. Check this guide for migration instructions
2. Review the [API Documentation](./API.md)
3. Search [existing GitHub issues](https://github.com/ehsaniara/joblet/issues)
4. Open a new issue with:
   - Current version
   - Deprecated feature being used
   - Migration blocker description
   - Proposed workaround (if any)

---

**Last Updated**: 2025-10-12
**Document Version**: 1.0
**Joblet Version**: v4.7.3
