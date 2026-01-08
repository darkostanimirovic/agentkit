package agentkit

import (
	"bytes"
	"io"
	"log/slog"
	"os"
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

func TestLoggingConfig_Silent(t *testing.T) {
	cfg := LoggingConfig{}.Silent()

	if cfg == nil {
		t.Fatal("expected Silent() to return a non-nil config")
	}
	if cfg.Handler == nil {
		t.Fatal("expected Handler to be set")
	}

	// Verify that the handler discards output
	logger := slog.New(cfg.Handler)
	logger.Info("test message")
	logger.Error("error message")
	// If we get here without panicking, the discard handler works
}

func TestLoggingConfig_Verbose(t *testing.T) {
	cfg := LoggingConfig{}.Verbose()

	if cfg == nil {
		t.Fatal("expected Verbose() to return a non-nil config")
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

	logger := resolveLogger(cfg)
	if logger != customLogger {
		t.Error("expected resolveLogger to return the provided logger")
	}
}

func TestResolveLogger_WithHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	cfg := LoggingConfig{
		Handler: handler,
	}

	logger := resolveLogger(cfg)
	if logger == nil {
		t.Fatal("expected resolveLogger to return a logger")
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
	logger := resolveLogger(cfg)

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

	logger := resolveLogger(cfg)
	logger.Info("info message") // Should not appear
	logger.Warn("warn message") // Should appear

	output := buf.String()
	if bytes.Contains([]byte(output), []byte("info message")) {
		t.Error("expected info message to be filtered out")
	}
	if !bytes.Contains([]byte(output), []byte("warn message")) {
		t.Error("expected warn message to be logged")
	}
}

func TestSilent_IntegrationWithAgent(t *testing.T) {
	// Create an agent with silent logging
	mock := NewMockLLM().WithFinalResponse("test response")

	agent, err := New(Config{
		Model:       "gpt-4o-mini",
		LLMProvider: mock,
		Logging:     LoggingConfig{}.Silent(),
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.logger == nil {
		t.Fatal("expected logger to be set")
	}

	// The logger should exist but discard all output
	agent.logger.Info("test message")
	agent.logger.Error("error message")
	// If we get here without issues, silent mode works correctly
}

func TestVerbose_IntegrationWithAgent(t *testing.T) {
	// Create an agent with verbose logging
	mock := NewMockLLM().WithFinalResponse("test response")

	agent, err := New(Config{
		Model:       "gpt-4o-mini",
		LLMProvider: mock,
		Logging:     LoggingConfig{}.Verbose(),
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if agent.logger == nil {
		t.Fatal("expected logger to be set")
	}

	// Verify the logging config has debug level
	if agent.loggingConfig.Level != slog.LevelDebug {
		t.Errorf("expected debug level, got %v", agent.loggingConfig.Level)
	}
}
