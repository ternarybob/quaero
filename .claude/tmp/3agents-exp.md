---
name: 3agents
description: Opus plans/reviews, Sonnet implements, Sonnet validates against user intent.
---

Execute: $ARGUMENTS

## CONFIG
```yaml
models: 
  planner: opus      # PHASE 1: breaks down request
  worker: sonnet     # PHASE 2: implements tasks
  validator: sonnet  # PHASE 3: checks work matches user request
  reviewer: opus     # PHASE 4: security/architecture review
opus_override: [security, authentication, crypto, state-machine, architectural-change]
critical_triggers: [security, authentication, authorization, payments, data-migration, crypto, api-breaking, database-schema]
paths: { root: ".", docs: "./docs", sandbox: "/tmp/3agents/" }
```

## RULES
- Tests: `/test/api`, `/test/ui` only
- Binaries: `go build -o /tmp/` - never in root
- Make technical decisions - only stop for architecture choices
- **EVERY run creates NEW workdir** - even continuations of previous work
- **NO phase proceeds without its document written first**

---

## PHASE 0: CLASSIFY (MANDATORY)

**GATE: Cannot proceed to Phase 1 until manifest.md exists**

1. **Type**: `feature` | `fix`
2. **Slug**: kebab-case from request
3. **Date**: `YYYYMMDD` (today)
4. **Workdir**: `./docs/{type}/{date}-{slug}/`
```bash
mkdir -p ./docs/{type}/{date}-{slug}/
```

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
```

---

## PHASE 1: PLAN (opus)

**GATE: Cannot proceed to Phase 2 until plan.md + all task-N.md exist**

**WRITE `{workdir}/plan.md`:**
```markdown
# Plan: {task}
Type: {feature|fix} | Workdir: {workdir}

## User Intent (from manifest)
{copy from manifest - validator will check against this}

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | ... | - | no | sonnet |

## Order
[1] → [2,3,4] → [5]
```

**WRITE `{workdir}/task-{N}.md`** for each task:
```markdown
# Task {N}: {desc}
Depends: {ids} | Critical: {no|yes:trigger} | Model: {sonnet|opus}

## Addresses User Intent
{which part of user request this task fulfills}

## Do
- {action}

## Accept
- [ ] {criterion}
```

---

## PHASE 2: IMPLEMENT (sonnet - worker)

**GATE: Each task writes step-N.md IMMEDIATELY after completion**

For each task in dependency order:
1. Read task-{N}.md
2. Work in `/tmp/3agents/task-{N}/`
3. Execute + verify compiles
4. Copy results to source
5. **WRITE step-N.md BEFORE next task**

**WRITE `{workdir}/step-{N}.md`:**
```markdown
# Step {N}: {desc}
Model: {used} | Status: ✅|⚠️|❌

## Done
- {action}: {outcome}

## Files Changed
- `{path}` - {what}

## Build Check
Build: ✅|❌ | Tests: ✅|❌|⏭️
```

**UPDATE `{workdir}/progress.md`** after each step:
```markdown
# Progress
| Task | Status | Validated | Note |
|------|--------|-----------|------|
| 1 | ✅ | ⏳ | done, awaiting validation |
| 2 | 🔄 | - | wip |
```

---

## PHASE 3: VALIDATE (sonnet - validator)

**Purpose: Verify implementation matches user's original request**

After all tasks complete (or after each task group):

1. **Re-read manifest.md** - get User Intent + Success Criteria
2. **Read all step-N.md** - what was actually done
3. **Compare implementation to intent**
4. **Run build/tests** for technical validation

**WRITE `{workdir}/validation.md`:**
```markdown
# Validation
Validator: sonnet | Date: {timestamp}

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

## Gaps
- {missing functionality}
- {deviation from request}
- {scope creep - did more than asked}

## Technical Check
Build: ✅|❌ | Tests: ✅|❌ ({N} passed, {N} failed)

## Verdict: ✅ MATCHES | ⚠️ PARTIAL | ❌ MISMATCH
{summary of how well implementation matches user intent}

## Required Fixes (if not ✅)
1. {fix needed}
```

**Update progress.md** - mark tasks as validated.

**If MISMATCH:** Create fix tasks, return to PHASE 2.

---

## PHASE 4: REVIEW (opus) — if any critical tasks

**WRITE `{workdir}/review.md`:**
```markdown
# Review
Triggers: {list}

## Security/Architecture Issues
- {issue or "None"}

## Verdict: ✅ APPROVED | ⚠️ NOTES | ❌ CHANGES_REQUIRED
- {action item}
```

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

## Validation: {verdict}
{from validation.md}

## Review: {verdict or "N/A"}

## Verify
Build: ✅ | Tests: ✅ ({N} passed)
```

Cleanup: `rm -rf /tmp/3agents/`

---

## AGENT ROLES
```
┌─────────────────────────────────────────────────────┐
│ USER REQUEST                                        │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 0: CLASSIFY                                   │
│ Extract intent + success criteria → manifest.md     │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 1: PLAN (opus)                                │
│ Break into tasks, each linked to user intent        │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 2: IMPLEMENT (sonnet - worker)                │
│ Execute tasks, write code, step-N.md for each       │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 3: VALIDATE (sonnet - validator)              │
│ Compare implementation to user intent               │
│ Check success criteria met                          │
│ ❌ MISMATCH → loop back to PHASE 2 with fixes      │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 4: REVIEW (opus) - if critical                │
│ Security/architecture review                        │
└─────────────────┬───────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────┐
│ PHASE 5: SUMMARY                                    │
│ Final report with validation status                 │
└─────────────────────────────────────────────────────┘
```

---

## ENFORCEMENT
```
PHASE 0 → manifest.md (with intent + criteria) EXISTS? → PHASE 1
PHASE 1 → plan.md + task-*.md EXIST? → PHASE 2  
PHASE 2 → step-N.md after EACH task → progress.md updated
PHASE 3 → validation.md written → MATCHES? continue : fix loop
PHASE 4 → review.md if critical
PHASE 5 → summary.md EXISTS? → DONE
```

**HARD STOPS:**
- User decision needed
- `CHANGES_REQUIRED` verdict
- Ambiguous requirements
- `MISMATCH` after 2 fix attempts

**NOT STOPS (fix and continue):**
- Compile errors
- Test failures
- `APPROVED_WITH_NOTES`
- `PARTIAL` match (with notes)

---

## CHECKLIST (verify before declaring done)
- [ ] manifest.md with intent + success criteria
- [ ] plan.md with tasks linked to intent
- [ ] task-{N}.md for each task
- [ ] step-{N}.md for each completed task
- [ ] progress.md current
- [ ] **validation.md with intent comparison**
- [ ] review.md (if critical)
- [ ] summary.md created
- [ ] /tmp/3agents/ cleaned

**Run stops when summary.md is written. Not before.**

---

## INVOKE
```
/3agents Add JWT authentication          → ./docs/feature/20251201-jwt-authentication/
/3agents Fix the login page crash        → ./docs/fix/20251201-login-page-crash/
/3agents Continue jwt-auth work          → ./docs/feature/20251201-jwt-auth-continued/
```