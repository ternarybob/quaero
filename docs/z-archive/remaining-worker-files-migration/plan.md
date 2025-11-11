# Plan: ARCH-006 - Remaining Worker Files Migration

## Steps

1. **Create AgentWorker File**
   - Skill: @code-architect
   - Files: `internal/jobs/worker/agent_worker.go` (NEW), `internal/jobs/processor/agent_executor.go` (READ)
   - User decision: no
   - Description: Copy agent_executor.go to worker/ package with transformations (AgentExecutor→AgentWorker, package processor→worker, receiver e→w)

2. **Create JobProcessor File**
   - Skill: @code-architect
   - Files: `internal/jobs/worker/job_processor.go` (NEW), `internal/jobs/processor/processor.go` (READ)
   - User decision: no
   - Description: Copy processor.go to worker/ package with minimal changes (file rename processor.go→job_processor.go, package declaration only)

3. **Update App Registration**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Description: Update app.go to use worker.JobProcessor and worker.NewAgentWorker() (lines 67, 271, 323)

4. **Remove Deprecated Files**
   - Skill: @go-coder
   - Files: `internal/jobs/processor/agent_executor.go`, `internal/jobs/processor/processor.go`
   - User decision: no
   - Description: Delete deprecated files immediately (breaking changes acceptable per ARCH-005 precedent)

5. **Update Architecture Documentation**
   - Skill: @none
   - Files: `AGENTS.md`, `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - User decision: no
   - Description: Update documentation to reflect ARCH-006 completion

6. **Compile and Validate**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Description: Verify compilation, build application, run tests

## Success Criteria

- AgentWorker file created in worker/ with correct transformations
- JobProcessor file created in worker/ with minimal changes
- app.go successfully imports and uses new worker package
- Application compiles and builds successfully
- Old processor files deleted (breaking changes acceptable)
- Documentation updated to show ARCH-006 complete
- ParentJobExecutor remains in processor/ (migrates in ARCH-007)
