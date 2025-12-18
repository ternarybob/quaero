# Architect Analysis: Fix web-search-asx.toml Error

## Error from Logs

```
time=14:15:23 level=ERR message="Job definition execution failed"
job_def_id=web-search-asx-gnp
error="step search_asx_gnp init failed: worker init failed: api_key is required for web_search"
```

## Root Cause

The `search_asx_gnp` step of type `web_search` is missing the required `api_key` parameter.

According to `docs/architecture/WORKERS.md:788`:
```
| `api_key` | string | No | Gemini API key |
```

However, the actual worker implementation **requires** the api_key (despite documentation saying "No").

## Evidence from Existing Job Definitions

All other jobs that use LLM services include the api_key:
- `codebase_assess.toml` - All summary steps have `api_key = "{google_gemini_api_key}"`
- `agent-document-generator.toml` - Has `api_key = "{google_gemini_api_key}"`

## Fix Required

Add `api_key = "{google_gemini_api_key}"` to the `search_asx_gnp` step in all 3 files:
1. `bin/job-definitions/web-search-asx.toml`
2. `deployments/local/job-definitions/web-search-asx.toml`
3. `test/config/job-definitions/web-search-asx.toml`

## Anti-Creation Verification

| Action | Type | Justification |
|--------|------|---------------|
| Modify web-search-asx.toml | MODIFY | Fix missing required parameter |

**No new code creation required** - this is a configuration fix.
