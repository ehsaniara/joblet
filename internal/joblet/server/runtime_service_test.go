package server

import (
	"context"
	"testing"

	pb "joblet/api/gen"
	"joblet/internal/joblet/auth/authfakes"
	"joblet/pkg/config"
	"joblet/pkg/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewRuntimeServiceServer(t *testing.T) {
	fakeAuth := &authfakes.FakeGRPCAuthorization{}
	testPlatform := platform.NewPlatform()
	testConfig := &config.Config{}

	server := NewRuntimeServiceServer(fakeAuth, "/opt/joblet/runtimes", testPlatform, testConfig)

	assert.NotNil(t, server)
	assert.Equal(t, fakeAuth, server.auth)
	assert.NotNil(t, server.resolver)
	assert.NotNil(t, server.runtimeInstaller)
	assert.Equal(t, "/opt/joblet/runtimes", server.runtimesPath)
}

func TestRuntimeServiceServer_ListRuntimes_AuthorizationFailed(t *testing.T) {
	fakeAuth := &authfakes.FakeGRPCAuthorization{}
	fakeAuth.AuthorizedReturns(status.Errorf(codes.PermissionDenied, "access denied"))

	testPlatform := platform.NewPlatform()
	testConfig := &config.Config{}

	server := NewRuntimeServiceServer(fakeAuth, "/tmp", testPlatform, testConfig)

	req := &pb.EmptyRequest{}
	resp, err := server.ListRuntimes(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "access denied")
}

func TestRuntimeServiceServer_GetRuntimeInfo_EmptyRuntime(t *testing.T) {
	fakeAuth := &authfakes.FakeGRPCAuthorization{}
	fakeAuth.AuthorizedReturns(nil)

	testPlatform := platform.NewPlatform()
	testConfig := &config.Config{}

	server := NewRuntimeServiceServer(fakeAuth, "/tmp", testPlatform, testConfig)

	req := &pb.RuntimeInfoReq{Runtime: ""}
	resp, err := server.GetRuntimeInfo(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	grpcStatus, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code())
	assert.Contains(t, grpcStatus.Message(), "runtime name is required")
}

func TestRuntimeServiceServer_ValidateRuntimeSpec_EmptySpec(t *testing.T) {
	fakeAuth := &authfakes.FakeGRPCAuthorization{}
	fakeAuth.AuthorizedReturns(nil)

	testPlatform := platform.NewPlatform()
	testConfig := &config.Config{}

	server := NewRuntimeServiceServer(fakeAuth, "/tmp", testPlatform, testConfig)

	req := &pb.ValidateRuntimeSpecRequest{RuntimeSpec: ""}
	resp, err := server.ValidateRuntimeSpec(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)
	assert.Contains(t, resp.Message, "cannot be empty")
}

func TestExtractLanguageFromName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "python", "python"},
		{"with version", "python-3.11", "python"},
		{"with variant", "python-3.11-ml", "python"},
		{"java runtime", "openjdk-21", "openjdk"},
		{"empty string", "", ""},
		{"no hyphen", "golang", "golang"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLanguageFromName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
