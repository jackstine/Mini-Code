package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerLevelFiltering(t *testing.T) {
	tests := []struct {
		name        string
		configLevel Level
		logLevel    Level
		shouldLog   bool
	}{
		{"debug at debug", LevelDebug, LevelDebug, true},
		{"info at debug", LevelDebug, LevelInfo, true},
		{"warn at debug", LevelDebug, LevelWarn, true},
		{"error at debug", LevelDebug, LevelError, true},
		{"debug at info", LevelInfo, LevelDebug, false},
		{"info at info", LevelInfo, LevelInfo, true},
		{"warn at info", LevelInfo, LevelWarn, true},
		{"error at info", LevelInfo, LevelError, true},
		{"debug at warn", LevelWarn, LevelDebug, false},
		{"info at warn", LevelWarn, LevelInfo, false},
		{"warn at warn", LevelWarn, LevelWarn, true},
		{"error at warn", LevelWarn, LevelError, true},
		{"debug at error", LevelError, LevelDebug, false},
		{"info at error", LevelError, LevelInfo, false},
		{"warn at error", LevelError, LevelWarn, false},
		{"error at error", LevelError, LevelError, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(LogConfig{
				Level:  tc.configLevel,
				Format: FormatText,
				Output: &buf,
			})

			switch tc.logLevel {
			case LevelDebug:
				logger.Debug("test", "test message")
			case LevelInfo:
				logger.Info("test", "test message")
			case LevelWarn:
				logger.Warn("test", "test message")
			case LevelError:
				logger.Error("test", "test message")
			}

			logged := buf.Len() > 0
			if logged != tc.shouldLog {
				t.Errorf("expected shouldLog=%v, got logged=%v", tc.shouldLog, logged)
			}
		})
	}
}

func TestLoggerCategoryFiltering(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		logCat     string
		shouldLog  bool
	}{
		{"no filter - http", nil, "http", true},
		{"no filter - api", nil, "api", true},
		{"filter http - http", []string{"http"}, "http", true},
		{"filter http - api", []string{"http"}, "api", false},
		{"filter http,api - http", []string{"http", "api"}, "http", true},
		{"filter http,api - api", []string{"http", "api"}, "api", true},
		{"filter http,api - tool", []string{"http", "api"}, "tool", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(LogConfig{
				Level:      LevelInfo,
				Format:     FormatText,
				Categories: tc.categories,
				Output:     &buf,
			})

			logger.Info(tc.logCat, "test message")

			logged := buf.Len() > 0
			if logged != tc.shouldLog {
				t.Errorf("expected shouldLog=%v, got logged=%v", tc.shouldLog, logged)
			}
		})
	}
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:  LevelDebug,
		Format: FormatText,
		Output: &buf,
	})

	logger.Info("http", "Request received", F("method", "POST"), F("path", "/prompt"))

	output := buf.String()

	// Check format: TIMESTAMP LEVEL [CATEGORY] MESSAGE key=value
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level in output: %s", output)
	}
	if !strings.Contains(output, "[http]") {
		t.Errorf("expected [http] category in output: %s", output)
	}
	if !strings.Contains(output, "Request received") {
		t.Errorf("expected message in output: %s", output)
	}
	if !strings.Contains(output, "method=POST") {
		t.Errorf("expected method=POST in output: %s", output)
	}
	if !strings.Contains(output, "path=/prompt") {
		t.Errorf("expected path=/prompt in output: %s", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:  LevelDebug,
		Format: FormatJSON,
		Output: &buf,
	})

	logger.Info("http", "Request received", F("method", "POST"), F("status", 200))

	// Parse JSON output
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if entry["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", entry["level"])
	}
	if entry["category"] != "http" {
		t.Errorf("expected category http, got %v", entry["category"])
	}
	if entry["message"] != "Request received" {
		t.Errorf("expected message 'Request received', got %v", entry["message"])
	}
	if entry["method"] != "POST" {
		t.Errorf("expected method POST, got %v", entry["method"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("expected status 200, got %v", entry["status"])
	}
}

func TestLoggerIsDebugEnabled(t *testing.T) {
	tests := []struct {
		level   Level
		enabled bool
	}{
		{LevelDebug, true},
		{LevelInfo, false},
		{LevelWarn, false},
		{LevelError, false},
	}

	for _, tc := range tests {
		t.Run(tc.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(LogConfig{
				Level:  tc.level,
				Output: &buf,
			})

			if logger.IsDebugEnabled() != tc.enabled {
				t.Errorf("expected IsDebugEnabled=%v, got %v", tc.enabled, logger.IsDebugEnabled())
			}
		})
	}
}

func TestLoggerFieldFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:  LevelInfo,
		Format: FormatText,
		Output: &buf,
	})

	// Test string with spaces is quoted
	logger.Info("test", "message", F("error", "file not found"))
	output := buf.String()
	if !strings.Contains(output, `error="file not found"`) {
		t.Errorf("expected quoted string with spaces: %s", output)
	}
}

func TestNopLogger(t *testing.T) {
	logger := NopLogger{}

	// Should not panic
	logger.Debug("test", "message")
	logger.Info("test", "message")
	logger.Warn("test", "message")
	logger.Error("test", "message")

	if logger.IsDebugEnabled() {
		t.Error("NopLogger should return false for IsDebugEnabled")
	}
}
