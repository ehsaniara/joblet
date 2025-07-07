package adapters

import (
	"context"
	pb "joblet/api/gen"
	"joblet/internal/joblet/state"
)

// GrpcStreamAdapter adapts gRPC stream to domain interface
type GrpcStreamAdapter struct {
	stream pb.JobletService_GetJobLogsServer
}

func NewGrpcStreamAdapter(stream pb.JobletService_GetJobLogsServer) state.DomainStreamer {
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
