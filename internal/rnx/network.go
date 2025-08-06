package rnx

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "joblet/api/gen"

	"github.com/spf13/cobra"
)

func newNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage job networks",
		Long:  "Create, list, and remove custom networks for job isolation",
	}

	cmd.AddCommand(newNetworkCreateCmd())
	cmd.AddCommand(newNetworkListCmd())
	cmd.AddCommand(newNetworkRemoveCmd())

	return cmd
}

func newNetworkCreateCmd() *cobra.Command {
	var cidr string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new network",
		Long: `Create a new custom network with specified CIDR range.

Examples:
  rnx network create backend --cidr=10.1.0.0/24
  rnx network create frontend --cidr=10.2.0.0/24
  rnx network create dev --cidr=192.168.100.0/24`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNetworkCreate(args[0], cidr)
		},
	}

	cmd.Flags().StringVar(&cidr, "cidr", "", "CIDR range for the network (required)")
	_ = cmd.MarkFlagRequired("cidr")

	return cmd
}

func newNetworkListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all networks",
		Long:  "Display all available networks including built-in and custom networks",
		Args:  cobra.NoArgs,
		RunE:  runNetworkList,
	}

	return cmd
}

func newNetworkRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a network",
		Long: `Remove a custom network. The network must not have any active jobs.

Examples:
  rnx network remove backend
  rnx network remove dev`,
		Args: cobra.ExactArgs(1),
		RunE: runNetworkRemove,
	}

	return cmd
}

func runNetworkCreate(name, cidr string) error {
	// Validate network name
	if name == "none" || name == "isolated" || name == "bridge" {
		return fmt.Errorf("cannot use reserved network name: %s", name)
	}

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.CreateNetworkReq{
		Name: name,
		Cidr: cidr,
	}

	resp, err := jobClient.CreateNetwork(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	fmt.Printf("Network created successfully:\n")
	fmt.Printf("  Name:   %s\n", resp.Name)
	fmt.Printf("  CIDR:   %s\n", resp.Cidr)
	fmt.Printf("  Bridge: %s\n", resp.Bridge)
	fmt.Printf("\nUse this network with: rnx run --network=%s <command>\n", name)

	return nil
}

func runNetworkList(cmd *cobra.Command, args []string) error {
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := jobClient.ListNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list networks: %v", err)
	}

	if len(resp.Networks) == 0 {
		fmt.Println("No networks found")
		return nil
	}

	// Sort networks by name
	sort.Slice(resp.Networks, func(i, j int) bool {
		// Built-in networks first
		iBuiltin := isBuiltinNetwork(resp.Networks[i].Name)
		jBuiltin := isBuiltinNetwork(resp.Networks[j].Name)
		if iBuiltin != jBuiltin {
			return iBuiltin
		}
		return resp.Networks[i].Name < resp.Networks[j].Name
	})

	// Display header
	fmt.Printf("%-15s %-18s %s\n", "NAME", "CIDR", "BRIDGE")
	fmt.Printf("%s %s %s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 18),
		strings.Repeat("-", 15))

	// Display networks
	for _, net := range resp.Networks {
		typeIndicator := ""
		if isBuiltinNetwork(net.Name) {
			typeIndicator = " (built-in)"
		}

		fmt.Printf("%-15s %-18s %s%s\n",
			net.Name,
			net.Cidr,
			net.Bridge,
			typeIndicator)
	}

	return nil
}

func runNetworkRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate not a built-in network
	if isBuiltinNetwork(name) {
		return fmt.Errorf("cannot remove built-in network: %s", name)
	}

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.RemoveNetworkReq{
		Name: name,
	}

	resp, err := jobClient.RemoveNetwork(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to remove network: %v", err)
	}

	if resp.Success {
		fmt.Printf("Network '%s' removed successfully\n", name)
	} else {
		fmt.Printf("Failed to remove network: %s\n", resp.Message)
	}

	return nil
}

func isBuiltinNetwork(name string) bool {
	return name == "none" || name == "isolated" || name == "bridge"
}
