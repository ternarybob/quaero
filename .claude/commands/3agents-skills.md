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
Type: {feature|fix} | Workdir: {workdir}

## User Intent (from manifest)
{copy from manifest - validator will check against this}

## Active Skills
{list from manifest, or "none"}

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | ... | - | no | sonnet | go |
| 2 | ... | 1 | no | sonnet | go |
| 3 | ... | - | no | sonnet | - |

## Order
[1,3] → [2]
```

**WRITE `{workdir}/task-{N}.md`** for each task:
```markdown
# Task {N}: {desc}
Depends: {ids} | Critical: {no|yes:trigger} | Model: {sonnet|opus} | Skill: {skill or "none"}

## Addresses User Intent
{which part of user request this task fulfills}

## Skill Patterns to Apply
{if skill assigned, list key patterns from SKILL.md to follow}
{or "N/A - no skill for this task"}

## Do
- {action}

## Accept
- [ ] {criterion}
```

---

## PHASE 2: IMPLEMENT (sonnet - worker)

**GATE: Each task writes step-N.md IMMEDIATELY after completion**

For each task in dependency order:

### Step 2.1: Load Task's Skill (if assigned)
```bash
cat .claude/skills/{task_skill}/SKILL.md
```

### Step 2.2: Execute
1. Read task-{N}.md
2. Work in `/tmp/3agents-skills/task-{N}/`
3. Apply skill patterns (if skill assigned)
4. Execute + verify compiles
5. Copy results to source
6. **WRITE step-N.md BEFORE next task**

**WRITE `{workdir}/step-{N}.md`:**
```markdown
# Step {N}: {desc}
Model: {used} | Skill: {used or "none"} | Status: ✅|⚠️|❌

## Done
- {action}: {outcome}

## Files Changed
- `{path}` - {what}

## Skill Compliance (if skill used)
- [x] {pattern followed}
- [x] {anti-pattern avoided}
- [ ] N/A - {reason}
{or "No skill applied"}

## Build Check
Build: ✅|❌ | Tests: ✅|❌|⏭️
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

After all tasks complete:

### Step 3.1: Load Context
1. **Re-read manifest.md** - get User Intent + Success Criteria + Active Skills
2. **Read all step-N.md** - what was actually done
3. **Load active skills** - for pattern verification

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
1. Create fix tasks in `{workdir}/fix-task-{N}.md`
2. Return to PHASE 2 to implement fixes
3. Re-validate after fixes
4. Max 2 fix iterations before escalating to user

---

## PHASE 4: REVIEW (opus) — if any critical tasks

**Purpose: Architectural review focused on separation of concerns and function design**

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
1. Create refactor tasks in `{workdir}/refactor-task-{N}.md`
2. Return to PHASE 2 to implement refactors
3. Re-review after changes
4. Max 2 refactor iterations before escalating to user

**Refactor tasks focus on:**
- Splitting large functions
- Extracting shared logic
- Fixing dependency direction
- Improving separation of concerns

---

## PHASE 5: SUMMARY

**GATE: Cannot complete until summary.md exists**

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
│ USER REQUEST                                        │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 0: CLASSIFY + SKILL DISCOVERY                 │
│ → manifest.md                                       │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 1: PLAN (opus)                                │
│ → plan.md + task-N.md                               │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 2: IMPLEMENT (sonnet)                         │◄──────────────┐
│ → step-N.md for each task                           │               │
└─────────────────┬───────────────────────────────────┘               │
                  ▼                                                   │
┌─────────────────────────────────────────────────────┐               │
│ PHASE 3: VALIDATE (opus)                            │               │
│ → validation.md                                     │               │
│ ❌ PARTIAL/MISMATCH → fix-task-N.md ────────────────┼───────────────┤
│ ✅ MATCHES → continue                               │               │
└─────────────────┬───────────────────────────────────┘               │
                  ▼                                                   │
┌─────────────────────────────────────────────────────┐               │
│ PHASE 4: REVIEW (opus) - if critical                │               │
│ → review.md                                         │               │
│ ❌ CHANGES_REQUIRED → refactor-task-N.md ───────────┼───────────────┘
│ ✅ APPROVED → continue                              │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 5: SUMMARY                                    │
│ → summary.md                                        │
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

**NOT STOPS (iterate and fix):**
- Compile errors → fix and retry
- Test failures → fix and retry
- `PARTIAL` match → create fix tasks, iterate
- `MISMATCH` (first time) → create fix tasks, iterate
- `CHANGES_REQUIRED` (first time) → create refactor tasks, iterate
- `APPROVED_WITH_NOTES` → proceed with notes
- No matching skills → proceed without

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