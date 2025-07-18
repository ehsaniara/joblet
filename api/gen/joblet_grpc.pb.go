// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v5.29.1
// source: joblet.proto

package __

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	JobletService_RunJob_FullMethodName       = "/joblet.JobletService/RunJob"
	JobletService_GetJobStatus_FullMethodName = "/joblet.JobletService/GetJobStatus"
	JobletService_StopJob_FullMethodName      = "/joblet.JobletService/StopJob"
	JobletService_GetJobLogs_FullMethodName   = "/joblet.JobletService/GetJobLogs"
	JobletService_ListJobs_FullMethodName     = "/joblet.JobletService/ListJobs"
)

// JobletServiceClient is the client API for JobletService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type JobletServiceClient interface {
	RunJob(ctx context.Context, in *RunJobReq, opts ...grpc.CallOption) (*RunJobRes, error)
	GetJobStatus(ctx context.Context, in *GetJobStatusReq, opts ...grpc.CallOption) (*GetJobStatusRes, error)
	StopJob(ctx context.Context, in *StopJobReq, opts ...grpc.CallOption) (*StopJobRes, error)
	GetJobLogs(ctx context.Context, in *GetJobLogsReq, opts ...grpc.CallOption) (JobletService_GetJobLogsClient, error)
	ListJobs(ctx context.Context, in *EmptyRequest, opts ...grpc.CallOption) (*Jobs, error)
}

type jobletServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewJobletServiceClient(cc grpc.ClientConnInterface) JobletServiceClient {
	return &jobletServiceClient{cc}
}

func (c *jobletServiceClient) RunJob(ctx context.Context, in *RunJobReq, opts ...grpc.CallOption) (*RunJobRes, error) {
	out := new(RunJobRes)
	err := c.cc.Invoke(ctx, JobletService_RunJob_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobletServiceClient) GetJobStatus(ctx context.Context, in *GetJobStatusReq, opts ...grpc.CallOption) (*GetJobStatusRes, error) {
	out := new(GetJobStatusRes)
	err := c.cc.Invoke(ctx, JobletService_GetJobStatus_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobletServiceClient) StopJob(ctx context.Context, in *StopJobReq, opts ...grpc.CallOption) (*StopJobRes, error) {
	out := new(StopJobRes)
	err := c.cc.Invoke(ctx, JobletService_StopJob_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobletServiceClient) GetJobLogs(ctx context.Context, in *GetJobLogsReq, opts ...grpc.CallOption) (JobletService_GetJobLogsClient, error) {
	stream, err := c.cc.NewStream(ctx, &JobletService_ServiceDesc.Streams[0], JobletService_GetJobLogs_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &jobletServiceGetJobLogsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type JobletService_GetJobLogsClient interface {
	Recv() (*DataChunk, error)
	grpc.ClientStream
}

type jobletServiceGetJobLogsClient struct {
	grpc.ClientStream
}

func (x *jobletServiceGetJobLogsClient) Recv() (*DataChunk, error) {
	m := new(DataChunk)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *jobletServiceClient) ListJobs(ctx context.Context, in *EmptyRequest, opts ...grpc.CallOption) (*Jobs, error) {
	out := new(Jobs)
	err := c.cc.Invoke(ctx, JobletService_ListJobs_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// JobletServiceServer is the server API for JobletService service.
// All implementations must embed UnimplementedJobletServiceServer
// for forward compatibility
type JobletServiceServer interface {
	RunJob(context.Context, *RunJobReq) (*RunJobRes, error)
	GetJobStatus(context.Context, *GetJobStatusReq) (*GetJobStatusRes, error)
	StopJob(context.Context, *StopJobReq) (*StopJobRes, error)
	GetJobLogs(*GetJobLogsReq, JobletService_GetJobLogsServer) error
	ListJobs(context.Context, *EmptyRequest) (*Jobs, error)
	mustEmbedUnimplementedJobletServiceServer()
}

// UnimplementedJobletServiceServer must be embedded to have forward compatible implementations.
type UnimplementedJobletServiceServer struct {
}

func (UnimplementedJobletServiceServer) RunJob(context.Context, *RunJobReq) (*RunJobRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunJob not implemented")
}
func (UnimplementedJobletServiceServer) GetJobStatus(context.Context, *GetJobStatusReq) (*GetJobStatusRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJobStatus not implemented")
}
func (UnimplementedJobletServiceServer) StopJob(context.Context, *StopJobReq) (*StopJobRes, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopJob not implemented")
}
func (UnimplementedJobletServiceServer) GetJobLogs(*GetJobLogsReq, JobletService_GetJobLogsServer) error {
	return status.Errorf(codes.Unimplemented, "method GetJobLogs not implemented")
}
func (UnimplementedJobletServiceServer) ListJobs(context.Context, *EmptyRequest) (*Jobs, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListJobs not implemented")
}
func (UnimplementedJobletServiceServer) mustEmbedUnimplementedJobletServiceServer() {}

// UnsafeJobletServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to JobletServiceServer will
// result in compilation errors.
type UnsafeJobletServiceServer interface {
	mustEmbedUnimplementedJobletServiceServer()
}

func RegisterJobletServiceServer(s grpc.ServiceRegistrar, srv JobletServiceServer) {
	s.RegisterService(&JobletService_ServiceDesc, srv)
}

func _JobletService_RunJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunJobReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobletServiceServer).RunJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: JobletService_RunJob_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobletServiceServer).RunJob(ctx, req.(*RunJobReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobletService_GetJobStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetJobStatusReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobletServiceServer).GetJobStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: JobletService_GetJobStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobletServiceServer).GetJobStatus(ctx, req.(*GetJobStatusReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobletService_StopJob_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopJobReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobletServiceServer).StopJob(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: JobletService_StopJob_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobletServiceServer).StopJob(ctx, req.(*StopJobReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobletService_GetJobLogs_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetJobLogsReq)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(JobletServiceServer).GetJobLogs(m, &jobletServiceGetJobLogsServer{stream})
}

type JobletService_GetJobLogsServer interface {
	Send(*DataChunk) error
	grpc.ServerStream
}

type jobletServiceGetJobLogsServer struct {
	grpc.ServerStream
}

func (x *jobletServiceGetJobLogsServer) Send(m *DataChunk) error {
	return x.ServerStream.SendMsg(m)
}

func _JobletService_ListJobs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EmptyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobletServiceServer).ListJobs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: JobletService_ListJobs_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobletServiceServer).ListJobs(ctx, req.(*EmptyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// JobletService_ServiceDesc is the grpc.ServiceDesc for JobletService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var JobletService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "joblet.JobletService",
	HandlerType: (*JobletServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RunJob",
			Handler:    _JobletService_RunJob_Handler,
		},
		{
			MethodName: "GetJobStatus",
			Handler:    _JobletService_GetJobStatus_Handler,
		},
		{
			MethodName: "StopJob",
			Handler:    _JobletService_StopJob_Handler,
		},
		{
			MethodName: "ListJobs",
			Handler:    _JobletService_ListJobs_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetJobLogs",
			Handler:       _JobletService_GetJobLogs_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "joblet.proto",
}
