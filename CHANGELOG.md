# Changelog

## [Unreleased]

### Fixed

#### Complete Tracing System Overhaul (2026-01-10)

Fixed four critical tracing issues that were causing incomplete and inaccurate trace data:

1. **Timing Race Condition** - Traces now have accurate timestamps
   - Fixed race condition where child spans could start before parent trace timestamp
   - Capture start time before goroutine launch
   - Pass explicit start time to `StartTrace()` via new `WithTraceStartTime()` option
   
2. **Reasoning Token Tracking** - Reasoning models (o1/o3) now properly tracked
   - Added `ReasoningTokens` field to `UsageInfo` and `ResponseUsage` structs
   - Reasoning tokens now included in total usage calculations
   - Cost tracking now accurate for reasoning models
   
3. **Stream Usage Data** - Better diagnostics for missing token data
   - Enhanced logging in `response.done` handler
   - Detailed diagnostics when usage data is unavailable
   - Improved debugging for API response format issues
   
4. **Sub-Agent Tracing** - Sub-agents now automatically inherit parent's tracer ✨
   - Added `WithTracer()` and `GetTracer()` to `context.go`
   - Parent agents add tracer to context in `Run()`
   - Sub-agents extract and use parent's tracer automatically
   - Complete trace continuity through agent hierarchies
   - **No configuration needed** - works automatically

**Impact:**
- Traces now show accurate timestamps with proper parent-child ordering
- Complete token counts including reasoning tokens for o1/o3 models
- Sub-agent LLM calls now properly traced and nested
- Accurate cost tracking across all models
- Detailed diagnostics for troubleshooting

**Files Changed:**
- `tracing.go` - Added `StartTime` to `TraceConfig`, `ReasoningTokens` to `UsageInfo`
- `tracing_langfuse.go` - Use explicit start time, include reasoning tokens
- `responses_api.go` - Added `ReasoningTokens` to `ResponseUsage`
- `agent.go` - Capture start time, add tracer to context, enhanced logging
- `agent_tracing.go` - Extract reasoning tokens, improved diagnostics
- `context.go` - Added `WithTracer()` and `GetTracer()` functions
- `subagent.go` - Inherit parent tracer from context
- `tracing_fixes_test.go` - Tests for timing and reasoning tokens
- `subagent_tracing_test.go` - Comprehensive sub-agent tracing tests

**Migration Notes:**
- All changes are backward compatible
- No code changes required for existing applications
- Sub-agent tracing works automatically (no manual tracer configuration needed)

**Testing:**
- All existing tests pass
- New tests verify timing accuracy
- New tests verify reasoning token tracking
- Comprehensive sub-agent tracer inheritance tests

See [TRACING_FIXES.md](TRACING_FIXES.md) for detailed technical documentation.

---

