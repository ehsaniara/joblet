package errors

import "errors"

var (
	ErrJobNotFound     = errors.New("job not found")
	ErrStreamCancelled = errors.New("stream cancelled by client")
)
