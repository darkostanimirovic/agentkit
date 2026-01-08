package agentkit

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// PendingFormatter formats the display message when a tool is about to execute
// It receives the tool name and parsed arguments
type PendingFormatter func(toolName string, args map[string]any) string

// ResultFormatter formats the display message when a tool completes
// It receives the tool name and the result returned by the handler
type ResultFormatter func(toolName string, result any) string

// Tool represents an agent tool with its metadata and handler
type Tool struct {
	name             string
	description      string
	parameters       map[string]any
	handler          ToolHandler
	pendingFormatter PendingFormatter
	resultFormatter  ResultFormatter
	concurrency      ConcurrencyMode
}

// ToolBuilder helps construct tools with a fluent API
type ToolBuilder struct {
	tool Tool
}

// NewTool creates a new tool builder
func NewTool(name string) *ToolBuilder {
	return &ToolBuilder{
		tool: Tool{
			name:        name,
			parameters:  map[string]any{},
			concurrency: ConcurrencyParallel,
		},
	}
}

// WithDescription sets the tool description
func (tb *ToolBuilder) WithDescription(desc string) *ToolBuilder {
	tb.tool.description = desc
	return tb
}

// WithParameter adds a parameter to the tool
func (tb *ToolBuilder) WithParameter(name string, schema *ParameterSchema) *ToolBuilder {
	if tb.tool.parameters["properties"] == nil {
		tb.tool.parameters = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		}
	}

	props := tb.tool.parameters["properties"].(map[string]any)
	props[name] = schema.ToMap()

	if schema.required {
		required := tb.tool.parameters["required"].([]string)
		tb.tool.parameters["required"] = append(required, name)
	}

	return tb
}

// WithRawParameters sets the full parameters schema for complex tools.
func (tb *ToolBuilder) WithRawParameters(params map[string]any) *ToolBuilder {
	tb.tool.parameters = params
	return tb
}

// WithJSONSchema sets the full JSON schema for complex tools.
func (tb *ToolBuilder) WithJSONSchema(schema map[string]any) *ToolBuilder {
	return tb.WithRawParameters(schema)
}

// WithHandler sets the tool handler function
func (tb *ToolBuilder) WithHandler(handler ToolHandler) *ToolBuilder {
	tb.tool.handler = handler
	return tb
}

// WithPendingFormatter sets the formatter for pending tool execution messages
func (tb *ToolBuilder) WithPendingFormatter(formatter PendingFormatter) *ToolBuilder {
	tb.tool.pendingFormatter = formatter
	return tb
}

// WithResultFormatter sets the formatter for tool result messages
func (tb *ToolBuilder) WithResultFormatter(formatter ResultFormatter) *ToolBuilder {
	tb.tool.resultFormatter = formatter
	return tb
}

// WithConcurrency controls whether a tool can run in parallel.
func (tb *ToolBuilder) WithConcurrency(mode ConcurrencyMode) *ToolBuilder {
	if mode == "" {
		mode = ConcurrencyParallel
	}
	tb.tool.concurrency = mode
	return tb
}

// Build returns the constructed tool
func (tb *ToolBuilder) Build() Tool {
	return tb.tool
}

// ToOpenAI converts the tool to OpenAI function definition
func (t *Tool) ToOpenAI() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        t.name,
			Description: t.description,
			Parameters:  t.parameters,
		},
	}
}

// Execute runs the tool handler
func (t *Tool) Execute(ctx context.Context, argsJSON string) (interface{}, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, err
	}
	return t.handler(ctx, args)
}

// Name returns the tool name
func (t *Tool) Name() string {
	return t.name
}

// FormatPending formats the pending message for this tool
func (t *Tool) FormatPending(args map[string]any) string {
	if t.pendingFormatter != nil {
		return t.pendingFormatter(t.name, args)
	}
	// Smart default: convert snake_case to Title Case
	displayName := formatToolName(t.name)
	return fmt.Sprintf("%s...", displayName)
}

// FormatResult formats the result message for this tool
func (t *Tool) FormatResult(result any) string {
	if t.resultFormatter != nil {
		return t.resultFormatter(t.name, result)
	}

	// Smart default: check for common error patterns in result
	if resultMap, ok := result.(map[string]any); ok {
		// Check for error field
		if errMsg, hasErr := resultMap["error"].(string); hasErr && errMsg != "" {
			return fmt.Sprintf("✗ %s", errMsg)
		}
		// Check for success field
		if success, hasSuccess := resultMap["success"].(bool); hasSuccess && !success {
			if msg, hasMsg := resultMap["message"].(string); hasMsg {
				return fmt.Sprintf("✗ %s", msg)
			}
			return fmt.Sprintf("✗ %s failed", formatToolName(t.name))
		}
	}

	// Default success message
	return fmt.Sprintf("✓ %s completed", formatToolName(t.name))
}

// formatToolName converts snake_case tool name to Title Case for display
// e.g., "assign_team" -> "Assign Team"
func formatToolName(name string) string {
	words := []rune{}
	capitalize := true

	for _, r := range name {
		switch {
		case r == '_' || r == '-':
			words = append(words, ' ')
			capitalize = true
		case capitalize:
			words = append(words, toUpper(r))
			capitalize = false
		default:
			words = append(words, r)
		}
	}

	return string(words)
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// ParameterSchema defines a tool parameter schema
type ParameterSchema struct {
	paramType   string
	description string
	required    bool
	enum        []string
	items       map[string]any
	properties  map[string]*ParameterSchema
}

const (
	paramTypeString = "string"
	paramTypeArray  = "array"
	paramTypeObject = "object"
)

// String creates a string parameter schema
func String() *ParameterSchema {
	return &ParameterSchema{paramType: paramTypeString}
}

// Array creates an array parameter schema
func Array(itemType string) *ParameterSchema {
	return &ParameterSchema{
		paramType: paramTypeArray,
		items:     map[string]any{"type": itemType},
	}
}

// ArrayOf creates an array parameter schema for complex item schemas.
func ArrayOf(itemSchema *ParameterSchema) *ParameterSchema {
	items := map[string]any{}
	if itemSchema != nil {
		items = itemSchema.ToMap()
	}

	return &ParameterSchema{
		paramType: paramTypeArray,
		items:     items,
	}
}

// Object creates an object parameter schema.
func Object() *ParameterSchema {
	return &ParameterSchema{
		paramType:  paramTypeObject,
		properties: map[string]*ParameterSchema{},
	}
}

// WithProperty adds a property to an object parameter schema.
func (ps *ParameterSchema) WithProperty(name string, schema *ParameterSchema) *ParameterSchema {
	if ps.paramType != paramTypeObject {
		ps.paramType = paramTypeObject
	}
	if ps.properties == nil {
		ps.properties = map[string]*ParameterSchema{}
	}
	ps.properties[name] = schema
	return ps
}

// WithDescription sets the parameter description
func (ps *ParameterSchema) WithDescription(desc string) *ParameterSchema {
	ps.description = desc
	return ps
}

// Required marks the parameter as required
func (ps *ParameterSchema) Required() *ParameterSchema {
	ps.required = true
	return ps
}

// Optional marks the parameter as optional
func (ps *ParameterSchema) Optional() *ParameterSchema {
	ps.required = false
	return ps
}

// WithEnum sets allowed values for the parameter
func (ps *ParameterSchema) WithEnum(values ...string) *ParameterSchema {
	ps.enum = values
	return ps
}

// ToMap converts the schema to a map for OpenAI
func (ps *ParameterSchema) ToMap() map[string]any {
	m := map[string]any{
		"type": ps.paramType,
	}

	if ps.description != "" {
		m["description"] = ps.description
	}

	if len(ps.enum) > 0 {
		m["enum"] = ps.enum
	}

	if len(ps.items) > 0 {
		m["items"] = ps.items
	}

	if len(ps.properties) > 0 {
		props := make(map[string]any, len(ps.properties))
		required := make([]string, 0, len(ps.properties))

		for name, schema := range ps.properties {
			if schema == nil {
				continue
			}
			props[name] = schema.ToMap()
			if schema.required {
				required = append(required, name)
			}
		}

		m["properties"] = props
		if len(required) > 0 {
			m["required"] = required
		}
	}

	return m
}
