package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/ehsaniara/joblet/pkg/client"
	"github.com/ehsaniara/joblet/pkg/registry"
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
  
  # Install from local codebase
  rnx runtime install openjdk-21
  
  # Get information about a specific runtime (language, database, etc.)
  rnx runtime info openjdk-21
  
  # Test a runtime
  rnx runtime test openjdk-21
  
  # Remove a runtime
  rnx runtime remove python-3.11-ml`,
	}

	cmd.AddCommand(NewRuntimeListCmd())
	cmd.AddCommand(NewRuntimeInfoCmd())
	cmd.AddCommand(NewRuntimeTestCmd())
	cmd.AddCommand(NewRuntimeInstallCmd())
	cmd.AddCommand(NewRuntimeValidateCmd())
	cmd.AddCommand(NewRuntimeRemoveCmd())

	return cmd
}

func NewRuntimeListCmd() *cobra.Command {
	var registryURL string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available runtimes",
		Long: `List all available runtime environments that can be used with the --runtime flag.

Examples:
  # List locally installed runtimes
  rnx runtime list

  # List available runtimes from the default registry
  rnx runtime list --registry

  # List available runtimes from a custom registry
  rnx runtime list --registry=myorg/custom-runtimes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeList(cmd, args, registryURL)
		},
	}

	cmd.Flags().StringVar(&registryURL, "registry", "", "List available runtimes from GitHub registry (default: ehsaniara/joblet-runtimes). Format: owner/repo")

	// Set NoOptDefVal so --registry works without a value
	cmd.Flags().Lookup("registry").NoOptDefVal = "ehsaniara/joblet-runtimes"

	return cmd
}

func runRuntimeList(cmd *cobra.Command, args []string, registryURL string) error {
	// If registry flag is provided, list runtimes from registry
	if cmd.Flags().Changed("registry") {
		// Normalize and validate registry URL (only GitHub registries allowed)
		normalizedURL, err := normalizeRegistryURL(registryURL)
		if err != nil {
			return fmt.Errorf("invalid registry: %w", err)
		}
		return runRegistryRuntimeList(normalizedURL)
	}

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
			fmt.Println("\nTo install runtimes, follow the runtime installation guide in the documentation.")
		}
		return nil
	}

	if common.JSONOutput {
		return outputRuntimesJSON(runtimes)
	}

	// Display runtimes in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUNTIME\tVERSION\tSIZE\tREGISTRY\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-------\t----\t--------\t-----------")

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

		// Registry source - for now, all locally installed runtimes show "local"
		// In the future, this could be tracked in runtime.yml during installation
		registry := "local"

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			runtimeID,
			rt.Version,
			sizeStr,
			registry,
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

	fmt.Println("\nUsage:")
	fmt.Printf("  rnx job run --runtime=%s <command>\n", runtimeSpec)

	return nil
}

func outputRuntimeInfoJSON(rt *pb.RuntimeInfo, runtimeSpec string) error {
	// Create JSON output structure
	output := map[string]interface{}{
		"name":        rt.Name,
		"version":     rt.Version,
		"description": rt.Description,
		"packages":    rt.Packages,
		"usage":       fmt.Sprintf("rnx job run --runtime=%s <command>", runtimeSpec),
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

func NewRuntimeInstallCmd() *cobra.Command {
	var force bool
	var registryURL string

	cmd := &cobra.Command{
		Use:   "install <runtime-spec>",
		Short: "Install a runtime environment with optional version",
		Long: `Install a runtime environment from an external registry.

Runtime Specification Format:
  - <runtime-name>@<version>  (e.g., python-3.11-ml@1.0.0)
  - <runtime-name>@latest     (explicitly request latest)
  - <runtime-name>            (defaults to latest)

The runtime is installed from the GitHub registry:
  - Default registry: ehsaniara/joblet-runtimes (used by default)
  - Custom registry: using --registry flag (format: owner/repo)

Examples:
  # Install specific version from default registry
  rnx runtime install python-3.11-ml@1.0.0

  # Install latest version from default registry
  rnx runtime install python-3.11-ml@latest

  # Install from default registry (implicit @latest)
  rnx runtime install python-3.11-ml

  # Use custom registry
  rnx runtime install python-3.11-ml --registry=myorg/custom-runtimes

  # Install specific version from custom registry
  rnx runtime install custom-runtime@2.0.0 --registry=myorg/runtimes

  # Force reinstall existing runtime
  rnx runtime install python-3.11-ml@1.0.0 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeInstall(cmd, args[0], force, registryURL)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall if runtime already exists")
	cmd.Flags().StringVar(&registryURL, "registry", "", "GitHub runtime registry (default: ehsaniara/joblet-runtimes). Format: owner/repo")

	// Set NoOptDefVal so --registry works without a value
	cmd.Flags().Lookup("registry").NoOptDefVal = "ehsaniara/joblet-runtimes"

	return cmd
}

func runRuntimeInstall(cmd *cobra.Command, runtimeSpec string, force bool, registryURL string) error {
	ctx := cmd.Context()

	// Parse runtime spec to extract name and version
	spec, err := runtime.ParseRuntimeSpec(runtimeSpec)
	if err != nil {
		return fmt.Errorf("invalid runtime specification: %w", err)
	}

	// Display what we're installing
	if spec.IsLatest() {
		fmt.Printf("🏗️  Installing runtime: %s (latest version)\n", spec.Name)
	} else {
		fmt.Printf("🏗️  Installing runtime: %s@%s\n", spec.Name, spec.Version)
	}

	// Create client and connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Install from external runtime registry
	// Normalize and validate registry URL (only GitHub registries allowed)
	normalizedURL, err := normalizeRegistryURL(registryURL)
	if err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}
	fmt.Printf("📦 Installing from external registry: %s\n", normalizedURL)

	return runStreamingRegistryRuntimeInstall(ctx, client, spec.Original, force, normalizedURL)
}

func NewRuntimeValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <runtime-spec>",
		Short: "Validate a runtime specification",
		Long: `Validate a runtime specification format and check if it's supported.

Examples:
  # Validate basic spec
  rnx runtime validate python-3.11-ml
  
  # Validate spec with variants
  rnx runtime validate python-3.11-ml
  
  # Validate spec with architecture
  rnx runtime validate openjdk-21`,
		Args: cobra.ExactArgs(1),
		RunE: runRuntimeValidate,
	}
}

func runRuntimeValidate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	runtimeSpec := args[0]

	// Create client and connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	req := &pb.ValidateRuntimeSpecRequest{
		RuntimeSpec: runtimeSpec,
	}

	resp, err := client.ValidateRuntimeSpec(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to validate runtime spec: %w", err)
	}

	if resp.Valid {
		fmt.Printf("Runtime specification is valid\n")
		fmt.Printf("Original: %s\n", runtimeSpec)
		fmt.Printf("Normalized: %s\n", resp.NormalizedSpec)

		if resp.SpecInfo != nil {
			fmt.Printf("\nParsed Information:\n")
			fmt.Printf("  Language: %s\n", resp.SpecInfo.Language)
			fmt.Printf("  Version: %s\n", resp.SpecInfo.Version)

			if len(resp.SpecInfo.Variants) > 0 {
				fmt.Printf("  Variants: %s\n", strings.Join(resp.SpecInfo.Variants, ", "))
			}

			fmt.Printf("  Architecture: %s\n", resp.SpecInfo.Architecture)
		}
	} else {
		fmt.Printf("Runtime specification is invalid\n")
		fmt.Printf("Error: %s\n", resp.Message)
		return fmt.Errorf("invalid runtime specification")
	}

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

// runRegistryRuntimeList lists available runtimes from the runtime registry
func runRegistryRuntimeList(registryURL string) error {
	fmt.Printf("📦 Fetching available runtimes from registry: %s\n", registryURL)
	fmt.Println()

	// Create registry client
	registryClient := registry.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch registry
	reg, err := registryClient.FetchRegistry(ctx, registryURL)
	if err != nil {
		return fmt.Errorf("failed to fetch runtime registry: %w", err)
	}

	// Display registry info
	fmt.Printf("✓ Registry fetched successfully\n")
	fmt.Printf("📅 Last updated: %s\n", reg.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Println()

	// Check if registry has runtimes
	if len(reg.Runtimes) == 0 {
		if common.JSONOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No runtimes available in this registry.")
		}
		return nil
	}

	// Handle JSON output
	if common.JSONOutput {
		return outputRegistryRuntimesJSON(reg)
	}

	// Display runtimes in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUNTIME\tVERSIONS\tLATEST\tPLATFORMS\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t--------\t------\t---------\t-----------")

	// Sort runtime names for consistent output
	runtimeNames := make([]string, 0, len(reg.Runtimes))
	for name := range reg.Runtimes {
		runtimeNames = append(runtimeNames, name)
	}
	sort.Strings(runtimeNames)

	// Display each runtime
	for _, name := range runtimeNames {
		versions := reg.Runtimes[name]

		// Get all version numbers
		versionList := make([]string, 0, len(versions))
		for version := range versions {
			versionList = append(versionList, version)
		}
		sort.Strings(versionList)

		// Determine latest version (last in sorted list)
		latestVersion := versionList[len(versionList)-1]
		latestEntry := versions[latestVersion]

		// Format versions display
		versionCount := len(versionList)
		var versionDisplay string
		if versionCount <= 3 {
			versionDisplay = strings.Join(versionList, ", ")
		} else {
			versionDisplay = fmt.Sprintf("%d versions", versionCount)
		}

		// Format platforms
		platformCount := len(latestEntry.Platforms)
		platformDisplay := fmt.Sprintf("%d platforms", platformCount)

		// Truncate description if too long
		desc := latestEntry.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name,
			versionDisplay,
			latestVersion,
			platformDisplay,
			desc,
		)
	}

	w.Flush()
	fmt.Printf("\nFound %d runtime(s) in registry\n", len(reg.Runtimes))
	fmt.Println()
	fmt.Println("Install with: rnx runtime install <runtime>@<version>")
	fmt.Println("Examples:")
	fmt.Println("  rnx runtime install python-3.11@1.3.1")
	fmt.Println("  rnx runtime install python-3.11-ml@latest")
	fmt.Println()
	fmt.Println("For custom registry:")
	fmt.Println("  rnx runtime install <runtime>@<version> --registry=owner/repo")

	return nil
}

// outputRegistryRuntimesJSON outputs registry runtimes as JSON
func outputRegistryRuntimesJSON(reg *registry.Registry) error {
	type RuntimeJSON struct {
		Name        string   `json:"name"`
		Versions    []string `json:"versions"`
		Latest      string   `json:"latest"`
		Description string   `json:"description"`
		Platforms   []string `json:"platforms"`
	}

	runtimes := make([]RuntimeJSON, 0, len(reg.Runtimes))

	for name, versions := range reg.Runtimes {
		// Get all version numbers
		versionList := make([]string, 0, len(versions))
		for version := range versions {
			versionList = append(versionList, version)
		}
		sort.Strings(versionList)

		// Get latest version
		latestVersion := versionList[len(versionList)-1]
		latestEntry := versions[latestVersion]

		runtimes = append(runtimes, RuntimeJSON{
			Name:        name,
			Versions:    versionList,
			Latest:      latestVersion,
			Description: latestEntry.Description,
			Platforms:   latestEntry.Platforms,
		})
	}

	// Sort by name for consistent output
	sort.Slice(runtimes, func(i, j int) bool {
		return runtimes[i].Name < runtimes[j].Name
	})

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(runtimes)
}

// runStreamingRegistryRuntimeInstall installs runtime from external registry with streaming logs
func runStreamingRegistryRuntimeInstall(ctx context.Context, client *client.JobClient, runtimeSpec string, force bool, registryURL string) error {
	fmt.Printf("Starting registry runtime installation...\n\n")

	// Create streaming installation request WITHOUT repository info
	// Server will detect this and route to registry
	req := &pb.InstallRuntimeRequest{
		RuntimeSpec:    runtimeSpec,
		ForceReinstall: force,
		RegistryUrl:    registryURL, // Use dedicated registry URL field
		// Repository, Path, Branch are empty - signals registry installation
	}

	stream, err := client.StreamingInstallRuntimeFromGithub(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start streaming registry runtime installation: %w", err)
	}

	// Process streaming chunks
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// Stream completed successfully
			break
		}
		if err != nil {
			return fmt.Errorf("streaming error: %w", err)
		}

		switch chunk.ChunkType.(type) {
		case *pb.RuntimeInstallationChunk_Progress:
			progress := chunk.GetProgress()
			fmt.Printf("📊 %s\n", progress.Message)

		case *pb.RuntimeInstallationChunk_Log:
			log := chunk.GetLog()
			fmt.Print(string(log.Data))

		case *pb.RuntimeInstallationChunk_Result:
			result := chunk.GetResult()
			if result.Success {
				fmt.Printf("\n🎉 %s\n", result.Message)
				if result.InstallPath != "" {
					fmt.Printf("📍 Installed at: %s\n", result.InstallPath)
				}
				return nil
			} else {
				return fmt.Errorf("registry runtime installation failed: %s", result.Message)
			}
		}
	}

	fmt.Printf("\nRegistry runtime installation completed successfully!\n")
	return nil
}

// normalizeRegistryURL converts shorthand GitHub repo format to full URL
// and validates that only shorthand format (owner/repo) is allowed
// Examples:
//   - "ehsaniara/joblet-runtimes" -> "https://github.com/ehsaniara/joblet-runtimes"
//   - "myorg/custom-runtimes" -> "https://github.com/myorg/custom-runtimes"
//   - "https://github.com/..." -> error (full URLs not allowed)
func normalizeRegistryURL(registryURL string) (string, error) {
	// If empty, return default
	if registryURL == "" {
		return "https://github.com/ehsaniara/joblet-runtimes", nil
	}

	// Reject full URLs - only shorthand format allowed
	if strings.HasPrefix(registryURL, "http://") || strings.HasPrefix(registryURL, "https://") {
		return "", fmt.Errorf("registry flag should not contain http:// or https://. Use shorthand format: owner/repo")
	}

	// Validate shorthand GitHub repo format (owner/repo)
	parts := strings.Split(registryURL, "/")
	if len(parts) >= 2 {
		// Extract just owner/repo (in case there are additional path segments)
		ownerRepo := fmt.Sprintf("%s/%s", parts[0], parts[1])
		return fmt.Sprintf("https://github.com/%s", ownerRepo), nil
	}

	// Invalid format
	return "", fmt.Errorf("invalid registry format. Expected: owner/repo")
}
