# Cost Tracking Example

This example demonstrates AgentKit's dynamic cost tracking features.

## Features Demonstrated

1. **Dynamic Pricing** - Automatic price fetching from models.dev API
2. **Custom Pricing** - Override with your own pricing
3. **Fallback Pricing** - Uses hardcoded prices if API unavailable
4. **Non-blocking** - Never slows down agent execution
5. **Fail-safe** - Conservative timeout and graceful degradation

## Running the Example

```bash
export OPENAI_API_KEY=your-key-here
go run main.go
```

## Key Concepts

### Automatic Price Fetching

On first cost calculation, AgentKit automatically fetches current model prices:

```go
// This triggers async price fetch (non-blocking)
cost := agentkit.CalculateCost("gpt-4o-mini", 1000, 500)
```

### Priority System

AgentKit uses a three-tier pricing system:

1. **Custom pricing** (highest priority)
   ```go
   agentkit.RegisterModelCost("my-model", agentkit.ModelCostConfig{
       InputCostPer1MTokens:  1.0,
       OutputCostPer1MTokens: 2.0,
   })
   ```

2. **Dynamic pricing** (from API)
   - Fetched automatically from https://models.dev/api.json
   - Cached for application lifetime
   - 5-second timeout

3. **Fallback pricing** (hardcoded)
   - Used if API fails or times out
   - Maintained in code as backup

### Configuration Options

```go
// Disable dynamic pricing
agentkit.ModelPricingAPIURL = ""

// Change timeout
agentkit.ModelPricingTimeout = 10 * time.Second

// Disable all cost calculation
agentkit.DisableCostCalculation = true

// Manually refresh prices
agentkit.RefreshModelCosts()
```

## Non-blocking Behavior

The price fetching is completely non-blocking:

```go
// First call starts async fetch
cost1 := agentkit.CalculateCost("gpt-4o", 1000, 500)
// Returns immediately using fallback prices

// Subsequent calls use fetched prices (if available)
cost2 := agentkit.CalculateCost("gpt-4o", 1000, 500)
// Uses dynamic prices if fetch completed
```

## Fail-safe Behavior

- **Network failure**: Uses fallback prices
- **Timeout (5s)**: Uses fallback prices
- **Invalid response**: Uses fallback prices
- **API unavailable**: Uses fallback prices

No matter what happens with the API, cost calculation never fails or blocks.

## See Also

- [Cost Tracking Documentation](../../docs/COST_TRACKING.md)
- [Main AgentKit README](../../README.md)
