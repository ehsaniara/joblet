package rnx

import (
	"fmt"
	"os"

	"joblet/pkg/client"
	"joblet/pkg/config"

	"github.com/spf13/cobra"
)

var (
	nodeConfig *config.ClientConfig
	configPath string
	nodeName   string
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
		nodeConfig, err = config.LoadClientConfig(configPath)
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
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "",
		"Path to client configuration file (searches common locations if not specified)")
	rootCmd.PersistentFlags().StringVar(&nodeName, "node", "default",
		"Node name from configuration file")

	// Add subcommands
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newNodesCmd())
	rootCmd.AddCommand(newHelpConfigCmd())
	rootCmd.AddCommand(newNetworkCmd())
	rootCmd.AddCommand(newVolumeCmd())
	rootCmd.AddCommand(newMonitorCmd())
}

// newJobClient creates a client based on configuration
func newJobClient() (*client.JobClient, error) {
	// nodeConfig should be loaded by PersistentPreRun
	if nodeConfig == nil {
		return nil, fmt.Errorf("no configuration loaded - this should not happen")
	}

	// Get the specified node
	node, err := nodeConfig.GetNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node configuration for '%s': %w", nodeName, err)
	}

	// Create client directly from node (no more file path handling needed)
	return client.NewJobClient(node)
}
