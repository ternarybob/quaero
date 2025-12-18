# Summary: Fix web-search-asx.toml Error

## Issue

Job execution failed with error:
```
error="step search_asx_gnp init failed: worker init failed: api_key is required for web_search"
```

## Root Cause

The `search_asx_gnp` step of type `web_search` was missing the required `api_key` parameter.

## Fix Applied

Added `api_key = "{google_gemini_api_key}"` to all 3 job definition files:

1. `bin/job-definitions/web-search-asx.toml`
2. `deployments/local/job-definitions/web-search-asx.toml`
3. `test/config/job-definitions/web-search-asx.toml`

## Final Job Definition Structure

```toml
[step.search_asx_gnp]
type = "web_search"
description = "Search for latest information on ASX listed company"
on_error = "fail"
api_key = "{google_gemini_api_key}"  # <-- FIXED
query = "find me latest information on ASX listed company ASX:GNP"
depth = 3
breadth = 3

[step.summarize_results]
type = "summary"
depends = "search_asx_gnp"
api_key = "{google_gemini_api_key}"
# ... rest of config

[step.email_summary]
type = "email"
depends = "summarize_results"
# ... rest of config
```

## Build Verification

- Main build: PASS
- MCP server: PASS

## Testing

Restart the server and run the "Web Search: ASX:GNP Company Info" job to verify the fix.
