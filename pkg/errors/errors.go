// Package errors provides standardized error handling for the Joblet system.
// It implements structured error types with proper wrapping and classification
// following Go 1.20+ error handling best practices.
package errors

import (
	"context"
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions
var (
	// Job-related errors
	ErrJobNotFound       = errors.New("job not found")
	ErrJobAlreadyExists  = errors.New("job already exists")
	ErrJobNotRunning     = errors.New("job is not running")
	ErrJobAlreadyRunning = errors.New("job is already running")
	ErrInvalidJobSpec    = errors.New("invalid job specification")
	ErrJobTimeout        = errors.New("job execution timeout")

	// Resource-related errors
	ErrResourceExhausted    = errors.New("resource exhausted")
	ErrInvalidResourceSpec  = errors.New("invalid resource specification")
	ErrResourceNotAvailable = errors.New("resource not available")

	// Runtime-related errors
	ErrInvalidRuntime  = errors.New("invalid runtime specification")
	ErrRuntimeNotFound = errors.New("runtime not found")
	ErrRuntimeFailed   = errors.New("runtime operation failed")

	// Network-related errors
	ErrNetworkNotFound = errors.New("network not found")
	ErrNetworkConflict = errors.New("network conflict")
	ErrNetworkFailed   = errors.New("network operation failed")

	// Volume-related errors
	ErrVolumeNotFound = errors.New("volume not found")
	ErrVolumeInUse    = errors.New("volume is in use")
	ErrVolumeFailed   = errors.New("volume operation failed")

	// System-related errors
	ErrPermissionDenied    = errors.New("permission denied")
	ErrTimeout             = errors.New("operation timed out")
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrFilesystemFailed    = errors.New("filesystem operation failed")
)

// JobError represents an error related to a specific job
type JobError struct {
	JobUUID   string
	Operation string
	Err       error
}

func (e *JobError) Error() string {
	return fmt.Sprintf("job %s: operation %s: %v", e.JobUUID, e.Operation, e.Err)
}

func (e *JobError) Unwrap() error {
	return e.Err
}

// RuntimeError represents an error related to a runtime operation
type RuntimeError struct {
	Runtime   string
	Operation string
	Err       error
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("runtime %s: operation %s: %v", e.Runtime, e.Operation, e.Err)
}

func (e *RuntimeError) Unwrap() error {
	return e.Err
}

// NetworkError represents an error related to network operations
type NetworkError struct {
	Network   string
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network %s: operation %s: %v", e.Network, e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// VolumeError represents an error related to volume operations
type VolumeError struct {
	Volume    string
	Operation string
	Err       error
}

func (e *VolumeError) Error() string {
	return fmt.Sprintf("volume %s: operation %s: %v", e.Volume, e.Operation, e.Err)
}

func (e *VolumeError) Unwrap() error {
	return e.Err
}

// FilesystemError represents an error related to filesystem operations
type FilesystemError struct {
	Path      string
	Operation string
	Err       error
}

func (e *FilesystemError) Error() string {
	return fmt.Sprintf("filesystem %s: operation %s: %v", e.Path, e.Operation, e.Err)
}

func (e *FilesystemError) Unwrap() error {
	return e.Err
}

// ConfigError represents an error related to configuration
type ConfigError struct {
	Component string
	Field     string
	Err       error
}

func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config %s.%s: %v", e.Component, e.Field, e.Err)
	}
	return fmt.Sprintf("config %s: %v", e.Component, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// Error wrapping constructors
func WrapJobError(jobID, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &JobError{JobUUID: jobID, Operation: operation, Err: err}
}

func WrapRuntimeError(runtime, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &RuntimeError{Runtime: runtime, Operation: operation, Err: err}
}

func WrapNetworkError(network, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &NetworkError{Network: network, Operation: operation, Err: err}
}

func WrapVolumeError(volume, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &VolumeError{Volume: volume, Operation: operation, Err: err}
}

func WrapFilesystemError(path, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &FilesystemError{Path: path, Operation: operation, Err: err}
}

func WrapConfigError(component, field string, err error) error {
	if err == nil {
		return nil
	}
	return &ConfigError{Component: component, Field: field, Err: err}
}

// Error classification functions
func IsJobError(err error) bool {
	var je *JobError
	return errors.As(err, &je)
}

func IsRuntimeError(err error) bool {
	var re *RuntimeError
	return errors.As(err, &re)
}

func IsNetworkError(err error) bool {
	var ne *NetworkError
	return errors.As(err, &ne)
}

func IsVolumeError(err error) bool {
	var ve *VolumeError
	return errors.As(err, &ve)
}

func IsFilesystemError(err error) bool {
	var fe *FilesystemError
	return errors.As(err, &fe)
}

func IsConfigError(err error) bool {
	var ce *ConfigError
	return errors.As(err, &ce)
}

// Specific error type checks
func IsResourceError(err error) bool {
	return errors.Is(err, ErrResourceExhausted) ||
		errors.Is(err, ErrInvalidResourceSpec) ||
		errors.Is(err, ErrResourceNotAvailable)
}

func IsTimeoutError(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrJobTimeout)
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrJobNotFound) ||
		errors.Is(err, ErrRuntimeNotFound) ||
		errors.Is(err, ErrNetworkNotFound) ||
		errors.Is(err, ErrVolumeNotFound)
}

func IsPermissionError(err error) bool {
	return errors.Is(err, ErrPermissionDenied)
}

// Error extraction helpers
func GetJobUUID(err error) (string, bool) {
	var je *JobError
	if errors.As(err, &je) {
		return je.JobUUID, true
	}
	return "", false
}

func GetRuntime(err error) (string, bool) {
	var re *RuntimeError
	if errors.As(err, &re) {
		return re.Runtime, true
	}
	return "", false
}

func GetNetwork(err error) (string, bool) {
	var ne *NetworkError
	if errors.As(err, &ne) {
		return ne.Network, true
	}
	return "", false
}

// Convenience functions for common error patterns
func NewJobNotFoundError(jobID string) error {
	return WrapJobError(jobID, "lookup", ErrJobNotFound)
}

func NewRuntimeNotFoundError(runtime string) error {
	return WrapRuntimeError(runtime, "lookup", ErrRuntimeNotFound)
}

func NewNetworkNotFoundError(network string) error {
	return WrapNetworkError(network, "lookup", ErrNetworkNotFound)
}

func NewVolumeNotFoundError(volume string) error {
	return WrapVolumeError(volume, "lookup", ErrVolumeNotFound)
}

func NewFilesystemError(path, operation string, err error) error {
	return WrapFilesystemError(path, operation, fmt.Errorf("%w: %v", ErrFilesystemFailed, err))
}

func NewConfigError(component, field string, err error) error {
	return WrapConfigError(component, field, fmt.Errorf("%w: %v", ErrInvalidConfig, err))
}

// Context-aware error handling
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// JoinErrors combines multiple errors into a single error
// Similar to errors.Join in Go 1.20+
func JoinErrors(errs ...error) error {
	var validErrs []error
	for _, err := range errs {
		if err != nil {
			validErrs = append(validErrs, err)
		}
	}

	if len(validErrs) == 0 {
		return nil
	}
	if len(validErrs) == 1 {
		return validErrs[0]
	}

	// Create a multi-error type
	return &multiError{errors: validErrs}
}

// multiError represents multiple errors
type multiError struct {
	errors []error
}

func (e *multiError) Error() string {
	if len(e.errors) == 0 {
		return ""
	}
	if len(e.errors) == 1 {
		return e.errors[0].Error()
	}

	msg := e.errors[0].Error()
	for _, err := range e.errors[1:] {
		msg += "; " + err.Error()
	}
	return msg
}

func (e *multiError) Unwrap() []error {
	return e.errors
}

// Is implements error comparison for multiError
func (e *multiError) Is(target error) bool {
	for _, err := range e.errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As implements error conversion for multiError
func (e *multiError) As(target interface{}) bool {
	for _, err := range e.errors {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}
