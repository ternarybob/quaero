# Plan: Refactor Job Interfaces

## Overview
Consolidate all job-related interfaces into `internal/interfaces/` to align with the project's clean architecture pattern, eliminate duplicate interface definitions, and resolve import cycle issues.

## Steps

1. **Create centralized job interfaces file**
   - Skill: @code-architect
   - Files: `internal/interfaces/job_interfaces.go` (NEW)
   - User decision: no
   - Action: Create new file consolidating JobManager, ParentJobOrchestrator, JobWorker, and JobSpawner interfaces

2. **Update job definition orchestrator**
   - Skill: @go-coder
   - Files: `internal/jobs/job_definition_orchestrator.go` (MODIFY)
   - User decision: no
   - Action: Remove duplicate interface definitions and update to use centralized interfaces

3. **Update database maintenance manager**
   - Skill: @go-coder
   - Files: `internal/jobs/manager/database_maintenance_manager.go` (MODIFY)
   - User decision: no
   - Action: Update imports and type references to use centralized interfaces

4. **Update job processor**
   - Skill: @go-coder
   - Files: `internal/jobs/worker/job_processor.go` (MODIFY)
   - User decision: no
   - Action: Update imports and type references to use centralized interfaces

5. **Update app initialization**
   - Skill: @go-coder
   - Files: `internal/app/app.go` (MODIFY)
   - User decision: no
   - Action: Verify import usage and ensure no unnecessary interface package imports

6. **Delete old interface files**
   - Skill: @go-coder
   - Files: `internal/jobs/manager/interfaces.go`, `internal/jobs/orchestrator/interfaces.go`, `internal/jobs/worker/interfaces.go` (DELETE)
   - User decision: no
   - Action: Remove obsolete interface files after all references updated

7. **Verify all implementations**
   - Skill: @go-coder
   - Files: All manager, worker, and orchestrator implementation files
   - User decision: no
   - Action: Verify interface compatibility for all concrete implementations

8. **Compile and test**
   - Skill: @test-writer
   - Files: All modified files
   - User decision: no
   - Action: Build application and run tests to verify refactoring success

## Success Criteria
- All job interfaces consolidated in `internal/interfaces/job_interfaces.go`
- No duplicate interface definitions in codebase
- All imports updated to reference centralized interfaces
- Application compiles without errors
- All tests pass
- No import cycle issues
