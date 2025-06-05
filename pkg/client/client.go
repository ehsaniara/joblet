package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc/credentials"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "job-worker/api/gen"
)

const (
	clientCertPath = "./certs/client-cert.pem"
	clientKeyPath  = "./certs/client-key.pem"

	caCertPath = "./certs/ca-cert.pem"
)

type JobClient struct {
	client pb.JobServiceClient
	conn   *grpc.ClientConn
}

func NewJobClient(serverAddr string) (*JobClient, error) {
	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key: %w", err)
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
		ServerName:   "job-worker",
	}

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.Dial(
		serverAddr,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
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

func (c *JobClient) CreateJob(ctx context.Context, job *pb.CreateJobReq) (*pb.CreateJobRes, error) {
	return c.client.CreateJob(ctx, job)
}

func (c *JobClient) GetJob(ctx context.Context, id string) (*pb.GetJobRes, error) {
	return c.client.GetJob(ctx, &pb.GetJobReq{Id: id})
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

func (c *JobClient) GetJobsStream(ctx context.Context, id string) (pb.JobService_GetJobsStreamClient, error) {
	stream, err := c.client.GetJobsStream(ctx, &pb.GetJobsStreamReq{Id: id})
	if err != nil {
		return nil, fmt.Errorf("failed to start stream: %v", err)
	}
	return stream, nil
}
