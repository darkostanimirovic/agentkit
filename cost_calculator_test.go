package agentkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCalculateCost_WithDynamicPricing(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"test-model": map[string]float64{
				"input":  1.0,
				"output": 2.0,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Configure to use test server
	originalURL := ModelPricingAPIURL
	ModelPricingAPIURL = server.URL
	defer func() { ModelPricingAPIURL = originalURL }()

	// Trigger fetch
	fetchModelCosts()
	time.Sleep(100 * time.Millisecond) // Wait for async fetch

	// Calculate cost
	cost := CalculateCost("test-model", 1000, 500)
	if cost == nil {
		t.Fatal("Expected cost to be calculated")
	}

	// Verify costs
	expectedPromptCost := 1000.0 * 1.0 / 1_000_000.0
	expectedCompletionCost := 500.0 * 2.0 / 1_000_000.0
	expectedTotal := expectedPromptCost + expectedCompletionCost

	if cost.PromptCost != expectedPromptCost {
		t.Errorf("Expected prompt cost %.6f, got %.6f", expectedPromptCost, cost.PromptCost)
	}
	if cost.CompletionCost != expectedCompletionCost {
		t.Errorf("Expected completion cost %.6f, got %.6f", expectedCompletionCost, cost.CompletionCost)
	}
	if cost.TotalCost != expectedTotal {
		t.Errorf("Expected total cost %.6f, got %.6f", expectedTotal, cost.TotalCost)
	}
}

func TestCalculateCost_FallbackToDefault(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Disable API
	originalURL := ModelPricingAPIURL
	ModelPricingAPIURL = ""
	defer func() { ModelPricingAPIURL = originalURL }()

	// Test with default model
	cost := CalculateCost("gpt-4o-mini", 1000, 500)
	if cost == nil {
		t.Fatal("Expected cost to be calculated from defaults")
	}

	// Should use default pricing
	if cost.TotalCost <= 0 {
		t.Error("Expected non-zero cost from defaults")
	}
}

func TestCalculateCost_APITimeout(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	originalURL := ModelPricingAPIURL
	originalTimeout := ModelPricingTimeout
	ModelPricingAPIURL = server.URL
	ModelPricingTimeout = 100 * time.Millisecond
	defer func() {
		ModelPricingAPIURL = originalURL
		ModelPricingTimeout = originalTimeout
	}()

	// Should not block
	start := time.Now()
	fetchModelCosts()
	elapsed := time.Since(start)

	// Should return immediately (not wait for timeout)
	if elapsed > 50*time.Millisecond {
		t.Errorf("fetchModelCosts blocked for %v, should be non-blocking", elapsed)
	}

	// Should still be able to calculate with fallback
	cost := CalculateCost("gpt-4o-mini", 1000, 500)
	if cost == nil {
		t.Fatal("Expected fallback cost calculation")
	}
}

func TestCalculateCost_InvalidAPIResponse(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Create server with invalid response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	originalURL := ModelPricingAPIURL
	ModelPricingAPIURL = server.URL
	defer func() { ModelPricingAPIURL = originalURL }()

	fetchModelCosts()
	time.Sleep(100 * time.Millisecond)

	// Should fallback to defaults
	cost := CalculateCost("gpt-4o-mini", 1000, 500)
	if cost == nil {
		t.Fatal("Expected fallback cost calculation")
	}
}

func TestCalculateCost_API404(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Create server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalURL := ModelPricingAPIURL
	ModelPricingAPIURL = server.URL
	defer func() { ModelPricingAPIURL = originalURL }()

	fetchModelCosts()
	time.Sleep(100 * time.Millisecond)

	// Should fallback to defaults
	cost := CalculateCost("gpt-4o-mini", 1000, 500)
	if cost == nil {
		t.Fatal("Expected fallback cost calculation")
	}
}

func TestRegisterModelCost_TakesPrecedence(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	costsFetched = false
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Register custom cost
	RegisterModelCost("custom-model", ModelCostConfig{
		InputCostPer1MTokens:  10.0,
		OutputCostPer1MTokens: 20.0,
	})

	// Calculate
	cost := CalculateCost("custom-model", 1000, 500)
	if cost == nil {
		t.Fatal("Expected cost to be calculated")
	}

	expectedPromptCost := 1000.0 * 10.0 / 1_000_000.0
	expectedCompletionCost := 500.0 * 20.0 / 1_000_000.0

	if cost.PromptCost != expectedPromptCost {
		t.Errorf("Expected prompt cost %.6f, got %.6f", expectedPromptCost, cost.PromptCost)
	}
	if cost.CompletionCost != expectedCompletionCost {
		t.Errorf("Expected completion cost %.6f, got %.6f", expectedCompletionCost, cost.CompletionCost)
	}
}

func TestCalculateCost_DisabledCalculation(t *testing.T) {
	originalValue := DisableCostCalculation
	DisableCostCalculation = true
	defer func() { DisableCostCalculation = originalValue }()

	cost := CalculateCost("gpt-4o-mini", 1000, 500)
	if cost != nil {
		t.Error("Expected nil when cost calculation is disabled")
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := CalculateCost("gpt-4o-mini", 0, 0)
	if cost != nil {
		t.Error("Expected nil for zero tokens")
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	// Reset state
	costsMutex.Lock()
	dynamicModelCosts = make(map[string]ModelCostConfig)
	fetchOnce = sync.Once{}
	costsMutex.Unlock()

	// Disable API
	originalURL := ModelPricingAPIURL
	ModelPricingAPIURL = ""
	defer func() { ModelPricingAPIURL = originalURL }()

	cost := CalculateCost("unknown-model-xyz", 1000, 500)
	if cost != nil {
		t.Error("Expected nil for unknown model")
	}
}
