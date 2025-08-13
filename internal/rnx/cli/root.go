package cli

import (
	"fmt"
	"os"

	"joblet/internal/rnx/common"
	"joblet/internal/rnx/jobs"
	"joblet/internal/rnx/resources"
	"joblet/internal/rnx/workflow"
	"joblet/pkg/config"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rnx",
	Short: "RNX - Remote eXecution client for Joblet",
	Long:  "RNX (Remote eXecution) - Command Line Interface to interact with Joblet gRPC services using embedded certificates",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip config loading for run command since it has DisableFlagParsing and handles config loading manually
		if cmd.Name() == "run" {
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
	// Global flags
	rootCmd.PersistentFlags().StringVar(&common.ConfigPath, "config", "",
		"Path to client configuration file (searches common locations if not specified)")
	rootCmd.PersistentFlags().StringVar(&common.NodeName, "node", "default",
		"Node name from configuration file")

	// Add subcommands
	rootCmd.AddCommand(jobs.NewRunCmd())
	rootCmd.AddCommand(jobs.NewStatusCmd())
	rootCmd.AddCommand(jobs.NewStopCmd())
	rootCmd.AddCommand(jobs.NewLogCmd())
	rootCmd.AddCommand(jobs.NewListCmd())
	rootCmd.AddCommand(NewNodesCmd())
	rootCmd.AddCommand(NewHelpConfigCmd())
	rootCmd.AddCommand(resources.NewNetworkCmd())
	rootCmd.AddCommand(resources.NewVolumeCmd())
	rootCmd.AddCommand(jobs.NewMonitorCmd())
	rootCmd.AddCommand(resources.NewRuntimeCmd())
	rootCmd.AddCommand(workflow.NewWorkflowCmd())
	rootCmd.AddCommand(NewAdminCmd())
}
