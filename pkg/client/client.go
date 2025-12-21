package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/pkg/config"

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

func (c *JobClient) DeleteAllJobs(ctx context.Context) (*pb.DeleteAllJobsRes, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.jobClient.DeleteAllJobs(ctx, &pb.DeleteAllJobsReq{})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.DeadlineExceeded {
				return nil, fmt.Errorf("timeout while deleting all jobs: server may still be processing the request")
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

// StreamJobMetrics streams live metrics for a running job
func (c *JobClient) StreamJobMetrics(ctx context.Context, id string) (pb.JobService_StreamJobMetricsClient, error) {
	stream, err := c.jobClient.StreamJobMetrics(ctx, &pb.StreamJobMetricsRequest{
		JobUuid: id,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start metrics stream: %v", err)
	}
	return stream, nil
}

// GetJobMetrics retrieves historical metrics for a completed job
func (c *JobClient) GetJobMetrics(ctx context.Context, id string, startTime, endTime int64, limit int32) (pb.JobService_GetJobMetricsClient, error) {
	stream, err := c.jobClient.GetJobMetrics(ctx, &pb.GetJobMetricsRequest{
		JobUuid:   id,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %v", err)
	}
	return stream, nil
}

// StreamJobTelematics streams live eBPF security events for a running job
func (c *JobClient) StreamJobTelematics(ctx context.Context, id string, types []string) (pb.JobService_StreamJobTelematicsClient, error) {
	stream, err := c.jobClient.StreamJobTelematics(ctx, &pb.StreamJobTelematicsRequest{
		JobUuid: id,
		Types:   types,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start telematics stream: %v", err)
	}
	return stream, nil
}

// GetJobTelematics retrieves historical eBPF security events for a completed job
func (c *JobClient) GetJobTelematics(ctx context.Context, id string, types []string, startTime, endTime int64, limit int32) (pb.JobService_GetJobTelematicsClient, error) {
	stream, err := c.jobClient.GetJobTelematics(ctx, &pb.GetJobTelematicsRequest{
		JobUuid:   id,
		Types:     types,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get telematics: %v", err)
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

func (c *JobClient) RemoveRuntime(ctx context.Context, req *pb.RuntimeRemoveReq) (*pb.RuntimeRemoveRes, error) {
	return c.runtimeClient.RemoveRuntime(ctx, req)
}

// BuildRuntime builds a runtime on the remote server from a YAML specification
func (c *JobClient) BuildRuntime(ctx context.Context, req *pb.BuildRuntimeRequest) (pb.RuntimeService_BuildRuntimeClient, error) {
	return c.runtimeClient.BuildRuntime(ctx, req)
}

// ValidateRuntimeYAML validates a runtime YAML specification without building
func (c *JobClient) ValidateRuntimeYAML(ctx context.Context, req *pb.ValidateRuntimeYAMLRequest) (*pb.ValidateRuntimeYAMLResponse, error) {
	return c.runtimeClient.ValidateRuntimeYAML(ctx, req)
}
