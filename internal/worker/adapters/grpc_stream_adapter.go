package adapters

import (
	"context"
	pb "job-worker/api/gen"
	"job-worker/internal/worker/interfaces"
)

// GrpcStreamAdapter adapts gRPC stream to domain interface
type GrpcStreamAdapter struct {
	stream pb.JobService_GetJobsStreamServer
}

func NewGrpcStreamAdapter(stream pb.JobService_GetJobsStreamServer) interfaces.DomainStreamer {
	return &GrpcStreamAdapter{stream: stream}
}

func (a *GrpcStreamAdapter) SendData(data []byte) error {
	return a.stream.Send(&pb.DataChunk{Payload: data})
}

func (a *GrpcStreamAdapter) SendKeepalive() error {
	return a.stream.Send(&pb.DataChunk{Payload: []byte{}})
}

func (a *GrpcStreamAdapter) Context() context.Context {
	return a.stream.Context()
}
