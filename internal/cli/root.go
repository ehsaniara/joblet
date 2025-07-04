package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"worker/pkg/client"
	"worker/pkg/config"
)

var (
	nodeConfig     *config.ClientConfig
	configPath     string
	nodeName       string
	serverOverride string
)

var rootCmd = &cobra.Command{
	Use:   "worker-cli",
	Short: "Worker CLI client",
	Long:  "Command Line Interface to interact with Worker gRPC services",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Only allow server override to bypass config file requirement
		if serverOverride != "" {
			return // Skip config loading when using direct server connection
		}

		// Load client configuration - REQUIRED
		var err error
		nodeConfig, err = config.LoadClientConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			fmt.Fprintf(os.Stderr, "Please:\n")
			fmt.Fprintf(os.Stderr, "1. Create a client-config.yml file, or\n")
			fmt.Fprintf(os.Stderr, "2. Specify config path with --config, or\n")
			fmt.Fprintf(os.Stderr, "3. Use direct connection with --server\n\n")
			fmt.Fprintf(os.Stderr, "Example client-config.yml:\n")
			fmt.Fprintf(os.Stderr, "version: \"3.0\"\n")
			fmt.Fprintf(os.Stderr, "nodes:\n")
			fmt.Fprintf(os.Stderr, "  default:\n")
			fmt.Fprintf(os.Stderr, "    address: \"localhost:50051\"\n")
			fmt.Fprintf(os.Stderr, "    cert: \"./certs/client-cert.pem\"\n")
			fmt.Fprintf(os.Stderr, "    key: \"./certs/client-key.pem\"\n")
			fmt.Fprintf(os.Stderr, "    ca: \"./certs/ca-cert.pem\"\n")
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
		"Path to client configuration file")
	rootCmd.PersistentFlags().StringVar(&nodeName, "node", "default",
		"Node name from configuration file")
	rootCmd.PersistentFlags().StringVarP(&serverOverride, "server", "s", "",
		"Override server address (format: host:port) - bypasses config file")

	// Certificate flags (only used with --server override)
	rootCmd.PersistentFlags().String("cert", "",
		"Path to client certificate file (only with --server)")
	rootCmd.PersistentFlags().String("key", "",
		"Path to client key file (only with --server)")
	rootCmd.PersistentFlags().String("ca", "",
		"Path to CA certificate file (only with --server)")

	// Mark cert flags as hidden since they're only for --server usage
	_ = rootCmd.PersistentFlags().MarkHidden("cert")
	_ = rootCmd.PersistentFlags().MarkHidden("key")
	_ = rootCmd.PersistentFlags().MarkHidden("ca")

	// Add subcommands
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newNodesCmd())
	rootCmd.AddCommand(newHelpConfigCmd())
}

// newJobClient creates a client based on configuration or flags
func newJobClient() (*client.JobClient, error) {
	// If server is overridden via flag, use flag-based config
	if serverOverride != "" {
		certPath, _ := rootCmd.PersistentFlags().GetString("cert")
		keyPath, _ := rootCmd.PersistentFlags().GetString("key")
		caPath, _ := rootCmd.PersistentFlags().GetString("ca")

		// Require all certificate paths when using --server
		if certPath == "" || keyPath == "" || caPath == "" {
			return nil, fmt.Errorf("when using --server, you must specify --cert, --key, and --ca paths")
		}

		// Create client config for direct connection
		clientCfg := client.ClientConfig{
			ServerAddr:     serverOverride,
			ClientCertPath: certPath,
			ClientKeyPath:  keyPath,
			CACertPath:     caPath,
		}
		return client.NewJobClient(clientCfg)
	}

	// Use configuration file (nodeConfig should be loaded by PersistentPreRun)
	if nodeConfig == nil {
		return nil, fmt.Errorf("no configuration loaded - this should not happen")
	}

	// Get the specified node
	node, err := nodeConfig.GetNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node configuration for '%s': %w", nodeName, err)
	}

	// Expand relative paths based on config file location
	if configPath != "" {
		configDir := filepath.Dir(configPath)
		node.ExpandPaths(configDir)
	}

	// Convert config.Node to client.ClientConfig
	clientCfg := client.ClientConfig{
		ServerAddr:     node.Address,
		ClientCertPath: node.Cert,
		ClientKeyPath:  node.Key,
		CACertPath:     node.CA,
	}

	return client.NewJobClient(clientCfg)
}
