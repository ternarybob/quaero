---
name: 3agents
description: Three-phase workflow. Opus plans/reviews, Sonnet executes (with Opus override for complex tasks).
---

Execute workflow for: $ARGUMENTS

## CONFIG

```yaml
models:
  planner: opus
  worker: sonnet
  validator: sonnet
  reviewer: opus

# Override worker to Opus for high-complexity tasks
complexity_override:
  high:
    model: opus
    indicators:
      - security
      - authentication
      - crypto
      - state machine
      - complex algorithm
      - architectural change

skills:
  code-architect: [architecture, design, refactoring]
  go-coder: [implementation, handlers, functions]
  test-writer: [tests, coverage]
  none: [documentation, planning]

critical_triggers:
  - security
  - authentication
  - authorization
  - payments
  - data-migration
  - crypto
  - api-breaking
  - database-schema
```

## RULES

- **Tests:** Only `/test/api` and `/test/ui`
- **Binaries:** `go build -o /tmp/` or `go run` - never in root
- **Decisions:** Make technical decisions - only stop for architecture choices
- **Complete:** Run ALL phases to completion
- **Document:** Write output files as you go - this is your audit trail

---

## WORKDIR SETUP

```yaml
# Define paths FIRST - used by all phases
paths:
  project_root: "."                                    # Where source code lives
  output: "{workdir}/"                                 # All documentation goes here
  sandbox_base: "/tmp/3agents/"                        # Isolated work areas
  
# Derived per-task:
  task_sandbox: "/tmp/3agents/task-{N}/"              # Each task gets isolation
```

```bash
# If $ARGUMENTS is a file path
DIR=$(dirname "$ARGUMENTS")
BASE=$(basename "$ARGUMENTS" .md)
WORKDIR="${DIR}/$(date +%Y%m%d-%H%M%S)-${BASE}/"

# If $ARGUMENTS is a task description  
WORKDIR="docs/features/$(date +%Y%m%d-%H%M%S)-${SLUG}/"

# Create output directory
mkdir -p "$WORKDIR"

# Create sandbox base
mkdir -p /tmp/3agents/
```

**Final workdir structure:**
```
{workdir}/
├── plan.md              # Dependency graph, execution groups
├── task-1.md            # Task 1 instructions (input)
├── task-2.md            # Task 2 instructions (input)
├── task-N.md            # ...
├── step-1.md            # Task 1 results (output)
├── step-2.md            # Task 2 results (output)
├── step-N.md            # ...
├── progress.md          # Live status tracking
├── final-review.md      # Security/architecture review
└── summary.md           # Final summary
```

**All paths are set before Phase 1 begins.**

---

## PHASE 1: PLAN (Opus)

**Model:** `claude-opus-4-5-20251101` - deep thinking for dependency analysis

**Think deeply before planning:**
1. What are ALL discrete tasks?
2. What depends on what? (dependency graph)
3. Which tasks can run CONCURRENTLY? (no shared dependencies)
4. Which must run SEQUENTIALLY? (has dependencies)
5. What's the critical path?
6. What triggers final review?

### 1.1 Create Plan

**Create: `{workdir}/plan.md`**

```markdown
# Plan: {task}

## Analysis
{dependencies, approach, risks}

## Dependency Graph
```
[1: Setup] 
    ↓
[2: Types] ──────┐
    ↓            │
[3: Handlers] ←──┤  (2,3,4 can run concurrently after 1)
    ↓            │
[4: Tests] ──────┘
    ↓
[5: Integration] (requires 2,3,4)
```

## Execution Groups

### Group 1: Sequential (Foundation)
Must complete before any concurrent work.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 1 | {desc} | none | no | low | Sonnet |

### Group 2: Concurrent
Can run in parallel after Group 1 completes.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 2 | {desc} | 1 | no | medium | Sonnet |
| 3 | {desc} | 1 | yes:security | high | **Opus** |
| 4 | {desc} | 1 | no | low | Sonnet |

### Group 3: Sequential (Integration)
Requires all concurrent tasks complete.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 5 | {desc} | 2,3,4 | yes:api-breaking | high | **Opus** |

## Execution Order
```
Sequential: [1]
Concurrent: [2] [3] [4]  ← can run in parallel
Sequential: [5] → [Final Review]
```

## Success Criteria
- {condition}
- {condition}
```

### 1.2 Generate Task Files

**For EACH task, create: `{workdir}/task-{N}.md`**

This is the instruction file an agent reads to execute the task.

```markdown
# Task {N}: {Description}

## Metadata
- **ID:** {N}
- **Group:** {1|2|3}
- **Mode:** sequential | concurrent
- **Skill:** @{skill}
- **Complexity:** low | medium | high
- **Model:** claude-sonnet-4-5-20250929 | claude-opus-4-5-20251101
- **Critical:** no | yes:{trigger}
- **Depends:** {task IDs or "none"}
- **Blocks:** {task IDs that depend on this}

## Model Selection
```yaml
# Default: Sonnet for standard work
model: claude-sonnet-4-5-20250929

# Override to Opus if ANY of:
# - Complexity: high
# - Critical: yes:security|authentication|crypto
# - Involves: state machine, complex algorithm, architectural change
```

## Paths
```yaml
sandbox: /tmp/3agents/task-{N}/
source: {project_root}/
output: {workdir}/
```

## Files to Modify
- `{path}` - {what to do}
- `{path}` - {what to do}

## Requirements
{Detailed description of what this task must accomplish}

## Acceptance Criteria
- [ ] {specific criterion}
- [ ] {specific criterion}
- [ ] Compiles successfully
- [ ] Tests pass (if applicable)

## Context
{Any relevant background, patterns to follow, constraints}

## Dependencies Input
{What this task receives from dependencies - or "N/A"}

## Output for Dependents  
{What this task produces that other tasks need - or "N/A"}
```

### 1.3 Plan Output Summary

After planning, workdir contains:
```
{workdir}/
├── plan.md           # Overall plan with dependency graph
├── task-1.md         # Task 1 instructions
├── task-2.md         # Task 2 instructions (concurrent)
├── task-3.md         # Task 3 instructions (concurrent)
├── task-4.md         # Task 4 instructions (concurrent)
└── task-5.md         # Task 5 instructions
```

---

## PHASE 2: EXECUTE (Sonnet default)

**Default Model:** `claude-sonnet-4-5-20250929` - fast execution
**Override to Opus:** High complexity or security-critical tasks

Execute tasks by reading task-{N}.md files, respecting dependency order.

### 2.0 Execution Order

```yaml
# Read from plan.md
groups:
  - group: 1
    mode: sequential
    tasks: [1]
    
  - group: 2
    mode: concurrent      # Note: Claude executes these one-by-one but they COULD parallelize
    tasks: [2, 3, 4]
    
  - group: 3
    mode: sequential
    tasks: [5]
```

**For concurrent groups:** Tasks have no interdependencies, so execution order within the group doesn't matter. In a multi-agent setup, these would run in parallel.

### 2.1 For EACH Task

1. **Read** `task-{N}.md` for instructions
2. **Setup sandbox:** `/tmp/3agents/task-{N}/`
3. **Copy** required files from `$source` to sandbox
4. **Execute** the task per instructions
5. **Verify** in sandbox (compile/test)
6. **Merge** changed files back to `$source`
7. **Write** `step-{N}.md` with results to `$output`
8. **Update** `progress.md`

### 2.2 Sandbox Setup (per task)

```yaml
params:
  task_id: {N}
  sandbox: "/tmp/3agents/task-{N}/"
  source: "{project_root}/"
  output: "{workdir}/"
  task_file: "{workdir}/task-{N}.md"    # Instructions to follow
```

```bash
# Before each task:
mkdir -p /tmp/3agents/task-{N}/
cp -r $source/{files_from_task_file} /tmp/3agents/task-{N}/
```

### 2.3 Execute Task

1. Read `task-{N}.md` for requirements
2. Work entirely in `$sandbox`
3. Compile: `cd $sandbox && go build -o /tmp/test ./...`
4. Test if applicable
5. On success: copy changed files back to `$source`

### 2.4 Write Step Results

**Create: `$output/step-{N}.md`** (execution results, NOT in sandbox)

```markdown
# Step {N}: {Description}

## Task Reference
- **Task File:** task-{N}.md
- **Group:** {N} ({sequential|concurrent})
- **Model Used:** claude-sonnet-4-5-20250929 | claude-opus-4-5-20251101
- **Dependencies:** {satisfied by steps X, Y}

## Params
- Sandbox: `/tmp/3agents/task-{N}/`
- Source: `{project_root}/`
- Output: `{workdir}/`

## Actions Taken
1. {what you did}
2. {what you did}

## Files Modified
- `{path}` - {what changed}
- `{path}` - {what changed}

## Decisions Made
- **{choice}**: {rationale}
- **{choice}**: {rationale}

## Acceptance Criteria
- [x] {criterion from task file}
- [x] {criterion from task file}
- [x] Compiles successfully
- [ ] Tests pass ← {if failed, note why}

## Verification
```bash
# Compilation (in sandbox)
cd /tmp/3agents/task-{N}/ && go build -o /tmp/test ./...
# Result: ✅ Pass | ❌ Fail: {error}

# Tests
go test ./... -run {relevant}
# Result: ✅ Pass | ❌ Fail | ⚙️ Skipped
```

## Merge Back
```bash
cp $sandbox/{file} $source/{file}
```

## Output for Dependents
{What downstream tasks can now use}

## Issues/Notes
- {any concerns, TODOs}

## Status: ✅ COMPLETE | ⚠️ PARTIAL | ❌ BLOCKED
```

### 2.5 Update Progress

**Update: `{workdir}/progress.md`**

```markdown
# Progress: {task}

Started: {timestamp}

## Group 1: Sequential (Foundation)
| Task | Description | Status | Quality | Notes |
|------|-------------|--------|---------|-------|
| 1 | {desc} | ✅ | 9/10 | |

## Group 2: Concurrent
| Task | Description | Status | Quality | Notes |
|------|-------------|--------|---------|-------|
| 2 | {desc} | ✅ | 8/10 | |
| 3 | {desc} | ✅ | 8/10 | Security review needed |
| 4 | {desc} | 🔄 | - | In progress |

## Group 3: Sequential (Integration)
| Task | Description | Status | Quality | Notes |
|------|-------------|--------|---------|-------|
| 5 | {desc} | ⏳ | - | Waiting on Group 2 |

## Dependency Status
- [x] Task 1 complete → unblocks [2,3,4]
- [x] Task 2 complete → unblocks [5]
- [x] Task 3 complete → unblocks [5]
- [ ] Task 4 in progress → blocks [5]

Last updated: {timestamp}
```

### 2.6 Handle Failures

- **Compilation fails:** Fix and retry (max 2), document in step file
- **Tests fail:** Fix if obvious, otherwise document and continue
- **Blocked:** Document why, skip if non-critical dependency

---

## PHASE 3: VALIDATE (Sonnet)

**Model:** `claude-sonnet-4-5-20250929` - quick validation

After ALL steps complete:

```yaml
params:
  source: "{project_root}/"           # Validate against merged source
  output: "{workdir}/"                # Write results here
  test_dirs: ["test/api", "test/ui"]
```

### 3.1 Full Compilation

```bash
cd $source && go build -o /tmp/final ./...
```

### 3.2 Full Test Suite

```bash
cd $source && go test ./test/api/... ./test/ui/...
```

### 3.3 Document Results

**Update: `$output/progress.md`** with final verification status

---

## PHASE 4: FINAL REVIEW (Opus)

**Model:** `claude-opus-4-5-20251101` - deep security/architecture review

**Run if ANY step has `Critical: yes:{trigger}`**

```yaml
params:
  source: "{project_root}/"           # Review merged source files
  output: "{workdir}/"                # Write review here
  triggers: ["{from plan}"]           # What triggered review
  changed_files: ["{all modified}"]   # Files to review
```

Review all changes for:
- Security vulnerabilities
- Architecture issues
- Breaking change impact
- Migration requirements

**Create: `$output/final-review.md`**

```markdown
# Final Review: {task}

## Scope
- Triggers: {list from plan}
- Steps reviewed: {N}
- Files changed: {N}

## Security Findings

### Critical Issues
{must fix before merge - or "None"}

### Warnings
- {concern}

### Passed
- {check}

## Architecture Findings

### Breaking Changes
{impact assessment}

### Migration Required
{steps if any}

## Code Quality
- {observation}
- {recommendation}

## Verdict

**Status:** ✅ APPROVED | ⚠️ APPROVED_WITH_NOTES | ❌ CHANGES_REQUIRED

### Required Actions (if CHANGES_REQUIRED)
1. {must do}

### Recommended Actions
1. [ ] {should do}
```

---

## PHASE 5: SUMMARY (Sonnet)

**Model:** `claude-sonnet-4-5-20250929` - summarization

```yaml
params:
  output: "{workdir}/"                # Write summary here
  source: "{project_root}/"           # List changed files from here
  sandbox_base: "/tmp/3agents/"       # Clean up after
```

**Create: `$output/summary.md`**

```markdown
# Complete: {task}

## Overview
{one paragraph summary of what was done}

## Execution Structure
```
Group 1 (Sequential):  [1] ✅
Group 2 (Concurrent):  [2] ✅  [3] ✅  [4] ✅
Group 3 (Sequential):  [5] ✅
Final Review:          ✅ APPROVED_WITH_NOTES
```

## Stats
| Metric | Value |
|--------|-------|
| Total Tasks | {N} |
| Sequential | {N} |
| Concurrent | {N} |
| Files Changed | {N} |
| Duration | {time} |
| Quality | {avg}/10 |

## Model Usage
| Phase | Model | Count |
|-------|-------|-------|
| Planning | Opus | 1 |
| Workers (standard) | Sonnet | {N} |
| Workers (complex) | Opus | {N} |
| Validation | Sonnet | 1 |
| Final Review | Opus | 1 |

## Task Summaries

### Task 1: {title} (Sequential)
{key points from step-1.md}

### Tasks 2-4: {titles} (Concurrent)
These tasks had no interdependencies:
- **Task 2:** {summary}
- **Task 3:** {summary}  
- **Task 4:** {summary}

### Task 5: {title} (Sequential - Integration)
{key points from step-5.md}

## Dependency Flow
```
[1] → [2,3,4] → [5] → [Review]
     (parallel)
```

## Final Review
**Status:** {verdict}
**Triggers:** {list}

### Action Items
1. [ ] {from final review}

## Verification
```bash
go build ./...     # ✅ Pass
go test ./test/... # ✅ {N} passed, {N} failed
```

## Files Modified
```
{tree or list of all changed files}
```

## Workdir Contents
```
{workdir}/
├── plan.md
├── task-1.md ... task-{N}.md    # Task instructions
├── step-1.md ... step-{N}.md    # Execution results
├── progress.md
├── final-review.md
└── summary.md
```

## Completed: {ISO8601}
```

---

## STOP CONDITIONS

### STOP for:
- `User decision: yes` in plan step
- Final review verdict: `CHANGES_REQUIRED`
- Ambiguous requirements (can't determine what to build)

### NEVER stop for:
- Moving to next step
- Compilation errors (fix or document)
- Test failures (fix or document)
- `APPROVED_WITH_NOTES` (continue, log warnings)

---

## EXECUTION CHECKLIST

Claude must complete ALL of these:

- [ ] Create workdir (`$output`)
- [ ] Create sandbox base (`/tmp/3agents/`)
- [ ] **PHASE 1: PLAN**
  - [ ] Analyze dependencies (what depends on what)
  - [ ] Identify concurrent vs sequential tasks
  - [ ] Write plan.md with dependency graph
  - [ ] Generate task-{N}.md for each task
- [ ] **PHASE 2: EXECUTE** (respect dependency order)
  - [ ] Execute Group 1 (sequential) tasks first
  - [ ] Execute Group 2 (concurrent) tasks
  - [ ] Execute Group 3+ (sequential) tasks
  - [ ] For each task:
    - [ ] Read task-{N}.md
    - [ ] Create sandbox `/tmp/3agents/task-{N}/`
    - [ ] Copy files, execute, verify
    - [ ] Merge back to `$source`
    - [ ] Write step-{N}.md to `$output`
    - [ ] Update progress.md
- [ ] **PHASE 3: VALIDATE** against `$source`
- [ ] **PHASE 4: FINAL REVIEW** (if critical triggers)
- [ ] **PHASE 5: SUMMARY**
  - [ ] Write summary.md
  - [ ] Cleanup: `rm -rf /tmp/3agents/`

**Do not stop until summary.md is written.**

---

## INVOKE

```
/3agents Add JWT authentication
/3agents docs/fixes/01-plan-xyz.md
```