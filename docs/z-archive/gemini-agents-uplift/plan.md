# Gemini Agents Uplift - Implementation Plan

## Status: COMPLETED

## Overview
Enable agent jobs to use either global API key or override with job-specific API key from `[steps.config]`. The same pattern applies to model and other Gemini settings.

## Implementation Summary

### Changes Made

1. **agent_manager.go** - Pass Gemini settings to child jobs
   - In `createAgentJob()`, now copies `resolved_api_key`, `model`, `timeout`, `rate_limit` from stepConfig to jobConfig
   - Settings are prefixed with `gemini_` in the job config

2. **agent_worker.go** - Extract and forward settings
   - In `Execute()`, extracts `gemini_api_key`, `gemini_model`, `gemini_timeout`, `gemini_rate_limit` from job config
   - Passes these in the agentInput map to the agent service

3. **agents/service.go** - Support per-request overrides
   - Added `clientCache` map to cache genai clients by API key
   - Modified `Execute()` to check for override settings in input
   - Added `getOrCreateClient()` for thread-safe client caching
   - Overrides are removed from input before passing to agent executor

### Configuration Pattern

Jobs can now optionally specify Gemini settings in `[steps.config]`:

```toml
[steps.config]
agent_type = "keyword_extractor"

# Optional: Override global API key (resolved from KV store)
api_key = "{google_gemini_api_key}"

# Optional: Override global model
model = "gemini-2.0-flash"

# Optional: Override timeout
timeout = "5m"

# Optional: Override rate limit
rate_limit = "2s"
```

If not specified, the global settings from the agent service initialization are used.

## Job Definition Status

| Job | API Key | Status | Notes |
|-----|---------|--------|-------|
| keyword-extractor-agent | Global | enabled=true | Uses global key |
| agent-document-generator | {google_gemini_api_key} | enabled=false | Example job |
| agent-web-enricher | {google_search_api_key} | enabled=false | Uses different key for web search |

## Testing
- Build passes: `go build ./...`
- Go vet passes: `go vet ./internal/queue/... ./internal/services/agents/...`
- Queue worker tests pass: `go test ./internal/queue/...`
