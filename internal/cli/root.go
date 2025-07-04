package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"worker/pkg/client"
	"worker/pkg/config"
)

var (
	cfg *config.CLIConfig
)

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Worker CLI client",
	Long:  "Command Line Interface to interact with the Worker gRPC service running in host machines",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// This runs before any subcommand
		// Validate configuration if needed
		if err := cfg.Validate(); err != nil {
			fmt.Printf("Warning: Configuration validation failed: %v\n", err)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Load CLI configuration using unified config
	cfg = config.LoadCLIConfig()

	// Add server flag with loaded default
	rootCmd.PersistentFlags().StringVarP(&cfg.ServerAddr, "server", "s", cfg.ServerAddr,
		"Server address in format host:port")

	// Add certificate flags with loaded defaults (usually not overridden via CLI)
	rootCmd.PersistentFlags().StringVar(&cfg.ClientCertPath, "cert", cfg.ClientCertPath,
		"Path to client certificate file")
	rootCmd.PersistentFlags().StringVar(&cfg.ClientKeyPath, "key", cfg.ClientKeyPath,
		"Path to client key file")
	rootCmd.PersistentFlags().StringVar(&cfg.CACertPath, "ca", cfg.CACertPath,
		"Path to CA certificate file")

	// Mark certificate flags as hidden since they're usually configured via config file
	_ = rootCmd.PersistentFlags().MarkHidden("cert")
	_ = rootCmd.PersistentFlags().MarkHidden("key")
	_ = rootCmd.PersistentFlags().MarkHidden("ca")

	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newListCmd())
}

// Convenience function for CLI commands
func newJobClient() (*client.JobClient, error) {
	return client.NewJobClientFromCLIConfig(cfg)
}
