package harness

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// StreamIterator is an interface for iterating over streaming events.
// It matches the pattern used by the Anthropic SDK's ssestream.Stream.
type StreamIterator interface {
	Next() bool
	Current() anthropic.MessageStreamEventUnion
	Err() error
}

// MessageStreamer is an interface for creating streaming message requests.
// This allows mocking the Anthropic client for testing.
type MessageStreamer interface {
	NewStreaming(ctx context.Context, params anthropic.MessageNewParams) StreamIterator
}

// realMessageStreamer wraps the real Anthropic client to implement MessageStreamer.
type realMessageStreamer struct {
	client anthropic.Client
}

// NewStreaming creates a new streaming request using the real Anthropic client.
func (r *realMessageStreamer) NewStreaming(ctx context.Context, params anthropic.MessageNewParams) StreamIterator {
	stream := r.client.Messages.NewStreaming(ctx, params)
	return &realStreamIterator{stream: stream}
}

// realStreamIterator wraps the real SDK stream to implement StreamIterator.
type realStreamIterator struct {
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
}

func (r *realStreamIterator) Next() bool {
	return r.stream.Next()
}

func (r *realStreamIterator) Current() anthropic.MessageStreamEventUnion {
	return r.stream.Current()
}

func (r *realStreamIterator) Err() error {
	return r.stream.Err()
}
