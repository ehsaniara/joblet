package client

import (
	"context"
	"fmt"

	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/pkg/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// PersistClient wraps the joblet-persist gRPC service client
type PersistClient struct {
	persistClient persistpb.PersistServiceClient
	conn          *grpc.ClientConn
}

// NewPersistClient creates a new persist client from a node configuration
func NewPersistClient(node *config.Node) (*PersistClient, error) {
	if node == nil {
		return nil, fmt.Errorf("node configuration cannot be nil")
	}

	// Get TLS configuration from embedded certificates
	tlsConfig, err := node.GetClientTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	creds := credentials.NewTLS(tlsConfig)

	// Use PersistAddress if set, otherwise default to same host as Address but port 50052
	address := node.PersistAddress
	if address == "" {
		// Extract host from Address and use port 50052
		// Note: This is a simple implementation that assumes Address format is "host:port"
		// For production, you might want more sophisticated parsing
		address = node.Address[:len(node.Address)-5] + "50052" // Replace last 5 chars (":50051") with "50052"
	}

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to joblet-persist %s: %w", address, err)
	}

	return &PersistClient{
		persistClient: persistpb.NewPersistServiceClient(conn),
		conn:          conn,
	}, nil
}

// Close closes the persist client connection
func (c *PersistClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConn returns the underlying gRPC connection
func (c *PersistClient) GetConn() *grpc.ClientConn {
	return c.conn
}

// QueryLogs queries historical logs for a job from disk storage
func (c *PersistClient) QueryLogs(ctx context.Context, req *persistpb.QueryLogsRequest) (persistpb.PersistService_QueryLogsClient, error) {
	stream, err := c.persistClient.QueryLogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start log query stream: %v", err)
	}
	return stream, nil
}

// QueryMetrics queries historical metrics for a job from disk storage
func (c *PersistClient) QueryMetrics(ctx context.Context, req *persistpb.QueryMetricsRequest) (persistpb.PersistService_QueryMetricsClient, error) {
	stream, err := c.persistClient.QueryMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start metrics query stream: %v", err)
	}
	return stream, nil
}
