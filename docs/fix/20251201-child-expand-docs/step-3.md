# Step 3: Ensure Child Jobs Have Document Count Populated
Model: sonnet | Status: ✅

## Done
- Modified `EventDocumentSaved` handler to also increment child job's document_count
- Child jobs now track their own document count in metadata

## Analysis
Previously, only the PARENT job's document_count was incremented when documents were saved. Child jobs had no way to display their own document count in the UI.

The `EventDocumentSaved` payload includes:
- `job_id`: The child job that saved the document
- `parent_job_id`: The parent job for aggregation
- `document_id`: The saved document's ID

## Fix Applied
Added code to increment the child job's document_count in addition to the parent's:

```go
// Also increment the CHILD job's document_count so it can display its own count
if jobID != "" && jobID != parentJobID {
    if err := m.jobMgr.IncrementDocumentCount(ctx, jobID); err != nil {
        m.logger.Debug().Err(err).
            Str("job_id", jobID).
            Str("document_id", documentID).
            Msg("Failed to increment document count for child job")
    }
}
```

## Files Changed
- `internal/queue/state/monitor.go` - Modified `EventDocumentSaved` handler (lines 509-523)
  - Now increments document_count for both parent AND child jobs
  - Child increment only occurs if job_id differs from parent_job_id

## Verify
Build: pending | Tests: pending
