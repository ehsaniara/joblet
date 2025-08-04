package buffer

import "errors"

// Common buffer errors that all implementations should use
// for consistent error handling across different buffer types.
var (
	// ErrBufferNotFound indicates that the requested buffer does not exist.
	ErrBufferNotFound = errors.New("buffer not found")

	// ErrBufferExists indicates that the buffer already exists during creation.
	ErrBufferExists = errors.New("buffer already exists")

	// ErrBufferClosed indicates that the buffer has been closed.
	ErrBufferClosed = errors.New("buffer is closed")

	// ErrBufferFull indicates that the buffer has reached its maximum capacity.
	ErrBufferFull = errors.New("buffer is full")

	// ErrInvalidCapacity indicates that the specified capacity is invalid.
	ErrInvalidCapacity = errors.New("invalid buffer capacity")

	// ErrInvalidSize indicates that the specified size is invalid.
	ErrInvalidSize = errors.New("invalid buffer size")

	// ErrReadOnly indicates that the buffer is read-only.
	ErrReadOnly = errors.New("buffer is read-only")

	// ErrWriteOnly indicates that the buffer is write-only.
	ErrWriteOnly = errors.New("buffer is write-only")

	// ErrMaxSubscribersExceeded indicates too many subscribers for a buffer.
	ErrMaxSubscribersExceeded = errors.New("maximum subscribers exceeded for buffer")

	// ErrSubscriberNotFound indicates that the subscriber does not exist.
	ErrSubscriberNotFound = errors.New("subscriber not found")

	// ErrInvalidOffset indicates that the specified offset is invalid.
	ErrInvalidOffset = errors.New("invalid buffer offset")

	// ErrCorruptedData indicates that the buffer data is corrupted.
	ErrCorruptedData = errors.New("buffer data corrupted")

	// ErrPermissionDenied indicates insufficient permissions for the operation.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrDiskFull indicates that the disk is full for persistent buffers.
	ErrDiskFull = errors.New("disk full")

	// ErrFileNotFound indicates that the buffer file does not exist.
	ErrFileNotFound = errors.New("buffer file not found")

	// ErrFileCorrupted indicates that the buffer file is corrupted.
	ErrFileCorrupted = errors.New("buffer file corrupted")

	// ErrCompressionFailed indicates that data compression failed.
	ErrCompressionFailed = errors.New("data compression failed")

	// ErrDecompressionFailed indicates that data decompression failed.
	ErrDecompressionFailed = errors.New("data decompression failed")

	// ErrTimeout indicates that an operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrIOError indicates a general I/O error occurred.
	ErrIOError = errors.New("I/O error")
)

// IsRetryableError checks if an error is transient and the operation can be retried.
func IsRetryableError(err error) bool {
	return errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrIOError) ||
		errors.Is(err, ErrDiskFull) ||
		errors.Is(err, ErrCompressionFailed) ||
		errors.Is(err, ErrDecompressionFailed)
}

// IsNotFoundError checks if an error indicates that a resource was not found.
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrBufferNotFound) ||
		errors.Is(err, ErrSubscriberNotFound) ||
		errors.Is(err, ErrFileNotFound)
}

// IsConflictError checks if an error indicates a resource already exists.
func IsConflictError(err error) bool {
	return errors.Is(err, ErrBufferExists)
}

// IsCapacityError checks if an error indicates capacity limits were exceeded.
func IsCapacityError(err error) bool {
	return errors.Is(err, ErrBufferFull) ||
		errors.Is(err, ErrMaxSubscribersExceeded) ||
		errors.Is(err, ErrDiskFull)
}

// IsValidationError checks if an error indicates invalid input parameters.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidCapacity) ||
		errors.Is(err, ErrInvalidSize) ||
		errors.Is(err, ErrInvalidOffset)
}

// IsPermissionError checks if an error indicates permission issues.
func IsPermissionError(err error) bool {
	return errors.Is(err, ErrPermissionDenied) ||
		errors.Is(err, ErrReadOnly) ||
		errors.Is(err, ErrWriteOnly)
}

// IsCorruptionError checks if an error indicates data corruption.
func IsCorruptionError(err error) bool {
	return errors.Is(err, ErrCorruptedData) ||
		errors.Is(err, ErrFileCorrupted)
}
