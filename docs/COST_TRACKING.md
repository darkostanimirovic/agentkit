# Cost Tracking in AgentKit

## Overview

AgentKit provides **optional cost estimation** for LLM calls with **automatic real-time pricing updates**. It's important to understand what data comes from OpenAI and what we estimate.

## What OpenAI Provides

OpenAI's API responses include **token usage** data:
- `input_tokens` - Number of tokens in the prompt
- `output_tokens` - Number of tokens generated
- `total_tokens` - Sum of input and output tokens
- `cached_tokens` - Tokens served from cache (reduced cost)
- `reasoning_tokens` - Tokens used for reasoning (O1 models)

**OpenAI does NOT provide cost information in API responses.**

## Dynamic Pricing (Default)

AgentKit **automatically fetches real-time model pricing** from `https://models.dev/api.json` on first cost calculation:

```go
// First call triggers async price fetch (non-blocking)
cost := CalculateCost("gpt-4o-mini", 1000, 500)

// Pricing is updated in background, never blocks execution
// Falls back to hardcoded prices if API unavailable
```

### How It Works

1. **First calculation** triggers an asynchronous API call to fetch current prices
2. **Never blocks** - uses a conservative 5-second timeout
3. **Fail-safe** - falls back to hardcoded prices if:
   - API is unreachable
   - Request times out
   - Response is invalid
4. **Cached** - fetched prices are cached for the application lifetime
5. **Refreshable** - can manually refresh with `RefreshModelCosts()`

### Configuration

```go
import "github.com/darkostanimirovic/agentkit"

// Disable dynamic pricing (use hardcoded only)
agentkit.ModelPricingAPIURL = ""

// Change API endpoint
agentkit.ModelPricingAPIURL = "https://your-pricing-api.com/prices"

// Adjust timeout (default: 5 seconds)
agentkit.ModelPricingTimeout = 10 * time.Second

// Manually refresh prices
agentkit.RefreshModelCosts()
```

## What AgentKit Estimates

AgentKit estimates costs using a **three-tier priority system**:

1. **Custom pricing** (via `RegisterModelCost()`) - highest priority
2. **Dynamic pricing** (from models.dev API) - fetched automatically
3. **Fallback pricing** (hardcoded) - used if API unavailable

```go
// Priority 1: Custom pricing (always used if set)
agentkit.RegisterModelCost("gpt-4o", agentkit.ModelCostConfig{
    InputCostPer1MTokens:  5.00,
    OutputCostPer1MTokens: 15.00,
})

// Priority 2: Dynamic pricing (fetched automatically from API)
// Happens in background on first CalculateCost() call

// Priority 3: Fallback pricing (hardcoded defaults)
// Used if API fetch fails or model not found in API
```

## Disabling Cost Calculation

If you don't need cost tracking or prefer to avoid pricing estimates:

```go
import "github.com/darkostanimirovic/agentkit"

func main() {
    // Disable all cost calculations
    agentkit.DisableCostCalculation = true
    
    // Now traces will include usage but not cost
    agent, _ := agentkit.New(agentkit.Config{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "gpt-4o-mini",
        Tracer: langfuseTracer,
    })
}
```

## Custom Pricing

You can register custom pricing for models:

```go
// Update pricing for a model
agentkit.RegisterModelCost("gpt-4o", agentkit.ModelCostConfig{
    InputCostPer1MTokens:  5.00,  // $5 per 1M input tokens
    OutputCostPer1MTokens: 15.00, // $15 per 1M output tokens
})

// Add pricing for a custom model
agentkit.RegisterModelCost("my-custom-model", agentkit.ModelCostConfig{
    InputCostPer1MTokens:  2.00,
    OutputCostPer1MTokens: 8.00,
})
```

## Pricing Accuracy

**Improvements with Dynamic Pricing:**

1. ✅ **Auto-updated** - Fetches current prices from models.dev API
2. ✅ **Non-blocking** - Never slows down agent execution
3. ✅ **Fail-safe** - Falls back to hardcoded prices if API unavailable
4. ✅ **Conservative timeout** - 5 second max, doesn't hang

**Remaining Limitations:**

1. **Cached tokens** - Reduced pricing for cached tokens not yet handled
2. **Volume discounts** - Enterprise agreements not reflected
3. **Regional pricing** - May vary by region
4. **API availability** - Depends on models.dev uptime (has fallback)

**Recommendation:** Dynamic pricing is enabled by default and provides reasonably accurate estimates. For critical billing, always verify actual costs in your OpenAI billing dashboard.

## Pricing Updates

**Default behavior** (dynamic pricing enabled):
- Prices fetched automatically from https://models.dev/api.json
- Updates happen on first cost calculation
- Can manually refresh with `RefreshModelCosts()`

**Fallback pricing** last updated: January 8, 2026

To force fallback pricing only:
```go
agentkit.ModelPricingAPIURL = "" // Disable dynamic pricing
```

## Usage in Traces

When tracing is enabled, both usage and cost (if enabled) are logged:

```go
// In Langfuse/other tracing backends, you'll see:
{
  "usage": {
    "prompt_tokens": 150,
    "completion_tokens": 75,
    "total_tokens": 225
  },
  "cost": {
    "prompt_cost": 0.0000225,      // Estimated
    "completion_cost": 0.0000450,   // Estimated  
    "total_cost": 0.0000675         // Estimated
  }
}
```

If cost calculation is disabled or model is unknown, cost will be `null`:

```json
{
  "usage": {
    "prompt_tokens": 150,
    "completion_tokens": 75,
    "total_tokens": 225
  },
  "cost": null
}
```

## Best Practices

1. **Always verify costs** in your OpenAI billing dashboard
2. **Set budgets** based on actual billing, not estimates
3. **Update pricing regularly** if you rely on cost tracking
4. **Consider disabling** cost calculation if you don't need it
5. **Use usage data** (tokens) as the source of truth for consumption tracking
