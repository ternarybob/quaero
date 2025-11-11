# Plan: Create Manager/Worker/Orchestrator Directory Structure (ARCH-003)

## Steps

1. **Create manager/ directory and interfaces.go**
   - Skill: @code-architect
   - Files: `internal/jobs/manager/` (new directory), `internal/jobs/manager/interfaces.go` (new file)
   - User decision: no
   - Description: Create new manager package directory and copy JobManager interface from executor/interfaces.go

2. **Create worker/ directory and interfaces.go**
   - Skill: @code-architect
   - Files: `internal/jobs/worker/` (new directory), `internal/jobs/worker/interfaces.go` (new file)
   - User decision: no
   - Description: Create new worker package directory and copy JobWorker and JobSpawner interfaces from internal/interfaces/job_executor.go

3. **Create orchestrator/ directory and interfaces.go**
   - Skill: @code-architect
   - Files: `internal/jobs/orchestrator/` (new directory), `internal/jobs/orchestrator/interfaces.go` (new file)
   - User decision: no
   - Description: Create new orchestrator package directory and define new JobOrchestrator interface

4. **Update AGENTS.md with directory structure notes**
   - Skill: @none
   - Files: `AGENTS.md`
   - User decision: no
   - Description: Document new directory structure and transition state in AGENTS.md

5. **Update MANAGER_WORKER_ARCHITECTURE.md with migration status**
   - Skill: @none
   - Files: `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - User decision: no
   - Description: Update architecture documentation to reflect ARCH-003 completion and explain temporary interface duplication

6. **Compile verification**
   - Skill: @go-coder
   - Files: All new interface files
   - User decision: no
   - Description: Verify all new interface files compile independently and no existing code is broken

## Success Criteria

- Three new directories created: `internal/jobs/manager/`, `internal/jobs/worker/`, `internal/jobs/orchestrator/`
- Each directory contains an `interfaces.go` file with appropriate package declaration and interface definitions
- JobManager interface copied to `manager/interfaces.go` with package name `manager`
- JobWorker and JobSpawner interfaces copied to `worker/interfaces.go` with package name `worker`
- JobOrchestrator interface created in `orchestrator/interfaces.go` with methods: StartMonitoring, StopMonitoring, GetMonitoringStatus
- All new interface files compile independently without errors
- Existing code in `internal/jobs/executor/` and `internal/jobs/processor/` remains unchanged and functional
- AGENTS.md updated with directory structure transition notes
- MANAGER_WORKER_ARCHITECTURE.md updated with ARCH-003 completion status
- No broken imports or compilation errors in existing codebase
