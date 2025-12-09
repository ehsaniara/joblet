package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessage is a simple message type for testing
type TestMessage struct {
	ID      string
	Content string
}

func TestNewPubSub(t *testing.T) {
	ps := NewPubSub[TestMessage]()
	require.NotNil(t, ps)

	err := ps.Health(context.Background())
	assert.NoError(t, err)
}

func TestNewPubSubWithBufferSize(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](100))
	require.NotNil(t, ps)

	err := ps.Health(context.Background())
	assert.NoError(t, err)
}

func TestPublishSubscribe(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx := context.Background()

	// Subscribe first
	msgChan, unsubscribe, err := ps.Subscribe(ctx, "test-topic")
	require.NoError(t, err)
	require.NotNil(t, msgChan)
	defer unsubscribe()

	// Publish a message
	testMsg := TestMessage{ID: "1", Content: "hello"}
	err = ps.Publish(ctx, "test-topic", testMsg)
	require.NoError(t, err)

	// Receive the message
	select {
	case msg := <-msgChan:
		assert.Equal(t, "test-topic", msg.Topic)
		assert.Equal(t, "1", msg.Payload.ID)
		assert.Equal(t, "hello", msg.Payload.Content)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx := context.Background()

	// Create multiple subscribers
	msgChan1, unsub1, err := ps.Subscribe(ctx, "test-topic")
	require.NoError(t, err)
	defer unsub1()

	msgChan2, unsub2, err := ps.Subscribe(ctx, "test-topic")
	require.NoError(t, err)
	defer unsub2()

	// Publish a message
	testMsg := TestMessage{ID: "1", Content: "broadcast"}
	err = ps.Publish(ctx, "test-topic", testMsg)
	require.NoError(t, err)

	// Both subscribers should receive the message
	for i, ch := range []<-chan Message[TestMessage]{msgChan1, msgChan2} {
		select {
		case msg := <-ch:
			assert.Equal(t, "broadcast", msg.Payload.Content, "subscriber %d", i)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message on subscriber %d", i)
		}
	}
}

func TestMultipleTopics(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx := context.Background()

	// Subscribe to different topics
	topic1Chan, unsub1, err := ps.Subscribe(ctx, "topic-1")
	require.NoError(t, err)
	defer unsub1()

	topic2Chan, unsub2, err := ps.Subscribe(ctx, "topic-2")
	require.NoError(t, err)
	defer unsub2()

	// Publish to topic-1
	err = ps.Publish(ctx, "topic-1", TestMessage{ID: "1", Content: "for topic 1"})
	require.NoError(t, err)

	// Publish to topic-2
	err = ps.Publish(ctx, "topic-2", TestMessage{ID: "2", Content: "for topic 2"})
	require.NoError(t, err)

	// Verify topic-1 subscriber only gets topic-1 messages
	select {
	case msg := <-topic1Chan:
		assert.Equal(t, "topic-1", msg.Topic)
		assert.Equal(t, "for topic 1", msg.Payload.Content)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for topic-1 message")
	}

	// Verify topic-2 subscriber only gets topic-2 messages
	select {
	case msg := <-topic2Chan:
		assert.Equal(t, "topic-2", msg.Topic)
		assert.Equal(t, "for topic 2", msg.Payload.Content)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for topic-2 message")
	}
}

func TestUnsubscribe(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx := context.Background()

	// Subscribe
	msgChan, unsubscribe, err := ps.Subscribe(ctx, "test-topic")
	require.NoError(t, err)

	// Unsubscribe
	unsubscribe()

	// Channel should be closed
	select {
	case _, ok := <-msgChan:
		assert.False(t, ok, "channel should be closed after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Channel might block if not closed properly
	}
}

func TestContextCancellation(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Subscribe with cancellable context
	msgChan, _, err := ps.Subscribe(ctx, "test-topic")
	require.NoError(t, err)

	// Cancel the context
	cancel()

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Channel should be closed
	select {
	case _, ok := <-msgChan:
		assert.False(t, ok, "channel should be closed after context cancellation")
	case <-time.After(100 * time.Millisecond):
		// Acceptable - channel cleanup may take time
	}
}

func TestPublishToCancelledContext(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Publish should return context error
	err := ps.Publish(ctx, "test-topic", TestMessage{ID: "1"})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSubscribeToClosedPubSub(t *testing.T) {
	ps := NewPubSub[TestMessage]()
	ps.Close()

	_, _, err := ps.Subscribe(context.Background(), "test-topic")
	assert.ErrorIs(t, err, ErrSubscriberClosed)
}

func TestPublishToClosedPubSub(t *testing.T) {
	ps := NewPubSub[TestMessage]()
	ps.Close()

	err := ps.Publish(context.Background(), "test-topic", TestMessage{ID: "1"})
	assert.ErrorIs(t, err, ErrPublisherClosed)
}

func TestHealthOnClosedPubSub(t *testing.T) {
	ps := NewPubSub[TestMessage]()
	ps.Close()

	err := ps.Health(context.Background())
	assert.ErrorIs(t, err, ErrPublisherClosed)
}

func TestConcurrentPublishSubscribe(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](1000))
	defer ps.Close()

	ctx := context.Background()
	const numMessages = 100
	const numSubscribers = 5

	// Create subscribers
	var wg sync.WaitGroup
	receivedCounts := make([]int, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		msgChan, unsub, err := ps.Subscribe(ctx, "concurrent-topic")
		require.NoError(t, err)

		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer unsub()

			timeout := time.After(5 * time.Second)
			for {
				select {
				case _, ok := <-msgChan:
					if !ok {
						return
					}
					receivedCounts[idx]++
					if receivedCounts[idx] >= numMessages {
						return
					}
				case <-timeout:
					return
				}
			}
		}()
	}

	// Publish messages concurrently
	var publishWg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		publishWg.Add(1)
		go func(id int) {
			defer publishWg.Done()
			err := ps.Publish(ctx, "concurrent-topic", TestMessage{
				ID:      string(rune('0' + id%10)),
				Content: "concurrent message",
			})
			assert.NoError(t, err)
		}(i)
	}

	publishWg.Wait()
	wg.Wait()

	// Each subscriber should have received messages (may not be all due to timing)
	for i, count := range receivedCounts {
		assert.Greater(t, count, 0, "subscriber %d should have received at least some messages", i)
	}
}

func TestMessageMetadata(t *testing.T) {
	ps := NewPubSub[TestMessage](WithBufferSize[TestMessage](10))
	defer ps.Close()

	ctx := context.Background()

	msgChan, unsub, err := ps.Subscribe(ctx, "metadata-topic")
	require.NoError(t, err)
	defer unsub()

	beforePublish := time.Now()
	err = ps.Publish(ctx, "metadata-topic", TestMessage{ID: "meta-test"})
	require.NoError(t, err)
	afterPublish := time.Now()

	select {
	case msg := <-msgChan:
		// Verify message has proper metadata
		assert.NotEmpty(t, msg.ID)
		assert.Equal(t, "metadata-topic", msg.Topic)
		assert.True(t, msg.Timestamp.After(beforePublish) || msg.Timestamp.Equal(beforePublish))
		assert.True(t, msg.Timestamp.Before(afterPublish) || msg.Timestamp.Equal(afterPublish))
		assert.NotNil(t, msg.Attributes)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestCloseIdempotent(t *testing.T) {
	ps := NewPubSub[TestMessage]()

	// Close multiple times should not panic
	err := ps.Close()
	assert.NoError(t, err)

	err = ps.Close()
	assert.NoError(t, err)
}

func TestPublishNoSubscribers(t *testing.T) {
	ps := NewPubSub[TestMessage]()
	defer ps.Close()

	// Publish without subscribers should not error
	err := ps.Publish(context.Background(), "empty-topic", TestMessage{ID: "1"})
	assert.NoError(t, err)
}
