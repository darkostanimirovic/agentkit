package agentkit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// ModelCostConfig defines the pricing for a specific model
type ModelCostConfig struct {
	InputCostPer1MTokens  float64 // Cost per 1M input tokens in USD
	OutputCostPer1MTokens float64 // Cost per 1M output tokens in USD
}

// DefaultModelCosts provides FALLBACK pricing for common OpenAI models.
// These are used when:
// 1. Dynamic pricing is disabled (ModelPricingAPIURL = "")
// 2. API fetch fails or times out
// 3. Model not found in API response
//
// By default, AgentKit fetches real-time pricing from models.dev API automatically.
// The API fetch is non-blocking and has a conservative timeout (5 seconds).
//
// Priority order for pricing:
// 1. Custom pricing (via RegisterModelCost) - highest priority
// 2. Dynamic pricing (from API) - fetched automatically
// 3. Fallback pricing (below) - used if API unavailable
//
// Fallback prices last updated: January 8, 2026
// Source: https://openai.com/api/pricing/
var DefaultModelCosts = map[string]ModelCostConfig{
	// GPT-5.2 (gpt-5.2)
	"gpt-5.2": {
		InputCostPer1MTokens:  2.50,
		OutputCostPer1MTokens: 10.00,
	},
	// GPT-4o
	"gpt-4o": {
		InputCostPer1MTokens:  5.00,
		OutputCostPer1MTokens: 15.00,
	},
	"gpt-4o-2024-11-20": {
		InputCostPer1MTokens:  2.50,
		OutputCostPer1MTokens: 10.00,
	},
	// GPT-4o-mini
	"gpt-4o-mini": {
		InputCostPer1MTokens:  0.150,
		OutputCostPer1MTokens: 0.600,
	},
	"gpt-4o-mini-2024-07-18": {
		InputCostPer1MTokens:  0.150,
		OutputCostPer1MTokens: 0.600,
	},
	// GPT-4 Turbo
	"gpt-4-turbo": {
		InputCostPer1MTokens:  10.00,
		OutputCostPer1MTokens: 30.00,
	},
	"gpt-4-turbo-2024-04-09": {
		InputCostPer1MTokens:  10.00,
		OutputCostPer1MTokens: 30.00,
	},
	// GPT-5 variants
	"gpt-5.1-codex-max": {
		InputCostPer1MTokens:  2.50,
		OutputCostPer1MTokens: 10.00,
	},
}

// DisableCostCalculation can be set to true to skip all cost calculations.
// This is useful if you don't need cost tracking or want to avoid outdated pricing estimates.
var DisableCostCalculation = false

// ModelPricingAPIURL is the endpoint for fetching real-time model pricing
// Set to empty string to disable dynamic price fetching
var ModelPricingAPIURL = "https://models.dev/api.json"

// ModelPricingTimeout is the timeout for fetching model prices from the API
var ModelPricingTimeout = 5 * time.Second

var (
	// dynamicModelCosts stores costs fetched from the API
	dynamicModelCosts = make(map[string]ModelCostConfig)
	// costsMutex protects concurrent access to dynamicModelCosts
	costsMutex sync.RWMutex
	// costsFetched tracks whether we've attempted to fetch costs
	costsFetched = false
	// fetchOnce ensures we only fetch costs once
	fetchOnce sync.Once
)

// modelsAPIResponse represents the response structure from models.dev API
type modelsAPIResponse struct {
	// The API returns model names as keys with nested pricing data
	// We'll parse this dynamically
	Models map[string]modelPricing `json:"-"`
}

type modelPricing struct {
	InputCost  float64 `json:"input_cost_per_million"`
	OutputCost float64 `json:"output_cost_per_million"`
}

// UnmarshalJSON handles the dynamic structure of the models.dev API
func (r *modelsAPIResponse) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Models = make(map[string]modelPricing)
	for modelName, modelData := range raw {
		var pricing struct {
			Input  float64 `json:"input"`
			Output float64 `json:"output"`
		}
		
		// Try to parse the pricing data
		if err := json.Unmarshal(modelData, &pricing); err != nil {
			continue // Skip models we can't parse
		}

		if pricing.Input > 0 || pricing.Output > 0 {
			r.Models[modelName] = modelPricing{
				InputCost:  pricing.Input,
				OutputCost: pricing.Output,
			}
		}
	}

	return nil
}

// fetchModelCosts fetches real-time model costs from the API
// This runs asynchronously and never blocks agent execution
func fetchModelCosts() {
	if ModelPricingAPIURL == "" {
		return // Disabled
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), ModelPricingTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", ModelPricingAPIURL, nil)
		if err != nil {
			return // Fail silently - will use fallback costs
		}

		client := &http.Client{
			Timeout: ModelPricingTimeout,
		}

		resp, err := client.Do(req)
		if err != nil {
			return // Fail silently - will use fallback costs
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return // Fail silently
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // Max 1MB
		if err != nil {
			return
		}

		var apiResp modelsAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return // Fail silently
		}

		// Update dynamic costs
		costsMutex.Lock()
		for model, pricing := range apiResp.Models {
			dynamicModelCosts[model] = ModelCostConfig{
				InputCostPer1MTokens:  pricing.InputCost,
				OutputCostPer1MTokens: pricing.OutputCost,
			}
		}
		costsFetched = true
		costsMutex.Unlock()
	}()
}

// getModelCost retrieves cost config for a model, checking dynamic costs first, then fallback
func getModelCost(model string) (ModelCostConfig, bool) {
	// Trigger async fetch on first call (non-blocking)
	fetchOnce.Do(fetchModelCosts)

	// Check dynamic costs first (if available)
	costsMutex.RLock()
	if cost, exists := dynamicModelCosts[model]; exists {
		costsMutex.RUnlock()
		return cost, true
	}
	costsMutex.RUnlock()

	// Fallback to hardcoded costs
	if cost, exists := DefaultModelCosts[model]; exists {
		return cost, true
	}

	return ModelCostConfig{}, false
}

// CalculateCost calculates the ESTIMATED cost of an LLM call based on token usage.
// Returns nil if:
// - Cost calculation is disabled (DisableCostCalculation = true)
// - No tokens were used
// - Model pricing is unknown
//
// NOTE: OpenAI's API provides usage (tokens) but NOT cost. This function estimates
// cost based on:
// 1. Real-time pricing from models.dev API (if available and fetched)
// 2. Fallback to hardcoded pricing (if API unavailable)
// 
// The API fetch happens asynchronously and never blocks execution.
func CalculateCost(model string, promptTokens, completionTokens int) *CostInfo {
	if DisableCostCalculation {
		return nil
	}

	if promptTokens == 0 && completionTokens == 0 {
		return nil
	}

	// Get model cost config (checks dynamic costs first, then fallback)
	costConfig, exists := getModelCost(model)
	if !exists {
		// Unknown model - return nil to indicate cost couldn't be calculated
		return nil
	}

	// Calculate costs (convert from per-1M to per-token, then multiply)
	promptCost := float64(promptTokens) * costConfig.InputCostPer1MTokens / 1_000_000.0
	completionCost := float64(completionTokens) * costConfig.OutputCostPer1MTokens / 1_000_000.0
	totalCost := promptCost + completionCost

	return &CostInfo{
		PromptCost:     promptCost,
		CompletionCost: completionCost,
		TotalCost:      totalCost,
	}
}

// RegisterModelCost registers a custom model cost configuration
// This takes precedence over both API-fetched and default pricing
func RegisterModelCost(model string, config ModelCostConfig) {
	costsMutex.Lock()
	defer costsMutex.Unlock()
	dynamicModelCosts[model] = config
}

// RefreshModelCosts triggers a fresh fetch of model costs from the API
// This is useful if you want to update prices without restarting the application
func RefreshModelCosts() {
	if ModelPricingAPIURL == "" {
		return
	}
	
	// Reset the fetch state to allow re-fetching
	costsMutex.Lock()
	costsFetched = false
	costsMutex.Unlock()
	
	// Trigger a new fetch
	fetchModelCosts()
}
