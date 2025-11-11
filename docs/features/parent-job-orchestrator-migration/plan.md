# Plan: Parent Job Orchestrator Migration (ARCH-007)

## Steps

1. **Create ParentJobOrchestrator File**
   - Skill: @code-architect
   - Files: `internal/jobs/orchestrator/parent_job_orchestrator.go` (NEW)
   - User decision: no
   - Copy from `internal/jobs/processor/parent_job_executor.go` with transformations:
     - Package: processor → orchestrator
     - Struct: ParentJobExecutor → ParentJobOrchestrator
     - Constructor: NewParentJobExecutor → NewParentJobOrchestrator
     - Receiver: (e *ParentJobExecutor) → (o *ParentJobOrchestrator)
     - All method bodies: e. → o.

2. **Update ParentJobOrchestrator Interface**
   - Skill: @code-architect
   - Files: `internal/jobs/orchestrator/interfaces.go`
   - User decision: no
   - Update interface to match implementation signature:
     - StartMonitoring(ctx context.Context, job *models.JobModel)
     - SubscribeToChildStatusChanges()
     - Remove speculative methods (StopMonitoring, GetMonitoringStatus)

3. **Update App Registration**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Update imports: processor → orchestrator
   - Update variable names: parentJobExecutor → parentJobOrchestrator
   - Update constructor calls: NewParentJobExecutor → NewParentJobOrchestrator

4. **Update JobExecutor Integration**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/job_executor.go`
   - User decision: no
   - Update imports: processor → orchestrator
   - Update field, parameter, and method calls
   - Update variable names: parentJobExecutor → parentJobOrchestrator

5. **Update Comment References**
   - Skill: @go-coder
   - Files: `internal/jobs/worker/job_processor.go`, `internal/interfaces/event_service.go`, `internal/jobs/manager.go`, `test/api/places_job_document_test.go`
   - User decision: no
   - Update all comments: ParentJobExecutor → ParentJobOrchestrator

6. **Delete Deprecated File**
   - Skill: @go-coder
   - Files: `internal/jobs/processor/parent_job_executor.go` (DELETE)
   - User decision: no
   - Remove old file immediately (breaking changes acceptable)
   - Verify processor/ directory is empty

7. **Update Architecture Documentation**
   - Skill: @none
   - Files: `AGENTS.md`, `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - User decision: no
   - Update directory structure to reflect ARCH-007 completion
   - Document orchestrator migration details
   - Update migration progress

8. **Compile and Validate**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Build application successfully
   - Run test suite
   - Verify parent job monitoring works end-to-end

## Success Criteria

- New file created in internal/jobs/orchestrator/
- ParentJobExecutor renamed to ParentJobOrchestrator throughout
- Interface signature matches implementation
- app.go successfully imports and uses orchestrator package
- job_executor.go successfully imports and uses orchestrator package
- All comments updated to use "orchestrator" terminology
- Old file deleted from processor/ directory
- processor/ directory is empty
- Application compiles and runs successfully
- All tests pass
- Parent job monitoring works correctly
- Documentation updated to reflect ARCH-007 completion
