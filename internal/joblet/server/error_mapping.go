package server

import (
	"errors"

	pkgerrors "github.com/ehsaniara/joblet/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// statusFromError turns a domain error into a gRPC status, picking the code
// from the wrapped sentinel. If err already has a gRPC status, it passes through.
// Anything not recognised becomes Internal.
func statusFromError(err error, msg string) error {
	if err == nil {
		return nil
	}
	if s, ok := status.FromError(err); ok && s.Code() != codes.Unknown {
		return err
	}
	return status.Errorf(codeForError(err), "%s: %v", msg, err)
}

func codeForError(err error) codes.Code {
	switch {
	case errors.Is(err, pkgerrors.ErrJobNotFound),
		errors.Is(err, pkgerrors.ErrRuntimeNotFound),
		errors.Is(err, pkgerrors.ErrVolumeNotFound):
		return codes.NotFound
	case errors.Is(err, pkgerrors.ErrInvalidJobSpec),
		errors.Is(err, pkgerrors.ErrInvalidResourceSpec):
		return codes.InvalidArgument
	case errors.Is(err, pkgerrors.ErrJobNotRunning),
		errors.Is(err, pkgerrors.ErrJobAlreadyRunning),
		errors.Is(err, pkgerrors.ErrVolumeInUse):
		return codes.FailedPrecondition
	default:
		return codes.Internal
	}
}
