package rnx

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHelpConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-help",
		Short: "Show configuration file examples with embedded certificates",
		Long:  "Display examples of rnx-config.yml file format with embedded certificates",
		RunE:  runConfigHelp,
	}

	return cmd
}

func runConfigHelp(cmd *cobra.Command, args []string) error {
	fmt.Println("RNX Configuration Help - Embedded Certificates")
	fmt.Println("==============================================")
	fmt.Println()
	fmt.Println("RNX requires a rnx-config.yml file with embedded certificates.")
	fmt.Println("This file contains all connection details and certificates in a single file.")
	fmt.Println()
	fmt.Println("Example rnx-config.yml with embedded certificates:")
	fmt.Println("------------------------------------------------")
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
      -----END CERTIFICATE-----`)

	fmt.Println()
	fmt.Println("File locations searched (in order):")
	fmt.Println("1. ./rnx-config.yml")
	fmt.Println("2. ./config/rnx-config.yml")
	fmt.Println("3. ~/.rnx/rnx-config.yml")
	fmt.Println("4. /etc/joblet/rnx-config.yml")
	fmt.Println("5. /opt/joblet/config/rnx-config.yml")
	fmt.Println()
	fmt.Println("Usage examples:")
	fmt.Println("  rnx list                    # uses 'default' node")
	fmt.Println("  rnx --node=viewer list      # uses 'viewer' node (read-only)")
	fmt.Println("  rnx --node=production list  # uses 'production' node")
	fmt.Println("  rnx --config=my-rnx-config.yml list  # uses custom config file")
	fmt.Println()
	fmt.Println("Getting the configuration file:")
	fmt.Println("-------------------------------")
	fmt.Println("1. From a Joblet server:")
	fmt.Println("   scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/")
	fmt.Println()
	fmt.Println("2. Generate new certificates:")
	fmt.Println("   JOBLET_SERVER_ADDRESS='your-server' /usr/local/bin/certs_gen_embedded.sh")
	fmt.Println()
	fmt.Println("Security notes:")
	fmt.Println("--------------")
	fmt.Println("⚠️  Keep rnx-config.yml secure - it contains private keys")
	fmt.Println("⚠️  Use file permissions 600 to restrict access")
	fmt.Println("⚠️  Different certificates provide different access levels")
	fmt.Println("⚠️  Never commit actual config files to version control")

	return nil
}
