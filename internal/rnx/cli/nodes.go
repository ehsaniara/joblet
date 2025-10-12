package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
)

func NewNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "List available nodes from configuration",
		Long:  "Display all configured nodes and their connection details from rnx-config.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodes(common.JSONOutput)
		},
	}

	return cmd
}

type NodeInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	NodeId  string `json:"nodeId,omitempty"`
	Status  string `json:"status"`
	Default bool   `json:"default"`
}

func runNodes(jsonOutput bool) error {
	// NodeConfig should be loaded by PersistentPreRun, but check anyway
	if common.NodeConfig == nil {
		return fmt.Errorf("no client configuration loaded. Please ensure rnx-config.yml exists")
	}

	nodeNames := common.NodeConfig.ListNodes()
	if len(nodeNames) == 0 {
		return fmt.Errorf("no nodes configured in rnx-config.yml")
	}

	// Sort nodes for consistent output
	sort.Strings(nodeNames)

	if jsonOutput {
		var nodes []NodeInfo

		for _, name := range nodeNames {
			node, err := common.NodeConfig.GetNode(name)
			status := "active"
			if err != nil {
				status = "error"
			}

			nodes = append(nodes, NodeInfo{
				Name:    name,
				Address: node.Address,
				NodeId:  node.NodeId,
				Status:  status,
				Default: name == "default",
			})
		}

		output, err := json.MarshalIndent(nodes, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		fmt.Println(string(output))
		return nil
	}

	// Text output (original format)
	fmt.Printf("Available nodes from configuration:\n\n")

	for _, name := range nodeNames {
		node, err := common.NodeConfig.GetNode(name)
		if err != nil {
			fmt.Printf("Error: %s - %v\n", name, err)
			continue
		}

		// Mark default node
		marker := "  "
		if name == "default" {
			marker = "* "
		}

		fmt.Printf("%s%s\n", marker, name)
		fmt.Printf("   Address: %s\n", node.Address)

		// Display nodeId if available
		nodeId := "-"
		if node.NodeId != "" {
			nodeId = node.NodeId
		}
		fmt.Printf("   Node ID: %s\n", nodeId)

		cert := "-"
		if node.Cert != "" {
			cert = "***"
		}
		fmt.Printf("   Cert:    %s\n", cert)

		key := "-"
		if node.Key != "" {
			key = "***"
		}
		fmt.Printf("   Key:     %s\n", key)

		ca := "-"
		if node.Cert != "" {
			ca = "***"
		}
		fmt.Printf("   CA:      %s\n", ca)

		fmt.Println()
	}

	fmt.Printf("Usage examples:\n")
	fmt.Printf("  rnx job list                    # uses 'default' node\n")
	for _, name := range nodeNames {
		if name != "default" {
			fmt.Printf("  rnx --node=%s job list         # uses '%s' node\n", name, name)
			break
		}
	}

	return nil
}
