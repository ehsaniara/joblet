package common

import (
	"fmt"

	"github.com/ehsaniara/joblet/pkg/client"
	"github.com/ehsaniara/joblet/pkg/config"
)

var (
	NodeConfig *config.ClientConfig
	ConfigPath string
	NodeName   string
	JSONOutput bool
)

// NewJobClient creates a client based on configuration
func NewJobClient() (*client.JobClient, error) {
	// NodeConfig should be loaded by PersistentPreRun
	if NodeConfig == nil {
		return nil, fmt.Errorf("no configuration loaded - this should not happen")
	}

	// Get the specified node
	node, err := NodeConfig.GetNode(NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node configuration for '%s': %w", NodeName, err)
	}

	// Create client directly from node (no more file path handling needed)
	return client.NewJobClient(node)
}

// NewPersistClient creates a persist client based on configuration
func NewPersistClient() (*client.PersistClient, error) {
	// NodeConfig should be loaded by PersistentPreRun
	if NodeConfig == nil {
		return nil, fmt.Errorf("no configuration loaded - this should not happen")
	}

	// Get the specified node
	node, err := NodeConfig.GetNode(NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node configuration for '%s': %w", NodeName, err)
	}

	// Create persist client from node
	return client.NewPersistClient(node)
}
