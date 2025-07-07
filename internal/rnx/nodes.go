package rnx

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "List available nodes from configuration",
		Long:  "Display all configured nodes and their connection details from rnx-config-template.yml",
		RunE:  runNodes,
	}

	return cmd
}

func runNodes(cmd *cobra.Command, args []string) error {
	// nodeConfig should be loaded by PersistentPreRun, but check anyway
	if nodeConfig == nil {
		return fmt.Errorf("no client configuration loaded. Please ensure rnx-config-template.yml exists")
	}

	nodes := nodeConfig.ListNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes configured in rnx-config-template.yml")
	}

	// Sort nodes for consistent output
	sort.Strings(nodes)

	fmt.Printf("Available nodes from configuration:\n\n")

	for _, name := range nodes {
		node, err := nodeConfig.GetNode(name)
		if err != nil {
			fmt.Printf("❌ %s: Error - %v\n", name, err)
			continue
		}

		// Mark default node
		marker := "  "
		if name == "default" {
			marker = "* "
		}

		fmt.Printf("%s%s\n", marker, name)
		fmt.Printf("   Address: %s\n", node.Address)

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
	fmt.Printf("  rnx list                    # uses 'default' node\n")
	for _, name := range nodes {
		if name != "default" {
			fmt.Printf("  rnx --node=%s list         # uses '%s' node\n", name, name)
			break
		}
	}

	return nil
}
