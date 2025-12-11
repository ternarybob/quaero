---
name: 3agents-skills
description: Opus plans/reviews/validates, Sonnet implements with skills.
---

Execute: $ARGUMENTS

## CONFIG
```yaml
models: 
  planner: opus      # PHASE 1: breaks down request, selects skills
  worker: sonnet     # PHASE 2: implements tasks with skill patterns
  validator: opus    # PHASE 3: validates + creates fix tasks if needed
  reviewer: opus     # PHASE 4: architecture review + creates refactor tasks if needed
opus_override: [security, authentication, crypto, state-machine, architectural-change]
critical_triggers: [security, authentication, authorization, payments, data-migration, crypto, api-breaking, database-schema]
paths: { root: ".", docs: "./docs", sandbox: "/tmp/3agents-skills/", skills: ".claude/skills/" }
```

## RULES
- Tests: `/test/api`, `/test/ui` only
- Binaries: `go build -o /tmp/` - never in root
- Make technical decisions - only stop for architecture choices
- **EVERY run creates NEW workdir** - even continuations of previous work
- **NO phase proceeds without its document written first**
- **Skills are optional** - if no matching skill exists, proceed without
- **QUALITY OVER SPEED** - correct code aligned to requirements beats quick finish
  - Take time to understand requirements fully
  - Implement correctly the first time
  - Don't skip validation or review steps
  - Iterate until right, not until done

## CONTEXT MANAGEMENT
- **CLEAR context at start of each phase** - don't carry forward conversation history
- **Documents are the source of truth** - read from markdown, not memory
- **Each task is self-contained** - task-N.md must have ALL info needed to execute
- **Each step records everything** - step-N.md must capture full state for validation

### Context Flow
```
PHASE 0: Read $ARGUMENTS only → Write manifest.md → CLEAR
PHASE 1: Read manifest.md + skills → Write plan.md + task-N.md → CLEAR
PHASE 2: Read task-N.md + skill → Execute → Write step-N.md → CLEAR (per task)
PHASE 3: Read manifest.md + step-N.md → Write validation.md → CLEAR
PHASE 4: Read validation.md + changed files → Write review.md → CLEAR
PHASE 5: Read all docs → Write summary.md → DONE
```

---

## PHASE 0: CLASSIFY + SKILL DISCOVERY (MANDATORY)

**GATE: Cannot proceed to Phase 1 until manifest.md exists**

### Step 0.1: Classify Request
1. **Type**: `feature` | `fix`
2. **Slug**: kebab-case from request
3. **Date**: `YYYYMMDD` (today)
4. **Workdir**: `./docs/{type}/{date}-{slug}/`
```bash
mkdir -p ./docs/{type}/{date}-{slug}/
```

### Step 0.2: Discover Available Skills
```bash
# Check what skills exist in project
ls -la .claude/skills/ 2>/dev/null || echo "No skills directory"
```

For each skill found, read its SKILL.md to understand:
- What patterns it provides
- What file types it applies to
- Key rules and anti-patterns

### Step 0.3: Assess Skill Relevance
Based on $ARGUMENTS, determine which skills (if any) apply:
- Go code changes → check for `go/SKILL.md`
- Frontend/templates → check for `frontend/SKILL.md`
- Architecture changes → check for `architecture/SKILL.md`
- No matching skill → proceed without (still valid)

**WRITE `{workdir}/manifest.md`:**
```markdown
# {Type}: {Title}
- Slug: {slug} | Type: {type} | Date: {YYYY-MM-DD}
- Request: "{original input}"
- Prior: {link to previous workdir if continuation, else "none"}

## User Intent
{Restate what user wants in clear terms - this is the validation target}

## Success Criteria
- [ ] {measurable criterion from user request}
- [ ] {another criterion}

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅/❌ | ✅/❌ | {why or why not} |
| frontend | .claude/skills/frontend/SKILL.md | ✅/❌ | ✅/❌ | {why or why not} |

**Active Skills:** {list or "none - proceeding without skills"}
```

---

## PHASE 1: PLAN (opus)

**GATE: Cannot proceed to Phase 2 until plan.md + all task-N.md exist**

### Step 1.1: Load Active Skills
If skills were marked relevant in manifest:
```bash
cat .claude/skills/{skill}/SKILL.md
```

### Step 1.2: Create Plan

**WRITE `{workdir}/plan.md`:**
```markdown
# Plan: {task}
Type: {feature|fix} | Workdir: {workdir} | Date: {YYYY-MM-DD}

## Context
Project: {project name}
Related files: {list key files that will be touched}

## User Intent (from manifest)
{copy FULL user intent from manifest - validator will check against this}

## Success Criteria (from manifest)
- [ ] {criterion 1}
- [ ] {criterion 2}

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| {skill} | {2-3 most relevant patterns from SKILL.md} |
{or "none - no skills apply"}

## Technical Approach
{Brief description of HOW this will be implemented - key decisions}

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| {path} | create/modify/delete | {why} |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | ... | - | no | sonnet | go | 2 |
| 2 | ... | 1 | no | sonnet | go | 1 |
| 3 | ... | - | no | sonnet | - | 1 |

## Execution Order
[1,3] → [2]

## Risks/Decisions
- {potential issue and how it will be handled}
```

**WRITE `{workdir}/task-{N}.md`** for each task:
```markdown
# Task {N}: {desc}
Workdir: {workdir} | Depends: {ids or "none"} | Critical: {no|yes:trigger}
Model: {sonnet|opus} | Skill: {skill or "none"}

## Context
This task is part of: {brief description of overall goal from plan}
Prior tasks completed: {list or "none - this is first"}

## User Intent Addressed
{which specific part of user request this task fulfills}
{copy relevant portion from manifest}

## Input State
Files that exist before this task:
- `{path}` - {current state/purpose}

## Output State  
Files after this task completes:
- `{path}` - {expected state/changes}

## Skill Patterns to Apply
{if skill assigned:}
### From {skill}/SKILL.md:
- **DO:** {pattern 1 - be specific}
- **DO:** {pattern 2}
- **DON'T:** {anti-pattern to avoid}
- **DON'T:** {anti-pattern}
{or "N/A - no skill for this task"}

## Implementation Steps
1. {specific action with file names}
2. {next action}
3. {verification step}

## Code Specifications
{If creating/modifying code, include:}
- Function signatures expected
- Key types/interfaces involved
- Error handling approach
- Test requirements

## Accept Criteria
- [ ] {specific, verifiable criterion}
- [ ] {another criterion}
- [ ] Build passes
- [ ] {test requirement if applicable}

## Handoff
After completion, next task(s): {task IDs or "validation"}
```

---

## PHASE 2: IMPLEMENT (sonnet - worker)

**GATE: Each task writes step-N.md IMMEDIATELY after completion**

**CONTEXT: Clear before each task. Read ONLY task-N.md + skill (if assigned).**

**PRIORITY: Correct implementation over quick completion.**
- Understand the task fully before coding
- Follow skill patterns precisely
- Verify against ALL accept criteria
- If unsure, re-read task-N.md - don't guess

For each task in dependency order:

### Step 2.1: Load Task Context (fresh each time)
```bash
cat {workdir}/task-{N}.md
cat .claude/skills/{task_skill}/SKILL.md  # if skill assigned
```

### Step 2.2: Execute (self-contained)
1. Read task-{N}.md - this has ALL info needed
2. Work in `/tmp/3agents-skills/task-{N}/`
3. Apply skill patterns listed in task-{N}.md
4. Execute implementation steps from task-{N}.md
5. Verify against accept criteria in task-{N}.md
6. Copy results to source
7. **WRITE step-N.md with FULL detail**
8. **CLEAR context before next task**

**WRITE `{workdir}/step-{N}.md`:**
```markdown
# Step {N}: {desc}
Workdir: {workdir} | Model: {used} | Skill: {used or "none"}
Status: ✅ Complete | ⚠️ Partial | ❌ Failed
Timestamp: {ISO timestamp}

## Task Reference
From task-{N}.md:
- Intent: {what this task was supposed to do}
- Accept criteria: {list from task}

## Implementation Summary
{2-3 sentences describing what was actually done}

## Files Changed
| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `{path}` | created/modified/deleted | +{N}/-{N} | {what changed} |

## Code Changes Detail
### {filename}
```{lang}
// Key changes (not full file, just important parts)
{function signature or key code block}
```
**Why:** {reasoning for this implementation}

## Skill Compliance
{if skill used:}
### {skill}/SKILL.md Checklist
- [x] {pattern followed} - {where/how}
- [x] {pattern followed} - {where/how}
- [x] {anti-pattern avoided} - {evidence}
- [ ] N/A: {pattern not applicable because...}
{or "No skill applied to this task"}

## Accept Criteria Verification
- [x] {criterion 1} - {evidence}
- [x] {criterion 2} - {evidence}
- [ ] {criterion failed} - {why}

## Build & Test
```
Build: ✅ Pass | ❌ Fail ({error if failed})
Tests: ✅ Pass ({N} passed) | ❌ Fail ({N} passed, {N} failed) | ⏭️ Skipped
```

## Issues Encountered
- {issue and how it was resolved, or "None"}

## State for Next Phase
Files ready for validation:
- `{path}` - {state}

Remaining work: {none, or what's left}
```

**UPDATE `{workdir}/progress.md`** after each step:
```markdown
# Progress
| Task | Skill | Status | Validated | Note |
|------|-------|--------|-----------|------|
| 1 | go | ✅ | ⏳ | done, awaiting validation |
| 2 | go | 🔄 | - | wip |
| 3 | - | ⏳ | - | pending |
```

---

## PHASE 3: VALIDATE (opus)

**Purpose: Verify implementation matches user's original request AND skill patterns**

**CONTEXT: Clear. Read ONLY from documents, not conversation history.**

**PRIORITY: Correctness over completion.**
- Don't approve partial solutions to "move on"
- Every success criterion must be MET, not "close enough"
- Skill patterns must be followed, not approximated
- If implementation is wrong, create fix tasks - don't let it slide

After all tasks complete:

### Step 3.1: Load Context (from documents only)
```bash
cat {workdir}/manifest.md      # User intent + success criteria
cat {workdir}/step-*.md        # What was actually done
cat .claude/skills/*/SKILL.md  # For pattern verification (if skills used)
```

Do NOT rely on conversation memory. Documents are truth.

### Step 3.2: Validate

**WRITE `{workdir}/validation.md`:**
```markdown
# Validation
Validator: opus | Date: {timestamp}

## User Request
"{original request from manifest}"

## User Intent
{from manifest}

## Success Criteria Check
- [ ] {criterion 1}: ✅ MET | ⚠️ PARTIAL | ❌ NOT MET - {evidence}
- [ ] {criterion 2}: ✅ MET | ⚠️ PARTIAL | ❌ NOT MET - {evidence}

## Implementation Review
| Task | Intent | Implemented | Match |
|------|--------|-------------|-------|
| 1 | {what it should do} | {what it did} | ✅|⚠️|❌ |
| 2 | ... | ... | ... |

## Skill Compliance (if skills used)
### {skill}/SKILL.md
| Pattern | Applied | Evidence |
|---------|---------|----------|
| {pattern} | ✅|❌ | {where/how} |

{or "No skills were used for this request"}

## Gaps
- {missing functionality}
- {deviation from request}
- {skill pattern violations}

## Technical Check
Build: ✅|❌ | Tests: ✅|❌ ({N} passed, {N} failed)

## Verdict: ✅ MATCHES | ⚠️ PARTIAL | ❌ MISMATCH
{summary of how well implementation matches user intent}

## Required Fixes (if not ✅)
1. {fix needed}
```

**Update progress.md** - mark tasks as validated.

**If MISMATCH or PARTIAL:**
1. Create fix tasks in `{workdir}/fix-task-{N}.md`:
```markdown
# Fix Task {N}: {issue to fix}
Source: validation.md | Iteration: {1|2}

## Problem
{exact issue from validation.md gaps}

## Root Cause
{what went wrong in original implementation}

## Fix Required
{specific change needed}

## Files to Modify
| File | Current State | Required State |
|------|--------------|----------------|
| `{path}` | {what it does now} | {what it should do} |

## Implementation
1. {specific fix step}
2. {verification}

## Accept Criteria
- [ ] {how to verify fix works}
```
2. Return to PHASE 2 to implement fixes (CLEAR context first)
3. Re-validate after fixes
4. Max 2 fix iterations before escalating to user

---

## PHASE 4: REVIEW (opus) — if any critical tasks

**Purpose: Architectural review focused on separation of concerns and function design**

**CONTEXT: Clear. Read ONLY from documents and changed files.**

**PRIORITY: Clean architecture over quick approval.**
- Don't approve "good enough" code
- Functions must do ONE thing - no exceptions
- Separation of concerns must be maintained
- If architecture is wrong, create refactor tasks - fix it now

### Step 4.1: Load Context
```bash
cat {workdir}/validation.md    # What was validated
cat {workdir}/step-*.md        # What was changed
# Read actual changed files listed in step-*.md
```

**WRITE `{workdir}/review.md`:**
```markdown
# Review
Triggers: {list}

## Architectural Assessment

### Separation of Concerns
| Component | Responsibility | Violations |
|-----------|---------------|------------|
| {file/package} | {single purpose} | ✅ Clean | ❌ {issue} |

### Function Design
| Function | Lines | Params | Does One Thing? | Issues |
|----------|-------|--------|-----------------|--------|
| {name} | {N} | {N} | ✅|❌ | {issue or "none"} |

**Principles checked:**
- [ ] Functions do ONE thing within their context
- [ ] Functions are small and focused (<50 lines preferred)
- [ ] Parameters are minimal and meaningful
- [ ] No hidden side effects
- [ ] Clear input → output relationship
- [ ] Appropriate abstraction level

### Dependency Flow
- [ ] Dependencies flow inward (domain doesn't depend on infrastructure)
- [ ] No circular dependencies
- [ ] Interfaces at boundaries

### Security/Data Issues
- {issue or "None"}

## Skill Concerns
- {any skill-related architectural issues}

## Verdict: ✅ APPROVED | ⚠️ NOTES | ❌ CHANGES_REQUIRED

### Action Items (if any)
1. {refactor needed}
```

**If CHANGES_REQUIRED or NOTES with refactors:**
1. Create refactor tasks in `{workdir}/refactor-task-{N}.md`:
```markdown
# Refactor Task {N}: {architectural issue}
Source: review.md | Iteration: {1|2}

## Architectural Issue
{exact issue from review.md}

## Principle Violated
{which principle: separation of concerns, function size, dependency direction, etc.}

## Current State
```{lang}
// problematic code
{code snippet showing the issue}
```

## Required Refactor
{what needs to change}

## Target State
```{lang}
// expected structure (signatures, not full implementation)
{target code structure}
```

## Files to Modify
| File | Refactor Type |
|------|--------------|
| `{path}` | split function / extract interface / move logic / etc. |

## Accept Criteria
- [ ] {architectural criterion met}
- [ ] Functions < 50 lines
- [ ] Single responsibility maintained
```
2. Return to PHASE 2 to implement refactors (CLEAR context first)
3. Re-review after changes
4. Max 2 refactor iterations before escalating to user

---

## PHASE 5: SUMMARY

**GATE: Cannot complete until summary.md exists**

**CONTEXT: Clear. Read ONLY from workdir documents.**

### Step 5.1: Load All Docs
```bash
cat {workdir}/manifest.md
cat {workdir}/plan.md
cat {workdir}/step-*.md
cat {workdir}/validation.md
cat {workdir}/review.md  # if exists
```

**WRITE `{workdir}/summary.md`:**
```markdown
# Complete: {task}
Type: {feature|fix} | Tasks: {N} | Files: {N}

## User Request
"{original}"

## Result
{1-2 sentences - what was delivered}

## Skills Used
{list or "none"}

## Validation: {verdict}
{from validation.md}

## Review: {verdict or "N/A"}

## Verify
Build: ✅ | Tests: ✅ ({N} passed)
```

Cleanup: `rm -rf /tmp/3agents-skills/`

---

## AGENT ROLES
```
┌─────────────────────────────────────────────────────┐
│ USER REQUEST ($ARGUMENTS)                           │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 0: CLASSIFY + SKILL DISCOVERY                 │
│ Read: $ARGUMENTS                                    │
│ Write: manifest.md                                  │
│ [CLEAR CONTEXT]                                     │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 1: PLAN (opus)                                │
│ Read: manifest.md + skills                          │
│ Write: plan.md + task-N.md                          │
│ [CLEAR CONTEXT]                                     │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 2: IMPLEMENT (sonnet)                         │◄──────────────┐
│ For each task:                                      │               │
│   Read: task-N.md + skill                           │               │
│   Write: step-N.md                                  │               │
│   [CLEAR CONTEXT]                                   │               │
└─────────────────┬───────────────────────────────────┘               │
                  ▼                                                   │
┌─────────────────────────────────────────────────────┐               │
│ PHASE 3: VALIDATE (opus)                            │               │
│ Read: manifest.md + step-*.md + skills              │               │
│ Write: validation.md                                │               │
│ ❌ PARTIAL/MISMATCH → fix-task-N.md ────────────────┼───────────────┤
│ ✅ MATCHES → [CLEAR CONTEXT]                        │               │
└─────────────────┬───────────────────────────────────┘               │
                  ▼                                                   │
┌─────────────────────────────────────────────────────┐               │
│ PHASE 4: REVIEW (opus) - if critical                │               │
│ Read: validation.md + step-*.md + changed files     │               │
│ Write: review.md                                    │               │
│ ❌ CHANGES_REQUIRED → refactor-task-N.md ───────────┼───────────────┘
│ ✅ APPROVED → [CLEAR CONTEXT]                       │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 5: SUMMARY                                    │
│ Read: all workdir docs                              │
│ Write: summary.md                                   │
│ [DONE]                                              │
└─────────────────────────────────────────────────────┘
```

---

## ENFORCEMENT
```
PHASE 0 → manifest.md EXISTS? → PHASE 1
PHASE 1 → plan.md + task-*.md EXIST? → PHASE 2  
PHASE 2 → step-N.md after EACH task → PHASE 3
PHASE 3 → validation.md → MATCHES? → PHASE 4 (or 5)
                        → PARTIAL/MISMATCH? → fix tasks → PHASE 2 (max 2 loops)
PHASE 4 → review.md → APPROVED? → PHASE 5
                    → CHANGES_REQUIRED? → refactor tasks → PHASE 2 (max 2 loops)
PHASE 5 → summary.md EXISTS? → DONE
```

**HARD STOPS:**
- User decision needed
- Ambiguous requirements
- `MISMATCH` after 2 fix iterations
- `CHANGES_REQUIRED` after 2 refactor iterations

**NOT STOPS (iterate until correct):**
- Compile errors → fix and retry (don't skip)
- Test failures → fix and retry (don't skip tests)
- `PARTIAL` match → create fix tasks, iterate (don't accept partial)
- `MISMATCH` (first time) → create fix tasks, iterate
- `CHANGES_REQUIRED` (first time) → create refactor tasks, iterate
- `APPROVED_WITH_NOTES` → proceed with notes
- No matching skills → proceed without

**NEVER:**
- Skip validation to finish faster
- Approve partial matches to avoid iteration
- Ignore skill pattern violations
- Leave "TODO" or "FIXME" in code
- Ship code that doesn't meet accept criteria

---

## CHECKLIST (verify before declaring done)
- [ ] manifest.md with intent + success criteria + skills assessment
- [ ] plan.md with tasks linked to intent and skills
- [ ] task-{N}.md for each task (with skill assignment)
- [ ] step-{N}.md for each completed task (with skill compliance)
- [ ] progress.md current
- [ ] validation.md with intent comparison + skill compliance
- [ ] fix-task-{N}.md (if validation found issues)
- [ ] review.md (if critical)
- [ ] refactor-task-{N}.md (if review found issues)
- [ ] summary.md created
- [ ] /tmp/3agents-skills/ cleaned

**Run stops when summary.md is written. Not before.**

---

## INVOKE
```
/3agents-skills Add rate limiting to crawl jobs    → ./docs/feature/20251209-rate-limiting/
/3agents-skills Fix the queue timeout issue        → ./docs/fix/20251209-queue-timeout/
/3agents-skills Continue rate limiting work        → ./docs/feature/20251209-rate-limiting-continued/
```