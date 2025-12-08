# Task 1: Fix ValidateAPIKeys to handle {xxx} pattern
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Fixes the intermittent API key validation failure by properly resolving `{xxx}` variable placeholders before validation.

## Do
1. Modify `ValidateAPIKeys` in `internal/jobs/service.go` to:
   - Check if the api_key value matches the `{key-name}` pattern
   - If so, extract the key name (without braces) and use that for lookup
   - This aligns with the existing `keyRefPattern` regex from `common/replacement.go`

## Accept
- [ ] `{google_gemini_api_key}` is correctly resolved to `google_gemini_api_key` for lookup
- [ ] API key validation passes when the key exists in KV store
- [ ] Code compiles without errors
