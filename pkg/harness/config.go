// Package harness provides the core agent orchestration framework that connects
// the Anthropic API with tools and event handling.
package harness

import "errors"

// Default configuration values
const (
	DefaultModel     = "claude-haiku-4-5-20251001"
	DefaultMaxTokens = 4096
	DefaultMaxTurns  = 10
)

// Config holds the configuration for a Harness instance.
type Config struct {
	// APIKey is the Anthropic API key. Required.
	APIKey string

	// Model is the Anthropic model to use. Default: "claude-3-haiku-20240307"
	Model string

	// MaxTokens is the maximum number of tokens in the response. Default: 4096
	MaxTokens int

	// SystemPrompt is an optional system prompt to set context for the agent.
	SystemPrompt string

	// MaxTurns is the maximum number of agent loop iterations. Default: 10
	MaxTurns int
}

// Validate checks the configuration and returns an error if invalid.
// It also applies defaults for optional fields.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("APIKey is required")
	}

	// Apply defaults
	if c.Model == "" {
		c.Model = DefaultModel
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = DefaultMaxTokens
	}
	if c.MaxTurns == 0 {
		c.MaxTurns = DefaultMaxTurns
	}

	return nil
}
