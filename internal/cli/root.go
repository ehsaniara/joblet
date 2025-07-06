package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"worker/pkg/client"
	"worker/pkg/config"
)

var (
	nodeConfig *config.ClientConfig
	configPath string
	nodeName   string
)

var rootCmd = &cobra.Command{
	Use:   "worker-cli",
	Short: "Worker CLI client",
	Long:  "Command Line Interface to interact with Worker gRPC services using embedded certificates",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Load client configuration - REQUIRED (no direct server connections)
		var err error
		nodeConfig, err = config.LoadClientConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintf(os.Stderr, "Please create a client-config.yml file with embedded certificates.\n")
			fmt.Fprintf(os.Stderr, "Use 'worker-cli config-help' for examples.\n")
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
