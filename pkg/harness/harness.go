package harness

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/user/harness/pkg/tool"
)

// ErrPromptInProgress is returned when Prompt is called while another prompt is running.
var ErrPromptInProgress = errors.New("another prompt is already in progress")

// Harness orchestrates the AI agent loop, connecting the Anthropic API
// with tools and event handling.
type Harness struct {
	client     anthropic.Client
	config     Config
	tools      map[string]tool.Tool
	toolParams []anthropic.ToolUnionParam
	handler    EventHandler
	messages   []anthropic.MessageParam

	// Concurrency control
	mu           sync.Mutex
	running      bool
	cancelFunc   context.CancelFunc
	runningCtx   context.Context
}

// NewHarness creates a new Harness with the given configuration, tools, and event handler.
// The handler may be nil - the harness will operate silently in that case.
func NewHarness(config Config, tools []tool.Tool, handler EventHandler) (*Harness, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create Anthropic client
	client := anthropic.NewClient(option.WithAPIKey(config.APIKey))

	// Convert tools to API format and build lookup map
	toolParams := make([]anthropic.ToolUnionParam, len(tools))
	toolMap := make(map[string]tool.Tool)
	for i, t := range tools {
		toolParams[i] = toolToParam(t)
		toolMap[t.Name()] = t
	}

	return &Harness{
		client:     client,
		config:     config,
		tools:      toolMap,
		toolParams: toolParams,
		handler:    handler,
		messages:   []anthropic.MessageParam{},
	}, nil
}

// toolToParam converts a Tool interface to Anthropic ToolUnionParam.
func toolToParam(t tool.Tool) anthropic.ToolUnionParam {
	// Parse the input schema to get the properties
	var schemaMap map[string]any
	json.Unmarshal(t.InputSchema(), &schemaMap)

	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        t.Name(),
			Description: anthropic.String(t.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:       "object",
				Properties: schemaMap["properties"],
			},
		},
	}
}

// Prompt sends a user message to the agent and runs the agent loop until completion.
// Returns an error if another prompt is already in progress, the API fails, or context is cancelled.
func (h *Harness) Prompt(ctx context.Context, content string) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return ErrPromptInProgress
	}
	h.running = true
	// Create a cancellable context for this prompt
	promptCtx, cancel := context.WithCancel(ctx)
	h.cancelFunc = cancel
	h.runningCtx = promptCtx
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.running = false
		h.cancelFunc = nil
		h.runningCtx = nil
		h.mu.Unlock()
	}()

	// Append user message to conversation history
	h.messages = append(h.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))

	// Run the agent loop
	return h.runAgentLoop(promptCtx)
}

// Cancel cancels the currently running prompt, if any.
// Safe to call when no prompt is running (no-op).
func (h *Harness) Cancel() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// ToolCall represents a tool invocation request from the agent.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// runAgentLoop runs the main agent loop until termination.
// Termination conditions:
// 1. No tool calls in response → end loop
// 2. MaxTurns exceeded → end loop
// 3. API error → return error
// 4. Context cancelled → return error
func (h *Harness) runAgentLoop(ctx context.Context) error {
	for turn := 0; turn < h.config.MaxTurns; turn++ {
		// Check context before making API call
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build system blocks if we have a system prompt
		var systemBlocks []anthropic.TextBlockParam
		if h.config.SystemPrompt != "" {
			systemBlocks = []anthropic.TextBlockParam{{Text: h.config.SystemPrompt}}
		}

		// Create streaming request
		stream := h.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(h.config.Model),
			MaxTokens: int64(h.config.MaxTokens),
			System:    systemBlocks,
			Messages:  h.messages,
			Tools:     h.toolParams,
		})

		// Accumulate streaming response
		message := anthropic.Message{}
		for stream.Next() {
			event := stream.Current()
			if err := message.Accumulate(event); err != nil {
				return err
			}

			// Emit events on ContentBlockStopEvent
			switch e := event.AsAny().(type) {
			case anthropic.ContentBlockStopEvent:
				h.emitBlockComplete(&message, e.Index)
			}
		}
		if stream.Err() != nil {
			return stream.Err()
		}

		// Append assistant message to history
		h.messages = append(h.messages, message.ToParam())

		// Process tool calls
		toolCalls := h.extractToolCalls(&message)
		if len(toolCalls) == 0 {
			return nil // No tool calls = done
		}

		// Execute tools sequentially with fail-fast
		toolResults, err := h.executeTools(ctx, toolCalls)
		if err != nil {
			return err // Context cancellation
		}

		// Append tool results as user message
		h.messages = append(h.messages, anthropic.NewUserMessage(toolResults...))
	}
	return nil // MaxTurns reached
}

// emitBlockComplete emits events for a completed content block.
func (h *Harness) emitBlockComplete(msg *anthropic.Message, index int64) {
	if h.handler == nil {
		return
	}

	if int(index) >= len(msg.Content) {
		return
	}

	block := msg.Content[index]
	switch b := block.AsAny().(type) {
	case anthropic.TextBlock:
		h.handler.OnText(b.Text)
	case anthropic.ToolUseBlock:
		inputJSON, _ := json.Marshal(b.Input)
		h.handler.OnToolCall(b.ID, b.Name, inputJSON)
	}
}

// extractToolCalls extracts tool call information from a message.
func (h *Harness) extractToolCalls(msg *anthropic.Message) []ToolCall {
	var calls []ToolCall
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.ToolUseBlock:
			inputJSON, _ := json.Marshal(b.Input)
			calls = append(calls, ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: inputJSON,
			})
		}
	}
	return calls
}

// executeTools executes tools sequentially with fail-fast behavior.
// Returns tool result blocks and an error if context was cancelled.
func (h *Harness) executeTools(ctx context.Context, calls []ToolCall) ([]anthropic.ContentBlockParamUnion, error) {
	var results []anthropic.ContentBlockParamUnion
	for _, call := range calls {
		// Check context before each tool execution
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := h.executeTool(ctx, call)
		isError := err != nil
		resultStr := result
		if isError {
			resultStr = err.Error()
		}

		// Emit tool result event
		if h.handler != nil {
			h.handler.OnToolResult(call.ID, resultStr, isError)
		}

		// Create tool result block
		results = append(results, anthropic.NewToolResultBlock(call.ID, resultStr, isError))

		// Fail-fast: stop on first error
		if isError {
			break
		}
	}
	return results, nil
}

// executeTool executes a single tool and returns its result.
func (h *Harness) executeTool(ctx context.Context, call ToolCall) (string, error) {
	t, ok := h.tools[call.Name]
	if !ok {
		return "", errors.New("unknown tool: " + call.Name)
	}
	return t.Execute(ctx, call.Input)
}

// Messages returns a copy of the current conversation history.
// Useful for debugging and testing.
func (h *Harness) Messages() []anthropic.MessageParam {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Return a copy to avoid race conditions
	msgs := make([]anthropic.MessageParam, len(h.messages))
	copy(msgs, h.messages)
	return msgs
}
