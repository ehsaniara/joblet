package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"joblet/internal/rnx/common"
	"joblet/pkg/version"

	"github.com/spf13/cobra"
)

// VersionInfo holds both client and server version information
type VersionInfo struct {
	Client *version.BuildInfo `json:"client"`
	Server *ServerVersionInfo `json:"server,omitempty"`
	Error  string             `json:"error,omitempty"`
}

type ServerVersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	GitTag    string `json:"git_tag"`
	BuildDate string `json:"build_date"`
	Component string `json:"component"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	var clientOnly bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display detailed version information including build date, git commit, platform details, and server version (if connected).",
		Example: `  rnx version              # Show detailed client and server version info
  rnx version --json       # Output version info as JSON
  rnx version --client     # Show only client version (no server connection)`,
		Run: func(cmd *cobra.Command, args []string) {
			clientInfo := version.GetBuildInfo()
			var serverInfo *ServerVersionInfo
			var serverErr error

			// Only try to get server version if not client-only mode
			if !clientOnly {
				serverInfo, serverErr = getServerVersion()
			}

			if common.JSONOutput {
				versionInfo := VersionInfo{
					Client: &clientInfo,
					Server: serverInfo,
				}
				if serverErr != nil {
					versionInfo.Error = serverErr.Error()
				}

				jsonOutput, err := json.MarshalIndent(versionInfo, "", "  ")
				if err != nil {
					fmt.Printf("Error formatting version info: %v\n", err)
					return
				}
				fmt.Println(string(jsonOutput))
			} else {
				// Display client version
				fmt.Printf("RNX Client:\n")
				fmt.Print(version.GetLongVersion())

				// Display server version if available
				if serverInfo != nil {
					fmt.Printf("\nJoblet Server (%s):\n", common.NodeName)
					fmt.Printf("joblet version %s (%s)\n", serverInfo.Version, shortenCommit(serverInfo.GitCommit))
					if serverInfo.BuildDate != "unknown" {
						fmt.Printf("Built: %s\n", serverInfo.BuildDate)
					}
					if serverInfo.GitCommit != "unknown" {
						fmt.Printf("Commit: %s\n", serverInfo.GitCommit)
					}
					fmt.Printf("Go: %s\n", serverInfo.GoVersion)
					fmt.Printf("Platform: %s\n", serverInfo.Platform)
				} else if serverErr != nil {
					fmt.Printf("\nJoblet Server: %v\n", serverErr)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&clientOnly, "client", false, "Show only client version (skip server connection)")

	return cmd
}

func getServerVersion() (*ServerVersionInfo, error) {
	// Check if we have configuration available
	if common.NodeConfig == nil {
		return nil, fmt.Errorf("no configuration available")
	}

	client, err := common.NewJobClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	systemStatus, err := client.GetSystemStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}

	if systemStatus.ServerVersion == nil {
		return nil, fmt.Errorf("server version not available")
	}

	return &ServerVersionInfo{
		Version:   systemStatus.ServerVersion.Version,
		GitCommit: systemStatus.ServerVersion.GitCommit,
		GitTag:    systemStatus.ServerVersion.GitTag,
		BuildDate: systemStatus.ServerVersion.BuildDate,
		Component: systemStatus.ServerVersion.Component,
		GoVersion: systemStatus.ServerVersion.GoVersion,
		Platform:  systemStatus.ServerVersion.Platform,
	}, nil
}

func shortenCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// AddVersionFlag adds a --version flag to the root command
func AddVersionFlag(rootCmd *cobra.Command) {
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	// Handle --version flag directly in the root command
	originalRun := rootCmd.Run
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			// Show both client and server versions like the version command
			serverInfo, serverErr := getServerVersion()

			// Display client version
			fmt.Printf("RNX Client:\n")
			fmt.Print(version.GetLongVersion())

			// Display server version if available
			if serverInfo != nil {
				fmt.Printf("\nJoblet Server (%s):\n", common.NodeName)
				fmt.Printf("joblet version %s (%s)\n", serverInfo.Version, shortenCommit(serverInfo.GitCommit))
				if serverInfo.BuildDate != "unknown" {
					fmt.Printf("Built: %s\n", serverInfo.BuildDate)
				}
				if serverInfo.GitCommit != "unknown" {
					fmt.Printf("Commit: %s\n", serverInfo.GitCommit)
				}
				fmt.Printf("Go: %s\n", serverInfo.GoVersion)
				fmt.Printf("Platform: %s\n", serverInfo.Platform)
			} else if serverErr != nil {
				fmt.Printf("\nJoblet Server: %v\n", serverErr)
			}
			return
		}
		// Call original run function or show help if no original run function
		if originalRun != nil {
			originalRun(cmd, args)
		} else {
			_ = cmd.Help()
		}
	}
}
