# Contributing to Job Worker

Thank you for your interest in contributing to Worker! This guide will help you get started with contributing to the project, whether you're reporting bugs, suggesting features, or submitting code changes.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)
- [Community](#community)

## Code of Conduct

This project adheres to a code of conduct to ensure a welcoming environment for all contributors. By participating, you are expected to uphold this code.

### Our Standards

- **Be respectful** and inclusive in all interactions
- **Be constructive** when giving feedback
- **Focus on what's best** for the community and project
- **Show empathy** towards other community members
- **Be patient** with new contributors

### Unacceptable Behavior

- Harassment, discrimination, or trolling
- Personal attacks or inflammatory comments
- Publishing private information without permission
- Any conduct that would be inappropriate in a professional setting

## Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.23+** installed
- **Git** for version control
- **Make** for build automation
- **Linux environment** for testing (or WSL/VM)
- **Basic understanding** of gRPC and Protocol Buffers

### Quick Start

```bash
# 1. Fork the repository on GitHub
# 2. Clone your fork
git clone https://github.com/your-username/worker.git
cd worker

# 3. Add upstream remote
git remote add upstream https://github.com/ehsaniara/worker.git

# 4. Set up development environment
make setup-dev

# 5. Run tests to verify setup
go test -v ./...

# 6. Create a development branch
git checkout -b feature/your-feature-name
```

## Development Setup

### Environment Setup

```bash
# Complete development setup
make setup-dev

# This creates:
# - bin/cli (CLI binary for testing)
# - bin/worker (server binary)
# - bin/job-init (initialization binary)
# - certs/ (TLS certificates for testing)
```

### Development Dependencies

```bash
# Install additional development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/tools/cmd/goimports@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Verify installation
golangci-lint version
protoc --version
```

### IDE Setup

#### VS Code

Recommended extensions:
- Go (Google)
- Protocol Buffer Language Support
- GitLens
- YAML Support

#### Settings

```json
{
    "go.formatTool": "goimports",
    "go.lintTool": "golangci-lint",
    "go.testFlags": ["-v", "-race"],
    "editor.formatOnSave": true
}
```

## How to Contribute

### Types of Contributions

We welcome various types of contributions:

#### 🐛 Bug Reports
- Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md)
- Include steps to reproduce
- Provide system information
- Include relevant logs

#### ✨ Feature Requests
- Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md)
- Explain the use case and motivation
- Consider implementation challenges
- Discuss alternatives

#### 📖 Documentation
- Fix typos or unclear explanations
- Add examples and use cases
- Improve API documentation
- Create tutorials or guides

#### 🔧 Code Contributions
- Bug fixes
- New features
- Performance improvements
- Refactoring and cleanup

### Finding Work

Good places to start:
- Issues labeled `good first issue`
- Issues labeled `help wanted`
- Documentation improvements
- Test coverage improvements

## Development Workflow

### 1. Issue Creation/Assignment

```bash
# Before starting work:
# 1. Check if an issue exists
# 2. Comment on the issue to express interest
# 3. Wait for maintainer feedback
# 4. Get issue assigned to you
```

### 2. Branch Creation

```bash
# Create feature branch from main
git checkout main
git pull upstream main
git checkout -b feature/issue-123-add-job-timeout

# Branch naming conventions:
# feature/issue-123-description
# bugfix/issue-456-fix-memory-leak
# docs/update-api-documentation
# refactor/improve-error-handling
```

### 3. Development Process

```bash
# Make your changes
# Add tests for new functionality
# Run tests frequently
go test -v ./...

# Run linting
golangci-lint run

# Build and test locally
make all
./bin/worker &
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo "test"
```

### 4. Commit Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```bash
# Format: type(scope): description
# Examples:
git commit -m "feat(api): add job timeout configuration"
git commit -m "fix(worker): resolve memory leak in job cleanup"
git commit -m "docs(api): update CreateJob documentation"
git commit -m "test(store): add unit tests for job storage"
git commit -m "refactor(auth): simplify certificate validation"

# Types:
# feat: new feature
# fix: bug fix
# docs: documentation changes
# test: adding or fixing tests
# refactor: code refactoring
# perf: performance improvements
# style: formatting changes
# ci: CI/CD changes
# chore: maintenance tasks
```

### 5. Keep Branch Updated

```bash
# Regularly sync with upstream
git fetch upstream
git rebase upstream/main

# Resolve conflicts if any
# Force push if needed (after rebase)
git push --force-with-lease origin feature/your-branch
```

## Coding Standards

### Go Code Style

```go
// Follow standard Go conventions
// Use gofmt and goimports
// Follow effective Go guidelines

// Example: Good error handling
func (w *worker) StartJob(ctx context.Context, command string) (*domain.Job, error) {
    if command == "" {
        return nil, fmt.Errorf("command cannot be empty")
    }
    
    job, err := w.createJob(command)
    if err != nil {
        return nil, fmt.Errorf("failed to create job: %w", err)
    }
    
    return job, nil
}

// Example: Good logging
func (w *worker) processJob(job *domain.Job) {
    logger := w.logger.WithField("jobId", job.Id)
    logger.Info("starting job processing")
    
    if err := w.executeJob(job); err != nil {
        logger.Error("job execution failed", "error", err)
        return
    }
    
    logger.Info("job completed successfully")
}
```

### Code Organization

```go
// Package structure
package worker

import (
    // Standard library first
    "context"
    "fmt"
    "time"
    
    // Third-party packages
    "google.golang.org/grpc"
    
    // Local packages
    "worker/internal/worker/domain"
    "worker/pkg/logger"
)

// Interface definitions before implementations
type Worker interface {
    StartJob(ctx context.Context, command string) (*domain.Job, error)
    StopJob(ctx context.Context, jobId string) error
}

// Implementation
type worker struct {
    store  interfaces.Store
    logger *logger.Logger
}
```

### Error Handling

```go
// Use structured errors
var (
    ErrJobNotFound = errors.New("job not found")
    ErrJobNotRunning = errors.New("job is not running")
)

// Wrap errors with context
func (w *worker) StopJob(ctx context.Context, jobId string) error {
    job, exists := w.store.GetJob(jobId)
    if !exists {
        return fmt.Errorf("stop job failed: %w", ErrJobNotFound)
    }
    
    if !job.IsRunning() {
        return fmt.Errorf("cannot stop job %s: %w", jobId, ErrJobNotRunning)
    }
    
    return nil
}
```

### Documentation Standards

```go
// Package documentation
// Package worker provides job execution capabilities with resource management.
// It implements a secure, distributed job execution system using gRPC and
// Linux cgroups for resource isolation.
package worker

// Public function documentation
// StartJob creates and starts a new job with the specified command and arguments.
// It sets up resource limits using cgroups and monitors the process execution.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - command: The command to execute
//   - args: Command arguments
//
// Returns:
//   - *domain.Job: The created job with metadata
//   - error: Any error that occurred during job creation
func (w *worker) StartJob(ctx context.Context, command string, args []string) (*domain.Job, error) {
    // Implementation...
}
```

## Testing Guidelines

### Test Structure

```go
// Test file naming: *_test.go
// Test function naming: TestFunctionName
// Benchmark function naming: BenchmarkFunctionName

func TestWorker_StartJob(t *testing.T) {
    tests := []struct {
        name        string
        command     string
        args        []string
        wantErr     bool
        expectedErr error
    }{
        {
            name:    "valid command",
            command: "echo",
            args:    []string{"hello"},
            wantErr: false,
        },
        {
            name:        "empty command",
            command:     "",
            args:        []string{},
            wantErr:     true,
            expectedErr: ErrInvalidCommand,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            w := setupTestWorker(t)
            
            job, err := w.StartJob(context.Background(), tt.command, tt.args)
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.ErrorIs(t, err, tt.expectedErr)
                assert.Nil(t, job)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, job)
            }
        })
    }
}
```

### Test Categories

#### Unit Tests
```bash
# Run unit tests
go test -v ./...

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

#### Integration Tests
```bash
# Run integration tests (requires special build tag)
go test -v -tags=integration ./test/integration/...
```

#### Benchmark Tests
```bash
# Run benchmarks
go test -bench=. -benchmem ./...
```

### Test Helpers

```go
// Use test helpers for common setup
func setupTestWorker(t *testing.T) *worker {
    t.Helper()
    
    store := &fakes.FakeStore{}
    logger := logger.NewTestLogger()
    
    return &worker{
        store:  store,
        logger: logger,
    }
}

// Use table-driven tests for multiple scenarios
// Use testify/assert for assertions
// Clean up resources in tests
```

### Mocking

```go
// Use counterfeiter for generating mocks
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Generate mocks for interfaces
//counterfeiter:generate . Store
type Store interface {
    GetJob(id string) (*domain.Job, bool)
    CreateJob(job *domain.Job)
}
```

## Documentation

### Types of Documentation

#### Code Documentation
- Package-level documentation
- Public function documentation
- Complex algorithm explanations
- Example usage in godoc

#### API Documentation
- Update `docs/API.md` for API changes
- Include request/response examples
- Document error conditions
- Add new endpoints or parameters

#### User Documentation
- Update `README.md` for user-facing changes
- Add examples for new features
- Update installation instructions
- Improve troubleshooting guides

### Documentation Standards

```markdown
# Use clear, concise language
# Include code examples
# Provide context and motivation
# Link to related documentation
# Update table of contents
# Use proper markdown formatting
```

## Pull Request Process

### Before Submitting

```bash
# 1. Ensure all tests pass
go test -v ./...

# 2. Run linting
golangci-lint run

# 3. Update documentation
# 4. Add/update tests
# 5. Rebase on latest main
git fetch upstream
git rebase upstream/main

# 6. Push to your fork
git push origin feature/your-branch
```

### Pull Request Template

When creating a PR, include:

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass locally
- [ ] Integration tests pass locally
- [ ] Added new tests for new functionality
- [ ] Manual testing performed

## Checklist
- [ ] My code follows the style guidelines of this project
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] Any dependent changes have been merged and published

## Screenshots (if applicable)
Add screenshots to help explain your changes.

## Additional Notes
Any additional information that reviewers should know.
```

### Review Process

1. **Automated Checks**: CI must pass (tests, linting)
2. **Code Review**: At least one maintainer review required
3. **Documentation Review**: Ensure docs are updated
4. **Manual Testing**: Verify changes work as expected
5. **Approval**: Maintainer approval required for merge

### Addressing Feedback

```bash
# Make requested changes
# Add new commits (don't squash during review)
git add .
git commit -m "address review feedback: improve error handling"
git push origin feature/your-branch

# After approval, squash commits if requested
git rebase -i upstream/main
```

## Release Process

### Versioning

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Workflow

1. **Version Planning**: Discuss upcoming release in issues
2. **Feature Freeze**: Stop adding new features
3. **Testing**: Comprehensive testing of release candidate
4. **Documentation**: Update changelog and documentation
5. **Tagging**: Create release tag
6. **Release**: GitHub release with binaries
7. **Announcement**: Notify community

### Contributing to Releases

- Test release candidates
- Report bugs in RC versions
- Help with documentation updates
- Assist with changelog creation

## Community

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: General questions and community chat
- **Pull Requests**: Code review and collaboration

### Getting Help

- Check existing issues and documentation first
- Use GitHub Discussions for questions
- Provide minimal reproduction cases
- Be patient and respectful

### Mentorship

New contributors can:
- Start with `good first issue` labels
- Ask for guidance in issue comments
- Request code review feedback
- Participate in discussions

Experienced contributors can:
- Help review pull requests
- Mentor new contributors
- Improve documentation
- Triage issues

## Recognition

Contributors are recognized through:
- **Contributors section** in README
- **Changelog mentions** for significant contributions
- **GitHub contributors graph**
- **Community appreciation** in discussions

## Questions?

If you have questions about contributing:

1. Check this guide and existing documentation
2. Search existing issues and discussions
3. Create a new discussion or issue
4. Tag maintainers if needed

Thank you for contributing to Worker! 🚀

---

**Happy Contributing!** Every contribution, no matter how small, helps make Worker better for everyone.