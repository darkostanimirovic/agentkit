package parallel

import (
	"testing"
)

func TestDefaultParallelConfig(t *testing.T) {
	cfg := DefaultParallelConfig()

	if cfg.Enabled {
		t.Error("expected Enabled=false")
	}
	if cfg.MaxConcurrent != 1 {
		t.Errorf("expected MaxConcurrent=1, got %d", cfg.MaxConcurrent)
	}
}
