package log

import (
	"os"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"DEBUG", LevelDebug},
		{"debug", LevelDebug},
		{"INFO", LevelInfo},
		{"info", LevelInfo},
		{"WARN", LevelWarn},
		{"warn", LevelWarn},
		{"ERROR", LevelError},
		{"error", LevelError},
		{"", LevelInfo},       // Default
		{"invalid", LevelInfo}, // Default
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseLevel(tc.input)
			if result != tc.expected {
				t.Errorf("ParseLevel(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"text", FormatText},
		{"TEXT", FormatText},
		{"", FormatText},       // Default
		{"invalid", FormatText}, // Default
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseFormat(tc.input)
			if result != tc.expected {
				t.Errorf("ParseFormat(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if tc.level.String() != tc.expected {
				t.Errorf("Level(%d).String() = %q, expected %q", tc.level, tc.level.String(), tc.expected)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env values
	origLevel := os.Getenv("HARNESS_LOG_LEVEL")
	origFormat := os.Getenv("HARNESS_LOG_FORMAT")
	origCategories := os.Getenv("HARNESS_LOG_CATEGORIES")
	origAgentLog := os.Getenv("HARNESS_AGENT_LOG")
	origAgentFormat := os.Getenv("HARNESS_AGENT_LOG_FORMAT")

	// Restore env values after test
	defer func() {
		os.Setenv("HARNESS_LOG_LEVEL", origLevel)
		os.Setenv("HARNESS_LOG_FORMAT", origFormat)
		os.Setenv("HARNESS_LOG_CATEGORIES", origCategories)
		os.Setenv("HARNESS_AGENT_LOG", origAgentLog)
		os.Setenv("HARNESS_AGENT_LOG_FORMAT", origAgentFormat)
	}()

	t.Run("defaults", func(t *testing.T) {
		os.Unsetenv("HARNESS_LOG_LEVEL")
		os.Unsetenv("HARNESS_LOG_FORMAT")
		os.Unsetenv("HARNESS_LOG_CATEGORIES")
		os.Unsetenv("HARNESS_AGENT_LOG")
		os.Unsetenv("HARNESS_AGENT_LOG_FORMAT")

		logConfig, agentConfig := LoadFromEnv()

		if logConfig.Level != LevelInfo {
			t.Errorf("expected default level INFO, got %v", logConfig.Level)
		}
		if logConfig.Format != FormatText {
			t.Errorf("expected default format text, got %v", logConfig.Format)
		}
		if logConfig.Categories != nil {
			t.Errorf("expected nil categories, got %v", logConfig.Categories)
		}
		if agentConfig.FilePath != "" {
			t.Errorf("expected empty file path, got %q", agentConfig.FilePath)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		os.Setenv("HARNESS_LOG_LEVEL", "DEBUG")
		os.Setenv("HARNESS_LOG_FORMAT", "json")
		os.Setenv("HARNESS_LOG_CATEGORIES", "http,api,tool")
		os.Setenv("HARNESS_AGENT_LOG", "/tmp/agent.log")
		os.Setenv("HARNESS_AGENT_LOG_FORMAT", "json")

		logConfig, agentConfig := LoadFromEnv()

		if logConfig.Level != LevelDebug {
			t.Errorf("expected level DEBUG, got %v", logConfig.Level)
		}
		if logConfig.Format != FormatJSON {
			t.Errorf("expected format json, got %v", logConfig.Format)
		}
		if len(logConfig.Categories) != 3 {
			t.Errorf("expected 3 categories, got %d", len(logConfig.Categories))
		}
		if agentConfig.FilePath != "/tmp/agent.log" {
			t.Errorf("expected file path /tmp/agent.log, got %q", agentConfig.FilePath)
		}
		if agentConfig.Format != FormatJSON {
			t.Errorf("expected agent format json, got %v", agentConfig.Format)
		}
	})
}

func TestParseCategories(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"http", []string{"http"}},
		{"http,api", []string{"http", "api"}},
		{"http, api, tool", []string{"http", "api", "tool"}},
		{" http , api , ", []string{"http", "api"}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := parseCategories(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("parseCategories(%q) returned %d items, expected %d", tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("parseCategories(%q)[%d] = %q, expected %q", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}
