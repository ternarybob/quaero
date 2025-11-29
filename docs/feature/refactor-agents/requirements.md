# Agent/Job Architecture Refactoring Requirements

**Version:** 1.0
**Created:** 2025-11-25
**Status:** Draft - Ready for Implementation

## Executive Summary

This document outlines requirements for refactoring the agent/job architecture to:

1. **Revert job type naming** from `"ai"` back to `"agent"` (better semantic clarity)
2. **Reorganize folder structure** to enforce separation of concerns between job definitions, queue operations, and execution state
3. **Clarify terminology** to distinguish between "jobs" (definitions), "queue" (execution), and "state" (runtime tracking)

## Background

### Current Issues

1. **Naming Confusion:** The job type was changed from `"agent"` to `"ai"` per `docs/z-archive/20251125-130000-ai-job-type-refactor/plan.md`, but "agent" is semantically more accurate for the agent framework
2. **Folder Organization:** The queue implementation (`internal/jobs/queue/`) is nested under the jobs folder, and state is separated from queue operations, violating the principle that execution state is part of the queue domain
3. **Terminology Overlap:** "Jobs" refers to both definitions (what to do) and queued work (execution), causing confusion

### Architectural Principles (from MANAGER_WORKER_ARCHITECTURE.md)

The system follows a **Manager/Worker/Monitor pattern** with two distinct domains:

1. **Actions Domain** - User-defined workflows (`JobDefinition` or `Job`)
2. **Queue Domain** - Job execution including:
   - Immutable queued work (`QueueJob`)
   - Runtime execution state (`QueueJobState`)
   - Queue operations (managers, workers, monitors)

**Key Principle:** Clear separation between:
- **Job/JobDefinition** = What to do (user-defined workflow) - **Actions Domain**
- **QueueJob** = Work to be done (immutable task definition) - **Queue Domain**
- **QueueJobState** = How it's going (runtime state, in-memory only) - **Queue Domain**

## Requirements

### 1. Revert Job Type Naming: "ai" → "agent"

**Rationale:** "Agent" better describes the agent framework's purpose (AI-powered agents that process documents).

#### 1.1 Model Changes

**File:** `internal/models/job_definition.go`

- ✅ **KEEP** `JobDefinitionTypeAI = "ai"` constant (for job definitions)
- ✅ **KEEP** AI-specific validation logic for operation types (scan/enrich/generate)
- ✅ **KEEP** Free-text action support for AI job definitions

**File:** `internal/models/job_model.go`

- **NO CHANGES** - `QueueJob.Type` is a string field, not a constant

#### 1.2 Queue Job Type Changes

**Files to Update:**

1. `internal/jobs/queue/managers/agent_manager.go`
   - Line 252: Change `"ai"` to `"agent"` in `NewQueueJobChild()` call
   - Update comment: "Agent job type for agent-powered document processing"

2. `internal/jobs/queue/workers/agent_worker.go`
   - Line 56: Change validation from `"ai"` to `"agent"`
   - Update `GetWorkerType()` to return `"agent"`

3. `internal/jobs/definitions/orchestrator.go`
   - Line 201: Change manager lookup from `"ai"` to `"agent"`
   - Update comment: "Route AI jobs to AgentManager (registered with manager type 'agent')"

4. `deployments/local/quaero.toml`
   - Line 93: Update comment to use "agent" instead of "ai" in job types list

#### 1.3 Documentation Updates

**Files to Update:**

1. `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - Line 208: Change comment from `"crawler", "agent"` (already correct, verify)
   - Line 293: Update AgentManager description to use "agent" action

2. `AGENTS.md`
   - Search for all references to `"ai"` job type and update to `"agent"`
   - Update agent framework documentation to clarify naming

3. Example job definitions (keep `type = "ai"` for JobDefinition, but document that queue jobs use `"agent"`)
   - `deployments/local/job-definitions/keyword-extractor-agent.toml`
   - `deployments/local/job-definitions/ai-document-generator.toml`
   - `deployments/local/job-definitions/ai-web-enricher.toml`

**Important:** Job definitions use `type = "ai"` (JobDefinitionType), but queue jobs use `type = "agent"` (QueueJob.Type). This is intentional and should be documented clearly.

### 2. Reorganize Folder Structure

**Current Structure:**
```
internal/
├── jobs/
│   ├── definitions/
│   │   └── orchestrator.go
│   ├── queue/              # ❌ Queue under jobs (wrong domain)
│   │   ├── lifecycle.go
│   │   ├── managers/
│   │   └── workers/
│   └── state/              # ❌ State separated from queue (wrong - state IS part of queue)
│       ├── monitor.go
│       ├── progress.go
│       ├── runtime.go
│       └── stats.go
└── queue/                  # ❌ Separate queue package (confusing)
    └── badger_manager.go
```

**Proposed Structure:**
```
internal/
├── actions/                # RENAMED from jobs/ - user-defined workflows
│   ├── definitions/
│   │   └── orchestrator.go
│   └── README.md          # Explains: "Actions are user-defined workflows"
└── queue/                 # CONSOLIDATED - all queue operations and state
    ├── badger_manager.go  # MOVED from internal/queue/
    ├── lifecycle.go       # MOVED from internal/jobs/queue/
    ├── managers/          # MOVED from internal/jobs/queue/managers/
    ├── workers/           # MOVED from internal/jobs/queue/workers/
    ├── state/             # MOVED from internal/jobs/state/ (state is part of queue domain)
    │   ├── monitor.go
    │   ├── progress.go
    │   ├── runtime.go
    │   └── stats.go
    └── README.md          # Explains: "Queue handles job execution and runtime state"
```

#### 2.1 Folder Reorganization Steps

**Phase 1: Create New Structure**

1. Create `internal/actions/` directory
2. Create `internal/actions/definitions/` directory
3. Create `internal/queue/managers/` directory (if not exists)
4. Create `internal/queue/workers/` directory (if not exists)
5. Create `internal/queue/state/` directory

**Phase 2: Move Files**

1. Move `internal/jobs/definitions/` → `internal/actions/definitions/`
2. Move `internal/jobs/queue/lifecycle.go` → `internal/queue/lifecycle.go`
3. Move `internal/jobs/queue/managers/` → `internal/queue/managers/`
4. Move `internal/jobs/queue/workers/` → `internal/queue/workers/`
5. Move `internal/jobs/state/` → `internal/queue/state/`
6. Move `internal/queue/badger_manager.go` → `internal/queue/badger_manager.go` (already in correct location)
7. Move `internal/queue/types.go` → `internal/queue/types.go` (already in correct location, if exists)

**Phase 3: Update Imports**

Update all import statements across the codebase:
- `internal/jobs/definitions` → `internal/actions/definitions`
- `internal/jobs/queue` → `internal/queue`
- `internal/jobs/queue/managers` → `internal/queue/managers`
- `internal/jobs/queue/workers` → `internal/queue/workers`
- `internal/jobs/state` → `internal/queue/state`

**Phase 4: Update Documentation**

1. Update `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` with new folder structure
2. Update `AGENTS.md` with new paths
3. Create `internal/actions/README.md` explaining action definitions
4. Create `internal/queue/README.md` explaining queue operations and state

**Phase 5: Cleanup**

1. Remove empty `internal/jobs/` directory
2. Verify all tests pass
3. Update build scripts if needed

### 3. Terminology Clarification

**Update all documentation and comments to use consistent terminology:**

| Term | Meaning | Location | Type |
|------|---------|----------|------|
| **Action** | User-defined workflow | `internal/actions/` | `JobDefinition` |
| **Queue Job** | Immutable work definition | `internal/queue/` | `QueueJob` |
| **Job State** | Runtime execution state | `internal/queue/state/` | `QueueJobState` |
| **Manager** | Creates parent jobs, orchestrates workflows | `internal/queue/managers/` | `StepManager` interface |
| **Worker** | Executes individual jobs from queue | `internal/queue/workers/` | `JobWorker` interface |
| **Monitor** | Tracks job progress and aggregates stats | `internal/queue/state/` | `JobMonitor` interface |

**Key Distinctions:**

- **Actions Domain** - Defines WHAT to do (user-facing, editable workflows)
- **Queue Domain** - Manages execution AND state (immutable jobs + runtime tracking)
  - Queue operations (lifecycle, managers, workers)
  - State tracking (monitor, progress, runtime)

## Implementation Plan

### Phase 1: Revert Job Type Naming (Low Risk)

**Estimated Effort:** 2-4 hours

1. Update `agent_manager.go` to use `"agent"` job type
2. Update `agent_worker.go` validation and worker type
3. Update `orchestrator.go` manager lookup
4. Update documentation and comments
5. Run tests to verify no regressions

**Success Criteria:**
- All agent jobs use `"agent"` queue job type
- Job definitions continue to use `type = "ai"`
- All tests pass
- Documentation clearly explains the distinction

### Phase 2: Reorganize Folder Structure (Medium Risk)

**Estimated Effort:** 4-8 hours

1. Create new folder structure
2. Move files to new locations
3. Update all import statements (use IDE refactoring tools)
4. Update documentation
5. Run full test suite
6. Verify build succeeds

**Success Criteria:**
- All files in correct locations
- No broken imports
- All tests pass
- Build succeeds
- Documentation updated

### Phase 3: Terminology Cleanup (Low Risk)

**Estimated Effort:** 2-4 hours

1. Update all documentation files
2. Update code comments
3. Create README files for new directories
4. Update AGENTS.md with new terminology

**Success Criteria:**
- Consistent terminology across all docs
- Clear README files in each directory
- Updated architecture diagrams

## Testing Requirements

### Unit Tests

- Verify agent worker accepts `"agent"` job type
- Verify agent manager creates jobs with `"agent"` type
- Verify orchestrator routes AI job definitions to agent manager

### Integration Tests

- Test full agent workflow (job definition → queue → execution)
- Test job processor routes `"agent"` jobs to agent worker
- Test monitor tracks agent job progress

### Regression Tests

- Verify existing agent jobs continue to work
- Verify keyword extractor example works
- Verify all job types (crawler, agent, places, etc.) work

## Risks and Mitigation

### Risk 1: Breaking Changes to Job Type

**Impact:** High - Existing queued jobs may fail
**Mitigation:**
- Implement migration script to update existing jobs in database
- Support both `"ai"` and `"agent"` temporarily during transition
- Clear communication about breaking change

### Risk 2: Import Statement Updates

**Impact:** Medium - Many files need import updates
**Mitigation:**
- Use IDE refactoring tools (automated find/replace)
- Run tests frequently during migration
- Commit changes incrementally

### Risk 3: Documentation Drift

**Impact:** Low - Docs may become outdated
**Mitigation:**
- Update docs as part of each phase
- Review all docs at end of project
- Create clear README files

## Success Criteria

1. ✅ All agent queue jobs use `"agent"` type (not `"ai"`)
2. ✅ Job definitions continue to use `type = "ai"` (JobDefinitionType)
3. ✅ Folder structure enforces separation of concerns
4. ✅ All imports updated to new paths
5. ✅ All tests pass
6. ✅ Build succeeds
7. ✅ Documentation updated and consistent
8. ✅ Clear README files in each directory

## References

- `docs/z-archive/20251125-130000-ai-job-type-refactor/plan.md` - Original AI job type refactor
- `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` - Current architecture
- `AGENTS.md` - Agent framework documentation
