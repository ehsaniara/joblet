package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	pb "worker/api/gen"
)

type JobClient struct {
	client pb.JobServiceClient
	conn   *grpc.ClientConn
}

type ClientConfig struct {
	ServerAddr     string
	ClientCertPath string
	ClientKeyPath  string
	CACertPath     string
}

// NewJobClient creates a new job client with the provided configuration
func NewJobClient(clientConfig ClientConfig) (*JobClient, error) {
	// Load client certificate
	clientCert, err := tls.LoadX509KeyPair(clientConfig.ClientCertPath, clientConfig.ClientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key from %s and %s: %w",
			clientConfig.ClientCertPath, clientConfig.ClientKeyPath, err)
	}

	// Load CA certificate
	caCert, err := os.ReadFile(clientConfig.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate from %s: %w", clientConfig.CACertPath, err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
		ServerName:   "worker",
	}

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.NewClient(
		clientConfig.ServerAddr,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server %s: %w", clientConfig.ServerAddr, err)
	}

	return &JobClient{
		client: pb.NewJobServiceClient(conn),
		conn:   conn,
	}, nil
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
