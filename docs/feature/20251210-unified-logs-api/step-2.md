# Step 2: Update UI to use unified /api/logs endpoint
Model: sonnet | Skill: frontend | Status: ✅

## Done
- Updated `fetchStepEvents()` to use `/api/logs?scope=job&job_id=${stepJobId}&include_children=false&limit=100&order=asc&level=info`
- Updated completed step log fetch to use same unified endpoint
- Updated `fetchHistoricalLogs()` to use `/api/logs?scope=job&job_id=${jobId}&include_children=true&order=asc`
- Updated `refreshStepEvents()` to use unified endpoint

## Files Changed
- `pages/queue.html` - 4 fetch calls updated from `/api/jobs/{id}/logs` to `/api/logs?scope=job&job_id={id}`

## Skill Compliance (frontend)
- [x] Async fetch patterns followed
- [x] Error handling preserved
- [x] Response data parsing unchanged (compatible)

## Build Check
Build: ⏳ | Tests: ⏭️
