# AgentKit Usage Guide

## Installation

### For Other Projects on This Machine

Add to your `go.mod`:
```bash
go get github.com/darkostanimirovic/agentkit@main
```

### For External Users

```bash
go get github.com/darkostanimirovic/agentkit@latest
```

## Import in Your Code

```go
import "github.com/darkostanimirovic/agentkit"
```

## Quick Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/darkostanimirovic/agentkit"
)

func main() {
    agent, err := agentkit.New(agentkit.Config{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "gpt-4o-mini",
    })
    if err != nil {
        panic(err)
    }
    
    events := agent.Run(context.Background(), "Hello!")
    for event := range events {
        if event.Type == agentkit.EventTypeFinalOutput {
            fmt.Println(event.Data["response"])
        }
    }
}
```

## Development

### Running Tests
```bash
make test
```

### Running Tests with Coverage
```bash
make coverage
```

### Formatting Code
```bash
make fmt
```

## Local Development Setup

If you want to develop agentkit locally and use it in another project:

1. In your project's `go.mod`, add a replace directive:
```go
replace github.com/darkostanimirovic/agentkit => /Users/darko/personal/agentkit
```

2. Run `go mod tidy` in your project

3. Your project will now use the local version of agentkit

## Publishing Updates

```bash
# Make your changes
git add .
git commit -m "Your change description"
git push

# Tag a release (optional but recommended)
git tag v0.1.0
git push origin v0.1.0
```

Users can then get specific versions:
```bash
go get github.com/darkostanimirovic/agentkit@v0.1.0
```
