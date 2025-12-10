# Fix: Crawler max_pages Limit Not Working

## Issue Summary

The `max_pages` configuration in crawler job definitions was not being respected. When configured with `max_pages = 10`, the crawler was spawning 99+ child jobs instead of limiting to 10.

## Root Causes

### 1. Config Format Mismatch in `extractCrawlConfig`

**Problem:** The `extractCrawlConfig` function only looked for config in nested format (`config["crawl_config"]`), but seed jobs from `StartCrawl` store config in flat format (`config["max_depth"]`, `config["max_pages"]`, etc.).

**Location:** `internal/queue/workers/crawler_worker.go` - `extractCrawlConfig()`

**Before:**
```go
func (w *CrawlerWorker) extractCrawlConfig(config map[string]interface{}) (*models.CrawlConfig, error) {
    crawlConfigRaw, ok := config["crawl_config"]
    if !ok {
        return &models.CrawlConfig{}, nil // Returns empty config with MaxPages=0
    }
    // ... only handles nested config
}
```

**After:** Function now handles both formats:
1. Nested: `config["crawl_config"] = CrawlConfig{...}` (from `spawnChildJob`)
2. Flat: `config["max_depth"]`, `config["max_pages"]`, etc. (from `StartCrawl` seed jobs)

### 2. Missing int64 Type Assertion in `buildCrawlConfig`

**Problem:** TOML parser uses `int64` for integer values, but `buildCrawlConfig` only checked for `float64` (JSON) and `int` (Go native).

**Location:** `internal/queue/workers/crawler_worker.go` - `buildCrawlConfig()`

**Before:**
```go
if v, ok := configMap["max_pages"].(float64); ok {
    config.MaxPages = int(v)
} else if v, ok := configMap["max_pages"].(int); ok {
    config.MaxPages = v
}
// Missing: int64 check for TOML-parsed values
```

**After:**
```go
if v, ok := configMap["max_pages"].(float64); ok {
    config.MaxPages = int(v)
} else if v, ok := configMap["max_pages"].(int); ok {
    config.MaxPages = v
} else if v, ok := configMap["max_pages"].(int64); ok {
    config.MaxPages = int(v)
}
```

## Files Modified

1. `internal/queue/workers/crawler_worker.go`
   - `extractCrawlConfig()` - Handle both nested and flat config formats
   - `buildCrawlConfig()` - Add `int64` type assertions for TOML compatibility

## Config Flow

```
TOML File (max_pages = 10)
    ↓
Job Definition Loaded (step.Config["max_pages"] = int64(10))
    ↓
buildCrawlConfig() - Creates CrawlConfig{MaxPages: 10}
    ↓
StartCrawl() - Stores flat config: messageConfig["max_pages"] = 10
    ↓
Job Enqueued to Queue
    ↓
Worker Execute() - Calls extractCrawlConfig(job.Config)
    ↓
extractCrawlConfig() - Extracts from flat config map
    ↓
max_pages limit enforced in child spawning logic
```

## Verification

Debug logging added to confirm fix:
```
DEBUG: max_pages=10, filtered=145, depth=1
Links found: 146 | filtered: 145 | followed: 9 | skipped: 136
```

The crawler now correctly limits child jobs to `max_pages - 1` (accounting for the seed URL).

## Related Issues

- WebSocket real-time updates were already working correctly
- The apparent lack of real-time updates was due to UI page refresh timing in tests

## Test Command

```powershell
go test -v -run TestWebSocketJobEvents_NewsCrawlerRealTime ./test/api/...
```

