package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHelpConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-help",
		Short: "Show configuration file examples",
		Long:  "Display examples of client-config.yml file format",
		RunE:  runConfigHelp,
	}

	return cmd
}

func runConfigHelp(cmd *cobra.Command, args []string) error {
	fmt.Println("Worker CLI Configuration Help")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("The worker-cli requires a client-config.yml file with connection details.")
	fmt.Println()
	fmt.Println("Example client-config.yml:")
	fmt.Println("-------------------------")
	fmt.Println(`version: "3.0"

nodes:
  default:
    address: "localhost:50051"
    cert: "./certs/client-cert.pem"
    key: "./certs/client-key.pem"
    ca: "./certs/ca-cert.pem"
  
  production:
    address: "prod-server:50051"
    cert: "./certs/prod/client-cert.pem"
    key: "./certs/prod/client-key.pem"
    ca: "./certs/prod/ca-cert.pem"
  
  staging:
    address: "staging-server:50051"
    cert: "./certs/staging/client-cert.pem"
    key: "./certs/staging/client-key.pem"
    ca: "./certs/staging/ca-cert.pem"`)

	fmt.Println()
	fmt.Println("File locations searched (in order):")
	fmt.Println("1. ./client-config.yml")
	fmt.Println("2. ./config/client-config.yml")
	fmt.Println("3. ~/.worker/client-config.yml")
	fmt.Println("4. /etc/worker/client-config.yml")
	fmt.Println()
	fmt.Println("Usage examples:")
	fmt.Println("  worker-cli list                    # uses 'default' node")
	fmt.Println("  worker-cli --node=production list  # uses 'production' node")
	fmt.Println("  worker-cli --config=my-config.yml list  # uses custom config file")
	fmt.Println()
	fmt.Println("Direct connection (bypasses config file):")
	fmt.Println("  worker-cli --server=host:port --cert=cert.pem --key=key.pem --ca=ca.pem list")

	return nil
}
