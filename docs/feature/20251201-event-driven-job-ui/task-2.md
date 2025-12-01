# Task 2: Create aggregated logs API endpoint for parent jobs
Depends: 1 | Critical: no | Model: sonnet

## Context
Currently `/api/jobs/{id}/logs` returns only that job's logs. Need endpoint that returns parent job logs + all child job logs merged and sorted by timestamp.

## Do
- Add new endpoint: `GET /api/jobs/{id}/logs/aggregated`
- Fetch parent job logs
- Fetch all child job IDs for parent
- Fetch logs from all children
- Merge and sort by timestamp
- Support query params: `level`, `order`, `limit`
- Handle pagination for large log sets

## API Response Format
```json
{
  "job_id": "parent-uuid",
  "total_entries": 150,
  "logs": [
    {
      "timestamp": "2024-12-01T10:30:01Z",
      "level": "info",
      "message": "Step 1 started: search_nearby_restaurants",
      "source_job_id": "parent-uuid",
      "step_name": "search_nearby_restaurants"
    },
    {
      "timestamp": "2024-12-01T10:30:02Z",
      "level": "info",
      "message": "Created 20 documents",
      "source_job_id": "child-uuid-1",
      "step_name": "search_nearby_restaurants"
    }
  ]
}
```

## Accept
- [ ] Endpoint returns aggregated logs from parent + all children
- [ ] Logs sorted by timestamp
- [ ] Level filtering works
- [ ] Build compiles without errors
