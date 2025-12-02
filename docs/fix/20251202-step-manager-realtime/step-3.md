# Step 3: Update UI handleJobLog to use manager_id
Model: sonnet | Status: ✅

## Done
- Added manager_id to job_log WebSocket handler event dispatch
- Changed handleJobLog to use manager_id as primary aggregation key
- Updated all variable references from parentId to aggregationId
- Maintained backwards compatibility with fallback to parent_job_id then job_id

## Files Changed
- `pages/queue.html` - WebSocket handler and handleJobLog method

## Build Check
Build: ✅ (frontend) | Tests: ⏭️
