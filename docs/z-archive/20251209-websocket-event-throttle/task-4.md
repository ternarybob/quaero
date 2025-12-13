# Task 4: Create Step Events API Endpoint

Skill: go | Status: pending

## Objective
Create REST API endpoint to fetch step events with pagination/filtering.

## Changes

### File: `internal/server/routes.go`

Add new route:
```go
r.Get("/api/jobs/{job_id}/events", s.app.JobsHandler.GetJobEvents)
```

### File: `internal/handlers/jobs_handler.go`

Add new handler method:

```go
// GetJobEvents returns events for a job with optional filtering
// GET /api/jobs/{job_id}/events?limit=100&since=2025-12-09T00:00:00Z&offset=0
func (h *Handler) GetJobEvents(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "job_id")
    if jobID == "" {
        http.Error(w, "job_id is required", http.StatusBadRequest)
        return
    }

    // Parse query params
    limit := 100
    if l := r.URL.Query().Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
            limit = parsed
        }
    }

    offset := 0
    if o := r.URL.Query().Get("offset"); o != "" {
        if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
            offset = parsed
        }
    }

    var since time.Time
    if s := r.URL.Query().Get("since"); s != "" {
        if parsed, err := time.Parse(time.RFC3339, s); err == nil {
            since = parsed
        }
    }

    // Get events from log storage
    ctx := r.Context()
    events, err := h.logStorage.GetJobLogs(ctx, jobID, limit, offset, since)
    if err != nil {
        h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job events")
        http.Error(w, "Failed to get events", http.StatusInternalServerError)
        return
    }

    render.JSON(w, r, map[string]interface{}{
        "job_id": jobID,
        "events": events,
        "count":  len(events),
        "limit":  limit,
        "offset": offset,
    })
}
```

### File: `internal/interfaces/log_storage.go` (if needed)

Ensure interface supports the query:
```go
type LogStorage interface {
    // ... existing methods ...
    GetJobLogs(ctx context.Context, jobID string, limit, offset int, since time.Time) ([]LogEntry, error)
}
```

### File: `internal/storage/badger/log_storage.go` (if needed)

Implement the query method if not already present.

## API Response Format

```json
{
    "job_id": "step-uuid-123",
    "events": [
        {
            "id": "log-uuid",
            "timestamp": "2025-12-09T12:34:56Z",
            "level": "info",
            "message": "Processing document...",
            "metadata": { "doc_id": "doc-123" }
        }
    ],
    "count": 50,
    "limit": 100,
    "offset": 0
}
```

## Validation
- Build compiles successfully
- API endpoint returns correct data
- Pagination works correctly
