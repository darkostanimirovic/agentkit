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
