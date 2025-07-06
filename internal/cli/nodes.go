package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "List available nodes from configuration",
		Long:  "Display all configured nodes and their connection details from client-config-template.yml",
		RunE:  runNodes,
	}

	return cmd
}

func runNodes(cmd *cobra.Command, args []string) error {
	// nodeConfig should be loaded by PersistentPreRun, but check anyway
	if nodeConfig == nil {
		return fmt.Errorf("no client configuration loaded. Please ensure client-config-template.yml exists")
	}

	nodes := nodeConfig.ListNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes configured in client-config-template.yml")
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
		fmt.Printf("   Cert:    %s\n", node.Cert)
		fmt.Printf("   Key:     %s\n", node.Key)
		fmt.Printf("   CA:      %s\n", node.CA)
		fmt.Println()
	}

	fmt.Printf("Usage examples:\n")
	fmt.Printf("  worker-cli list                    # uses 'default' node\n")
	for _, name := range nodes {
		if name != "default" {
			fmt.Printf("  worker-cli --node=%s list         # uses '%s' node\n", name, name)
			break
		}
	}

	return nil
}
