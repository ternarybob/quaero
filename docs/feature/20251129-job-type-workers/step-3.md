# Step 3: Refactor Workers to Unified Interface

- Task: task-3.md | Group: 3 | Model: sonnet

## Actions
1. Created workers package in `internal/queue/workers/`
2. Created 6 StepWorker adapter implementations
3. Updated app.go to register all step workers with Orchestrator
4. Added type-specific validation to each worker

## Files
- `internal/queue/workers/agent_step_worker.go` - NEW: Wraps AgentManager
- `internal/queue/workers/crawler_step_worker.go` - NEW: Wraps CrawlerManager
- `internal/queue/workers/places_step_worker.go` - NEW: Wraps PlacesSearchManager
- `internal/queue/workers/web_search_step_worker.go` - NEW: Wraps WebSearchManager
- `internal/queue/workers/github_repo_step_worker.go` - NEW: Wraps GitHubRepoManager
- `internal/queue/workers/github_actions_step_worker.go` - NEW: Wraps GitHubActionsManager
- `internal/app/app.go` - Added step worker registration

## Decisions
- Used adapter pattern to wrap existing managers
- Each wrapper delegates CreateJobs to manager.CreateParentJob
- Added validation for type-specific config fields
- Dual registration: both legacy StepManager and new StepWorker

## Verify
Compile: ✅ | Tests: ✅

## Status: ✅ COMPLETE
