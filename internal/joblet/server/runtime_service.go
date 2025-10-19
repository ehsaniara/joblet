package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/internal/joblet/core"
	"github.com/ehsaniara/joblet/internal/joblet/runtime"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RuntimeServiceServer implements the RuntimeService gRPC interface
type RuntimeServiceServer struct {
	pb.UnimplementedRuntimeServiceServer
	auth             auth.GRPCAuthorization
	resolver         *runtime.Resolver
	runtimeInstaller *core.RuntimeInstaller
	runtimesPath     string
	logger           *logger.Logger
}

var _ pb.RuntimeServiceServer = (*RuntimeServiceServer)(nil)

// NewRuntimeServiceServer creates a new gRPC runtime service for managing execution environments
func NewRuntimeServiceServer(auth auth.GRPCAuthorization, runtimesBasePath string, platform platform.Platform, config *config.Config) *RuntimeServiceServer {
	runtimeLogger := logger.New().WithField("component", "runtime-grpc")

	return &RuntimeServiceServer{
		auth:             auth,
		resolver:         runtime.NewResolver(runtimesBasePath, platform),
		runtimeInstaller: core.NewRuntimeInstaller(config, runtimeLogger, platform),
		runtimesPath:     runtimesBasePath,
		logger:           runtimeLogger,
	}
}

// ListRuntimes returns all available runtime environments with their metadata
func (s *RuntimeServiceServer) ListRuntimes(ctx context.Context, req *pb.EmptyRequest) (*pb.RuntimesRes, error) {
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

	return &pb.RuntimesRes{
		Runtimes: pbRuntimes,
	}, nil
}

// GetRuntimeInfo returns detailed metadata and configuration for a specific runtime
func (s *RuntimeServiceServer) GetRuntimeInfo(ctx context.Context, req *pb.RuntimeInfoReq) (*pb.RuntimeInfoRes, error) {
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
		return &pb.RuntimeInfoRes{
			Found: false,
		}, nil
	}

	// Convert to protobuf format
	pbRuntime := &pb.RuntimeInfo{
		Name:        config.Name,
		Language:    extractLanguageFromName(config.Name),
		Version:     config.Version,
		Description: config.Description,
		SizeBytes:   0, // Would need to calculate
		Packages:    config.Packages,
		Available:   true,
		Requirements: &pb.RuntimeRequirements{
			Architectures: config.Requirements.Architectures,
			Gpu:           true, // GPU support is now implemented
		},
	}

	return &pb.RuntimeInfoRes{
		Runtime: pbRuntime,
		Found:   true,
	}, nil
}

// TestRuntime validates runtime availability and basic functionality
func (s *RuntimeServiceServer) TestRuntime(ctx context.Context, req *pb.RuntimeTestReq) (*pb.RuntimeTestRes, error) {
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
		return &pb.RuntimeTestRes{
			Success:  false,
			Output:   "",
			Error:    err.Error(),
			ExitCode: 1,
		}, nil
	}

	// Basic test passed
	return &pb.RuntimeTestRes{
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

// InstallRuntimeFromGithub installs a runtime from a GitHub repository using dedicated chroot (no job system)
func (s *RuntimeServiceServer) InstallRuntimeFromGithub(ctx context.Context, req *pb.InstallRuntimeRequest) (*pb.InstallRuntimeResponse, error) {
	log := s.logger.WithFields(
		"operation", "InstallRuntimeFromGithub",
		"runtimeSpec", req.RuntimeSpec,
		"repository", req.Repository,
		"branch", req.Branch,
		"path", req.Path,
		"forceReinstall", req.ForceReinstall,
	)

	log.Info("direct runtime installation request received (no job system)")

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.RuntimeSpec == "" {
		return nil, status.Errorf(codes.InvalidArgument, "runtime spec is required")
	}

	// Set defaults
	repository := req.Repository
	if repository == "" {
		repository = "ehsaniara/joblet"
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	resolvedPath := req.Path
	if resolvedPath == "" {
		// Auto-detect path based on runtime spec
		resolvedPath = s.autoDetectRuntimePath(req.RuntimeSpec)
	}

	log.Info("executing direct runtime installation", "repository", repository, "branch", branch, "path", resolvedPath)

	// Use direct runtime installer (no job system, no namespaces, no cgroups)
	installReq := &core.RuntimeInstallRequest{
		RuntimeSpec:    req.RuntimeSpec,
		Repository:     repository,
		Branch:         branch,
		Path:           resolvedPath,
		ForceReinstall: req.ForceReinstall,
	}

	result, err := s.runtimeInstaller.InstallFromGithub(ctx, installReq)
	if err != nil {
		log.Error("direct runtime installation failed", "error", err)
		return &pb.InstallRuntimeResponse{
			BuildJobUuid: "", // No job UUID since this is direct execution
			RuntimeSpec:  req.RuntimeSpec,
			Status:       "failed",
			Message:      fmt.Sprintf("Installation failed: %v", err),
			Repository:   repository,
			ResolvedPath: resolvedPath,
		}, status.Errorf(codes.Internal, "runtime installation failed: %v", err)
	}

	var responseStatus string
	if result.Success {
		responseStatus = "completed"
		log.Info("direct runtime installation completed successfully", "duration", result.Duration)
	} else {
		responseStatus = "failed"
		log.Error("direct runtime installation failed", "message", result.Message)
	}

	return &pb.InstallRuntimeResponse{
		BuildJobUuid: "", // No job UUID since this is direct execution
		RuntimeSpec:  req.RuntimeSpec,
		Status:       responseStatus,
		Message:      result.Message,
		Repository:   repository,
		ResolvedPath: resolvedPath,
	}, nil
}

// ValidateRuntimeSpec validates a runtime specification
func (s *RuntimeServiceServer) ValidateRuntimeSpec(ctx context.Context, req *pb.ValidateRuntimeSpecRequest) (*pb.ValidateRuntimeSpecResponse, error) {
	log := s.logger.WithFields(
		"operation", "ValidateRuntimeSpec",
		"runtimeSpec", req.RuntimeSpec,
	)

	// Authorization check
	if err := s.auth.Authorized(ctx, auth.GetJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return nil, err
	}

	if req.RuntimeSpec == "" {
		return &pb.ValidateRuntimeSpecResponse{
			Valid:   false,
			Message: "Runtime spec cannot be empty",
		}, nil
	}

	// Parse and validate runtime spec
	specInfo, valid, message := s.parseRuntimeSpec(req.RuntimeSpec)

	normalizedSpec := req.RuntimeSpec
	if valid {
		normalizedSpec = s.normalizeRuntimeSpec(specInfo)
	}

	return &pb.ValidateRuntimeSpecResponse{
		Valid:          valid,
		Message:        message,
		NormalizedSpec: normalizedSpec,
		SpecInfo:       specInfo,
	}, nil
}

// RemoveRuntime removes an installed runtime and cleans up its files
func (s *RuntimeServiceServer) RemoveRuntime(ctx context.Context, req *pb.RuntimeRemoveReq) (*pb.RuntimeRemoveRes, error) {
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
		return &pb.RuntimeRemoveRes{
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
			return &pb.RuntimeRemoveRes{
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
		return &pb.RuntimeRemoveRes{
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
		return &pb.RuntimeRemoveRes{
			Success: false,
			Message: fmt.Sprintf("Failed to remove runtime: %v", err),
		}, nil
	}

	log.Info("runtime removed successfully", "freedBytes", totalSize, "scope", removalScope)
	return &pb.RuntimeRemoveRes{
		Success:         true,
		Message:         fmt.Sprintf("Runtime '%s' removed successfully (%s)", req.Runtime, removalScope),
		FreedSpaceBytes: totalSize,
	}, nil
}

func (s *RuntimeServiceServer) autoDetectRuntimePath(runtimeSpec string) string {
	// Parse runtime spec to determine path in the main repository
	// e.g., "python:3.11-ml" -> "runtimes/python-3.11-ml"
	parts := strings.Split(runtimeSpec, ":")
	if len(parts) == 2 {
		return fmt.Sprintf("runtimes/%s-%s", parts[0], parts[1])
	}
	return fmt.Sprintf("runtimes/%s", runtimeSpec)
}

func (s *RuntimeServiceServer) parseRuntimeSpec(spec string) (*pb.RuntimeSpecInfo, bool, string) {
	if spec == "" {
		return nil, false, "Runtime spec cannot be empty"
	}

	// Parse format: language:version[-variant1[-variant2...]][@architecture]
	// Examples: python-3.11-ml, openjdk-21, golang-1.21@arm64

	// Split by @ for architecture
	parts := strings.Split(spec, "@")
	mainPart := parts[0]
	architecture := "amd64" // default
	if len(parts) > 1 {
		architecture = parts[1]
	}

	// Split by : for language:version
	langVersion := strings.Split(mainPart, ":")
	if len(langVersion) != 2 {
		return nil, false, "Runtime spec must be in format language:version (e.g., python:3.11)"
	}

	language := langVersion[0]
	versionPart := langVersion[1]

	// Split version part by - for variants
	versionVariants := strings.Split(versionPart, "-")
	version := versionVariants[0]
	variants := versionVariants[1:] // remaining parts are variants

	// Validate language
	validLanguages := []string{"python", "java", "golang", "node", "ruby", "php"}
	validLang := false
	for _, valid := range validLanguages {
		if language == valid {
			validLang = true
			break
		}
	}
	if !validLang {
		return nil, false, fmt.Sprintf("Unsupported language: %s. Supported: %s", language, strings.Join(validLanguages, ", "))
	}

	// Validate architecture
	validArchs := []string{"amd64", "arm64", "x86_64"}
	validArch := false
	for _, valid := range validArchs {
		if architecture == valid {
			validArch = true
			break
		}
	}
	if !validArch {
		return nil, false, fmt.Sprintf("Unsupported architecture: %s. Supported: %s", architecture, strings.Join(validArchs, ", "))
	}

	specInfo := &pb.RuntimeSpecInfo{
		Language:     language,
		Version:      version,
		Variants:     variants,
		Architecture: architecture,
	}

	return specInfo, true, "Runtime spec is valid"
}

func (s *RuntimeServiceServer) normalizeRuntimeSpec(specInfo *pb.RuntimeSpecInfo) string {
	normalized := fmt.Sprintf("%s:%s", specInfo.Language, specInfo.Version)

	if len(specInfo.Variants) > 0 {
		normalized += "-" + strings.Join(specInfo.Variants, "-")
	}

	if specInfo.Architecture != "amd64" {
		normalized += "@" + specInfo.Architecture
	}

	return normalized
}

// StreamingInstallRuntimeFromGithub streams runtime installation from GitHub repository
func (s *RuntimeServiceServer) StreamingInstallRuntimeFromGithub(req *pb.InstallRuntimeRequest, stream pb.RuntimeService_StreamingInstallRuntimeFromGithubServer) error {
	log := s.logger.WithFields(
		"operation", "StreamingInstallRuntimeFromGithub",
		"runtimeSpec", req.RuntimeSpec,
		"repository", req.Repository,
		"branch", req.Branch,
		"path", req.Path,
		"forceReinstall", req.ForceReinstall,
	)

	log.Info("streaming runtime installation request received")

	// Authorization check
	if err := s.auth.Authorized(stream.Context(), auth.RunJobOp); err != nil {
		log.Warn("authorization failed", "error", err)
		return err
	}

	if req.RuntimeSpec == "" {
		return status.Errorf(codes.InvalidArgument, "runtime spec is required")
	}

	// Route based on Repository field:
	// - If Repository is empty -> use external registry (DEFAULT)
	// - If Repository is provided -> use GitHub direct installation
	if req.Repository == "" {
		log.Info("no repository specified, using external registry")
		return s.installFromRegistryStreaming(req, stream)
	}

	// Set defaults for GitHub installation
	repository := req.Repository

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	resolvedPath := req.Path
	if resolvedPath == "" {
		resolvedPath = s.autoDetectRuntimePath(req.RuntimeSpec)
	}

	log.Info("starting streaming runtime installation from GitHub", "repository", repository, "branch", branch, "resolvedPath", resolvedPath)

	// Create streaming adapter
	streamer := &grpcRuntimeStreamer{stream: stream}

	// Create request with streaming support
	installReq := &core.RuntimeInstallRequest{
		RuntimeSpec:    req.RuntimeSpec,
		Repository:     repository,
		Branch:         branch,
		Path:           resolvedPath,
		ForceReinstall: req.ForceReinstall,
		Streamer:       streamer, // Add streaming support
	}

	result, err := s.runtimeInstaller.InstallFromGithub(stream.Context(), installReq)

	// Send final result
	var success bool
	var message string
	var installPath string

	if err != nil {
		log.Error("streaming runtime installation failed", "error", err)
		success = false
		message = fmt.Sprintf("Runtime installation failed: %v", err)
	} else {
		success = result.Success
		message = result.Message
		installPath = result.InstallPath
	}

	finalChunk := &pb.RuntimeInstallationChunk{
		ChunkType: &pb.RuntimeInstallationChunk_Result{
			Result: &pb.RuntimeInstallationResult{
				Success:     success,
				Message:     message,
				RuntimeSpec: req.RuntimeSpec,
				InstallPath: installPath,
			},
		},
	}

	if err := stream.Send(finalChunk); err != nil {
		log.Error("failed to send final result", "error", err)
		return err
	}

	log.Info("streaming runtime installation completed")
	return nil
}

// installFromRegistryStreaming handles streaming installation from external registry
func (s *RuntimeServiceServer) installFromRegistryStreaming(req *pb.InstallRuntimeRequest, stream pb.RuntimeService_StreamingInstallRuntimeFromGithubServer) error {
	log := s.logger.WithFields(
		"operation", "installFromRegistryStreaming",
		"runtimeSpec", req.RuntimeSpec,
		"forceReinstall", req.ForceReinstall,
	)

	log.Info("installing runtime from external registry with streaming")

	// Create streaming adapter
	streamer := &grpcRuntimeStreamer{stream: stream}

	// Send initial progress
	if err := streamer.SendProgress("🚀 Starting installation from external registry"); err != nil {
		return err
	}

	// Extract registry URL from Branch field (temporary until we add dedicated proto field)
	// TODO: Add dedicated registry_url field to InstallRuntimeRequest proto
	registryURL := req.Branch

	// Create registry install request
	registryReq := &core.RuntimeInstallFromRegistryRequest{
		RuntimeSpec:    req.RuntimeSpec,
		ForceReinstall: req.ForceReinstall,
		RegistryURL:    registryURL, // Custom registry URL (empty = use default)
		Streamer:       streamer,
	}

	// Install from registry
	result, err := s.runtimeInstaller.InstallFromRegistry(stream.Context(), registryReq)

	// Send final result
	var success bool
	var message string
	var installPath string

	if err != nil {
		log.Error("registry installation failed", "error", err)
		success = false
		message = fmt.Sprintf("Runtime installation failed: %v", err)
	} else {
		success = result.Success
		message = result.Message
		installPath = result.InstallPath
	}

	finalChunk := &pb.RuntimeInstallationChunk{
		ChunkType: &pb.RuntimeInstallationChunk_Result{
			Result: &pb.RuntimeInstallationResult{
				Success:     success,
				Message:     message,
				RuntimeSpec: req.RuntimeSpec,
				InstallPath: installPath,
			},
		},
	}

	if err := stream.Send(finalChunk); err != nil {
		log.Error("failed to send final result", "error", err)
		return err
	}

	log.Info("streaming registry installation completed", "success", success)
	return nil
}

// grpcRuntimeStreamer adapts gRPC stream to RuntimeInstallationStreamer interface
type grpcRuntimeStreamer struct {
	stream interface {
		Send(*pb.RuntimeInstallationChunk) error
	}
}

func (g *grpcRuntimeStreamer) SendProgress(message string) error {
	chunk := &pb.RuntimeInstallationChunk{
		ChunkType: &pb.RuntimeInstallationChunk_Progress{
			Progress: &pb.RuntimeInstallationProgress{
				Message: message,
			},
		},
	}
	return g.stream.Send(chunk)
}

func (g *grpcRuntimeStreamer) SendLog(data []byte) error {
	chunk := &pb.RuntimeInstallationChunk{
		ChunkType: &pb.RuntimeInstallationChunk_Log{
			Log: &pb.RuntimeInstallationLog{
				Data: data,
			},
		},
	}
	return g.stream.Send(chunk)
}
