package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/ehsaniara/joblet/pkg/runtime"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"

	"github.com/spf13/cobra"
)

func NewRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Manage pre-built runtime environments",
		Long: `Manage pre-built runtime environments for fast job execution.

Runtimes provide pre-installed environments and services that can be mounted into jobs,
eliminating the need to install dependencies on every job run. Runtimes can include
programming languages, databases, message queues, web servers, or any other services.

Examples:
  # List available runtimes
  rnx runtime list

  # Build a runtime from YAML specification
  rnx runtime build ./python-ml/runtime.yaml

  # Get information about a specific runtime
  rnx runtime info python-3.11-ml

  # Test a runtime
  rnx runtime test python-3.11-ml

  # Remove a runtime
  rnx runtime remove python-3.11-ml`,
	}

	cmd.AddCommand(NewRuntimeListCmd())
	cmd.AddCommand(NewRuntimeInfoCmd())
	cmd.AddCommand(NewRuntimeTestCmd())
	cmd.AddCommand(NewRuntimeBuildCmd())
	cmd.AddCommand(NewRuntimeInstallCmd())
	cmd.AddCommand(NewRuntimeValidateCmd())
	cmd.AddCommand(NewRuntimeRemoveCmd())

	return cmd
}

func NewRuntimeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available runtimes",
		Long: `List all available runtime environments that can be used with the --runtime flag.

Examples:
  # List locally installed runtimes
  rnx runtime list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeList(cmd, args)
		},
	}

	return cmd
}

func runRuntimeList(cmd *cobra.Command, args []string) error {
	// Create client and connect to server for local runtimes
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Get runtimes from server via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.ListRuntimes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list runtimes: %w", err)
	}

	runtimes := resp.Runtimes

	if len(runtimes) == 0 {
		if common.JSONOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No runtimes available.")
			fmt.Println("\nTo build runtimes, use: rnx runtime build <path-to-runtime.yaml>")
		}
		return nil
	}

	if common.JSONOutput {
		return outputRuntimesJSON(runtimes)
	}

	// Display runtimes in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUNTIME\tVERSION\tLANGUAGE\tSIZE\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-------\t--------\t----\t-----------")

	for _, rt := range runtimes {
		// Use runtime name directly (aligned with builder-runtime-final.md design)
		runtimeID := rt.Name

		// Format size
		sizeStr := formatSize(rt.SizeBytes)

		// Truncate description if too long
		desc := rt.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		// Language from runtime metadata
		language := rt.Language
		if language == "" {
			language = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			runtimeID,
			rt.Version,
			language,
			sizeStr,
			desc,
		)
	}

	w.Flush()

	fmt.Println("\nUse 'rnx runtime info <runtime>' for detailed information about a specific runtime.")

	return nil
}

func NewRuntimeInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <runtime>",
		Short: "Get detailed information about a runtime",
		Long:  `Display detailed information about a specific runtime including installed packages, mount points, and requirements.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRuntimeInfo,
	}
}

func runRuntimeInfo(cmd *cobra.Command, args []string) error {
	runtimeSpec := args[0]

	// Parse runtime spec to extract name and version
	spec, err := runtime.ParseRuntimeSpec(runtimeSpec)
	if err != nil {
		return fmt.Errorf("invalid runtime specification: %w", err)
	}

	// Create client and connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Get runtime info from server via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RuntimeInfoReq{Runtime: spec.Name}
	resp, err := client.GetRuntimeInfo(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get runtime info: %w", err)
	}

	if !resp.Found {
		return fmt.Errorf("runtime not found: %s", spec.Name)
	}

	rt := resp.Runtime

	// Check if JSON output is requested
	if common.JSONOutput {
		return outputRuntimeInfoJSON(rt, spec.String())
	}

	// Display runtime information
	fmt.Printf("Runtime: %s\n", rt.Name)
	fmt.Printf("Version: %s\n", rt.Version)
	if !spec.IsLatest() {
		fmt.Printf("Requested Version: %s\n", spec.Version)
	}
	fmt.Printf("Description: %s\n", rt.Description)

	// Display language info
	if rt.Language != "" || rt.LanguageVersion != "" {
		fmt.Printf("\nLanguage: %s %s\n", rt.Language, rt.LanguageVersion)
	}

	// Display size
	if rt.SizeBytes > 0 {
		fmt.Printf("Size: %s\n", formatSize(rt.SizeBytes))
	}

	// Display requirements
	if rt.Requirements != nil && (len(rt.Requirements.Architectures) > 0 || rt.Requirements.Gpu) {
		fmt.Println("\nRequirements:")
		if rt.Requirements.Gpu {
			fmt.Println("  GPU: Required")
		}
		if len(rt.Requirements.Architectures) > 0 {
			fmt.Printf("  Architectures: %s\n", strings.Join(rt.Requirements.Architectures, ", "))
		}
	}

	// Display pre-installed packages
	if len(rt.Packages) > 0 {
		fmt.Println("\nPre-installed Packages:")
		for _, pkg := range rt.Packages {
			fmt.Printf("  - %s\n", pkg)
		}
	}

	// Display libraries (if any)
	if len(rt.Libraries) > 0 {
		fmt.Println("\nLibrary Patterns:")
		for _, lib := range rt.Libraries {
			fmt.Printf("  - %s\n", lib)
		}
	}

	// Display environment variables (if any)
	if len(rt.Environment) > 0 {
		fmt.Println("\nEnvironment Variables:")
		for k, v := range rt.Environment {
			// Truncate long values
			if len(v) > 60 {
				v = v[:57] + "..."
			}
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	// Display build info
	if rt.BuildInfo != nil && rt.BuildInfo.BuiltAt != "" {
		fmt.Println("\nBuild Info:")
		fmt.Printf("  Built: %s\n", rt.BuildInfo.BuiltAt)
		if rt.BuildInfo.Platform != "" {
			fmt.Printf("  Platform: %s\n", rt.BuildInfo.Platform)
		}
	}

	fmt.Println("\nUsage:")
	fmt.Printf("  rnx job run --runtime=%s <command>\n", runtimeSpec)

	return nil
}

func outputRuntimeInfoJSON(rt *pb.RuntimeInfo, runtimeSpec string) error {
	// Create JSON output structure
	output := map[string]interface{}{
		"name":             rt.Name,
		"version":          rt.Version,
		"description":      rt.Description,
		"language":         rt.Language,
		"language_version": rt.LanguageVersion,
		"size_bytes":       rt.SizeBytes,
		"size":             formatSize(rt.SizeBytes),
		"packages":         rt.Packages,
		"libraries":        rt.Libraries,
		"environment":      rt.Environment,
		"usage":            fmt.Sprintf("rnx job run --runtime=%s <command>", runtimeSpec),
	}

	// Add requirements if they exist
	if rt.Requirements != nil {
		requirements := make(map[string]interface{})
		if rt.Requirements.Gpu {
			requirements["gpu"] = true
		}
		if len(rt.Requirements.Architectures) > 0 {
			requirements["architectures"] = rt.Requirements.Architectures
		}
		if len(requirements) > 0 {
			output["requirements"] = requirements
		}
	}

	// Add build info if available
	if rt.BuildInfo != nil {
		output["build_info"] = map[string]interface{}{
			"built_at":   rt.BuildInfo.BuiltAt,
			"built_with": rt.BuildInfo.BuiltWith,
			"platform":   rt.BuildInfo.Platform,
		}
	}

	// Add original YAML if available
	if rt.OriginalYaml != "" {
		output["original_yaml"] = rt.OriginalYaml
	}

	// Marshal and print JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func NewRuntimeTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <runtime>",
		Short: "Test a runtime environment",
		Long:  `Run basic validation tests on a runtime to ensure it's working correctly.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRuntimeTest,
	}
}

func runRuntimeTest(cmd *cobra.Command, args []string) error {
	runtimeSpec := args[0]

	fmt.Printf("Testing runtime: %s\n", runtimeSpec)

	// Create client and connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Test runtime via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RuntimeTestReq{Runtime: runtimeSpec}
	resp, err := client.TestRuntime(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to test runtime: %w", err)
	}

	if resp.Success {
		fmt.Printf("Runtime test passed\n")
		if resp.Output != "" {
			fmt.Printf("Output: %s\n", resp.Output)
		}
	} else {
		fmt.Printf("✗ Runtime test failed\n")
		if resp.Error != "" {
			fmt.Printf("Error: %s\n", resp.Error)
		}
		fmt.Printf("Exit code: %d\n", resp.ExitCode)
		return fmt.Errorf("runtime test failed")
	}

	testCmd := "echo 'Runtime available'"

	fmt.Printf("\nTo test the runtime in a job:\n")
	fmt.Printf("  rnx job run --runtime=%s %s\n", runtimeSpec, testCmd)

	return nil
}

// outputRuntimesJSON outputs the runtimes in JSON format
func outputRuntimesJSON(runtimes []*pb.RuntimeInfo) error {
	// Convert protobuf runtimes to a simpler structure for JSON output
	type jsonRuntime struct {
		ID          string   `json:"id"`
		Language    string   `json:"language"`
		Version     string   `json:"version"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		SizeBytes   int64    `json:"size_bytes"`
		Size        string   `json:"size"`
		Packages    []string `json:"packages,omitempty"`
		Available   bool     `json:"available"`
	}

	jsonRuntimes := make([]jsonRuntime, len(runtimes))
	for i, rt := range runtimes {
		// Use runtime name directly (aligned with builder-runtime-final.md design)
		runtimeID := rt.Name

		jsonRuntimes[i] = jsonRuntime{
			ID:          runtimeID,
			Language:    rt.Language,
			Version:     rt.Version,
			Name:        rt.Name,
			Description: rt.Description,
			SizeBytes:   rt.SizeBytes,
			Size:        formatSize(rt.SizeBytes),
			Packages:    rt.Packages,
			Available:   rt.Available,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonRuntimes)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func NewRuntimeBuildCmd() *cobra.Command {
	var verbose bool
	var forceRebuild bool

	cmd := &cobra.Command{
		Use:   "build <path>",
		Short: "Build a runtime from a YAML specification",
		Long: `Build a runtime environment from a runtime.yaml specification file.

The build process uses OverlayFS-based isolation to ensure the host system
is never modified. System packages are installed in an isolated chroot,
and only the resulting binaries/libraries are copied to the runtime directory.

The build process follows a 14-phase pipeline:
  1. Parse and validate YAML specification
  2. Detect platform (distro, architecture, package manager)
  3. Check available disk space
  4. Validate that required packages exist
  5. Prepare runtime directories
  6. Execute pre_install hook (if defined)
  7. Install base language packages (in OverlayFS-isolated chroot)
  8. Install language-specific packages (pip, npm)
  9. Execute post_install hook (if defined)
  10. Copy binaries from isolated overlay to runtime directory
  11. Copy libraries from isolated overlay to runtime directory
  12. Copy configuration files
  13. Generate runtime.yml for the server
  14. Validate the build

To validate without building, use 'rnx runtime validate <path>'.

Examples:
  # Build from a runtime.yaml file
  rnx runtime build ./python-ml/runtime.yaml

  # Build from a directory (looks for runtime.yaml)
  rnx runtime build ./python-ml

  # Build with verbose output
  rnx runtime build --verbose ./python-ml

  # Force rebuild even if runtime exists
  rnx runtime build --force ./python-ml

  # Validate without building
  rnx runtime validate ./python-ml

See docs/design/RUNTIME_YAML_QUICKREF.md for the runtime.yaml format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeBuild(cmd.Context(), args[0], verbose, forceRebuild)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.Flags().BoolVarP(&forceRebuild, "force", "f", false, "Force rebuild even if runtime already exists")

	return cmd
}

func runRuntimeBuild(ctx context.Context, path string, verbose bool, forceRebuild bool) error {
	// Read the YAML file
	content, err := os.ReadFile(path)
	if err != nil {
		// Try as a directory
		yamlPath := filepath.Join(path, "runtime.yaml")
		content, err = os.ReadFile(yamlPath)
		if err != nil {
			yamlPath = filepath.Join(path, "runtime.yml")
			content, err = os.ReadFile(yamlPath)
			if err != nil {
				return fmt.Errorf("failed to read runtime.yaml: %w", err)
			}
		}
	}

	// Create gRPC client
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Call BuildRuntime RPC
	req := &pb.BuildRuntimeRequest{
		YamlContent:  string(content),
		DryRun:       false, // Validation is now done via 'rnx runtime validate'
		Verbose:      verbose,
		ForceRebuild: forceRebuild,
	}

	stream, err := client.BuildRuntime(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start build: %w", err)
	}

	// Process streaming response
	var finalResult *pb.BuildResult
	for {
		progress, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("build stream error: %w", err)
		}

		switch p := progress.ProgressType.(type) {
		case *pb.BuildRuntimeProgress_Phase:
			if !common.JSONOutput {
				fmt.Printf("[%d/%d] %s: %s\n", p.Phase.PhaseNumber, p.Phase.TotalPhases, p.Phase.PhaseName, p.Phase.Message)
			}
		case *pb.BuildRuntimeProgress_Log:
			if !common.JSONOutput && (verbose || p.Log.Level != "debug") {
				fmt.Printf("[%s] %s\n", strings.ToUpper(p.Log.Level), p.Log.Message)
			}
		case *pb.BuildRuntimeProgress_Result:
			finalResult = p.Result
		}
	}

	if finalResult == nil {
		return fmt.Errorf("build completed but no result received")
	}

	// Display results
	if common.JSONOutput {
		return outputBuildResultFromProto(finalResult)
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	if finalResult.Success {
		fmt.Printf("BUILD SUCCESSFUL\n")
	} else {
		fmt.Printf("BUILD FAILED\n")
	}
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("\nRuntime: %s\n", finalResult.RuntimeName)
	fmt.Printf("Version: %s\n", finalResult.RuntimeVersion)
	fmt.Printf("Duration: %dms\n", finalResult.BuildDurationMs)

	if finalResult.InstallPath != "" {
		fmt.Printf("Output: %s\n", finalResult.InstallPath)
	}

	if finalResult.SizeBytes > 0 {
		fmt.Printf("Size: %d MB\n", finalResult.SizeBytes/1024/1024)
	}

	if !finalResult.Success {
		fmt.Printf("\nError: %s\n", finalResult.Message)
		return fmt.Errorf("build failed")
	}

	fmt.Printf("\nTo use this runtime:\n")
	fmt.Printf("  rnx job run --runtime=%s <command>\n", finalResult.RuntimeName)

	return nil
}

func outputBuildResultFromProto(result *pb.BuildResult) error {
	type jsonResult struct {
		Success     bool   `json:"success"`
		Name        string `json:"name"`
		Version     string `json:"version"`
		InstallPath string `json:"install_path,omitempty"`
		SizeBytes   int64  `json:"size_bytes,omitempty"`
		DurationMs  int64  `json:"duration_ms"`
		Message     string `json:"message,omitempty"`
	}

	output := jsonResult{
		Success:     result.Success,
		Name:        result.RuntimeName,
		Version:     result.RuntimeVersion,
		InstallPath: result.InstallPath,
		SizeBytes:   result.SizeBytes,
		DurationMs:  result.BuildDurationMs,
		Message:     result.Message,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func NewRuntimeInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "install <runtime-spec>",
		Short:      "DEPRECATED: Use 'rnx runtime build' instead",
		Deprecated: "GitHub-based installation is deprecated. Use 'rnx runtime build <path>' with a runtime.yaml file instead.",
		Long: `DEPRECATED: This command is no longer supported.

Use 'rnx runtime build' to build runtimes from YAML specifications:

  # Build a runtime from a YAML file
  rnx runtime build ./my-runtime/runtime.yaml

  # Preview a build without executing
  rnx runtime build --dry-run ./my-runtime

See docs/design/RUNTIME_YAML_QUICKREF.md for the runtime.yaml format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("GitHub-based installation is deprecated. Use 'rnx runtime build <path>' with a runtime.yaml file instead")
		},
	}

	return cmd
}

func NewRuntimeValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a runtime YAML specification",
		Long: `Validate a runtime YAML specification comprehensively.

This command performs server-side validation including:
  - YAML syntax and schema validation
  - Runtime name and version format validation
  - Platform detection (distro, architecture)
  - Disk space availability check
  - Package validation (base packages, pip, npm)
  - Existing runtime conflict detection

Examples:
  # Validate a runtime.yaml file
  rnx runtime validate ./python-ml/runtime.yaml

  # Validate from a directory (looks for runtime.yaml)
  rnx runtime validate ./python-ml

  # Validate with JSON output
  rnx runtime validate --json ./python-ml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeValidate(cmd.Context(), args[0])
		},
	}
}

func runRuntimeValidate(ctx context.Context, path string) error {
	// Read the YAML file
	content, err := os.ReadFile(path)
	if err != nil {
		// Try as a directory
		yamlPath := filepath.Join(path, "runtime.yaml")
		content, err = os.ReadFile(yamlPath)
		if err != nil {
			yamlPath = filepath.Join(path, "runtime.yml")
			content, err = os.ReadFile(yamlPath)
			if err != nil {
				return fmt.Errorf("failed to read runtime.yaml: %w", err)
			}
		}
	}

	// Create gRPC client
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Call ValidateRuntimeYAML RPC
	req := &pb.ValidateRuntimeYAMLRequest{
		YamlContent: string(content),
	}

	resp, err := client.ValidateRuntimeYAML(ctx, req)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Output results
	if common.JSONOutput {
		return outputValidateResultJSON(resp)
	}

	// Text output
	if resp.Valid {
		fmt.Printf("✓ Runtime specification is valid\n")
	} else {
		fmt.Printf("✗ Runtime specification is invalid\n")
	}

	// Show errors
	if len(resp.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range resp.Errors {
			fmt.Printf("  • %s\n", e)
		}
	}

	// Show warnings
	if len(resp.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, w := range resp.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	// Show spec info if valid
	if resp.Valid && resp.SpecInfo != nil {
		info := resp.SpecInfo
		fmt.Printf("\nSpecification:\n")
		fmt.Printf("  Name:        %s\n", info.Name)
		fmt.Printf("  Version:     %s\n", info.Version)
		fmt.Printf("  Language:    %s %s\n", info.Language, info.LanguageVersion)
		if info.Description != "" {
			fmt.Printf("  Description: %s\n", info.Description)
		}

		if len(info.PipPackages) > 0 {
			fmt.Printf("  Pip:         %s\n", strings.Join(info.PipPackages, ", "))
		}

		if len(info.NpmPackages) > 0 {
			fmt.Printf("  NPM:         %s\n", strings.Join(info.NpmPackages, ", "))
		}

		if info.HasHooks {
			fmt.Printf("  Hooks:       defined\n")
		}

		if info.RequiresGpu {
			fmt.Printf("  GPU:         required\n")
		}
	}

	if !resp.Valid {
		return fmt.Errorf("validation failed")
	}

	return nil
}

func outputValidateResultJSON(resp *pb.ValidateRuntimeYAMLResponse) error {
	output := map[string]interface{}{
		"valid":    resp.Valid,
		"message":  resp.Message,
		"errors":   resp.Errors,
		"warnings": resp.Warnings,
	}

	if resp.SpecInfo != nil {
		output["spec_info"] = map[string]interface{}{
			"name":             resp.SpecInfo.Name,
			"version":          resp.SpecInfo.Version,
			"language":         resp.SpecInfo.Language,
			"language_version": resp.SpecInfo.LanguageVersion,
			"description":      resp.SpecInfo.Description,
			"pip_packages":     resp.SpecInfo.PipPackages,
			"npm_packages":     resp.SpecInfo.NpmPackages,
			"has_hooks":        resp.SpecInfo.HasHooks,
			"requires_gpu":     resp.SpecInfo.RequiresGpu,
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func NewRuntimeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <runtime>",
		Short: "Remove a runtime environment",
		Long: `Remove an installed runtime environment and clean up its files.

Removal Behavior:
  - Without version: Removes ALL versions of the runtime
  - With @version:   Removes ONLY the specific version

Examples:
  # Remove ALL versions of Python 3.11 ML runtime
  rnx runtime remove python-3.11-ml

  # Remove ONLY version 1.3.1 of Python 3.11 ML runtime
  rnx runtime remove python-3.11-ml@1.3.1

  # Remove ALL versions of Java 21 runtime
  rnx runtime remove openjdk-21

  # Remove ONLY version 1.0.0 of Java 21 runtime
  rnx runtime remove openjdk-21@1.0.0`,
		Args: cobra.ExactArgs(1),
		RunE: runRuntimeRemove,
	}
}

func runRuntimeRemove(cmd *cobra.Command, args []string) error {
	runtimeSpec := args[0]

	fmt.Printf("🗑️  Removing runtime: %s\n", runtimeSpec)

	// Create client and connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Remove runtime via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RuntimeRemoveReq{Runtime: runtimeSpec}
	resp, err := client.RemoveRuntime(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to remove runtime: %w", err)
	}

	if resp.Success {
		fmt.Printf("Runtime removed successfully\n")
		if resp.Message != "" {
			fmt.Printf("Message: %s\n", resp.Message)
		}
		if resp.FreedSpaceBytes > 0 {
			fmt.Printf("Freed space: %s\n", formatSize(resp.FreedSpaceBytes))
		}
	} else {
		fmt.Printf("Failed to remove runtime\n")
		if resp.Message != "" {
			fmt.Printf("Error: %s\n", resp.Message)
		}
		return fmt.Errorf("runtime removal failed")
	}

	return nil
}
