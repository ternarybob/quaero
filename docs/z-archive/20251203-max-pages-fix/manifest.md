# Fix: Crawler max_pages Limit Not Working

- Slug: max-pages-fix | Type: fix | Date: 2025-12-03
- Request: "Execute: docs\fix\queue_manager\max_pages_fix.md"
- Prior: none

## User Intent

Implement the fix described in `docs/fix/queue_manager/max_pages_fix.md` to make the `max_pages` configuration in crawler job definitions work correctly. Currently when configured with `max_pages = 10`, the crawler spawns 99+ child jobs instead of limiting to 10.

The fix requires two changes to `internal/queue/workers/crawler_worker.go`:
1. Update `extractCrawlConfig()` to handle both nested and flat config formats
2. Update `buildCrawlConfig()` to add `int64` type assertions for TOML compatibility

## Success Criteria

- [ ] `extractCrawlConfig()` handles both nested format (`config["crawl_config"]`) and flat format (`config["max_pages"]`, etc.)
- [ ] `buildCrawlConfig()` includes `int64` type assertions for `max_pages`, `max_depth`, and similar integer fields
- [ ] Code compiles without errors
- [ ] Existing tests pass
- [ ] Crawler respects `max_pages` limit when configured
