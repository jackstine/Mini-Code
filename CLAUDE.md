## Resource Materials
if you need to reference material about a package please use the context7 resource defined.
- for anthropic sdk in go please use https://context7.com/anthropics/anthropic-sdk-go
- for open tui https://context7.com/anomalyco/opentui
- for solid JS documentation https://context7.com/solidjs/solid-docs
- for solid JS in particular https://context7.com/solidjs/solid
- for zod https://context7.com/colinhacks/zod
- for bun https://context7.com/oven-sh/bun

## Running the Application

### Go Backend
```bash
cd /Users/jake/Projects/harness
go run cmd/harness/main.go
```

### TUI (TypeScript)
```bash
cd /Users/jake/Projects/harness/tui
bun run build   # Build with Solid plugin
bun run start   # Run the built app
bun run dev     # Build and run
```

### Testing
```bash
# Go tests
go test ./...

# TUI type checking
cd tui && bun run typecheck
```


