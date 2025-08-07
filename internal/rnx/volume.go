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

func newVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage job volumes",
		Long:  "Create, list, and remove persistent volumes for job data sharing",
	}

	cmd.AddCommand(newVolumeCreateCmd())
	cmd.AddCommand(newVolumeListCmd())
	cmd.AddCommand(newVolumeRemoveCmd())

	return cmd
}

func newVolumeCreateCmd() *cobra.Command {
	var size string
	var volumeType string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new volume",
		Long: `Create a new persistent volume for sharing data between jobs.

Volume Types:
  filesystem - Directory-based persistent storage (default)
  memory     - Temporary memory-based storage (tmpfs)

Examples:
  rnx volume create backend --size=1GB
  rnx volume create cache --size=500MB --type=memory
  rnx volume create data --size=2GB --type=filesystem`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolumeCreate(args[0], size, volumeType)
		},
	}

	cmd.Flags().StringVar(&size, "size", "", "Size limit for the volume (e.g., 1GB, 500MB) (required)")
	cmd.Flags().StringVar(&volumeType, "type", "filesystem", "Volume type: filesystem or memory")
	_ = cmd.MarkFlagRequired("size")

	return cmd
}

func newVolumeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all volumes",
		Long:  "Display all available volumes with their size, type, and usage information",
		Args:  cobra.NoArgs,
		RunE:  runVolumeList,
	}

	return cmd
}

func newVolumeRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a volume",
		Long: `Remove a volume. The volume must not be in use by any active jobs.

Examples:
  rnx volume remove backend
  rnx volume remove cache`,
		Args: cobra.ExactArgs(1),
		RunE: runVolumeRemove,
	}

	return cmd
}

func runVolumeCreate(name, size, volumeType string) error {
	// Validate volume type
	if volumeType != "filesystem" && volumeType != "memory" {
		return fmt.Errorf("invalid volume type: %s (must be 'filesystem' or 'memory')", volumeType)
	}

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.CreateVolumeReq{
		Name: name,
		Size: size,
		Type: volumeType,
	}

	resp, err := jobClient.CreateVolume(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create volume: %v", err)
	}

	fmt.Printf("Volume created successfully:\n")
	fmt.Printf("  Name: %s\n", resp.Name)
	fmt.Printf("  Size: %s\n", resp.Size)
	fmt.Printf("  Type: %s\n", resp.Type)
	fmt.Printf("  Path: %s\n", resp.Path)
	fmt.Printf("\nUse this volume with: rnx run --volume=%s <command>\n", name)

	return nil
}

func runVolumeList(cmd *cobra.Command, args []string) error {
	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := jobClient.ListVolumes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list volumes: %v", err)
	}

	if len(resp.Volumes) == 0 {
		fmt.Println("No volumes found")
		return nil
	}

	// Sort volumes by name
	sort.Slice(resp.Volumes, func(i, j int) bool {
		return resp.Volumes[i].Name < resp.Volumes[j].Name
	})

	// Display header
	fmt.Printf("%-15s %-8s %-12s %s\n", "NAME", "SIZE", "TYPE", "CREATED")
	fmt.Printf("%s %s %s %s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 8),
		strings.Repeat("-", 12),
		strings.Repeat("-", 25))

	// Display volumes
	for _, vol := range resp.Volumes {
		// Parse creation time
		createdTime := "unknown"
		if vol.CreatedTime != "" {
			if t, err := time.Parse(time.RFC3339, vol.CreatedTime); err == nil {
				createdTime = t.Format("2006-01-02 15:04:05")
			}
		}

		fmt.Printf("%-15s %-8s %-12s %s\n",
			vol.Name,
			vol.Size,
			vol.Type,
			createdTime)
	}

	return nil
}

func runVolumeRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	jobClient, err := newJobClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RemoveVolumeReq{
		Name: name,
	}

	resp, err := jobClient.RemoveVolume(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to remove volume: %v", err)
	}

	if resp.Success {
		fmt.Printf("Volume '%s' removed successfully\n", name)
	} else {
		fmt.Printf("Failed to remove volume: %s\n", resp.Message)
	}

	return nil
}
