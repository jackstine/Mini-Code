// Package log provides structured logging for the harness application.
// It supports two distinct logging systems:
// - Server Logging: Debug harness internals (stderr, controlled by HARNESS_LOG_LEVEL)
// - Agent Interaction Logging: View conversation flow (file, controlled by HARNESS_AGENT_LOG)
package log

import (
	"io"
	"os"
	"strings"
)

// Level represents a log level.
type Level int

const (
	// LevelDebug is for verbose diagnostic information.
	LevelDebug Level = iota
	// LevelInfo is for normal operational events.
	LevelInfo
	// LevelWarn is for potentially harmful situations.
	LevelWarn
	// LevelError is for error events where the application continues.
	LevelError
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level.
// Returns LevelInfo if the string is not recognized.
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Format represents an output format.
type Format int

const (
	// FormatText is human-readable text format.
	FormatText Format = iota
	// FormatJSON is JSON format (NDJSON).
	FormatJSON
)

// ParseFormat parses a string into a Format.
// Returns FormatText if the string is not recognized.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	default:
		return FormatText
	}
}

// LogConfig holds configuration for the server logger.
type LogConfig struct {
	// Level is the minimum log level (DEBUG, INFO, WARN, ERROR). Default: INFO
	Level Level
	// Format is the output format (text, json). Default: text
	Format Format
	// Categories is the list of categories to enable. Empty means all.
	Categories []string
	// Output is the destination for log output. Default: os.Stderr
	Output io.Writer
}

// AgentLogConfig holds configuration for the agent interaction logger.
type AgentLogConfig struct {
	// FilePath is the file path for agent logs. Empty means disabled.
	FilePath string
	// Format is the output format (text, json). Default: text
	Format Format
	// MaxSize is the maximum file size in bytes before rotation. Default: 10MB
	MaxSize int64
	// MaxFiles is the maximum number of rotated files to keep. Default: 5
	MaxFiles int
}

// Default values
const (
	DefaultMaxSize  = 10 * 1024 * 1024 // 10MB
	DefaultMaxFiles = 5
)

// LoadFromEnv loads logging configuration from environment variables.
func LoadFromEnv() (LogConfig, AgentLogConfig) {
	logConfig := LogConfig{
		Level:      ParseLevel(os.Getenv("HARNESS_LOG_LEVEL")),
		Format:     ParseFormat(os.Getenv("HARNESS_LOG_FORMAT")),
		Categories: parseCategories(os.Getenv("HARNESS_LOG_CATEGORIES")),
		Output:     os.Stderr,
	}

	agentConfig := AgentLogConfig{
		FilePath: os.Getenv("HARNESS_AGENT_LOG"),
		Format:   ParseFormat(os.Getenv("HARNESS_AGENT_LOG_FORMAT")),
		MaxSize:  DefaultMaxSize,
		MaxFiles: DefaultMaxFiles,
	}

	return logConfig, agentConfig
}

// parseCategories parses a comma-separated list of categories.
func parseCategories(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	categories := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			categories = append(categories, trimmed)
		}
	}
	return categories
}
