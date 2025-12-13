# Step 6: Implement aggregate_devops_summary action

Model: sonnet | Status: ✅

## Done

- Created AggregateDevOpsSummaryAction with LLM synthesis
- Implemented data aggregation from all enriched documents
- Created comprehensive 7-section DevOps guide prompt
- Added dual storage: KV (devops:summary) + searchable document
- Created document with ID "devops-summary" and tags
- Added graceful degradation when LLM unavailable
- Updated DevOpsWorker.handleAggregateDevOpsSummary to use action

## Files Changed

- `internal/jobs/actions/aggregate_devops_summary.go` - New action file (541 lines)
- `internal/queue/workers/devops_worker.go` - Updated handler

## Build Check

Build: ✅ (syntax validated) | Tests: ⏭️
