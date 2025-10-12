package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/ehsaniara/joblet/pkg/version"

	"github.com/spf13/cobra"
)

func showVersion() {
	fmt.Printf("rnx %s\n", version.GetShortVersion())

	// Try to get server version from default node
	if serverVersion := getServerVersion(); serverVersion != "" {
		fmt.Printf("joblet %s\n", serverVersion)
	}
}

func showJSONVersion() {
	clientInfo := version.GetBuildInfo()

	data := map[string]interface{}{
		"rnx": map[string]interface{}{
			"version":      version.GetShortVersion(),
			"git_commit":   clientInfo.GitCommit,
			"git_tag":      clientInfo.GitTag,
			"build_date":   clientInfo.BuildDate,
			"go_version":   clientInfo.GoVersion,
			"platform":     fmt.Sprintf("%s/%s", clientInfo.Platform, clientInfo.Architecture),
			"proto_commit": clientInfo.ProtoCommit,
		},
	}

	// Try to get server version from default node
	if serverVersion := getServerVersion(); serverVersion != "" {
		data["joblet"] = map[string]interface{}{
			"version": serverVersion,
		}
	}

	jsonOutput, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonOutput))
}

func getServerVersion() string {
	if common.NodeConfig == nil {
		return ""
	}

	client, err := common.NewJobClient()
	if err != nil {
		return ""
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	systemStatus, err := client.GetSystemStatus(ctx)
	if err != nil || systemStatus.ServerVersion == nil {
		return ""
	}

	return version.FormatVersion(systemStatus.ServerVersion.Version, systemStatus.ServerVersion.GitCommit)
}

// AddVersionFlag adds a --version flag to the root command
func AddVersionFlag(rootCmd *cobra.Command) {
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")

	originalRun := rootCmd.Run
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			if common.JSONOutput {
				showJSONVersion()
			} else {
				showVersion()
			}
			return
		}
		if originalRun != nil {
			originalRun(cmd, args)
		} else {
			_ = cmd.Help()
		}
	}
}
