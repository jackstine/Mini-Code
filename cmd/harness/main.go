// Command harness runs the AI agent server with file tools.
package main

import (
	"fmt"
	stdlog "log"
	"os"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/log"
	"github.com/user/harness/pkg/server"
	"github.com/user/harness/pkg/tool"
)

func main() {
	// Initialize logging from environment
	logConfig, agentLogConfig := log.LoadFromEnv()
	logger := log.NewLogger(logConfig)
	agentLogger := log.NewAgentLogger(agentLogConfig)

	// Close agent logger on exit if created
	if agentLogger != nil {
		defer agentLogger.Close()
	}

	logger.Info("harness", "Starting harness server")

	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		stdlog.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Configure the harness
	config := harness.Config{
		APIKey:       apiKey,
		Model:        getEnvOrDefault("HARNESS_MODEL", harness.DefaultModel),
		MaxTokens:    harness.DefaultMaxTokens,
		MaxTurns:     harness.DefaultMaxTurns,
		SystemPrompt: getEnvOrDefault("HARNESS_SYSTEM_PROMPT", ""),
	}

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.NewListDirTool(),
		tool.NewGrepTool(),
	}

	// Create harness with nil handler initially
	// The server will set up the SSE event handler
	h, err := harness.NewHarness(config, tools, nil)
	if err != nil {
		logger.Error("harness", "Failed to create harness", log.F("error", err.Error()))
		stdlog.Fatalf("Failed to create harness: %v", err)
	}

	// Create server
	addr := getEnvOrDefault("HARNESS_ADDR", ":8080")
	srv := server.NewServer(h, addr, logger)

	// Create logging event handler that wraps SSE handler
	// This logs agent interactions to file while still broadcasting to SSE clients
	eventHandler := log.NewLoggingEventHandler(srv.EventHandler(), agentLogger)

	// Re-create harness with the logging event handler
	h, err = harness.NewHarness(config, tools, eventHandler)
	if err != nil {
		logger.Error("harness", "Failed to create harness with event handler", log.F("error", err.Error()))
		stdlog.Fatalf("Failed to create harness with event handler: %v", err)
	}

	// Set logger on harness for API and tool logging
	h.SetLogger(logger)

	// Update server with the new harness
	srv = server.NewServer(h, addr, logger)

	// Set up user prompt logging for agent interaction log
	srv.SetUserPromptLogger(eventHandler.LogUserPrompt)

	logger.Info("harness", "Server configured",
		log.F("addr", addr),
		log.F("model", config.Model),
		log.F("tools", "read,list_dir,grep"),
	)

	fmt.Printf("Harness server starting on %s\n", addr)
	fmt.Printf("Model: %s\n", config.Model)
	fmt.Printf("Tools: read, list_dir, grep\n")

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("harness", "Server error", log.F("error", err.Error()))
		stdlog.Fatalf("Server error: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
