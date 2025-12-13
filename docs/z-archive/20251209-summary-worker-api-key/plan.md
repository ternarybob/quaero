# Plan: Fix Summary Worker API Key

Type: fix | Workdir: ./docs/fix/20251209-summary-worker-api-key/

## User Intent (from manifest)

Fix the Codebase Assessment Pipeline so the summary steps (generate_index, generate_summary, generate_map) have access to the Gemini API key.

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add api_key to summary steps in codebase_assess.toml | - | no | sonnet |

## Order

[1]

## Analysis

The codebase_assess.toml has three summary steps that all fail with the same error:
- `generate_index` (step 6/9)
- `generate_summary` (step 8/9)
- `generate_map` (step 9/9)

Each step uses `type = "summary"` which invokes the `SummaryWorker`. Looking at `summary_worker.go:98-122`, the worker requires `api_key` in step config.

The fix is simple: add `api_key = "{google_gemini_api_key}"` to each summary step. The variable will be resolved from `.env` which already has `google_gemini_api_key` defined.
