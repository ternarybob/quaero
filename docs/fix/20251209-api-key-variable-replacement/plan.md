# Plan: API Key Variable Replacement Fix
Type: fix | Workdir: ./docs/fix/20251209-api-key-variable-replacement/

## User Intent (from manifest)
1. Fix the intermittent API key validation failure where `{google_gemini_api_key}` is not resolved
2. Ensure ALL configuration access uses the centralized variable replacement strategy (checking for `{xxx}` pattern and replacing from .env)
3. Consolidate config access to a single service - no duplicate/custom config parsing scattered in code files
4. The config service should be injected/passed into services that need it

## Root Cause Analysis
The `ValidateAPIKeys` function in `internal/jobs/service.go` receives `api_key = "{google_gemini_api_key}"` from the TOML config. It passes this literal string (with braces) to `ResolveAPIKey()`, which looks for a KV key named `{google_gemini_api_key}` instead of `google_gemini_api_key` (without braces).

The solution is to:
1. Detect if the api_key value contains `{xxx}` pattern
2. If so, extract the key name (without braces) and use that for lookup
3. This aligns with the existing variable replacement strategy

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix ValidateAPIKeys to handle {xxx} pattern | - | no | sonnet |
| 2 | Build and verify fix | 1 | no | sonnet |

## Order
[1] → [2]

## Notes
- The codebase already has proper centralized config via ConfigService
- Variable replacement with `{xxx}` pattern is implemented in `common/replacement.go`
- The issue is specifically that `ValidateAPIKeys` doesn't apply this pattern before validation
- No structural changes needed - just update the validation logic
