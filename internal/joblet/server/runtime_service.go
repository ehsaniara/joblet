package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/internal/joblet/runtime"
	"github.com/ehsaniara/joblet/pkg/builder"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RuntimeServiceServer implements the RuntimeService gRPC interface
type RuntimeServiceServer struct {
	pb.UnimplementedRuntimeServiceServer
	auth         auth.GRPCAuthorization
	resolver     *runtime.Resolver
	runtimesPath string
	logger       *logger.Logger
}

var _ pb.RuntimeServiceServer = (*RuntimeServiceServer)(nil)

// NewRuntimeServiceServer creates a new gRPC runtime service for managing execution environments
func NewRuntimeServiceServer(auth auth.GRPCAuthorization, runtimesBasePath string, platform platform.Platform) *RuntimeServiceServer {
	runtimeLogger := logger.New().WithField("component", "runtime-grpc")

	return &RuntimeServiceServer{
		auth:         auth,
		resolver:     runtime.NewResolver(runtimesBasePath, platform),
		runtimesPath: runtimesBasePath,
		logger:       runtimeLogger,
	}
}

// ListRuntimes returns all available runtime environments with their metadata
func (s *RuntimeServiceServer) ListRuntimes(ctx context.Context, req *pb.EmptyRequest) (*pb.ListRuntimesResponse, error) {
	log := s.logger.WithField("operation", "ListRuntimes")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Get runtimes from resolver
	runtimeInfos, err := s.resolver.ListRuntimes()
	if err != nil {
		log.Error("failed to list runtimes", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list runtimes: %v", err)
	}

	// Convert to protobuf format
	pbRuntimes := make([]*pb.RuntimeInfo, 0, len(runtimeInfos))
	for _, info := range runtimeInfos {
		pbRuntime := &pb.RuntimeInfo{
			Name:        info.Name,
			Language:    info.Language,
			Version:     info.Version,
			Description: info.Description,
			SizeBytes:   info.Size,
			Packages:    []string{}, // Will be filled from runtime config if available
			Available:   info.Available,
			Requirements: &pb.RuntimeRequirements{
				Architectures: []string{"x86_64", "amd64"},
				Gpu:           false,
			},
		}

		pbRuntimes = append(pbRuntimes, pbRuntime)
	}

	return &pb.ListRuntimesResponse{
		Runtimes: pbRuntimes,
	}, nil
}

// GetRuntimeInfo returns detailed metadata and configuration for a specific runtime
func (s *RuntimeServiceServer) GetRuntimeInfo(ctx context.Context, req *pb.GetRuntimeInfoRequest) (*pb.GetRuntimeInfoResponse, error) {
	log := s.logger.WithFields("operation", "GetRuntimeInfo", "runtime", req.Runtime)

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate request
	if req.Runtime == "" {
		return nil, status.Errorf(codes.InvalidArgument, "runtime name is required")
	}

	// Resolve runtime
	config, err := s.resolver.ResolveRuntime(req.Runtime)
	if err != nil {
		return &pb.GetRuntimeInfoResponse{
			Found: false,
		}, nil
	}

	// Calculate runtime size
	runtimePath, _ := s.resolver.FindRuntimeDirectory(req.Runtime)
	var sizeBytes int64
	if runtimePath != "" {
		_ = filepath.Walk(runtimePath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				sizeBytes += info.Size()
			}
			return nil
		})
	}

	// Convert to protobuf format
	pbRuntime := &pb.RuntimeInfo{
		Name:            config.Name,
		Language:        config.Language,
		Version:         config.Version,
		Description:     config.Description,
		SizeBytes:       sizeBytes,
		Packages:        config.Packages,
		Available:       true,
		LanguageVersion: config.LanguageVersion,
		Libraries:       config.Libraries,
		Environment:     config.Environment,
		OriginalYaml:    config.OriginalYAML,
		Requirements: &pb.RuntimeRequirements{
			Architectures: config.Requirements.Architectures,
			Gpu:           config.Requirements.GPU,
		},
		BuildInfo: &pb.RuntimeBuildInfo{
			BuiltAt:   config.BuildInfo.BuiltAt,
			BuiltWith: config.BuildInfo.BuiltWith,
			Platform:  config.BuildInfo.Platform,
		},
	}

	return &pb.GetRuntimeInfoResponse{
		Runtime: pbRuntime,
		Found:   true,
	}, nil
}

// TestRuntime validates runtime availability and basic functionality
func (s *RuntimeServiceServer) TestRuntime(ctx context.Context, req *pb.TestRuntimeRequest) (*pb.TestRuntimeResponse, error) {
	log := s.logger.WithFields("operation", "TestRuntime", "runtime", req.Runtime)

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate request
	if req.Runtime == "" {
		return nil, status.Errorf(codes.InvalidArgument, "runtime name is required")
	}

	// Try to resolve runtime
	_, err := s.resolver.ResolveRuntime(req.Runtime)
	if err != nil {
		return &pb.TestRuntimeResponse{
			Success:  false,
			Output:   "",
			Error:    err.Error(),
			ExitCode: 1,
		}, nil
	}

	// Basic test passed
	return &pb.TestRuntimeResponse{
		Success:  true,
		Output:   "Runtime resolution successful",
		Error:    "",
		ExitCode: 0,
	}, nil
}

// extractLanguageFromName extracts language from runtime name (e.g., "python-3.11-ml" -> "python")
func extractLanguageFromName(name string) string {
	// Simple extraction - take first part before hyphen
	if len(name) == 0 {
		return ""
	}

	for i, char := range name {
		if char == '-' {
			return name[:i]
		}
	}

	return name // No hyphen found, return whole name
}

// RemoveRuntime removes an installed runtime and cleans up its files
func (s *RuntimeServiceServer) RemoveRuntime(ctx context.Context, req *pb.RemoveRuntimeRequest) (*pb.RemoveRuntimeResponse, error) {
	log := s.logger.WithFields(
		"operation", "RemoveRuntime",
		"runtime", req.Runtime,
	)

	log.Info("runtime removal request received")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.Runtime == "" {
		return &pb.RemoveRuntimeResponse{
			Success: false,
			Message: "Runtime name is required",
		}, nil
	}

	// Determine the path to remove based on whether version is specified
	var runtimePath string
	var removalScope string

	if strings.Contains(req.Runtime, "@") {
		// Version-specific removal: python-3.11-ml@1.3.1
		// Use resolver to find the specific version directory
		resolvedPath, err := s.resolver.FindRuntimeDirectory(req.Runtime)
		if err != nil {
			return &pb.RemoveRuntimeResponse{
				Success: false,
				Message: fmt.Sprintf("Runtime '%s' not found", req.Runtime),
			}, nil
		}
		runtimePath = resolvedPath
		removalScope = "specific version"
		log.Info("removing specific runtime version", "spec", req.Runtime, "path", runtimePath)
	} else {
		// Remove entire runtime (all versions): python-3.11-ml
		runtimePath = filepath.Join(s.runtimesPath, req.Runtime)
		removalScope = "all versions"
		log.Info("removing all runtime versions", "spec", req.Runtime, "path", runtimePath)
	}

	// Check if runtime path exists
	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		return &pb.RemoveRuntimeResponse{
			Success: false,
			Message: fmt.Sprintf("Runtime '%s' not found", req.Runtime),
		}, nil
	}

	// Calculate directory size before removal
	var totalSize int64
	err := filepath.Walk(runtimePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		log.Warn("failed to calculate runtime size", "error", err)
	}

	// Remove the runtime directory
	log.Info("removing runtime directory", "path", runtimePath)
	if err := os.RemoveAll(runtimePath); err != nil {
		log.Error("failed to remove runtime directory", "error", err)
		return &pb.RemoveRuntimeResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to remove runtime: %v", err),
		}, nil
	}

	log.Info("runtime removed successfully", "freedBytes", totalSize, "scope", removalScope)
	return &pb.RemoveRuntimeResponse{
		Success:         true,
		Message:         fmt.Sprintf("Runtime '%s' removed successfully (%s)", req.Runtime, removalScope),
		FreedSpaceBytes: totalSize,
	}, nil
}

// grpcBuildLogger implements builder.BuildLogger and streams logs via gRPC
type grpcBuildLogger struct {
	stream  pb.RuntimeService_BuildRuntimeServer
	verbose bool
}

func (l *grpcBuildLogger) Debug(format string, args ...interface{}) {
	if l.verbose {
		_ = l.stream.Send(&pb.BuildRuntimeProgress{
			ProgressType: &pb.BuildRuntimeProgress_Log{
				Log: &pb.BuildLogLine{
					Level:     "debug",
					Message:   fmt.Sprintf(format, args...),
					Timestamp: time.Now().UnixNano(),
				},
			},
		})
	}
}

func (l *grpcBuildLogger) Info(format string, args ...interface{}) {
	_ = l.stream.Send(&pb.BuildRuntimeProgress{
		ProgressType: &pb.BuildRuntimeProgress_Log{
			Log: &pb.BuildLogLine{
				Level:     "info",
				Message:   fmt.Sprintf(format, args...),
				Timestamp: time.Now().UnixNano(),
			},
		},
	})
}

func (l *grpcBuildLogger) Warn(format string, args ...interface{}) {
	_ = l.stream.Send(&pb.BuildRuntimeProgress{
		ProgressType: &pb.BuildRuntimeProgress_Log{
			Log: &pb.BuildLogLine{
				Level:     "warn",
				Message:   fmt.Sprintf(format, args...),
				Timestamp: time.Now().UnixNano(),
			},
		},
	})
}

func (l *grpcBuildLogger) Error(format string, args ...interface{}) {
	_ = l.stream.Send(&pb.BuildRuntimeProgress{
		ProgressType: &pb.BuildRuntimeProgress_Log{
			Log: &pb.BuildLogLine{
				Level:     "error",
				Message:   fmt.Sprintf(format, args...),
				Timestamp: time.Now().UnixNano(),
			},
		},
	})
}

func (l *grpcBuildLogger) Phase(phase int, total int, name string, message string) {
	_ = l.stream.Send(&pb.BuildRuntimeProgress{
		ProgressType: &pb.BuildRuntimeProgress_Phase{
			Phase: &pb.BuildPhaseProgress{
				PhaseNumber: int32(phase),
				TotalPhases: int32(total),
				PhaseName:   name,
				Message:     message,
			},
		},
	})
}

// BuildRuntime builds a runtime from a YAML specification
func (s *RuntimeServiceServer) BuildRuntime(req *pb.BuildRuntimeRequest, stream pb.RuntimeService_BuildRuntimeServer) error {
	log := s.logger.WithField("operation", "BuildRuntime")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	// Validate request
	if req.YamlContent == "" {
		return status.Errorf(codes.InvalidArgument, "yaml_content is required")
	}

	startTime := time.Now()

	// Create gRPC logger
	grpcLogger := &grpcBuildLogger{
		stream:  stream,
		verbose: req.Verbose,
	}

	// Create builder with gRPC logger
	b := builder.NewBuilder(s.runtimesPath, grpcLogger)

	// Build the runtime
	result, err := b.Build(stream.Context(), req.YamlContent, req.DryRun)
	if err != nil {
		log.Error("build failed", "error", err)
		// Send failure result
		_ = stream.Send(&pb.BuildRuntimeProgress{
			ProgressType: &pb.BuildRuntimeProgress_Result{
				Result: &pb.BuildResult{
					Success:         false,
					Message:         err.Error(),
					BuildDurationMs: time.Since(startTime).Milliseconds(),
				},
			},
		})
		return nil // Don't return error, we sent it in the stream
	}

	// Send success result
	_ = stream.Send(&pb.BuildRuntimeProgress{
		ProgressType: &pb.BuildRuntimeProgress_Result{
			Result: &pb.BuildResult{
				Success:         true,
				Message:         "Build completed successfully",
				RuntimeName:     result.Name,
				RuntimeVersion:  result.Version,
				InstallPath:     result.InstallPath,
				SizeBytes:       result.SizeBytes,
				BuildDurationMs: time.Since(startTime).Milliseconds(),
			},
		},
	})

	log.Info("build completed", "runtime", result.Name, "version", result.Version, "duration", time.Since(startTime))
	return nil
}

// ValidateRuntimeYAML validates a runtime YAML specification without building
// This performs comprehensive validation including:
// - YAML syntax and schema validation
// - Spec name and version format validation
// - Platform detection
// - Disk space availability check
// - Package validation (base packages, pip, npm)
// - Existing runtime conflict detection
func (s *RuntimeServiceServer) ValidateRuntimeYAML(ctx context.Context, req *pb.ValidateRuntimeYAMLRequest) (*pb.ValidateRuntimeYAMLResponse, error) {
	log := s.logger.WithField("operation", "ValidateRuntimeYAML")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	// Validate request
	if req.YamlContent == "" {
		return &pb.ValidateRuntimeYAMLResponse{
			Valid:   false,
			Message: "yaml_content is required",
			Errors:  []string{"yaml_content cannot be empty"},
		}, nil
	}

	// Create builder for comprehensive validation (dry run mode)
	b := builder.NewBuilder(s.runtimesPath, nil)

	// Run dry-run build which performs phases 1-4:
	// 1. Parse & Validate YAML
	// 2. Detect Platform
	// 3. Check Disk Space
	// 4. Validate Packages
	result, err := b.Build(ctx, req.YamlContent, true) // dryRun = true

	var errors []string
	var warnings []string

	// Collect errors from phases
	if err != nil {
		errors = append(errors, err.Error())
	}
	for _, phase := range result.Phases {
		if !phase.Success && phase.Error != nil {
			errors = append(errors, fmt.Sprintf("Phase %d (%s): %s", phase.Phase, phase.Name, phase.Error.Error()))
		}
	}

	// If there are errors, return failure
	if len(errors) > 0 {
		return &pb.ValidateRuntimeYAMLResponse{
			Valid:   false,
			Message: "Validation failed",
			Errors:  errors,
		}, nil
	}

	// Parse the spec to get details (already validated by builder)
	spec, _ := builder.ParseRuntimeYAML([]byte(req.YamlContent))

	// Check if runtime already exists
	runtimePath := filepath.Join(s.runtimesPath, spec.Name, spec.Version)
	if _, err := os.Stat(runtimePath); err == nil {
		warnings = append(warnings, fmt.Sprintf("Runtime '%s' version '%s' already exists at %s", spec.Name, spec.Version, runtimePath))
	}

	// Check if other versions exist
	runtimeDir := filepath.Join(s.runtimesPath, spec.Name)
	if entries, err := os.ReadDir(runtimeDir); err == nil && len(entries) > 0 {
		var versions []string
		for _, e := range entries {
			if e.IsDir() && e.Name() != spec.Version {
				versions = append(versions, e.Name())
			}
		}
		if len(versions) > 0 {
			warnings = append(warnings, fmt.Sprintf("Other versions of '%s' exist: %v", spec.Name, versions))
		}
	}

	// Build spec info
	specInfo := &pb.RuntimeYAMLInfo{
		Name:            spec.Name,
		Version:         spec.Version,
		Language:        spec.Base.Language,
		LanguageVersion: spec.Base.Version,
		Description:     spec.Description,
		PipPackages:     spec.Pip,
		NpmPackages:     spec.Npm,
		HasHooks:        spec.Hooks != nil && (spec.Hooks.PreInstall != "" || spec.Hooks.PostInstall != ""),
		RequiresGpu:     spec.Requirements != nil && spec.Requirements.GPU,
	}

	message := "YAML specification is valid"
	if len(warnings) > 0 {
		message = fmt.Sprintf("YAML specification is valid (with %d warning(s))", len(warnings))
	}

	log.Info("validation completed", "runtime", spec.Name, "version", spec.Version, "warnings", len(warnings))

	return &pb.ValidateRuntimeYAMLResponse{
		Valid:    true,
		Message:  message,
		Warnings: warnings,
		SpecInfo: specInfo,
	}, nil
}
