# Plan: Rename Interfaces - StepExecutor → JobManager, JobExecutor → JobWorker

## Steps

1. **Rename StepExecutor interface to JobManager**
   - Skill: @code-architect
   - Files: `internal/jobs/executor/interfaces.go`
   - User decision: no
   - Rename interface and methods: ExecuteStep → CreateParentJob, GetStepType → GetManagerType
   - Update interface comments to reflect manager responsibilities

2. **Update CrawlerStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/crawler_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

3. **Update AgentStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/agent_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

4. **Update DatabaseMaintenanceStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/database_maintenance_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

5. **Update TransformStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/transform_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

6. **Update ReindexStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/reindex_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

7. **Update PlacesSearchStepExecutor to implement JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - Update method signatures and comments to match new interface

8. **Update JobExecutor orchestrator to use JobManager interface**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/job_executor.go`
   - User decision: no
   - Update field types, method signatures, method calls, and log messages

9. **Rename JobExecutor interface to JobWorker**
   - Skill: @code-architect
   - Files: `internal/interfaces/job_executor.go`
   - User decision: no
   - Rename interface and method: GetJobType → GetWorkerType
   - Update interface comments to reflect worker responsibilities

10. **Update CrawlerExecutor to implement JobWorker interface**
    - Skill: @go-coder
    - Files: `internal/jobs/processor/crawler_executor.go`
    - User decision: no
    - Update method signatures and comments to match new interface

11. **Update AgentExecutor to implement JobWorker interface**
    - Skill: @go-coder
    - Files: `internal/jobs/processor/agent_executor.go`
    - User decision: no
    - Update method signatures and comments to match new interface

12. **Update DatabaseMaintenanceExecutor to implement JobWorker interface**
    - Skill: @go-coder
    - Files: `internal/jobs/executor/database_maintenance_executor.go`
    - User decision: no
    - Update method signatures and comments, add deprecation notice

13. **Update JobProcessor to use JobWorker interface**
    - Skill: @go-coder
    - Files: `internal/jobs/processor/processor.go`
    - User decision: no
    - Update field types, method signatures, method calls, and log messages

14. **Update app.go registrations and log messages**
    - Skill: @go-coder
    - Files: `internal/app/app.go`
    - User decision: no
    - Update all registration log messages to use new terminology

15. **Update test file comment**
    - Skill: @none
    - Files: `test/api/places_job_document_test.go`
    - User decision: no
    - Update comment referencing ParentJobExecutor to JobOrchestrator

16. **Compile and verify all changes**
    - Skill: @go-coder
    - Files: All modified files
    - User decision: no
    - Run compilation test to ensure no errors

17. **Run test suite to verify no regressions**
    - Skill: @test-writer
    - Files: `test/api/`, `test/ui/`
    - User decision: no
    - Run full test suite and verify all tests pass

## Success Criteria

- All code compiles without errors
- All tests pass (run full test suite: `test/api/` and `test/ui/`)
- Interface names reflect Manager/Worker pattern (JobManager, JobWorker)
- Method names clearly indicate purpose (CreateParentJob vs Execute, GetManagerType vs GetWorkerType)
- Log messages use consistent terminology (manager, worker, not executor)
- No functional regressions - only naming changes
- Test file comments updated to match new terminology
