# Project Structure

AgentKit follows Go's standard project layout conventions.

## Directory Structure

```
agentkit/
├── .git/                   # Git repository
├── .gitignore             # Git ignore patterns
├── LICENSE                # MIT License
├── README.md              # Main documentation
├── CONTRIBUTING.md        # Contribution guidelines
├── SETUP_SUMMARY.md       # Setup instructions
├── Makefile               # Build commands
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
│
├── *.go                   # Core library files (root package)
├── *_test.go              # Test files
│
├── docs/                  # Documentation
│   ├── COMMUNITY_FEEDBACK.md
│   ├── MIGRATION.md
│   └── USAGE.md
│
├── examples/              # Example applications
│   ├── README.md
│   ├── basic/            # Basic agent example
│   ├── multi-agent/      # Multi-agent orchestration
│   └── rag/              # RAG implementation
│
└── internal/             # Private packages (not importable)
    └── testutil/         # Test utilities
        └── testutil.go
```

## Package Organization

### Root Package (`github.com/darkostanimirovic/agentkit`)
All public APIs live in the root package for simplicity:
- `agent.go` - Core agent implementation
- `tool.go` - Tool builder and definitions
- `event.go` - Event types and utilities
- `context.go` - Context helpers
- `conversation.go` - Conversation management
- `approval.go` - Approval flows
- `retry.go` - Retry logic
- `timeout.go` - Timeout management
- `logging.go` - Logging configuration
- `middleware.go` - Middleware support
- `parallel.go` - Parallel execution
- `subagent.go` - Sub-agent composition
- `struct_schema.go` - Struct-based schemas
- `llm_provider.go` - LLM provider interface
- `mock_llm.go` - Mock LLM for testing
- `responses_api.go` - OpenAI Responses API

### `examples/`
Self-contained example applications demonstrating various features:
- Each example is a standalone Go program
- Can be run with `go run main.go`
- Demonstrates real-world usage patterns

### `internal/`
Private packages not exposed in the public API:
- `testutil` - Shared test utilities
- Other internal packages can be added as needed

### `docs/`
Additional documentation:
- Migration guides
- Usage tutorials
- Community feedback

## Import Path

```go
import "github.com/darkostanimirovic/agentkit"
```

## Why This Structure?

1. **Single Package**: Simple, widely-used Go libraries often expose all APIs from the root package (e.g., `net/http`, `encoding/json`)
2. **Flat is Better**: No deep package hierarchies to navigate
3. **Examples Separate**: Examples are runnable applications, not part of the library
4. **Internal for Tests**: Shared test utilities don't pollute the public API
5. **Standard Layout**: Follows Go community conventions

## Adding New Features

- **Public API**: Add to root `*.go` files
- **Tests**: Add corresponding `*_test.go` files
- **Examples**: Create new directory under `examples/`
- **Internal Utilities**: Add to `internal/` subdirectories
