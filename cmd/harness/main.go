// Command harness runs the AI agent server with file tools.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/server"
	"github.com/user/harness/pkg/tool"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
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
		log.Fatalf("Failed to create harness: %v", err)
	}

	// Create server
	addr := getEnvOrDefault("HARNESS_ADDR", ":8080")
	srv := server.NewServer(h, addr)

	// Re-create harness with SSE event handler
	h, err = harness.NewHarness(config, tools, srv.EventHandler())
	if err != nil {
		log.Fatalf("Failed to create harness with event handler: %v", err)
	}

	// Update server with the new harness
	srv = server.NewServer(h, addr)

	fmt.Printf("Harness server starting on %s\n", addr)
	fmt.Printf("Model: %s\n", config.Model)
	fmt.Printf("Tools: read, list_dir, grep\n")

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
