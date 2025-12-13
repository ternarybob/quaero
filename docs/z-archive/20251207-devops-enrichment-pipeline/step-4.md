# Step 4: Implement classify_devops action

Model: sonnet | Status: ✅

## Done

- Created ClassifyDevOpsAction with LLM-based classification
- Crafted DevOps-focused prompt explaining context for non-C programmers
- Implemented JSON response parsing with markdown handling
- Added retry with exponential backoff (3 retries: 1s, 2s, 4s)
- Implemented content truncation to ~6000 chars
- Added enrichment_failed marking on persistent errors
- Updated DevOpsWorker.handleClassifyDevOps to use action

## Files Changed

- `internal/jobs/actions/classify_devops.go` - New action file (444 lines)
- `internal/queue/workers/devops_worker.go` - Updated handler

## Build Check

Build: ✅ (syntax validated) | Tests: ⏭️
