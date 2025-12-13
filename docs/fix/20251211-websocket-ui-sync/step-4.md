# Step 4: Update UI for new message format
Model: opus | Skill: - | Status: ✅

## Done
- Added handler for `job_update` WebSocket message in queue.html
- Added `jobList:handleJobUpdate` event listener
- Created `handleJobUpdate(update)` method that:
  - Updates job status in allJobs when context=job
  - Updates step status in jobTreeData when context=job_step
  - Auto-expands running/failed steps
  - Calls fetchJobStructure when refresh_logs=true
- Created `fetchJobStructure(jobId)` method for lightweight status fetching
- Created `fetchStepLogs(jobId, stepName, stepIdx)` for log fetching on expanded steps

## Files Changed
- `pages/queue.html`:
  - Message handler (lines 1593-1601)
  - Event listener (line 2297)
  - handleJobUpdate method (lines 4297-4353)
  - fetchJobStructure method (lines 4355-4397)
  - fetchStepLogs method (lines 4399-4418)

## Skill Compliance
N/A - no skill for this task

## Build Check
Build: ✅ | Tests: ⏭️
