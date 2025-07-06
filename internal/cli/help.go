package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHelpConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-help",
		Short: "Show configuration file examples with embedded certificates",
		Long:  "Display examples of client-config.yml file format with embedded certificates",
		RunE:  runConfigHelp,
	}

	return cmd
}

func runConfigHelp(cmd *cobra.Command, args []string) error {
	fmt.Println("Worker CLI Configuration Help - Embedded Certificates")
	fmt.Println("====================================================")
	fmt.Println()
	fmt.Println("The worker-cli requires a client-config.yml file with embedded certificates.")
	fmt.Println("This file contains all connection details and certificates in a single file.")
	fmt.Println()
	fmt.Println("Example client-config.yml with embedded certificates:")
	fmt.Println("----------------------------------------------------")
	fmt.Println(`version: "3.0"

nodes:
  default:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBCwUAMEUxCzAJBgNV
      BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
      ... (full certificate content) ...
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC66iJCE6liQCu+
      ... (full private key content) ...
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      MIIDQTCCAimgAwIBAgITBmyfz5m/jAo54vB4ikPmljZbyjANBgkqhkiG9w0BAQsF
      ... (full CA certificate content) ...
      -----END CERTIFICATE-----
  
  viewer:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ... (viewer certificate with read-only permissions) ...
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      ... (viewer private key) ...
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      ... (same CA certificate as above) ...
      -----END CERTIFICATE-----
  
  production:
    address: "prod-server:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ... (production client certificate) ...
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      ... (production private key) ...
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      ... (production CA certificate) ...
      -----END CERTIFICATE-----`)

	fmt.Println()
	fmt.Println("File locations searched (in order):")
	fmt.Println("1. ./client-config.yml")
	fmt.Println("2. ./config/client-config.yml")
	fmt.Println("3. ~/.worker/client-config.yml")
	fmt.Println("4. /etc/worker/client-config.yml")
	fmt.Println("5. /opt/worker/config/client-config.yml")
	fmt.Println()
	fmt.Println("Usage examples:")
	fmt.Println("  worker-cli list                    # uses 'default' node")
	fmt.Println("  worker-cli --node=viewer list      # uses 'viewer' node (read-only)")
	fmt.Println("  worker-cli --node=production list  # uses 'production' node")
	fmt.Println("  worker-cli --config=my-config.yml list  # uses custom config file")
	fmt.Println()
	fmt.Println("Getting the configuration file:")
	fmt.Println("-------------------------------")
	fmt.Println("1. From a Worker server:")
	fmt.Println("   scp server:/opt/worker/config/client-config.yml ~/.worker/")
	fmt.Println()
	fmt.Println("2. Generate new certificates:")
	fmt.Println("   WORKER_SERVER_ADDRESS='your-server' /usr/local/bin/certs_gen.sh")
	fmt.Println()
	fmt.Println("Security notes:")
	fmt.Println("--------------")
	fmt.Println("⚠️  Keep client-config.yml secure - it contains private keys")
	fmt.Println("⚠️  Use file permissions 600 to restrict access")
	fmt.Println("⚠️  Different certificates provide different access levels")
	fmt.Println("⚠️  Never commit actual config files to version control")

	return nil
}
