package log

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// F is a convenience function to create a Field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Logger defines the interface for server logging.
type Logger interface {
	// Debug logs a debug-level message.
	Debug(category string, message string, fields ...Field)
	// Info logs an info-level message.
	Info(category string, message string, fields ...Field)
	// Warn logs a warn-level message.
	Warn(category string, message string, fields ...Field)
	// Error logs an error-level message.
	Error(category string, message string, fields ...Field)
	// IsDebugEnabled returns true if debug-level logging is enabled.
	// Use this to avoid expensive formatting operations when debug is disabled.
	IsDebugEnabled() bool
}

// serverLogger is the concrete implementation of Logger.
type serverLogger struct {
	mu         sync.Mutex
	config     LogConfig
	categories map[string]struct{} // nil means all categories
}

// NewLogger creates a new Logger with the given configuration.
func NewLogger(config LogConfig) Logger {
	// Build category lookup map
	var cats map[string]struct{}
	if len(config.Categories) > 0 {
		cats = make(map[string]struct{})
		for _, c := range config.Categories {
			cats[c] = struct{}{}
		}
	}

	return &serverLogger{
		config:     config,
		categories: cats,
	}
}

// Debug logs a debug-level message.
func (l *serverLogger) Debug(category string, message string, fields ...Field) {
	l.log(LevelDebug, category, message, fields)
}

// Info logs an info-level message.
func (l *serverLogger) Info(category string, message string, fields ...Field) {
	l.log(LevelInfo, category, message, fields)
}

// Warn logs a warn-level message.
func (l *serverLogger) Warn(category string, message string, fields ...Field) {
	l.log(LevelWarn, category, message, fields)
}

// Error logs an error-level message.
func (l *serverLogger) Error(category string, message string, fields ...Field) {
	l.log(LevelError, category, message, fields)
}

// IsDebugEnabled returns true if debug-level logging is enabled.
func (l *serverLogger) IsDebugEnabled() bool {
	return l.config.Level <= LevelDebug
}

// log performs the actual logging.
func (l *serverLogger) log(level Level, category string, message string, fields []Field) {
	// Check level
	if level < l.config.Level {
		return
	}

	// Check category
	if l.categories != nil {
		if _, ok := l.categories[category]; !ok {
			return
		}
	}

	// Format and write
	var output string
	timestamp := time.Now().UTC()
	if l.config.Format == FormatJSON {
		output = l.formatJSON(timestamp, level, category, message, fields)
	} else {
		output = l.formatText(timestamp, level, category, message, fields)
	}

	l.mu.Lock()
	io.WriteString(l.config.Output, output)
	l.mu.Unlock()
}

// formatText formats a log entry as text.
func (l *serverLogger) formatText(timestamp time.Time, level Level, category string, message string, fields []Field) string {
	// Format: TIMESTAMP LEVEL [CATEGORY] MESSAGE key=value key2=value2
	ts := timestamp.Format(time.RFC3339Nano)
	result := fmt.Sprintf("%s %-5s [%s] %s", ts, level.String(), category, message)

	for _, f := range fields {
		result += fmt.Sprintf(" %s=%v", f.Key, formatValue(f.Value))
	}

	return result + "\n"
}

// formatJSON formats a log entry as JSON.
func (l *serverLogger) formatJSON(timestamp time.Time, level Level, category string, message string, fields []Field) string {
	entry := map[string]any{
		"timestamp": timestamp.Format(time.RFC3339Nano),
		"level":     level.String(),
		"category":  category,
		"message":   message,
	}

	// Add fields directly to the entry (not nested under "fields")
	for _, f := range fields {
		entry[f.Key] = f.Value
	}

	data, _ := json.Marshal(entry)
	return string(data) + "\n"
}

// formatValue formats a value for text output.
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		// Quote strings that contain spaces
		if containsSpace(val) {
			return fmt.Sprintf("%q", val)
		}
		return val
	case error:
		return fmt.Sprintf("%q", val.Error())
	default:
		return fmt.Sprintf("%v", val)
	}
}

// containsSpace returns true if s contains whitespace.
func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return true
		}
	}
	return false
}

// NopLogger is a logger that does nothing. Useful for testing.
type NopLogger struct{}

func (NopLogger) Debug(category string, message string, fields ...Field) {}
func (NopLogger) Info(category string, message string, fields ...Field)  {}
func (NopLogger) Warn(category string, message string, fields ...Field)  {}
func (NopLogger) Error(category string, message string, fields ...Field) {}
func (NopLogger) IsDebugEnabled() bool                                   { return false }
