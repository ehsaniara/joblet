package pubsub

import "errors"

// Pub-sub errors for in-memory implementation.
var (
	// ErrPublisherClosed indicates that the publisher has been closed.
	ErrPublisherClosed = errors.New("publisher is closed")

	// ErrSubscriberClosed indicates that the subscriber has been closed.
	ErrSubscriberClosed = errors.New("subscriber is closed")
)
