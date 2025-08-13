package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewAdminCmd() *cobra.Command {
	var port int
	var bindAddress string

	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Launch the Joblet Admin UI server",
		Long:  "Start the Node.js admin server that provides the web-based UI for managing jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminServer(port, bindAddress)
		},
	}

	adminCmd.Flags().IntVarP(&port, "port", "p", 5173, "Port to run the admin server on")
	adminCmd.Flags().StringVarP(&bindAddress, "bind-address", "b", "0.0.0.0", "Address to bind the server to")

	return adminCmd
}

func runAdminServer(port int, bindAddress string) error {
	// Get the rnx binary directory
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	rnxDir := filepath.Dir(executable)

	// Try production layout first: /opt/joblet/bin/rnx -> /opt/joblet/admin/server
	adminServerDir := filepath.Join(rnxDir, "..", "admin", "server")

	// If not found, try development layout: ./bin/rnx -> ./admin/server
	if _, err := os.Stat(adminServerDir); os.IsNotExist(err) {
		adminServerDir = filepath.Join(rnxDir, "..", "admin", "server")
	}

	// Check if admin server directory exists
	if _, err := os.Stat(adminServerDir); os.IsNotExist(err) {
		return fmt.Errorf("admin server not found at %s", adminServerDir)
	}

	// Check if package.json exists
	packageJsonPath := filepath.Join(adminServerDir, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return fmt.Errorf("admin server package.json not found at %s", packageJsonPath)
	}

	fmt.Printf("🚀 Starting Joblet Admin Server...\n")
	fmt.Printf("📂 Server directory: %s\n", adminServerDir)
	fmt.Printf("🌐 Address: http://%s:%d\n", bindAddress, port)
	fmt.Printf("⏹️  Press Ctrl+C to stop\n\n")

	// Set environment variables
	env := os.Environ()
	env = append(env, fmt.Sprintf("PORT=%d", port))
	env = append(env, fmt.Sprintf("BIND_ADDRESS=%s", bindAddress))
	env = append(env, fmt.Sprintf("RNX_PATH=%s", executable))

	// Run npm start in the admin server directory
	cmd := exec.Command("npm", "start")
	cmd.Dir = adminServerDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
