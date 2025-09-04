package client

import (
	"context"
	"fmt"
	"time"

	pb "joblet/api/gen"
	"joblet/pkg/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type JobClient struct {
	jobClient        pb.JobServiceClient
	networkClient    pb.NetworkServiceClient
	volumeClient     pb.VolumeServiceClient
	monitoringClient pb.MonitoringServiceClient
	runtimeClient    pb.RuntimeServiceClient
	conn             *grpc.ClientConn
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
		jobClient:        pb.NewJobServiceClient(conn),
		networkClient:    pb.NewNetworkServiceClient(conn),
		volumeClient:     pb.NewVolumeServiceClient(conn),
		monitoringClient: pb.NewMonitoringServiceClient(conn),
		runtimeClient:    pb.NewRuntimeServiceClient(conn),
		conn:             conn,
	}, nil
}

func (c *JobClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *JobClient) GetConn() *grpc.ClientConn {
	return c.conn
}

func (c *JobClient) RunJob(ctx context.Context, job *pb.RunJobRequest) (*pb.RunJobResponse, error) {
	return c.jobClient.RunJob(ctx, job)
}

func (c *JobClient) GetJobStatus(ctx context.Context, id string) (*pb.GetJobStatusRes, error) {
	return c.jobClient.GetJobStatus(ctx, &pb.GetJobStatusReq{Uuid: id})
}

func (c *JobClient) StopJob(ctx context.Context, id string) (*pb.StopJobRes, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.jobClient.StopJob(ctx, &pb.StopJobReq{Uuid: id})
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

func (c *JobClient) DeleteJob(ctx context.Context, id string) (*pb.DeleteJobRes, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.jobClient.DeleteJob(ctx, &pb.DeleteJobReq{Uuid: id})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.DeadlineExceeded {
				return nil, fmt.Errorf("timeout while deleting job %s: server may still be processing the request", id)
			}
		}
		return nil, err
	}
	return resp, nil
}

func (c *JobClient) ListJobs(ctx context.Context) (*pb.Jobs, error) {
	return c.jobClient.ListJobs(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) GetJobLogs(ctx context.Context, id string) (pb.JobService_GetJobLogsClient, error) {
	stream, err := c.jobClient.GetJobLogs(ctx, &pb.GetJobLogsReq{Uuid: id})
	if err != nil {
		return nil, fmt.Errorf("failed to start log stream: %v", err)
	}
	return stream, nil
}

func (c *JobClient) CreateNetwork(ctx context.Context, req *pb.CreateNetworkReq) (*pb.CreateNetworkRes, error) {
	return c.networkClient.CreateNetwork(ctx, req)
}

func (c *JobClient) ListNetworks(ctx context.Context) (*pb.Networks, error) {
	return c.networkClient.ListNetworks(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) RemoveNetwork(ctx context.Context, req *pb.RemoveNetworkReq) (*pb.RemoveNetworkRes, error) {
	return c.networkClient.RemoveNetwork(ctx, req)
}

func (c *JobClient) CreateVolume(ctx context.Context, req *pb.CreateVolumeReq) (*pb.CreateVolumeRes, error) {
	return c.volumeClient.CreateVolume(ctx, req)
}

func (c *JobClient) ListVolumes(ctx context.Context) (*pb.Volumes, error) {
	return c.volumeClient.ListVolumes(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) RemoveVolume(ctx context.Context, req *pb.RemoveVolumeReq) (*pb.RemoveVolumeRes, error) {
	return c.volumeClient.RemoveVolume(ctx, req)
}

// Monitoring service methods

func (c *JobClient) GetSystemStatus(ctx context.Context) (*pb.SystemStatusRes, error) {
	return c.monitoringClient.GetSystemStatus(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) StreamSystemMetrics(ctx context.Context, req *pb.StreamMetricsReq) (pb.MonitoringService_StreamSystemMetricsClient, error) {
	return c.monitoringClient.StreamSystemMetrics(ctx, req)
}

// Runtime service methods

func (c *JobClient) ListRuntimes(ctx context.Context) (*pb.RuntimesRes, error) {
	return c.runtimeClient.ListRuntimes(ctx, &pb.EmptyRequest{})
}

func (c *JobClient) GetRuntimeInfo(ctx context.Context, req *pb.RuntimeInfoReq) (*pb.RuntimeInfoRes, error) {
	return c.runtimeClient.GetRuntimeInfo(ctx, req)
}

func (c *JobClient) TestRuntime(ctx context.Context, req *pb.RuntimeTestReq) (*pb.RuntimeTestRes, error) {
	return c.runtimeClient.TestRuntime(ctx, req)
}

func (c *JobClient) InstallRuntimeFromGithub(ctx context.Context, req *pb.InstallRuntimeRequest) (*pb.InstallRuntimeResponse, error) {
	return c.runtimeClient.InstallRuntimeFromGithub(ctx, req)
}

func (c *JobClient) InstallRuntimeFromLocal(ctx context.Context, req *pb.InstallRuntimeFromLocalRequest) (*pb.InstallRuntimeResponse, error) {
	return c.runtimeClient.InstallRuntimeFromLocal(ctx, req)
}

func (c *JobClient) ValidateRuntimeSpec(ctx context.Context, req *pb.ValidateRuntimeSpecRequest) (*pb.ValidateRuntimeSpecResponse, error) {
	return c.runtimeClient.ValidateRuntimeSpec(ctx, req)
}

func (c *JobClient) StreamingInstallRuntimeFromGithub(ctx context.Context, req *pb.InstallRuntimeRequest) (pb.RuntimeService_StreamingInstallRuntimeFromGithubClient, error) {
	return c.runtimeClient.StreamingInstallRuntimeFromGithub(ctx, req)
}

func (c *JobClient) StreamingInstallRuntimeFromLocal(ctx context.Context, req *pb.InstallRuntimeFromLocalRequest) (pb.RuntimeService_StreamingInstallRuntimeFromLocalClient, error) {
	return c.runtimeClient.StreamingInstallRuntimeFromLocal(ctx, req)
}

func (c *JobClient) RemoveRuntime(ctx context.Context, req *pb.RuntimeRemoveReq) (*pb.RuntimeRemoveRes, error) {
	return c.runtimeClient.RemoveRuntime(ctx, req)
}
