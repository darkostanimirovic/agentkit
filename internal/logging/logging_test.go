package logging

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestDefaultLoggingConfig(t *testing.T) {
	cfg := DefaultLoggingConfig()

	if cfg.Level != slog.LevelInfo {
		t.Errorf("expected Level to be Info, got %v", cfg.Level)
	}
	if !cfg.LogPrompts {
		t.Error("expected LogPrompts to be true")
	}
	if cfg.LogResponses {
		t.Error("expected LogResponses to be false")
	}
	if cfg.LogToolCalls {
		t.Error("expected LogToolCalls to be false")
	}
}

func TestSilentLoggingConfig(t *testing.T) {
	cfg := LoggingConfig{}.Silent()

	if cfg == nil {
		t.Fatal("expected Silent() to return a config")
	}
	if cfg.Handler == nil {
		t.Error("expected Handler to be set")
	}
}

func TestVerboseLoggingConfig(t *testing.T) {
	cfg := LoggingConfig{}.Verbose()

	if cfg == nil {
		t.Fatal("expected Verbose() to return a config")
	}
	if cfg.Level != slog.LevelDebug {
		t.Errorf("expected Level to be Debug, got %v", cfg.Level)
	}
}

func TestResolveLogger_WithProvidedLogger(t *testing.T) {
	var buf bytes.Buffer
	customLogger := slog.New(slog.NewTextHandler(&buf, nil))

	cfg := LoggingConfig{
		Logger: customLogger,
	}

	logger := ResolveLogger(cfg)
	if logger != customLogger {
		t.Error("expected ResolveLogger to return the provided logger")
	}
}

func TestResolveLogger_WithHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	cfg := LoggingConfig{
		Handler: handler,
	}

	logger := ResolveLogger(cfg)
	if logger == nil {
		t.Fatal("expected ResolveLogger to return a logger")
	}

	// Test that the logger uses the provided handler
	logger.Info("test message")
	if buf.Len() == 0 {
		t.Error("expected message to be written to buffer")
	}
}

func TestResolveLogger_DefaultToStderr(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := LoggingConfig{}
	logger := ResolveLogger(cfg)

	// Write a log message
	logger.Info("test message to stderr")

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read what was written
	output, _ := io.ReadAll(r)
	if len(output) == 0 {
		t.Error("expected default logger to write to stderr")
	}
	if !bytes.Contains(output, []byte("test message to stderr")) {
		t.Error("expected log message to be written to stderr")
	}
}

func TestResolveLogger_WithLevel(t *testing.T) {
	var buf bytes.Buffer

	cfg := LoggingConfig{
		Level:   slog.LevelWarn,
		Handler: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}),
	}

	logger := ResolveLogger(cfg)
	logger.Info("info message") // Should not appear
	logger.Warn("warn message") // Should appear

	output := buf.String()
	if bytes.Contains([]byte(output), []byte("info message")) {
		t.Error("expected info message to be filtered out")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("expected warn message to appear")
	}
}
func TestResolvePromptLogPath(t *testing.T) {
	tests := []struct {
		name     string
		cfg      LoggingConfig
		expected string
	}{
		{
			name:     "default path",
			cfg:      LoggingConfig{},
			expected: defaultPromptLogPath,
		},
		{
			name:     "custom path",
			cfg:      LoggingConfig{PromptLogPath: "/tmp/custom.log"},
			expected: "/tmp/custom.log",
		},
		{
			name:     "empty path falls back to default",
			cfg:      LoggingConfig{PromptLogPath: "   "},
			expected: defaultPromptLogPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolvePromptLogPath(tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key       string
		sensitive bool
	}{
		{"api_key", true},
		{"API_KEY", true},
		{"  apikey  ", true},
		{"password", true},
		{"token", true},
		{"Authorization", true},
		{"secret", true},
		{"normal_field", false},
		{"username", false},
		{"data", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			if result != tt.sensitive {
				t.Errorf("expected isSensitiveKey(%q) = %v, got %v", tt.key, tt.sensitive, result)
			}
		})
	}
}

func TestRedactAny(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name: "redact map with api_key",
			input: map[string]any{
				"api_key": "secret123",
				"data":    "public",
			},
			expected: map[string]any{
				"api_key": "[redacted]",
				"data":    "public",
			},
		},
		{
			name: "redact nested map",
			input: map[string]any{
				"config": map[string]any{
					"password": "secret",
					"host":     "localhost",
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"password": "[redacted]",
					"host":     "localhost",
				},
			},
		},
		{
			name: "redact map[string]string",
			input: map[string]string{
				"token": "abc123",
				"value": "normal",
			},
			expected: map[string]string{
				"token": "[redacted]",
				"value": "normal",
			},
		},
		{
			name: "redact array",
			input: []any{
				map[string]any{"api_key": "secret"},
				"normal string",
			},
			expected: []any{
				map[string]any{"api_key": "[redacted]"},
				"normal string",
			},
		},
		{
			name:     "non-sensitive value",
			input:    "just a string",
			expected: "just a string",
		},
		{
			name:     "number",
			input:    42,
			expected: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactAny(tt.input)
			// Simple comparison - in production you'd use deep equality
			if !compareValues(result, tt.expected) {
				t.Errorf("redactAny() mismatch:\ngot:  %+v\nwant: %+v", result, tt.expected)
			}
		})
	}
}

func TestRedactSensitiveValue(t *testing.T) {
	input := map[string]any{
		"api_key": "secret123",
		"data":    "public",
	}

	result := redactSensitiveValue(input)
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected result to be map[string]any")
	}

	if resultMap["api_key"] != "[redacted]" {
		t.Errorf("expected api_key to be redacted, got %v", resultMap["api_key"])
	}
	if resultMap["data"] != "public" {
		t.Errorf("expected data to remain public, got %v", resultMap["data"])
	}
}

func TestSanitizePromptLogPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{
			name:      "valid path",
			path:      "prompts.log",
			shouldErr: false,
		},
		{
			name:      "valid absolute path",
			path:      "/tmp/prompts.log",
			shouldErr: false,
		},
		{
			name:      "empty path",
			path:      "",
			shouldErr: true,
		},
		{
			name:      "whitespace path",
			path:      "   ",
			shouldErr: true,
		},
		{
			name:      "root path",
			path:      "/",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizePromptLogPath(tt.path)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error for path %q, got nil", tt.path)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for path %q, got %v", tt.path, err)
				}
				if result == "" {
					t.Errorf("expected non-empty result for valid path %q", tt.path)
				}
			}
		})
	}
}

func TestWriteJSONLine(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "agentkit-logging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := tmpDir + "/test.log"

	// Write some data
	data := map[string]any{
		"message": "test",
		"value":   123,
	}

	err = writeJSONLine(logPath, data)
	if err != nil {
		t.Fatalf("writeJSONLine failed: %v", err)
	}

	// Verify the file was created and contains the data
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !bytes.Contains(content, []byte("test")) {
		t.Error("Expected log file to contain 'test'")
	}
	if !bytes.Contains(content, []byte("123")) {
		t.Error("Expected log file to contain '123'")
	}
}

func TestEnsureLogDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentkit-logging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nestedPath := tmpDir + "/nested/dir/file.log"

	err = ensureLogDir(nestedPath)
	if err != nil {
		t.Fatalf("ensureLogDir failed: %v", err)
	}

	// Verify the directory was created
	dirPath := tmpDir + "/nested/dir"
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Expected directory to be created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected path to be a directory")
	}
}

// Helper function for comparing values in tests
func compareValues(a, b any) bool {
	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for k, v := range va {
			if !compareValues(v, vb[k]) {
				return false
			}
		}
		return true
	case map[string]string:
		vb, ok := b.(map[string]string)
		if !ok || len(va) != len(vb) {
			return false
		}
		for k, v := range va {
			if v != vb[k] {
				return false
			}
		}
		return true
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !compareValues(va[i], vb[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}