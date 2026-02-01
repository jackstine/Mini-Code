package harness

import "testing"

func TestConfig_Validate_RequiresAPIKey(t *testing.T) {
	c := Config{}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for missing APIKey")
	}
	if err.Error() != "APIKey is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfig_Validate_DefaultModel(t *testing.T) {
	c := Config{APIKey: "test-key"}
	err := c.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Model != DefaultModel {
		t.Errorf("expected Model to default to %q, got %q", DefaultModel, c.Model)
	}
}

func TestConfig_Validate_DefaultMaxTokens(t *testing.T) {
	c := Config{APIKey: "test-key"}
	err := c.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.MaxTokens != DefaultMaxTokens {
		t.Errorf("expected MaxTokens to default to %d, got %d", DefaultMaxTokens, c.MaxTokens)
	}
}

func TestConfig_Validate_DefaultMaxTurns(t *testing.T) {
	c := Config{APIKey: "test-key"}
	err := c.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.MaxTurns != DefaultMaxTurns {
		t.Errorf("expected MaxTurns to default to %d, got %d", DefaultMaxTurns, c.MaxTurns)
	}
}

func TestConfig_Validate_SystemPromptOptional(t *testing.T) {
	// Empty system prompt should be valid
	c := Config{APIKey: "test-key", SystemPrompt: ""}
	err := c.Validate()
	if err != nil {
		t.Fatalf("empty SystemPrompt should be valid: %v", err)
	}

	// Non-empty system prompt should be valid
	c.SystemPrompt = "You are a helpful assistant."
	err = c.Validate()
	if err != nil {
		t.Fatalf("non-empty SystemPrompt should be valid: %v", err)
	}
}

func TestConfig_Validate_PreservesCustomValues(t *testing.T) {
	c := Config{
		APIKey:       "test-key",
		Model:        "claude-3-opus-20240229",
		MaxTokens:    8192,
		SystemPrompt: "Custom prompt",
		MaxTurns:     20,
	}
	err := c.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom values should be preserved
	if c.Model != "claude-3-opus-20240229" {
		t.Errorf("custom Model should be preserved, got %q", c.Model)
	}
	if c.MaxTokens != 8192 {
		t.Errorf("custom MaxTokens should be preserved, got %d", c.MaxTokens)
	}
	if c.SystemPrompt != "Custom prompt" {
		t.Errorf("custom SystemPrompt should be preserved, got %q", c.SystemPrompt)
	}
	if c.MaxTurns != 20 {
		t.Errorf("custom MaxTurns should be preserved, got %d", c.MaxTurns)
	}
}
