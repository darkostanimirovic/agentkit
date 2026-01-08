# AgentKit Project Structure Summary

## Overview

AgentKit follows Go's standard single-package library layout with organized subdirectories for examples, documentation, and internal utilities.

## Directory Layout

```
agentkit/
├── Root Package (github.com/darkostanimirovic/agentkit)
│   ├── agent.go                    # Core agent orchestration
│   ├── tool.go                     # Tool builder and registration
│   ├── event.go                    # Event system
│   ├── context.go                  # Context and dependency injection
│   ├── conversation.go             # Conversation management
│   ├── conversation_memory.go      # In-memory conversation store
│   ├── approval.go                 # Approval flows
│   ├── retry.go                    # Retry logic
│   ├── timeout.go                  # Timeout configuration
│   ├── logging.go                  # Logging setup
│   ├── middleware.go               # Middleware support
│   ├── parallel.go                 # Parallel tool execution
│   ├── subagent.go                 # Sub-agent composition
│   ├── struct_schema.go            # Struct-based schemas
│   ├── llm_provider.go             # LLM provider interface
│   ├── mock_llm.go                 # Mock LLM for testing
│   ├── responses_api.go            # OpenAI Responses API
│   ├── tool_concurrency.go         # Tool concurrency modes
│   ├── event_helpers.go            # Event helper functions
│   └── *_test.go                   # Comprehensive test suite
│
├── docs/                           # Documentation
│   ├── PROJECT_STRUCTURE.md        # This structure guide
│   ├── USAGE.md                    # Usage instructions
│   ├── MIGRATION.md                # Migration guide
│   └── COMMUNITY_FEEDBACK.md       # Community input
│
├── examples/                       # Working examples
│   ├── README.md                   # Examples documentation
│   ├── basic/                      # Simple agent example
│   │   ├── main.go
│   │   ├── go.mod
│   │   └── go.sum
│   ├── multi-agent/                # Multi-agent orchestration
│   │   ├── main.go
│   │   ├── go.mod
│   │   └── go.sum
│   └── rag/                        # RAG implementation
│       ├── main.go
│       ├── go.mod
│       └── go.sum
│
└── internal/                       # Private packages
    └── testutil/                   # Test utilities
        └── testutil.go

## Root Files

├── .gitignore                      # Git ignore patterns
├── LICENSE                         # MIT License
├── README.md                       # Main documentation
├── CONTRIBUTING.md                 # Contribution guidelines
├── SETUP_SUMMARY.md                # Initial setup summary
├── Makefile                        # Build commands
├── go.mod                          # Go module definition
└── go.sum                          # Go module checksums
```

## Import Paths

**Main Package:**
```go
import "github.com/darkostanimirovic/agentkit"
```

**Internal Packages (not importable externally):**
```go
import "github.com/darkostanimirovic/agentkit/internal/testutil"
```

## Design Rationale

### Why Single Package?

1. **Simplicity** - All public APIs accessible from one import
2. **Go Convention** - Follows stdlib pattern (net/http, encoding/json)
3. **No Package Cycles** - Flat structure avoids circular dependencies
4. **Easy Discovery** - All types visible in one place

### Why This Layout?

1. **Examples as Applications** - Each example is self-contained with its own go.mod
2. **Docs Separate** - Keeps root clean while organizing documentation
3. **Internal for Privacy** - Test utilities don't pollute public API
4. **Standard Go** - Follows Go project layout conventions

## File Organization Principles

### Root Package Files
- One concept per file
- Test files alongside implementation
- Clear naming (agent.go, tool.go, event.go)

### Examples
- Each is a complete, runnable application
- Uses replace directive for local development
- Can be copied and modified by users

### Documentation
- Technical docs in docs/
- API docs via godoc
- README for quick start

## Adding New Features

| Feature Type | Location | Example |
|-------------|----------|---------|
| Public API | Root `*.go` | `agent.go`, `tool.go` |
| Tests | Root `*_test.go` | `agent_test.go` |
| Examples | `examples/newfeature/` | `examples/rag/` |
| Internal Utilities | `internal/pkgname/` | `internal/testutil/` |
| Documentation | `docs/` | `docs/USAGE.md` |

## Testing Structure

```bash
# Test main package
go test .

# Test all packages (including internal)
go test ./...

# Test specific example
cd examples/basic && go test ./...
```

## Benefits of This Structure

✅ **Simple Imports** - One package to import
✅ **Clear Organization** - Examples, docs, internal separated
✅ **Standard Go** - Follows community conventions
✅ **Easy to Navigate** - Flat structure, logical grouping
✅ **Maintainable** - Clear boundaries between public/private
✅ **Extensible** - Easy to add examples and docs
✅ **Testable** - Each package independently testable

## References

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Standard Go Project Layout](https://go.dev/doc/modules/layout)
- [Effective Go](https://go.dev/doc/effective_go)
