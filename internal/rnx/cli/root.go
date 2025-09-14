package cli

import (
	"fmt"
	"os"

	"joblet/internal/rnx/common"
	"joblet/internal/rnx/jobs"
	"joblet/internal/rnx/resources"
	"joblet/pkg/config"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rnx",
	Short: "RNX - Remote eXecution client for Joblet",
	Long: `RNX (Remote eXecution) - Command Line Interface to interact with Joblet gRPC services using embedded certificates

RNX provides a complete interface for job execution, workflow management, and resource control
on Joblet servers. It supports immediate execution, scheduling, and comprehensive monitoring.

Key Features:
  - Execute jobs with resource limits and scheduling
  - Manage multi-job workflows with dependencies  
  - Create and manage networks, volumes, and runtimes
  - Monitor remote server resources, job performance, and volume usage
  - Stream real-time logs from running jobs

Quick Examples:
  rnx run python script.py                    # Run a simple job
  rnx run --workflow=pipeline.yaml            # Execute a workflow
  rnx list --workflow                         # List all workflows
  rnx status <job-uuid>                       # Check job status (supports short UUIDs)
  rnx log <job-uuid>                          # Stream job logs (supports short UUIDs)
  rnx delete <job-uuid>                       # Delete a specific job
  rnx delete-all                              # Delete all non-running jobs
  rnx monitor status                          # View remote server metrics and volumes
  rnx monitor top --json                      # JSON output for dashboards

Note: Job and workflow UUIDs support short-form usage (first 8 characters)
if they uniquely identify the resource.

Use 'rnx <command> --help' for detailed information about any command.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip config loading for run command since it has DisableFlagParsing and handles config loading manually
		if cmd.Name() == "run" {
			return
		}

		// Check if --version flag is used - version commands can work without config
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag || cmd.Name() == "version" {
			// Try to load config but don't exit on failure
			common.NodeConfig, _ = config.LoadClientConfig(common.ConfigPath)
			return
		}

		// Load client configuration - REQUIRED (no direct server connections)
		var err error
		common.NodeConfig, err = config.LoadClientConfig(common.ConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintf(os.Stderr, "Please create a rnx-config.yml file with embedded certificates.\n")
			fmt.Fprintf(os.Stderr, "Use 'rnx config-help' for examples.\n")
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Disable Cobra's automatic completion command generation
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Global flags
	rootCmd.PersistentFlags().StringVar(&common.ConfigPath, "config", "",
		"Path to client configuration file (searches common locations if not specified)")
	rootCmd.PersistentFlags().StringVar(&common.NodeName, "node", "default",
		"Node name from configuration file")
	rootCmd.PersistentFlags().BoolVar(&common.JSONOutput, "json", false,
		"Output in JSON format")

	// Add subcommands
	rootCmd.AddCommand(jobs.NewRunCmd())
	rootCmd.AddCommand(jobs.NewStatusCmd())
	rootCmd.AddCommand(jobs.NewStopCmd())
	rootCmd.AddCommand(jobs.NewDeleteCmd())
	rootCmd.AddCommand(jobs.NewDeleteAllCmd())
	rootCmd.AddCommand(jobs.NewLogCmd())
	rootCmd.AddCommand(jobs.NewListCmd())
	rootCmd.AddCommand(NewNodesCmd())
	rootCmd.AddCommand(NewHelpConfigCmd())
	rootCmd.AddCommand(resources.NewNetworkCmd())
	rootCmd.AddCommand(resources.NewVolumeCmd())
	rootCmd.AddCommand(jobs.NewMonitorCmd())
	rootCmd.AddCommand(resources.NewRuntimeCmd())
	rootCmd.AddCommand(NewAdminCmd())
	rootCmd.AddCommand(NewVersionCmd())

	// Add --version flag support
	AddVersionFlag(rootCmd)
}
