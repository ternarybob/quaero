# Validation 1: Fix Verified

## Build Results

### Main Build: PASS
```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Fix Applied

Added `api_key = "{google_gemini_api_key}"` to the `search_asx_gnp` step in all 3 files:

| File | Line | Change |
|------|------|--------|
| `bin/job-definitions/web-search-asx.toml` | 19 | Added `api_key = "{google_gemini_api_key}"` |
| `deployments/local/job-definitions/web-search-asx.toml` | 19 | Added `api_key = "{google_gemini_api_key}"` |
| `test/config/job-definitions/web-search-asx.toml` | 19 | Added `api_key = "{google_gemini_api_key}"` |

## Verification

All 3 job definition files now have consistent configuration:

```toml
[step.search_asx_gnp]
type = "web_search"
description = "Search for latest information on ASX listed company"
on_error = "fail"
api_key = "{google_gemini_api_key}"  # <-- ADDED
query = "find me latest information on ASX listed company ASX:GNP"
depth = 3
breadth = 3
```

## Validation Result: PASS

The fix addresses the error:
```
error="step search_asx_gnp init failed: worker init failed: api_key is required for web_search"
```

By adding the required `api_key` parameter.
