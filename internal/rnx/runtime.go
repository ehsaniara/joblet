package rnx

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	pb "joblet/api/gen"

	"github.com/spf13/cobra"
)

func newRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Manage pre-built runtime environments",
		Long: `Manage pre-built runtime environments for fast job execution.

Runtimes provide pre-installed language environments and packages that can be
mounted into jobs, eliminating the need to install dependencies on every job run.

Examples:
  # List available runtimes
  rnx runtime list
  
  # Get information about a specific runtime
  rnx runtime info python:3.11+ml
  
  # Test a runtime
  rnx runtime test java:17
  
  # Install a new runtime (admin only)
  rnx runtime install python:3.11 --packages="numpy pandas scikit-learn"`,
	}

	cmd.AddCommand(newRuntimeListCmd())
	cmd.AddCommand(newRuntimeInfoCmd())
	cmd.AddCommand(newRuntimeTestCmd())

	return cmd
}

func newRuntimeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available runtimes",
		Long:  `List all available runtime environments that can be used with the --runtime flag.`,
		RunE:  runRuntimeList,
	}
}

func runRuntimeList(cmd *cobra.Command, args []string) error {
	// Create client and connect to server
	client, err := newJobClient()
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
		fmt.Println("No runtimes available.")
		fmt.Println("\nTo install runtimes, follow the runtime installation guide in the documentation.")
		return nil
	}

	// Display runtimes in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUNTIME\tVERSION\tSIZE\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-------\t----\t-----------")

	for _, rt := range runtimes {
		// Format runtime identifier
		runtimeID := fmt.Sprintf("%s:%s", rt.Language, strings.TrimPrefix(rt.Name, rt.Language+"-"))

		// Format size
		sizeStr := formatSize(rt.SizeBytes)

		// Truncate description if too long
		desc := rt.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			runtimeID,
			rt.Version,
			sizeStr,
			desc,
		)
	}

	w.Flush()

	fmt.Println("\nUse 'rnx runtime info <runtime>' for detailed information about a specific runtime.")

	return nil
}

func newRuntimeInfoCmd() *cobra.Command {
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

	// Create client and connect to server
	client, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Get runtime info from server via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RuntimeInfoReq{Runtime: runtimeSpec}
	resp, err := client.GetRuntimeInfo(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get runtime info: %w", err)
	}

	if !resp.Found {
		return fmt.Errorf("runtime not found: %s", runtimeSpec)
	}

	rt := resp.Runtime

	// Display runtime information
	fmt.Printf("Runtime: %s\n", rt.Name)
	fmt.Printf("Version: %s\n", rt.Version)
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
	fmt.Printf("  rnx run --runtime=%s <command>\n", runtimeSpec)

	return nil
}

func newRuntimeTestCmd() *cobra.Command {
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
	client, err := newJobClient()
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
		fmt.Printf("✓ Runtime test passed\n")
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

	// Parse runtime spec to determine test command suggestion
	var language string
	if strings.Contains(runtimeSpec, ":") {
		parts := strings.Split(runtimeSpec, ":")
		language = parts[0]
	} else if strings.Contains(runtimeSpec, "-") {
		parts := strings.Split(runtimeSpec, "-")
		language = parts[0]
	} else {
		language = runtimeSpec
	}

	var testCmd string
	switch language {
	case "python":
		testCmd = "python --version"
	case "java":
		testCmd = "java -version"
	case "node":
		testCmd = "node --version"
	case "go":
		testCmd = "go version"
	default:
		testCmd = "echo 'Hello from runtime'"
	}

	fmt.Printf("\nTo test the runtime in a job:\n")
	fmt.Printf("  rnx run --runtime=%s %s\n", runtimeSpec, testCmd)

	return nil
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
