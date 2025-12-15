---
name: 3agents
description: Three-phase workflow. Opus plans, executes, validates - all inline.
---

Execute workflow for: $ARGUMENTS

## CONFIG

```yaml
model: claude-opus-4-5-20251101

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

```bash
# If $ARGUMENTS is a file path
DIR=$(dirname "$ARGUMENTS")
BASE=$(basename "$ARGUMENTS" .md)
WORKDIR="${DIR}/$(date +%Y%m%d-%H%M%S)-${BASE}/"

# If $ARGUMENTS is a task description  
WORKDIR="docs/features/$(date +%Y%m%d-%H%M%S)-${SLUG}/"
```

Create the workdir before starting.

---

## PHASE 1: PLAN

**Think deeply before planning:**
1. What are ALL discrete tasks?
2. What depends on what?
3. What's the critical path?
4. What triggers final review?

**Create: `{workdir}/plan.md`**

```markdown
# Plan: {task}

## Analysis
{dependencies, approach, risks}

## Steps

### Step 1: {Description}
- Skill: @{skill}
- Files: {paths}
- Critical: no | yes:{trigger}
- Depends: none

### Step 2: {Description}
- Skill: @{skill}
- Files: {paths}
- Critical: yes:security
- Depends: Step 1

### Step 3: {Description}
- Skill: @{skill}
- Files: {paths}
- Critical: yes:api-breaking
- Depends: Step 2
- User decision: {if needed}

## Execution Order
1 → 2 → 3 → Final Review (if critical)

## Success Criteria
- {condition}
- {condition}
```

---

## PHASE 2: EXECUTE

For EACH step in plan.md:

### 2.1 Execute the Step

Do the implementation work.

### 2.2 Write Step File

**Create: `{workdir}/step-{N}.md`**

```markdown
# Step {N}: {Description}

## Actions Taken
1. {what you did}
2. {what you did}

## Files Modified
- `{path}` - {what changed}
- `{path}` - {what changed}

## Decisions Made
- **{choice}**: {rationale}
- **{choice}**: {rationale}

## Verification
```bash
# Compilation
go build -o /tmp/test ./...
# Result: ✅ Pass | ❌ Fail: {error}

# Tests (if applicable)
go test ./test/api/... -run {relevant}
# Result: ✅ Pass | ❌ Fail | ⚙️ Skipped
```

## Issues/Notes
- {any concerns, TODOs, or observations}

## Status: ✅ COMPLETE | ⚠️ PARTIAL | ❌ BLOCKED
```

### 2.3 Update Progress

**Create/Update: `{workdir}/progress.md`**

```markdown
# Progress: {task}

Started: {timestamp}

| Step | Description | Status | Quality | Notes |
|------|-------------|--------|---------|-------|
| 1 | {desc} | ✅ | 9/10 | |
| 2 | {desc} | ✅ | 8/10 | Minor TODO |
| 3 | {desc} | 🔄 | - | In progress |
| 4 | {desc} | ⏳ | - | Waiting |

Last updated: {timestamp}
```

### 2.4 Handle Failures

- **Compilation fails:** Fix and retry (max 2 attempts), document in step file
- **Tests fail:** Fix if obvious, otherwise document and continue
- **Blocked:** Document why, continue to next step if possible

---

## PHASE 3: VALIDATE

After ALL steps complete:

1. **Full compilation check**
   ```bash
   go build -o /tmp/final ./...
   ```

2. **Full test suite**
   ```bash
   go test ./test/api/... ./test/ui/...
   ```

3. **Document results in progress.md**

---

## PHASE 4: FINAL REVIEW

**Run if ANY step has `Critical: yes:{trigger}`**

Review all changes for:
- Security vulnerabilities
- Architecture issues
- Breaking change impact
- Migration requirements

**Create: `{workdir}/final-review.md`**

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

## PHASE 5: SUMMARY

**Create: `{workdir}/summary.md`**

```markdown
# Complete: {task}

## Overview
{one paragraph summary of what was done}

## Stats
| Metric | Value |
|--------|-------|
| Steps | {N} |
| Files Changed | {N} |
| Duration | {time} |
| Quality | {avg}/10 |

## Changes by Step

### Step 1: {title}
{copy key points from step-1.md}

### Step 2: {title}
{copy key points from step-2.md}

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

- [ ] Create workdir
- [ ] Write plan.md
- [ ] For each step:
  - [ ] Execute implementation
  - [ ] Write step-{N}.md
  - [ ] Update progress.md
  - [ ] Verify (compile/test)
- [ ] Run final validation
- [ ] Write final-review.md (if critical triggers)
- [ ] Write summary.md

**Do not stop until summary.md is written.**

---

## INVOKE

```
/3agents Add JWT authentication
/3agents docs/fixes/01-plan-xyz.md
```