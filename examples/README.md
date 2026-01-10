# AgentKit Examples

This directory contains example applications demonstrating various AgentKit features.

## Running Examples

All examples require an OpenAI API key:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Available Examples

### Basic Agent (`basic/`)
A simple agent with a single tool that demonstrates:
- Basic agent setup
- Tool registration with parameters
- Event handling

```bash
cd examples/basic
go run main.go
```

### Multi-Agent System (`multi-agent/`)
Demonstrates agent composition with:
- Multiple specialized agents
- Agent orchestration
- Agents as tools for delegation

```bash
cd examples/multi-agent
go run main.go
```

### RAG (Retrieval Augmented Generation) (`rag/`)
Shows how to build a RAG system with:
- Knowledge base retrieval
- Context injection
- Search tool integration

```bash
cd examples/rag
go run main.go
```

## Creating Your Own Example

1. Create a new directory under `examples/`
2. Create a `main.go` file
3. Import agentkit: `import "github.com/darkostanimirovic/agentkit"`
4. Build your agent with tools and configuration
5. Document your example in this README
