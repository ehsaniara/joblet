package platform

import "fmt"

// PlatformError represents a platform-specific error
type PlatformError struct {
	Platform  string
	Operation string
	Err       error
}

func (e *PlatformError) Error() string {
	return fmt.Sprintf("platform %s: operation %s failed: %v", e.Platform, e.Operation, e.Err)
}

func (e *PlatformError) Unwrap() error {
	return e.Err
}

// NewPlatformError creates a new platform error
func NewPlatformError(platform, operation string, err error) error {
	return &PlatformError{
		Platform:  platform,
		Operation: operation,
		Err:       err,
	}
}

// UnsupportedOperationError represents an unsupported operation error
type UnsupportedOperationError struct {
	Platform  string
	Operation string
}

func (e *UnsupportedOperationError) Error() string {
	return fmt.Sprintf("operation %s not supported on platform %s", e.Operation, e.Platform)
}

// NewUnsupportedOperationError creates a new unsupported operation error
func NewUnsupportedOperationError(platform, operation string) error {
	return &UnsupportedOperationError{
		Platform:  platform,
		Operation: operation,
	}
}
