package errors

import "fmt"

// Error types
type ErrorType string

const (
	ErrNotFound    ErrorType = "NOT_FOUND"
	ErrInvalid     ErrorType = "INVALID"
	ErrInternal    ErrorType = "INTERNAL"
	ErrUnavailable ErrorType = "UNAVAILABLE"
)

// PersistError represents a joblet-persist error
type PersistError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *PersistError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *PersistError) Unwrap() error {
	return e.Err
}

// New creates a new error
func New(errType ErrorType, message string) error {
	return &PersistError{
		Type:    errType,
		Message: message,
	}
}

// Wrap wraps an existing error
func Wrap(errType ErrorType, message string, err error) error {
	return &PersistError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// NotFound creates a not found error
func NotFound(message string) error {
	return New(ErrNotFound, message)
}

// Invalid creates an invalid error
func Invalid(message string) error {
	return New(ErrInvalid, message)
}

// Internal creates an internal error
func Internal(message string, err error) error {
	return Wrap(ErrInternal, message, err)
}

// Unavailable creates an unavailable error
func Unavailable(message string) error {
	return New(ErrUnavailable, message)
}
