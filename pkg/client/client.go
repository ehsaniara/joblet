package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	pb "worker/api/gen"
	"worker/pkg/config"
)

type JobClient struct {
	client pb.JobServiceClient
	conn   *grpc.ClientConn
}

// NewJobClient creates a new job client from a node configuration
func NewJobClient(node *config.Node) (*JobClient, error) {
	if node == nil {
		return nil, fmt.Errorf("node configuration cannot be nil")
	}

	// Get TLS configuration from embedded certificates
	tlsConfig, err := node.GetClientTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.NewClient(
		node.Address,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server %s: %w", node.Address, err)
	}

	return &JobClient{
		client: pb.NewJobServiceClient(conn),
		conn:   conn,
	}, nil
}

// NewJobClientFromConfig creates a client from client config and node name
func NewJobClientFromConfig(clientConfig *config.ClientConfig, nodeName string) (*JobClient, error) {
	if clientConfig == nil {
		return nil, fmt.Errorf("client configuration cannot be nil")
	}

	node, err := clientConfig.GetNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node configuration: %w", err)
	}

	return NewJobClient(node)
}

func (c *JobClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *JobClient) RunJob(ctx context.Context, job *pb.RunJobReq) (*pb.RunJobRes, error) {
	return c.client.RunJob(ctx, job)
}

func (c *JobClient) GetJobStatus(ctx context.Context, id string) (*pb.GetJobStatusRes, error) {
	return c.client.GetJobStatus(ctx, &pb.GetJobStatusReq{Id: id})
}

func (c *JobClient) StopJob(ctx context.Context, id string) (*pb.StopJobRes, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.StopJob(ctx, &pb.StopJobReq{Id: id})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.DeadlineExceeded {
				return nil, fmt.Errorf("timeout while stopping job %s: server may still be processing the request", id)
			}
		}
		return nil, err
	}
	return resp, nil
}

func (c *JobClient) ListJobs(ctx context.Context) (*pb.Jobs, error) {
	return c.client.ListJobs(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) GetJobLogs(ctx context.Context, id string) (pb.JobService_GetJobLogsClient, error) {
	stream, err := c.client.GetJobLogs(ctx, &pb.GetJobLogsReq{Id: id})
	if err != nil {
		return nil, fmt.Errorf("failed to start log stream: %v", err)
	}
	return stream, nil
}
