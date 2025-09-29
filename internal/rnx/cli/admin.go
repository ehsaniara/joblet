package cli

import (
	"encoding/json"
	"fmt"
	"joblet/internal/rnx/common"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	adminServerDir := ""

	// Try different installation layouts in order of preference

	// 1. Dynamic homebrew detection: get homebrew prefix and check share/rnx/admin
	if homebrewPrefix := getHomebrewPrefix(); homebrewPrefix != "" {
		homebrewPath := filepath.Join(homebrewPrefix, "share", "rnx", "admin", "server")
		if _, err := os.Stat(homebrewPath); err == nil {
			adminServerDir = homebrewPath
		}
	}

	// 2. Static homebrew paths (fallback)
	if adminServerDir == "" {
		homebrewPaths := []string{
			"/opt/homebrew/share/rnx/admin/server", // Apple Silicon homebrew
			"/usr/local/share/rnx/admin/server",    // Intel homebrew
		}

		for _, path := range homebrewPaths {
			if _, err := os.Stat(path); err == nil {
				adminServerDir = path
				break
			}
		}
	}

	// 3. Relative to binary location (production/development)
	if adminServerDir == "" {
		// Production layout: /opt/joblet/bin/rnx -> /opt/joblet/admin/server
		prodPath := filepath.Join(rnxDir, "..", "admin", "server")
		if _, err := os.Stat(prodPath); err == nil {
			adminServerDir = prodPath
		} else {
			// Development layout: ./bin/rnx -> ./admin/server
			devPath := filepath.Join(rnxDir, "..", "admin", "server")
			if _, err := os.Stat(devPath); err == nil {
				adminServerDir = devPath
			}
		}
	}

	// Check if admin server directory exists
	if adminServerDir == "" || func() bool {
		_, err := os.Stat(adminServerDir)
		return os.IsNotExist(err)
	}() {
		return fmt.Errorf(`admin UI not found - the admin UI may not be installed

Possible solutions:
• If using Homebrew: brew reinstall rnx --with-admin
• If using packages: ensure admin UI was included in installation
• Checked locations:
  - Homebrew: %s
  - Production: %s
  - Development: %s`,
			getHomebrewAdminPath(),
			filepath.Join(rnxDir, "..", "admin", "server"),
			filepath.Join(rnxDir, "..", "admin", "server"))
	}

	// Check if package.json exists
	packageJsonPath := filepath.Join(adminServerDir, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return fmt.Errorf("admin server package.json not found at %s", packageJsonPath)
	}

	if common.JSONOutput {
		serverInfo := map[string]interface{}{
			"status":         "starting",
			"directory":      adminServerDir,
			"bind_address":   bindAddress,
			"port":           port,
			"url":            fmt.Sprintf("http://%s:%d", bindAddress, port),
			"browser_opened": false,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(serverInfo); err != nil {
			return fmt.Errorf("failed to output JSON: %v", err)
		}
	} else {
		fmt.Printf("Starting Joblet Admin Server...\n")
		fmt.Printf("Server directory: %s\n", adminServerDir)
		fmt.Printf("Address: http://%s:%d\n", bindAddress, port)
		fmt.Printf("Press Ctrl+C to stop\n\n")

		// Try to open browser automatically (like Kiali does)
		url := fmt.Sprintf("http://localhost:%d", port)
		go func() {
			// Wait a moment for server to start
			time.Sleep(2 * time.Second)

			var openCmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				openCmd = exec.Command("open", url)
			case "linux":
				openCmd = exec.Command("xdg-open", url)
			case "windows":
				openCmd = exec.Command("cmd", "/c", "start", url)
			}

			if openCmd != nil {
				_ = openCmd.Run() // Ignore errors - browser opening is optional
				fmt.Printf("Browser opened at %s\n", url)
			}
		}()
	}

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

// getHomebrewPrefix dynamically detects the homebrew installation prefix
func getHomebrewPrefix() string {
	// Try to run 'brew --prefix' to get the actual homebrew prefix
	cmd := exec.Command("brew", "--prefix")
	output, err := cmd.Output()
	if err == nil {
		prefix := strings.TrimSpace(string(output))
		if prefix != "" {
			return prefix
		}
	}

	// Fallback: check if we can detect from the PATH
	if brewPath, err := exec.LookPath("brew"); err == nil {
		// brew is typically at PREFIX/bin/brew, so get parent of parent
		brewDir := filepath.Dir(brewPath)
		if filepath.Base(brewDir) == "bin" {
			return filepath.Dir(brewDir)
		}
	}

	return ""
}

// getHomebrewAdminPath returns the expected homebrew admin path for error messages
func getHomebrewAdminPath() string {
	if prefix := getHomebrewPrefix(); prefix != "" {
		return filepath.Join(prefix, "share", "rnx", "admin", "server")
	}
	return "/opt/homebrew/share/rnx/admin/server or /usr/local/share/rnx/admin/server"
}
