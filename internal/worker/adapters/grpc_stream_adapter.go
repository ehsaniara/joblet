package adapters

import (
	"context"
	pb "worker/api/gen"
	"worker/internal/worker/store"
)

// GrpcStreamAdapter adapts gRPC stream to domain interface
type GrpcStreamAdapter struct {
	stream pb.JobService_GetJobLogsServer
}

func NewGrpcStreamAdapter(stream pb.JobService_GetJobLogsServer) store.DomainStreamer {
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
