# Plan: max_pages fix verification

Type: fix | Workdir: ./docs/fix/20251203-max-pages-fix/

## User Intent (from manifest)

Implement the fix described in `docs/fix/queue_manager/max_pages_fix.md` to make the `max_pages` configuration in crawler job definitions work correctly. Currently when configured with `max_pages = 10`, the crawler spawns 99+ child jobs instead of limiting to 10.

## Analysis

After reading `internal/queue/workers/crawler_worker.go`, the fix has **already been implemented**:

1. **`extractCrawlConfig()` (lines 403-491)**: Already handles both formats:
   - Nested: `config["crawl_config"]` (from spawnChildJob)
   - Flat: `config["max_depth"]`, `config["max_pages"]`, etc. (from StartCrawl seed jobs)
   - Includes `int64` type assertions for all integer fields

2. **`buildCrawlConfig()` (lines 1549-1638)**: Already includes `int64` type assertions:
   - `max_depth`: lines 1563-1569
   - `max_pages`: lines 1571-1577
   - `concurrency`: lines 1579-1585

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Verify code compiles | - | no | sonnet |
| 2 | Run existing tests to verify fix works | 1 | no | sonnet |

## Order

[1] → [2]
