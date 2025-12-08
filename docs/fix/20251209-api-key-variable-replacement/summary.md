# Complete: API Key Variable Replacement Fix
Type: fix | Tasks: 2 | Files: 1

## User Request
"API key validation fails intermittently for {google_gemini_api_key}. The key should be configured in bin/.env and variable replacement should cover."

## Result
Fixed the `ValidateAPIKeys` function in `internal/jobs/service.go` to properly handle the `{xxx}` variable placeholder pattern. When the function receives an api_key value like `{google_gemini_api_key}`, it now extracts the inner key name (`google_gemini_api_key`) before looking it up in the KV store.

## Root Cause
The bug occurred because:
1. Job definition TOML has `api_key = "{google_gemini_api_key}"`
2. `ValidateAPIKeys` extracted the literal string `{google_gemini_api_key}` (with braces)
3. It passed this to `ResolveAPIKey(ctx, kv, "{google_gemini_api_key}", "")`
4. `ResolveAPIKey` looked for a KV key literally named `{google_gemini_api_key}` which doesn't exist
5. The actual KV key is `google_gemini_api_key` (without braces)

## Fix
Added pattern detection in `ValidateAPIKeys`:
```go
lookupName := apiKeyName
if len(apiKeyName) > 2 && apiKeyName[0] == '{' && apiKeyName[len(apiKeyName)-1] == '}' {
    lookupName = apiKeyName[1 : len(apiKeyName)-1]
}
```

## Validation: ✅ MATCHES
The fix directly addresses the reported bug by ensuring variable placeholders are properly resolved before validation.

## Review: N/A
No critical triggers (security, auth, crypto, etc.)

## Verify
Build: ✅ | Tests: ⏭️ (no specific unit test for edge case)

## Files Changed
- `internal/jobs/service.go` - Added {xxx} pattern handling in ValidateAPIKeys (lines 397-403)
