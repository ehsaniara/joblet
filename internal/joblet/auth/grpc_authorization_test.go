package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Helper function to create a mock context with peer information
func createMockContext(organizationalUnits []string) context.Context {
	// Create a mock certificate
	cert := &x509.Certificate{
		Subject: pkix.Name{
			OrganizationalUnit: organizationalUnits,
		},
	}

	// Create TLS info
	tlsInfo := credentials.TLSInfo{
		State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		},
	}

	// Create peer with TLS info
	p := &peer.Peer{
		AuthInfo: tlsInfo,
	}

	// Create context with peer
	return peer.NewContext(context.Background(), p)
}

func createMockContextNoPeer() context.Context {
	return context.Background()
}

func createMockContextNoTLS() context.Context {
	p := &peer.Peer{
		AuthInfo: nil, // No TLS info
	}
	return peer.NewContext(context.Background(), p)
}

func createMockContextNoCerts() context.Context {
	tlsInfo := credentials.TLSInfo{
		State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{}, // Empty certificates
		},
	}

	p := &peer.Peer{
		AuthInfo: tlsInfo,
	}
	return peer.NewContext(context.Background(), p)
}

func TestGrpcAuthorization_ExtractClientRole(t *testing.T) {
	auth := NewGRPCAuthorization().(*grpcAuthorization)

	tests := []struct {
		name         string
		context      context.Context
		expectedRole ClientRole
		expectError  bool
	}{
		{
			name:         "Admin role",
			context:      createMockContext([]string{"admin"}),
			expectedRole: AdminRole,
			expectError:  false,
		},
		{
			name:         "Maintainer role",
			context:      createMockContext([]string{"maintainer"}),
			expectedRole: MaintainerRole,
			expectError:  false,
		},
		{
			name:         "Developer role",
			context:      createMockContext([]string{"developer"}),
			expectedRole: DeveloperRole,
			expectError:  false,
		},
		{
			name:         "Reader role",
			context:      createMockContext([]string{"reader"}),
			expectedRole: ReaderRole,
			expectError:  false,
		},
		{
			name:         "Viewer maps to reader (legacy alias)",
			context:      createMockContext([]string{"viewer"}),
			expectedRole: ReaderRole,
			expectError:  false,
		},
		{
			name:         "Admin role (case insensitive)",
			context:      createMockContext([]string{"ADMIN"}),
			expectedRole: AdminRole,
			expectError:  false,
		},
		{
			name:         "Viewer role (case insensitive)",
			context:      createMockContext([]string{"VIEWER"}),
			expectedRole: ReaderRole,
			expectError:  false,
		},
		{
			name:         "Multiple OUs with admin",
			context:      createMockContext([]string{"something", "admin", "other"}),
			expectedRole: AdminRole,
			expectError:  false,
		},
		{
			name:         "Multiple role OUs resolve to least privileged",
			context:      createMockContext([]string{"admin", "reader"}),
			expectedRole: ReaderRole,
			expectError:  false,
		},
		{
			name:         "Maintainer and developer OUs resolve to developer",
			context:      createMockContext([]string{"developer", "maintainer"}),
			expectedRole: DeveloperRole,
			expectError:  false,
		},
		{
			name:         "Unknown role",
			context:      createMockContext([]string{"unknown"}),
			expectedRole: UnknownRole,
			expectError:  false,
		},
		{
			name:         "Empty OUs",
			context:      createMockContext([]string{}),
			expectedRole: UnknownRole,
			expectError:  false,
		},
		{
			name:        "No peer information",
			context:     createMockContextNoPeer(),
			expectError: true,
		},
		{
			name:        "No TLS information",
			context:     createMockContextNoTLS(),
			expectError: true,
		},
		{
			name:        "No client certificates",
			context:     createMockContextNoCerts(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := auth.extractClientRole(tt.context)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if role != tt.expectedRole {
				t.Errorf("Expected role %v, got %v", tt.expectedRole, role)
			}
		})
	}
}

func TestGrpcAuthorization_IsOperationAllowed(t *testing.T) {
	auth := NewGRPCAuthorization().(*grpcAuthorization)

	tests := []struct {
		role      ClientRole
		operation Operation
		allowed   bool
	}{
		// Admin role - should allow all operations, including destructive ones
		{AdminRole, RunJobOp, true},
		{AdminRole, GetJobOp, true},
		{AdminRole, StopJobOp, true},
		{AdminRole, ListJobsOp, true},
		{AdminRole, RemoveRuntimeOp, true},
		{AdminRole, RemoveNetworkOp, true},
		{AdminRole, RemoveVolumeOp, true},

		// Maintainer role - provisioning and jobs, but no removal of shared infrastructure
		{MaintainerRole, RunJobOp, true},
		{MaintainerRole, BuildRuntimeOp, true},
		{MaintainerRole, ValidateRuntimeOp, true},
		{MaintainerRole, CreateNetworkOp, true},
		{MaintainerRole, CreateVolumeOp, true},
		{MaintainerRole, GetJobOp, true},
		{MaintainerRole, RemoveRuntimeOp, false},
		{MaintainerRole, RemoveNetworkOp, false},
		{MaintainerRole, RemoveVolumeOp, false},

		// Developer role - job execution on existing infrastructure only
		{DeveloperRole, RunJobOp, true},
		{DeveloperRole, StopJobOp, true},
		{DeveloperRole, DeleteJobOp, true},
		{DeveloperRole, TestRuntimeOp, true},
		{DeveloperRole, GetJobOp, true},
		{DeveloperRole, ListRuntimesOp, true},
		{DeveloperRole, ListNetworksOp, true},
		{DeveloperRole, ListVolumesOp, true},
		{DeveloperRole, BuildRuntimeOp, false},
		{DeveloperRole, ValidateRuntimeOp, false},
		{DeveloperRole, CreateNetworkOp, false},
		{DeveloperRole, CreateVolumeOp, false},
		{DeveloperRole, RemoveRuntimeOp, false},
		{DeveloperRole, RemoveNetworkOp, false},
		{DeveloperRole, RemoveVolumeOp, false},

		// Reader role - should allow only read operations
		{ReaderRole, RunJobOp, false},
		{ReaderRole, GetJobOp, true},
		{ReaderRole, StopJobOp, false},
		{ReaderRole, DeleteJobOp, false},
		{ReaderRole, ListJobsOp, true},
		{ReaderRole, GetJobLogsOp, true},
		{ReaderRole, GetJobStatusOp, true},
		{ReaderRole, ListRuntimesOp, true},
		{ReaderRole, GetRuntimeInfoOp, true},
		{ReaderRole, ListNetworksOp, true},
		{ReaderRole, ListVolumesOp, true},
		{ReaderRole, GetMetricsOp, true},
		{ReaderRole, QueryLogsOp, true},
		{ReaderRole, QueryMetricsOp, true},
		{ReaderRole, TestRuntimeOp, false},
		{ReaderRole, BuildRuntimeOp, false},
		{ReaderRole, CreateNetworkOp, false},
		{ReaderRole, CreateVolumeOp, false},

		// Unknown role - should not allow any operations
		{UnknownRole, RunJobOp, false},
		{UnknownRole, GetJobOp, false},
		{UnknownRole, StopJobOp, false},
		{UnknownRole, ListJobsOp, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role)+"_"+string(tt.operation), func(t *testing.T) {
			allowed := auth.isOperationAllowed(tt.role, tt.operation)
			if allowed != tt.allowed {
				t.Errorf("Expected %v for role %v and operation %v, got %v",
					tt.allowed, tt.role, tt.operation, allowed)
			}
		})
	}
}

func TestGrpcAuthorization_Authorized(t *testing.T) {
	auth := NewGRPCAuthorization()

	tests := []struct {
		name         string
		context      context.Context
		operation    Operation
		expectError  bool
		expectedCode codes.Code
	}{
		{
			name:        "Admin can run jobs",
			context:     createMockContext([]string{"admin"}),
			operation:   RunJobOp,
			expectError: false,
		},
		{
			name:        "Admin can get jobs",
			context:     createMockContext([]string{"admin"}),
			operation:   GetJobOp,
			expectError: false,
		},
		{
			name:        "Admin can stop jobs",
			context:     createMockContext([]string{"admin"}),
			operation:   StopJobOp,
			expectError: false,
		},
		{
			name:        "Admin can list jobs",
			context:     createMockContext([]string{"admin"}),
			operation:   ListJobsOp,
			expectError: false,
		},
		{
			name:        "Admin can read job logs",
			context:     createMockContext([]string{"admin"}),
			operation:   GetJobLogsOp,
			expectError: false,
		},
		{
			name:        "Viewer can get jobs",
			context:     createMockContext([]string{"viewer"}),
			operation:   GetJobOp,
			expectError: false,
		},
		{
			name:        "Viewer can list jobs",
			context:     createMockContext([]string{"viewer"}),
			operation:   ListJobsOp,
			expectError: false,
		},
		{
			name:        "Viewer can read job logs",
			context:     createMockContext([]string{"viewer"}),
			operation:   GetJobLogsOp,
			expectError: false,
		},
		{
			name:         "Viewer cannot run jobs",
			context:      createMockContext([]string{"viewer"}),
			operation:    RunJobOp,
			expectError:  true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:         "Viewer cannot stop jobs",
			context:      createMockContext([]string{"viewer"}),
			operation:    StopJobOp,
			expectError:  true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:         "Unknown role cannot access anything",
			context:      createMockContext([]string{"unknown"}),
			operation:    GetJobOp,
			expectError:  true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:         "No peer information",
			context:      createMockContextNoPeer(),
			operation:    GetJobOp,
			expectError:  true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "No TLS information",
			context:      createMockContextNoTLS(),
			operation:    GetJobOp,
			expectError:  true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "No client certificates",
			context:      createMockContextNoCerts(),
			operation:    GetJobOp,
			expectError:  true,
			expectedCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.Authorized(tt.context, tt.operation)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				// Check error code
				if st, ok := status.FromError(err); ok {
					if st.Code() != tt.expectedCode {
						t.Errorf("Expected error code %v, got %v", tt.expectedCode, st.Code())
					}
				} else {
					t.Errorf("Expected gRPC status error, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestClientRole_String(t *testing.T) {
	tests := []struct {
		role     ClientRole
		expected string
	}{
		{AdminRole, "admin"},
		{MaintainerRole, "maintainer"},
		{DeveloperRole, "developer"},
		{ReaderRole, "reader"},
		{UnknownRole, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.role) != tt.expected {
			t.Errorf("Expected string representation %v, got %v", tt.expected, string(tt.role))
		}
	}
}

func TestOperation_String(t *testing.T) {
	tests := []struct {
		operation Operation
		expected  string
	}{
		{RunJobOp, "run_job"},
		{GetJobOp, "get_job"},
		{StopJobOp, "stop_job"},
		{ListJobsOp, "list_jobs"},
	}

	for _, tt := range tests {
		if string(tt.operation) != tt.expected {
			t.Errorf("Expected string representation %v, got %v", tt.expected, string(tt.operation))
		}
	}
}

// Benchmark tests
func BenchmarkGrpcAuthorization_ExtractClientRole(b *testing.B) {
	auth := NewGRPCAuthorization().(*grpcAuthorization)
	ctx := createMockContext([]string{"admin"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = auth.extractClientRole(ctx)
	}
}

func BenchmarkGrpcAuthorization_IsOperationAllowed(b *testing.B) {
	auth := NewGRPCAuthorization().(*grpcAuthorization)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.isOperationAllowed(AdminRole, RunJobOp)
	}
}

func BenchmarkGrpcAuthorization_Authorized(b *testing.B) {
	auth := NewGRPCAuthorization()
	ctx := createMockContext([]string{"admin"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = auth.Authorized(ctx, RunJobOp)
	}
}

func TestNewGRPCAuthorization(t *testing.T) {
	auth := NewGRPCAuthorization()
	if auth == nil {
		t.Error("Expected non-nil authorization instance")
	}
}
