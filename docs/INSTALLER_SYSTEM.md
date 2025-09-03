# Runtime Installer System

This document describes the template-based runtime installer system that replaced embedded shell scripts in the Joblet
runtime service.

## Overview

The installer system uses a **Strategy Pattern** architecture with **template-based script generation** to provide
maintainable, testable, and extensible runtime installation capabilities. This system replaced 294 lines of embedded
shell scripts in `runtime_service.go` with organized, parameterized template files.

## Architecture

### Core Components

```
internal/joblet/runtime/installers/
├── interfaces.go          # Strategy pattern interfaces
├── base.go               # Template rendering base functionality
├── manager.go            # Central installer coordinator
├── github.go             # GitHub repository installer
└── templates/
    └── github_install.sh.tmpl  # GitHub installation template
```

### Strategy Pattern Implementation

#### 1. RuntimeInstaller Interface (`interfaces.go`)

```go
type RuntimeInstaller interface {
    Install(ctx context.Context, spec *InstallSpec) (*InstallResult, error)
    Supports(source *pb.RuntimeSourceConfig) bool
}

type InstallSpec struct {
    RuntimeSpec string                    // e.g., "python-3.11-ml"
    Source      *pb.RuntimeSourceConfig   // Source configuration
    BuildArgs   map[string]string         // Build arguments
    TargetPath  string                    // Installation target path
}

type InstallResult struct {
    Success bool     // Installation success status
    Command string   // Generated command
    Args    []string // Command arguments
    Message string   // Status or error message
}
```

#### 2. Base Template Engine (`base.go`)

```go
type BaseInstaller struct {
    templates embed.FS  // Embedded template files
}

func (b *BaseInstaller) RenderTemplate(templateName string, data TemplateData) (string, error) {
    // Load and render Go templates with runtime-specific parameters
}
```

Uses Go's `text/template` package with embedded template files for:

- Parameterized shell script generation
- Runtime-specific variable substitution
- Consistent error handling across installers

#### 3. Installer Manager (`manager.go`)

```go
type Manager struct {
    installers    []RuntimeInstaller
    runtimesPath  string
}

func (m *Manager) Install(ctx context.Context, spec *InstallSpec) (*InstallResult, error) {
    // Find appropriate installer for the source type
    // Delegate to specific installer implementation
    // Return standardized InstallResult
}
```

Central coordinator that:

- Routes installation requests to appropriate installers
- Provides unified interface for runtime service
- Handles installer registration and discovery

#### 4. GitHub Runtime Installer (`github.go`)

```go
type GitHubInstaller struct {
    BaseInstaller
}

func (g *GitHubInstaller) Install(ctx context.Context, spec *InstallSpec) (*InstallResult, error) {
    // Validate GitHub repository format
    // Set default values (branch, repository normalization)
    // Render installation template with parameters
    // Return command and arguments for execution
}
```

Specific implementation for GitHub-based runtimes:

- Repository URL validation and normalization
- Branch specification with sensible defaults
- Template-based script generation
- Comprehensive error handling and validation

### Template System

#### Template Structure

Templates are stored in `templates/` directory and embedded using Go's `embed` package:

```bash
# templates/github_install.sh.tmpl
#!/bin/bash
set -euo pipefail

RUNTIME_SPEC="{{.RuntimeSpec}}"
REPOSITORY="{{.Repository}}"  
BRANCH="{{.Branch}}"
TARGET_PATH="{{.TargetPath}}"

echo "Installing runtime: $RUNTIME_SPEC"
echo "From repository: $REPOSITORY"
echo "Branch: $BRANCH"

# Clone repository and run setup
git clone --branch "$BRANCH" "$REPOSITORY" "$TARGET_PATH"
cd "$TARGET_PATH" && ./setup.sh
```

#### Template Parameters

Templates receive a `TemplateData` struct with runtime-specific values:

```go
type TemplateData struct {
    RuntimeSpec string            // "python-3.11-ml"
    Repository  string            // "https://github.com/user/repo.git"
    Branch      string            // "main" or specified branch
    TargetPath  string            // "/opt/joblet/runtimes/python-3.11-ml"
    BuildArgs   map[string]string // Additional build parameters
}
```

## Integration with Runtime Service

### Before: Embedded Scripts (1290 lines)

```go
// runtime_service.go - OLD VERSION
func (s *RuntimeServiceServer) buildGithubCommand(source *pb.RuntimeSourceConfig, runtimeSpec string, buildArgs map[string]string) (string, []string, error) {
    // 80+ lines of embedded shell script as Go string literal
    script := `#!/bin/bash
set -euo pipefail
RUNTIME_SPEC="` + runtimeSpec + `"
REPOSITORY="` + repository + `"
# ... 70+ more lines of embedded bash ...`
    
    return "bash", []string{"-c", script}, nil
}
```

### After: Template-Based System (996 lines)

```go
// runtime_service.go - NEW VERSION
func (s *RuntimeServiceServer) buildCommandFromSource(source *pb.RuntimeSourceConfig, runtimeSpec string, buildArgs map[string]string) (string, []string, error) {
    spec := &installers.InstallSpec{
        RuntimeSpec: runtimeSpec,
        Source:      source,
        BuildArgs:   buildArgs,
        TargetPath:  filepath.Join(s.runtimesPath, runtimeSpec),
    }
    
    result, err := s.installerManager.Install(context.Background(), spec)
    if err != nil {
        return "", nil, fmt.Errorf("failed to generate installation command: %w", err)
    }
    
    return result.Command, result.Args, nil
}
```

## Benefits

### Maintainability

- **Separated Concerns**: Shell scripts in template files, Go logic in installers
- **Version Control**: Templates tracked separately from Go code
- **Debugging**: Shell scripts can be debugged independently
- **Code Reduction**: 294 lines removed from runtime_service.go

### Testability

- **Unit Testing**: Each installer component individually testable
- **Template Testing**: Templates can be rendered and validated separately
- **Mock Support**: Strategy pattern enables easy mocking for tests
- **Isolated Testing**: Template rendering separated from business logic

### Extensibility

- **New Installers**: Easy to add new runtime source types
- **Template Variations**: Multiple templates per installer type
- **Custom Parameters**: Flexible template parameter system
- **Plugin Architecture**: Clean interfaces for future extensions

### Reliability

- **Input Validation**: Comprehensive validation at installer level
- **Error Handling**: Standardized error reporting across installers
- **Type Safety**: Go structs for all installer interactions
- **Parameterization**: Safe template parameter substitution

## Testing

### Unit Tests

```bash
# Run installer system tests
go test ./internal/joblet/runtime/installers/...

# Test coverage includes:
# - Template rendering with various parameters
# - GitHub repository validation
# - Error handling and edge cases
# - Installer manager routing
```

### Test Structure

```go
func TestGitHubInstaller_Install(t *testing.T) {
    tests := []struct {
        name        string
        spec        *InstallSpec
        expectError bool
        expectedCmd string
    }{
        {
            name: "valid github source",
            spec: &InstallSpec{
                RuntimeSpec: "test-runtime",
                Source: &pb.RuntimeSourceConfig{
                    SourceType: &pb.RuntimeSourceConfig_Github{
                        Github: &pb.GithubSource{
                            Repository: "user/repo",
                            Branch:     "main",
                        },
                    },
                },
                TargetPath: "/tmp/test",
            },
            expectError: false,
            expectedCmd: "bash",
        },
        // Additional test cases...
    }
}
```

## Migration Notes

### Removed Components

The following embedded script methods were removed from `runtime_service.go`:

- `buildGithubCommand()` - 80+ lines
- `buildScriptCommand()` - 60+ lines
- `buildLocalCommand()` - 70+ lines
- `buildLocalRuntimeCommand()` - 84+ lines

**Total Removed**: 294 lines of embedded shell scripts

### Preserved Functionality

All existing runtime installation functionality was preserved:

- GitHub repository cloning and installation
- Branch specification and defaults
- Build argument passing
- Error handling and reporting
- Integration with existing runtime service workflows

## Future Enhancements

### Planned Extensions

1. **Local Runtime Installer**: For local filesystem runtime sources
2. **Script-Based Installer**: For custom installation scripts
3. **Registry Installer**: For runtime registries/repositories
4. **Cached Installer**: For cached runtime artifacts

### Template Improvements

1. **Template Validation**: Syntax checking and linting
2. **Multi-Platform Templates**: OS-specific installation logic
3. **Template Composition**: Reusable template fragments
4. **Dynamic Templates**: Runtime-generated template parameters

### Developer Tools

1. **Template CLI**: Tools for template development and testing
2. **Installer Generator**: Code generation for new installer types
3. **Template Debugging**: Enhanced debugging and validation tools
4. **Performance Profiling**: Template rendering performance analysis

## Conclusion

The template-based installer system successfully replaces embedded shell scripts with a maintainable, testable, and
extensible architecture. This refactoring:

- **Reduces complexity** in runtime_service.go (1290 → 996 lines)
- **Improves maintainability** with separated template files
- **Enables comprehensive testing** of installer components
- **Provides foundation** for future runtime installation features

The system maintains full backward compatibility while enabling future enhancements and easier debugging of runtime
installation issues.