---
name: 3agents
description: Opus plans/reviews, Sonnet executes. Dependency-aware task execution.
---

Execute: $ARGUMENTS

## CONFIG
```yaml
models: { planner: opus, worker: sonnet, validator: sonnet, reviewer: opus }
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
```

---

## PHASE 1: PLAN (opus)

**GATE: Cannot proceed to Phase 2 until plan.md + all task-N.md exist**

**WRITE `{workdir}/plan.md`:**
```markdown
# Plan: {task}
Type: {feature|fix} | Workdir: {workdir}

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

## Do
- {action}

## Accept
- [ ] {criterion}
```

---

## PHASE 2: EXECUTE (sonnet/opus per task)

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

## Verify
Build: ✅|❌ | Tests: ✅|❌|⏭️
```

**UPDATE `{workdir}/progress.md`** after each step:
```markdown
# Progress
| Task | Status | Note |
|------|--------|------|
| 1 | ✅ | done |
| 2 | 🔄 | wip |
```

---

## PHASE 3: VALIDATE (sonnet)
```bash
go build -o /tmp/final ./...
go test ./test/api/... ./test/ui/...
```

Update progress.md with final build/test status.

---

## PHASE 4: REVIEW (opus) — if any critical tasks

**WRITE `{workdir}/review.md`:**
```markdown
# Review
Triggers: {list}

## Issues
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

## Result
{1-2 sentences}

## Review: {verdict or "N/A"}

## Verify
Build: ✅ | Tests: ✅ ({N} passed)
```

Cleanup: `rm -rf /tmp/3agents/`

---

## ENFORCEMENT
```
PHASE 0 → manifest.md EXISTS? → PHASE 1
PHASE 1 → plan.md + task-*.md EXIST? → PHASE 2  
PHASE 2 → step-N.md after EACH task → progress.md updated
PHASE 3 → progress.md updated with results
PHASE 4 → review.md if critical
PHASE 5 → summary.md EXISTS? → DONE
```

**HARD STOPS:**
- User decision needed
- `CHANGES_REQUIRED` verdict
- Ambiguous requirements

**NOT STOPS (fix and continue):**
- Compile errors
- Test failures
- `APPROVED_WITH_NOTES`

---

## CHECKLIST (verify before declaring done)
- [ ] manifest.md created
- [ ] plan.md created
- [ ] task-{N}.md for each task
- [ ] step-{N}.md for each completed task
- [ ] progress.md current
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