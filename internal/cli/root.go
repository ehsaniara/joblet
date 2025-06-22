package cli

import (
	"github.com/spf13/cobra"
	"job-worker/internal/config"
)

var (
	cfg = config.NewConfig()
)

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "Job Worker CLI client",
	Long:  "Command Line Interface to interact with the Job Worker gRPC service running in host machines",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfg.ServerAddr, "server", "s", "192.168.1.161:50051", "Address format host:port")

	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newListCmd())
}
