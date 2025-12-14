# Test Run 2

File: test/ui/job_definition_general_test.go
Test: TestJobDefinitionErrorGeneratorComprehensive
Date: 2025-12-14

## Root Cause Analysis

The test fails because logs are not appearing in the UI. After tracing through the code:

1. **error_generator_worker.go** stores logs with lowercase level: `AddJobLog(ctx, job.ID, "info", message)`

2. **log_storage.go AppendLog** stores the level AS-IS: `Level: level` (so `"info"`)

3. **log_storage.go GetLogsByLevel** normalizes the query level to 3-letter format: `normalizeLevel("info")` → `"INF"`

4. **The query fails** because `Level.Eq("INF")` doesn't match stored `Level="info"`

## The Bug

```go
// In log_storage.go AppendLog (line 113):
func (s *LogStorage) AppendLog(ctx context.Context, jobID string, entry models.LogEntry) error {
    entry.JobIDField = jobID
    entry.LineNumber = s.getNextLineNumber(ctx, jobID)
    // BUG: Level is NOT normalized before storage!
    // entry.Level is stored as "info" but GetLogsByLevel queries for "INF"
    ...
}
```

## Fix Required

Normalize the level when storing logs in `AppendLog`:

```go
// Add before inserting:
entry.Level = normalizeLevel(entry.Level)
```

This ensures:
- Stored level: `"INF"`
- Queried level: `"INF"`
- Match: ✓
