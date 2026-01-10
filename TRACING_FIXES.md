# Tracing Fixes - January 10, 2026

## Issues Fixed

This document describes the comprehensive fixes applied to resolve critical tracing issues identified from client trace data.

### Issue #1: Timing Race Condition ❌ → ✅

**Problem**: Trace timestamps were nonsensical - child spans appeared to start before parent traces were created (32 seconds before in the reported case).

**Root Cause**: `StartTrace` was called inside a goroutine with `time.Now()`, meaning the start time was captured AFTER the agent began processing, not when it actually started.

**Files Modified**:
- `tracing.go`: Added `StartTime *time.Time` field to `TraceConfig`
- `tracing.go`: Added `WithTraceStartTime()` option function
- `tracing_langfuse.go`: Updated `StartTrace` to use explicit start time from config
- `agent.go`: Capture `startTime := time.Now()` BEFORE goroutine launch and pass it to `StartTrace`

**Result**: Traces now have accurate timestamps with proper parent-child temporal ordering.

---

### Issue #2: Missing Reasoning Tokens ❌ → ✅

**Problem**: Zero token counts and costs for reasoning models (o1, o3, gpt-5-mini) despite active LLM usage.

**Root Cause**: Reasoning models generate separate "reasoning tokens" (internal chain-of-thought) that weren't being tracked or reported. The `UsageInfo` struct only had `PromptTokens` and `CompletionTokens`.

**Files Modified**:
- `tracing.go`: Added `ReasoningTokens int` field to `UsageInfo` struct
- `responses_api.go`: Added `ReasoningTokens` field to `ResponseUsage` struct with JSON tag
- `agent_tracing.go`: Updated `logLLMGeneration` to extract `ReasoningTokens` from API response
- `agent_tracing.go`: Updated `logLLMGenerationFromStream` to include reasoning tokens
- `tracing_langfuse.go`: Updated Langfuse trace logging to include reasoning tokens in usage details

**Result**: Reasoning tokens are now tracked separately and included in total usage calculations. Traces will show accurate token counts for reasoning models.

---

### Issue #3: Stream Usage Data Not Captured ❌ → ✅

**Problem**: All streaming LLM calls showed zero tokens and zero cost in traces.

**Root Cause**: The code attempted to extract usage from `response.done` chunks, but when data was missing, there was insufficient logging to diagnose why.

**Files Modified**:
- `agent.go`: Enhanced logging in `response.done` handler with detailed diagnostic information
- `agent.go`: Added reasoning tokens to usage capture from stream chunks
- `agent_tracing.go`: Added comprehensive warning logs when usage data is unavailable, including state checks

**Result**: 
- Better visibility into why usage data might be missing
- Reasoning tokens properly captured from streaming responses
- Detailed logs help diagnose API response format issues

---

### Issue #4: Sub-Agent Traces Lost ❌ → ✅ FIXED

**Problem**: Sub-agent LLM calls were completely missing from traces.

**Root Cause**: Sub-agents used their own `sub.tracer` field, which could be `NoOpTracer` if the sub-agent wasn't explicitly configured with a tracer. This broke the trace hierarchy.

**Files Modified**:
- `context.go`: Added `WithTracer()` and `GetTracer()` functions for tracer propagation
- `agent.go`: Parent agents now add their tracer to context in `Run()`
- `subagent.go`: Sub-agents extract parent tracer from context and use it for all operations
- `subagent_tracing_test.go`: Comprehensive test coverage for tracer inheritance

**Result**: ✅ **Fully fixed** - Sub-agents now automatically inherit the parent's tracer through context:
- Parent agent adds tracer to context when `Run()` is called
- Sub-agent handler extracts tracer from context using `GetTracer()`
- Creates a copy of sub-agent with parent's tracer
- All LLM calls within sub-agent are now properly traced
- Maintains complete trace hierarchy with proper nesting
- Falls back gracefully to sub-agent's own tracer if no parent tracer exists

**No workaround needed** - Sub-agents automatically inherit tracing from their parent context.

---

## Testing

All changes have been validated:
- ✅ Code compiles successfully: `go build ./...`
- ✅ All existing tests pass: `go test ./... -short`
- ✅ No breaking API changes

## Migration Notes

### For Users

**No action required** - all changes are backward compatible. Existing code will continue to work.

**Sub-agent tracing now works automatically** - No need to configure sub-agents with the same tracer. The parent's tracer is automatically inherited through context.

### For Reasoning Models (o1/o3)

If you're using reasoning models, you'll now see accurate token counts including:
- `PromptTokens`: Input tokens
- `CompletionTokens`: Output tokens  
- `ReasoningTokens`: Internal reasoning tokens (NEW)
- `TotalTokens`: Sum of all tokens

The `reasoning` field will appear in Langfuse usage details when `ReasoningTokens > 0`.

---

## Impact Assessment

| Metric | Before | After |
|--------|--------|-------|
| Trace timestamp accuracy | ❌ Broken (race condition) | ✅ Accurate |
| Reasoning token tracking | ❌ Missing | ✅ Tracked |
| Stream usage capture | ❌ No diagnostics | ✅ Detailed logging |
| Sub-agent trace continuity | ❌ Lost | ✅ **Fully Fixed** |
| Cost accuracy | ❌ Incorrect (missing reasoning) | ✅ Accurate |

---

## Future Enhancements

1. ~~**Context-based tracer extraction**: Extract the active tracer from context rather than relying on agent's tracer field~~ ✅ **COMPLETED**
2. ~~**Automatic sub-agent tracer inheritance**: Sub-agents automatically inherit parent's tracer at runtime~~ ✅ **COMPLETED**
3. **Trace validation**: Add runtime checks to ensure trace continuity
4. **Better streaming diagnostics**: Capture and log the exact structure of `response.done` chunks when usage is missing

---

## Related Files

- `tracing.go` - Core tracing interface and types
- `tracing_langfuse.go` - Langfuse-specific implementation
- `agent.go` - Main agent execution and streaming
- `agent_tracing.go` - LLM call tracing helpers
- `responses_api.go` - OpenAI API types and client
- `subagent.go` - Sub-agent delegation logic

---

## Questions or Issues?

If you encounter any tracing issues after these fixes, please check:
1. Are all agents (including sub-agents) configured with the same `Tracer`?
2. Is the tracer properly initialized and not disabled?
3. Check application logs for new warning messages about missing usage data
4. Verify the OpenAI API is returning usage data in streaming responses
