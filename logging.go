package agentkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const defaultPromptLogPath = "agent-prompts.log"

// LoggingConfig configures logging behavior for AgentKit.
type LoggingConfig struct {
	// Logger overrides the logger used by AgentKit if provided.
	Logger *slog.Logger

	// Handler is used to build a logger if Logger is nil.
	Handler slog.Handler

	// Level is used when creating a default handler if Logger and Handler are nil.
	Level slog.Level

	// LogPrompts enables prompt logging to file.
	LogPrompts bool

	// LogResponses enables logging LLM response summaries.
	LogResponses bool

	// LogToolCalls enables logging tool call summaries.
	LogToolCalls bool

	// RedactSensitive enables best-effort redaction of sensitive fields in logs.
	RedactSensitive bool

	// PromptLogPath overrides the prompt log file path.
	PromptLogPath string
}

// DefaultLoggingConfig returns default logging configuration.
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:        slog.LevelInfo,
		LogPrompts:   true,
		LogResponses: false,
		LogToolCalls: false,
	}
}

func resolveLogger(cfg LoggingConfig) *slog.Logger {
	if cfg.Logger != nil {
		return cfg.Logger
	}
	if cfg.Handler != nil {
		return slog.New(cfg.Handler)
	}

	level := cfg.Level
	if level == 0 {
		level = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func resolvePromptLogPath(cfg LoggingConfig) string {
	if strings.TrimSpace(cfg.PromptLogPath) != "" {
		return cfg.PromptLogPath
	}
	return defaultPromptLogPath
}

var sensitiveKeys = map[string]struct{}{
	"api_key":        {},
	"apikey":         {},
	"authorization":  {},
	"token":          {},
	"password":       {},
	"secret":         {},
	"access_token":   {},
	"refresh_token":  {},
	"client_secret":  {},
	"private_key":    {},
	"session_token":  {},
	"bearer":         {},
	"x-api-key":      {},
	"openai_api_key": {},
}

func redactSensitiveValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return value
	}

	return redactAny(decoded)
}

func redactAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(v))
		for key, val := range v {
			if isSensitiveKey(key) {
				redacted[key] = "[redacted]"
				continue
			}
			redacted[key] = redactAny(val)
		}
		return redacted
	case map[string]string:
		redacted := make(map[string]string, len(v))
		for key, val := range v {
			if isSensitiveKey(key) {
				redacted[key] = "[redacted]"
				continue
			}
			redacted[key] = val
		}
		return redacted
	case []any:
		redacted := make([]any, len(v))
		for i, item := range v {
			redacted[i] = redactAny(item)
		}
		return redacted
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	_, ok := sensitiveKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func writeJSONLine(path string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	safePath, err := sanitizePromptLogPath(path)
	if err != nil {
		return err
	}

	if err := ensureLogDir(safePath); err != nil {
		return err
	}

	file, err := os.OpenFile(safePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G304 -- path sanitized by sanitizePromptLogPath
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

func sanitizePromptLogPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("prompt log path is empty")
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("resolve prompt log path: %w", err)
	}
	if absPath == string(filepath.Separator) || absPath == "." {
		return "", errors.New("prompt log path is invalid")
	}

	return absPath, nil
}

func ensureLogDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create prompt log directory: %w", err)
	}
	return nil
}
