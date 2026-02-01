package testutil_test

import (
	"context"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/user/harness/pkg/testutil"
)

func TestMessageBuilder_TextOnly(t *testing.T) {
	stream := testutil.NewMessageBuilder().AddText("Hello world").Build()

	// Verify events are generated
	eventCount := 0
	for stream.Next() {
		eventCount++
	}

	// Should have: MessageStart, ContentBlockStart, ContentBlockStop, MessageStop
	if eventCount != 4 {
		t.Errorf("expected 4 events, got %d", eventCount)
	}

	if stream.Err() != nil {
		t.Errorf("unexpected error: %v", stream.Err())
	}
}

func TestMessageBuilder_ToolUse(t *testing.T) {
	stream := testutil.NewMessageBuilder().
		AddToolUse("tool_123", "my_tool", map[string]string{"key": "value"}).
		BuildWithToolUse()

	// Collect events
	var events []anthropic.MessageStreamEventUnion
	for stream.Next() {
		events = append(events, stream.Current())
	}

	// Should have: MessageStart, ContentBlockStart, ContentBlockStop, MessageStop
	if len(events) != 4 {
		t.Errorf("expected 4 events, got %d", len(events))
	}

	// First event should be message_start
	if events[0].Type != "message_start" {
		t.Errorf("expected first event to be message_start, got %s", events[0].Type)
	}

	// Second event should be content_block_start
	if events[1].Type != "content_block_start" {
		t.Errorf("expected second event to be content_block_start, got %s", events[1].Type)
	}
}

func TestMessageBuilder_MultipleBlocks(t *testing.T) {
	stream := testutil.NewMessageBuilder().
		AddText("Let me help you").
		AddToolUse("tool_1", "search", map[string]string{"query": "test"}).
		BuildWithToolUse()

	eventCount := 0
	for stream.Next() {
		eventCount++
	}

	// Should have: MessageStart, 2x(ContentBlockStart + ContentBlockStop), MessageStop
	// = 1 + 2 + 2 + 1 = 6
	if eventCount != 6 {
		t.Errorf("expected 6 events for 2 content blocks, got %d", eventCount)
	}
}

func TestMessageBuilder_ThinkingBlock(t *testing.T) {
	stream := testutil.NewMessageBuilder().
		AddThinking("I need to think about this...").
		AddText("Here's my answer").
		Build()

	var events []anthropic.MessageStreamEventUnion
	for stream.Next() {
		events = append(events, stream.Current())
	}

	// Verify we have the right number of events
	if len(events) != 6 {
		t.Errorf("expected 6 events, got %d", len(events))
	}
}

func TestMockMessageStreamer(t *testing.T) {
	streamer := testutil.NewMockMessageStreamer()
	streamer.AddResponse(testutil.TextOnlyResponse("First response"))
	streamer.AddResponse(testutil.TextOnlyResponse("Second response"))

	// First call
	stream1 := streamer.NewStreaming(context.Background(), anthropic.MessageNewParams{})
	for stream1.Next() {
		// drain
	}

	// Verify params were recorded
	if len(streamer.RecordedParams) != 1 {
		t.Errorf("expected 1 recorded param, got %d", len(streamer.RecordedParams))
	}

	// Second call
	stream2 := streamer.NewStreaming(context.Background(), anthropic.MessageNewParams{})
	for stream2.Next() {
		// drain
	}

	if len(streamer.RecordedParams) != 2 {
		t.Errorf("expected 2 recorded params, got %d", len(streamer.RecordedParams))
	}
}

func TestMockMessageStreamer_Reset(t *testing.T) {
	streamer := testutil.NewMockMessageStreamer()
	streamer.AddResponse(testutil.TextOnlyResponse("Response"))

	// Use the response
	streamer.NewStreaming(context.Background(), anthropic.MessageNewParams{})

	// Reset
	streamer.Reset()

	if len(streamer.RecordedParams) != 0 {
		t.Errorf("expected 0 recorded params after reset, got %d", len(streamer.RecordedParams))
	}
}

func TestErrorResponse(t *testing.T) {
	stream := testutil.ErrorResponse(context.DeadlineExceeded)

	// Should not have any events
	if stream.Next() {
		t.Error("expected no events")
	}

	// Should have error
	if stream.Err() != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", stream.Err())
	}
}

func TestPresetFixtures(t *testing.T) {
	tests := []struct {
		name   string
		stream *testutil.MockStreamWithMessage
	}{
		{"TextOnlyResponse", testutil.TextOnlyResponse("Hello")},
		{"SingleToolResponse", testutil.SingleToolResponse("id", "tool", nil)},
		{"TextAndToolResponse", testutil.TextAndToolResponse("text", "id", "tool", nil)},
		{"ThinkingResponse", testutil.ThinkingResponse("thinking", "text")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventCount := 0
			for tt.stream.Next() {
				eventCount++
			}
			if eventCount == 0 {
				t.Errorf("%s: expected events, got none", tt.name)
			}
			if tt.stream.Err() != nil {
				t.Errorf("%s: unexpected error: %v", tt.name, tt.stream.Err())
			}
		})
	}
}

func TestMultiToolResponse(t *testing.T) {
	tools := []struct{ ID, Name string; Input any }{
		{ID: "t1", Name: "tool1", Input: map[string]string{"a": "1"}},
		{ID: "t2", Name: "tool2", Input: map[string]string{"b": "2"}},
	}

	stream := testutil.MultiToolResponse(tools)

	var events []anthropic.MessageStreamEventUnion
	for stream.Next() {
		events = append(events, stream.Current())
	}

	// Should have: MessageStart, 2x(ContentBlockStart + ContentBlockStop), MessageStop
	// = 1 + 4 + 1 = 6
	if len(events) != 6 {
		t.Errorf("expected 6 events for 2 tools, got %d", len(events))
	}
}

func TestMustMarshal(t *testing.T) {
	data := testutil.MustMarshal(map[string]string{"key": "value"})
	expected := `{"key":"value"}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestMustMarshal_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unmarshalable value")
		}
	}()

	// Channels cannot be marshaled to JSON
	testutil.MustMarshal(make(chan int))
}
