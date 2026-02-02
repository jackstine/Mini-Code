.PHONY: all build build-backend build-tui run run-backend run-tui dev \
        test test-backend test-tui test-verbose test-coverage test-e2e \
        fmt vet tidy typecheck clean clean-backend clean-tui deps help

# Configuration
BIN := bin/harness
MAIN := ./cmd/harness
BUN := /Users/jake/.bun/bin/bun
TUI_DIR := tui

# Default target
all: build

# ============================================================================
# Build
# ============================================================================

build: build-backend build-tui ## Build both backend and TUI

build-backend: ## Build the Go backend binary
	go build -o $(BIN) $(MAIN)

build-tui: ## Build the TUI frontend
	cd $(TUI_DIR) && $(BUN) run build

# ============================================================================
# Run
# ============================================================================

run: run-backend ## Run the backend server

run-backend: build-backend ## Build and run the Go backend
	./$(BIN)

run-tui: build-tui ## Build and run the TUI
	cd $(TUI_DIR) && $(BUN) run start

dev: ## Run backend and TUI (use in separate terminals)
	@echo "Run in separate terminals:"
	@echo "  make run-backend"
	@echo "  make run-tui"

dev-backend: ## Run backend without building (go run)
	go run $(MAIN)

dev-tui: ## Build and run TUI in one step
	cd $(TUI_DIR) && $(BUN) run dev

# ============================================================================
# Test
# ============================================================================

test: test-backend test-tui ## Run all tests

test-backend: ## Run Go tests
	go test ./...

test-tui: typecheck ## Run TUI type checking

test-verbose: ## Run Go tests with verbose output
	go test -v ./...

test-coverage: ## Run tests with coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-unit: ## Run unit tests only (pkg/)
	go test -v ./pkg/...

test-e2e: ## Run end-to-end tests
	go test -v ./tests/e2e/...

# ============================================================================
# Code Quality
# ============================================================================

fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy Go modules
	go mod tidy

typecheck: ## TypeScript type checking
	cd $(TUI_DIR) && $(BUN) run typecheck

check: fmt vet test ## Run all checks (format, vet, test)

# ============================================================================
# Dependencies
# ============================================================================

deps: deps-backend deps-tui ## Install all dependencies

deps-backend: ## Download Go dependencies
	go mod download
	go mod tidy

deps-tui: ## Install TUI dependencies
	cd $(TUI_DIR) && $(BUN) install

# ============================================================================
# Clean
# ============================================================================

clean: clean-backend clean-tui clean-coverage ## Clean all build artifacts

clean-backend: ## Remove Go build artifacts
	rm -rf bin/
	rm -f $(BIN)

clean-tui: ## Remove TUI build artifacts
	rm -rf $(TUI_DIR)/dist/

clean-coverage: ## Remove coverage files
	rm -f coverage.out coverage.html

clean-all: clean ## Clean everything including node_modules
	rm -rf $(TUI_DIR)/node_modules/

# ============================================================================
# Help
# ============================================================================

help: ## Show this help
	@echo "Harness - AI Agent Terminal Application"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Environment variables:"
	@echo "  ANTHROPIC_API_KEY     Required for running the server"
	@echo "  HARNESS_ADDR          Server address (default: :8080)"
	@echo "  HARNESS_MODEL         Claude model (default: claude-3-haiku-20240307)"
	@echo "  HARNESS_LOG_LEVEL     Log level: DEBUG, INFO, WARN, ERROR"
