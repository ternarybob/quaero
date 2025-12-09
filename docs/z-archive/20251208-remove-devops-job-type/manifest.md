# Fix: Remove DevOps Job Type - Use Direct Worker Names
- Slug: remove-devops-job-type | Type: fix | Date: 2025-12-08
- Request: "Remove 'devops' as a job type and replace with direct worker names/functions. The 'devops' job type is incorrectly termed - workers should be named after their actual function (e.g., DependencyGraphWorker). Remove the action configuration pattern where devops type delegates to workers."
- Prior: none

## User Intent
The user wants to eliminate the abstract "devops" job type that acts as a dispatcher to various workers based on an "action" configuration. Instead, each worker should be directly addressable by its actual function name. This is a breaking change to simplify the job type system.

## Success Criteria
- [x] Remove "devops" as a job type entirely
- [x] Each worker type is directly addressable (e.g., "dependency_graph", "code_summary", etc.)
- [x] No "action" configuration needed - job type maps directly to worker
- [x] All existing devops workers renamed to reflect their actual function
- [x] Tests in test/ui/devops_enrichment_test.go updated to match new code
- [x] Build passes
- [x] Tests pass

## Changes Made
1. Removed `WorkerTypeDevOps` from `internal/models/worker_type.go`
   - Removed from constants
   - Removed from `IsValid()` function
   - Removed from `AllWorkerTypes()` function

2. Removed DevOps worker registration from `internal/app/app.go`
   - Removed `NewDevOpsWorker` creation and registration
   - Individual enrichment workers remain registered separately

3. Deleted `internal/queue/workers/devops_worker.go`
   - This was the dispatcher that routed based on "action" configuration

4. Updated TOML job definitions:
   - `test/config/job-definitions/devops_enrich.toml`
   - `test/bin/job-definitions/devops_enrich.toml`
   - `bin/job-definitions/devops_enrich.toml`
   - Changed from `type = "devops"` + `action = "X"` to `type = "X"` directly
   - Mapping:
     - `action = "extract_structure"` -> `type = "extract_structure"`
     - `action = "analyze_build_system"` -> `type = "analyze_build"`
     - `action = "classify_devops"` -> `type = "classify"`
     - `action = "build_dependency_graph"` -> `type = "dependency_graph"`
     - `action = "aggregate_devops_summary"` -> `type = "aggregate_summary"`

5. Updated test file `internal/models/job_definition_test.go`
   - Removed `WorkerTypeDevOps` test cases
   - Updated expected type count from 19 to 18

## Breaking Changes
This is a breaking change. Job definitions using `type = "devops"` with `action` configuration must be updated to use the direct worker type.
