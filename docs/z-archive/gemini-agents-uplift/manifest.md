# Gemini Agents Uplift - Manifest

## Classification
**Type:** CODE_CHANGE
**Complexity:** MEDIUM
**Estimated Tasks:** 4-6

## Problem Statement
Agent jobs have inconsistent Gemini API configurations:
- `agent-document-generator.toml` and `agent-web-enricher.toml` have `api_key = "{google_gemini_api_key}"` in `[steps.config]` but jobs are disabled
- `keyword-extractor-agent.toml` has no api_key listed but job is enabled

The current implementation resolves the API key in `agent_manager.go` but never passes it to the agent execution layer.

## Requirements
An agent type job can either:
1. Use the global API key (default behavior)
2. Override with job-specific API key from `[steps.config]`

The same pattern applies to model and other Gemini settings.

## Key Files
- `internal/queue/managers/agent_manager.go` - Resolves API key but doesn't propagate
- `internal/queue/workers/agent_worker.go` - Executes agent jobs
- `internal/services/agents/service.go` - Creates genai client with global API key
- `bin/job-definitions/agent-document-generator.toml`
- `bin/job-definitions/agent-web-enricher.toml`
- `bin/job-definitions/keyword-extractor-agent.toml`

## Analysis Summary
1. `agent_manager.go` already resolves API key from step config (lines 66-81)
2. Resolved key stored in `stepConfig["resolved_api_key"]` but NOT passed to child jobs
3. `agent_worker.go` doesn't check for job-specific API key
4. `agents/service.go` creates single genai client at initialization with global key

## Solution Approach
1. Pass resolved API key (and other Gemini settings) from manager to child jobs via job config
2. Modify agent worker to extract and use job-specific settings
3. Modify agent service to support per-request API key override
4. Standardize job definition configurations
