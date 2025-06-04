package auth

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"strings"
)

type ClientRole string

const (
	AdminRole   ClientRole = "admin"
	ViewerRole  ClientRole = "viewer"
	UnknownRole ClientRole = "unknown"
)

type Operation string

const (
	CreateJobOp  Operation = "create_job"
	GetJobOp     Operation = "get_job"
	StopJobOp    Operation = "stop_job"
	ListJobsOp   Operation = "list_jobs"
	StreamJobsOp Operation = "stream_jobs"
)

//counterfeiter:generate . GrpcAuthorization
type GrpcAuthorization interface {
	Authorized(ctx context.Context, operation Operation) error
}

type grpcAuthorization struct {
}

func NewGrpcAuthorization() GrpcAuthorization {
	return &grpcAuthorization{}
}

func (s *grpcAuthorization) extractClientRole(ctx context.Context) (ClientRole, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return UnknownRole, fmt.Errorf("no peer information found")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return UnknownRole, fmt.Errorf("no TLS information found")
	}

	if len(tlsInfo.State.PeerCertificates) == 0 {
		return UnknownRole, fmt.Errorf("no client certificate found")
	}

	clientCert := tlsInfo.State.PeerCertificates[0]

	// extract role from Organizational Unit (OU)
	for _, ou := range clientCert.Subject.OrganizationalUnit {
		switch strings.ToLower(ou) {
		case "admin":
			return AdminRole, nil
		case "viewer":
			return ViewerRole, nil
		}
	}

	// default to viewer role for backward compatibility
	return UnknownRole, nil
}

func (s *grpcAuthorization) isOperationAllowed(role ClientRole, operation Operation) bool {
	switch role {
	case AdminRole:
		return true
	case ViewerRole:
		switch operation {
		case GetJobOp, ListJobsOp, StreamJobsOp:
			return true
		case CreateJobOp, StopJobOp:
			return false
		default:
			return false
		}
	default:
		return false
	}
}

func (s *grpcAuthorization) Authorized(ctx context.Context, operation Operation) error {
	role, err := s.extractClientRole(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "failed to extract client role: %v", err)
	}

	if !s.isOperationAllowed(role, operation) {
		return status.Errorf(codes.PermissionDenied, "role %s is not allowed to perform operation %s", role, operation)
	}

	return nil
}
