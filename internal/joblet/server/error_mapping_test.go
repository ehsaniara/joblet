package server

import (
	"errors"
	"fmt"
	"testing"

	pkgerrors "github.com/ehsaniara/joblet/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStatusFromError_NilReturnsNil(t *testing.T) {
	assert.Nil(t, statusFromError(nil, "anything"))
}

func TestStatusFromError_PassesThroughExistingStatus(t *testing.T) {
	original := status.Error(codes.PermissionDenied, "denied")
	assert.Equal(t, original, statusFromError(original, "ignored"))
}

func TestStatusFromError_MapsSentinelsToCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"job not found", pkgerrors.ErrJobNotFound, codes.NotFound},
		{"runtime not found", pkgerrors.ErrRuntimeNotFound, codes.NotFound},
		{"volume not found", pkgerrors.ErrVolumeNotFound, codes.NotFound},
		{"invalid job spec", pkgerrors.ErrInvalidJobSpec, codes.InvalidArgument},
		{"invalid resource spec", pkgerrors.ErrInvalidResourceSpec, codes.InvalidArgument},
		{"job not running", pkgerrors.ErrJobNotRunning, codes.FailedPrecondition},
		{"job already running", pkgerrors.ErrJobAlreadyRunning, codes.FailedPrecondition},
		{"volume in use", pkgerrors.ErrVolumeInUse, codes.FailedPrecondition},
		{"unknown error", errors.New("anything else"), codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := status.FromError(statusFromError(tc.err, "op"))
			assert.True(t, ok)
			assert.Equal(t, tc.want, s.Code())
		})
	}
}

func TestStatusFromError_UnwrapsSentinelThroughChain(t *testing.T) {
	wrapped := fmt.Errorf("upstream wrap: %w",
		fmt.Errorf("%w: job ABCD", pkgerrors.ErrJobNotFound))
	s, ok := status.FromError(statusFromError(wrapped, "lookup"))
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, s.Code())
}
