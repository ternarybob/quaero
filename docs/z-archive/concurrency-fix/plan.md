# Plan: Fix queue concurrency limitation

## Analysis
**Root Cause**: The global queue concurrency is set to 2 in `NewDefaultConfig()`:
```go
Queue: QueueConfig{
    Concurrency: 2, // Reduced from 3 to minimize database lock contention
}
```

This limits the entire job processor to running only 2 job processing goroutines, regardless of individual job concurrency settings.

**Job-level vs Queue-level Concurrency**:
- Job-level `concurrency = 5` (in news-crawler.toml) = how many URLs a single crawler job crawls in parallel
- Queue-level `Config.Queue.Concurrency = 2` = how many jobs can be processed simultaneously

**Solution**: Increase the default queue concurrency to a reasonable value (e.g., 10) and document how to configure it.

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Increase default queue concurrency to 10 | none | no | low | sonnet |
| 2 | Add [queue] section to quaero.toml with concurrency | 1 | no | low | sonnet |
| 3 | Add concurrency to github-repo-collector.toml | none | no | low | sonnet |
| 4 | Create TestNewsCrawlerConcurrency test | 1,2 | no | medium | sonnet |

## Order
Concurrent: [1,2,3] → Sequential: [4] → Validate
