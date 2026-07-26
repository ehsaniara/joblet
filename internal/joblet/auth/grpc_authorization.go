package auth

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type ClientRole string

const (
	// AdminRole can perform every operation, including destructive ones
	// (removing runtimes, networks, and volumes).
	AdminRole ClientRole = "admin"
	// MaintainerRole is intended for automation with deterministic outcomes:
	// it can provision infrastructure (build runtimes, create networks and
	// volumes) and run jobs, but cannot remove shared infrastructure.
	MaintainerRole ClientRole = "maintainer"
	// DeveloperRole runs jobs using existing runtimes, networks, and volumes,
	// but cannot create or remove infrastructure.
	DeveloperRole ClientRole = "developer"
	// ReaderRole has read-only access to jobs, logs, and telemetry, for
	// reporting and observability. Certificates with OU "reader" or
	// "viewer" both map to this role.
	ReaderRole  ClientRole = "reader"
	UnknownRole ClientRole = "unknown"
)

type Operation string

const (
	// Job operations
	RunJobOp       Operation = "run_job"
	GetJobOp       Operation = "get_job"
	StopJobOp      Operation = "stop_job"
	DeleteJobOp    Operation = "delete_job"
	ListJobsOp     Operation = "list_jobs"
	GetJobLogsOp   Operation = "get_job_logs"
	GetJobStatusOp Operation = "get_job_status"

	// Network operations
	CreateNetworkOp Operation = "create_network"
	ListNetworksOp  Operation = "list_networks"
	RemoveNetworkOp Operation = "remove_network"

	// Volume operations
	CreateVolumeOp Operation = "create_volume"
	ListVolumesOp  Operation = "list_volumes"
	RemoveVolumeOp Operation = "remove_volume"

	// Runtime operations
	ListRuntimesOp    Operation = "list_runtimes"
	GetRuntimeInfoOp  Operation = "get_runtime_info"
	TestRuntimeOp     Operation = "test_runtime"
	ValidateRuntimeOp Operation = "validate_runtime"
	BuildRuntimeOp    Operation = "build_runtime"
	RemoveRuntimeOp   Operation = "remove_runtime"

	// Monitoring operations (live system metrics)
	GetMetricsOp Operation = "get_metrics"

	// Persist operations (historical data queries)
	QueryLogsOp    Operation = "query_logs"
	QueryMetricsOp Operation = "query_metrics"
)

// readOps are available to every role, including reader.
var readOps = []Operation{
	GetJobOp, ListJobsOp, GetJobLogsOp, GetJobStatusOp,
	ListNetworksOp, ListVolumesOp,
	ListRuntimesOp, GetRuntimeInfoOp,
	GetMetricsOp, QueryLogsOp, QueryMetricsOp,
}

// developerOps adds job execution on top of read access.
var developerOps = append([]Operation{
	RunJobOp, StopJobOp, DeleteJobOp, TestRuntimeOp,
}, readOps...)

// maintainerOps adds infrastructure provisioning on top of job execution.
// Removal of shared infrastructure stays admin-only.
var maintainerOps = append([]Operation{
	BuildRuntimeOp, ValidateRuntimeOp, CreateNetworkOp, CreateVolumeOp,
}, developerOps...)

// AdminRole is not listed: it is allowed everything.
var roleOperations = map[ClientRole]map[Operation]bool{
	ReaderRole:     opSet(readOps),
	DeveloperRole:  opSet(developerOps),
	MaintainerRole: opSet(maintainerOps),
}

func opSet(ops []Operation) map[Operation]bool {
	set := make(map[Operation]bool, len(ops))
	for _, op := range ops {
		set[op] = true
	}
	return set
}

//counterfeiter:generate . GRPCAuthorization
type GRPCAuthorization interface {
	Authorized(ctx context.Context, operation Operation) error
}

type grpcAuthorization struct {
}

func NewGRPCAuthorization() GRPCAuthorization {
	return &grpcAuthorization{}
}

// noOpAuthorization is a no-op authorization that allows all operations
// Used for internal services (like persist) that run on Unix sockets without TLS
type noOpAuthorization struct {
}

// NewNoOpAuthorization creates a no-op authorization that trusts all requests
// This should ONLY be used for internal IPC via Unix domain sockets
func NewNoOpAuthorization() GRPCAuthorization {
	return &noOpAuthorization{}
}

func (n *noOpAuthorization) Authorized(ctx context.Context, operation Operation) error {
	// Always allow - trust is established by Unix socket file permissions
	return nil
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

	// A certificate carrying more than one role OU gets the least privileged
	// of them, so an ambiguous certificate can never escalate.
	role := UnknownRole
	for _, ou := range clientCert.Subject.OrganizationalUnit {
		var candidate ClientRole
		switch strings.ToLower(ou) {
		case "admin":
			candidate = AdminRole
		case "maintainer":
			candidate = MaintainerRole
		case "developer":
			candidate = DeveloperRole
		case "reader", "viewer": // "viewer" is an accepted alias for reader
			candidate = ReaderRole
		default:
			continue
		}
		if role == UnknownRole || rolePrivilege[candidate] < rolePrivilege[role] {
			role = candidate
		}
	}

	return role, nil
}

// rolePrivilege orders roles for least-privilege selection when a certificate
// carries more than one role OU.
var rolePrivilege = map[ClientRole]int{
	ReaderRole:     1,
	DeveloperRole:  2,
	MaintainerRole: 3,
	AdminRole:      4,
}

func (s *grpcAuthorization) isOperationAllowed(role ClientRole, operation Operation) bool {
	if role == AdminRole {
		return true
	}
	return roleOperations[role][operation]
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
