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
- Document as you go - write all output files

---

## PHASE 0: CLASSIFY

Analyse user input to determine:

1. **Type**: `feature` or `fix`
   - `feature`: New functionality, enhancements, additions
   - `fix`: Bug repairs, corrections, patches, resolving issues

2. **Slug**: kebab-case name from request (e.g., "Add JWT auth" → `jwt-auth`, "Fix login crash" → `login-crash`)

3. **Date**: Current date as `YYYYMMDD`

4. **Workdir**: `./docs/{type}/{date}-{slug}/`

Create workdir:
```bash
mkdir -p ./docs/{type}/{date}-{slug}/
```

Create `{workdir}/manifest.md`:
```markdown
# {Type}: {Title}
- Slug: {slug}
- Type: {feature|fix}
- Date: {YYYY-MM-DD}
- Created: {timestamp}
- Request: "{original user input}"
```

---

## PHASE 1: PLAN (opus)

Create `{workdir}/plan.md`:
```markdown
# Plan: {task}

## Classification
- Type: {feature|fix}
- Workdir: {workdir}

## Analysis
{deps, approach, risks}

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | ... | none | no | low | sonnet |
| 2 | ... | 1 | yes:security | high | opus |

## Order
Sequential: [1] → Concurrent: [2,3,4] → Sequential: [5] → Review
```

Create `{workdir}/task-{N}.md` for each:
```markdown
# Task {N}: {desc}
- Group: {N} | Mode: sequential|concurrent | Model: sonnet|opus
- Skill: @{skill} | Critical: no|yes:{trigger} | Depends: {ids}
- Sandbox: /tmp/3agents/task-{N}/ | Source: {root}/ | Output: {workdir}/

## Files
- `{path}` - {action}

## Requirements
{what to do}

## Acceptance
- [ ] {criterion}
- [ ] Compiles
- [ ] Tests pass
```

---

## PHASE 2: EXECUTE (sonnet default, opus if complex)

For each task in dependency order:
1. Read `task-{N}.md`
2. `mkdir -p /tmp/3agents/task-{N}/` + copy files
3. Execute in sandbox
4. `go build -o /tmp/test ./...`
5. Copy changed files back to source
6. Write `step-{N}.md` + update `progress.md`

Create `{workdir}/step-{N}.md`:
```markdown
# Step {N}: {desc}
- Task: task-{N}.md | Group: {N} | Model: {used}

## Actions
1. {did}

## Files
- `{path}` - {change}

## Decisions
- {choice}: {why}

## Verify
Compile: ✅|❌ | Tests: ✅|❌|⚙️

## Status: ✅ COMPLETE | ⚠️ PARTIAL | ❌ BLOCKED
```

Update `{workdir}/progress.md`:
```markdown
# Progress
| Task | Status | Notes |
|------|--------|-------|
| 1 | ✅ | |
| 2 | 🔄 | in progress |

Deps: [x] 1→[2,3,4] [ ] 4→[5]
```

---

## PHASE 3: VALIDATE (sonnet)
```bash
go build -o /tmp/final ./...
go test ./test/api/... ./test/ui/...
```
Update progress.md with results.

---

## PHASE 4: REVIEW (opus)

Run if any `Critical: yes:{trigger}` in plan.

Create `{workdir}/final-review.md`:
```markdown
# Review: {task}
Triggers: {list} | Files: {N}

## Security
Critical: {issues|None} | Warnings: {list}

## Architecture
Breaking: {assessment} | Migration: {steps}

## Verdict
**Status:** ✅ APPROVED | ⚠️ APPROVED_WITH_NOTES | ❌ CHANGES_REQUIRED
Actions: 1. {item}
```

---

## PHASE 5: SUMMARY (sonnet)

Create `{workdir}/summary.md`:
```markdown
# Complete: {task}

## Classification
- Type: {feature|fix}
- Location: {workdir}

{one paragraph overview}

## Stats
Tasks: {N} | Files: {N} | Duration: {time}
Models: Planning=opus, Workers={N}×sonnet/{N}×opus, Review=opus

## Tasks
- Task 1: {summary}
- Task 2-4 (concurrent): {summaries}

## Review: {verdict}
Actions: {list}

## Verify
go build ✅ | go test ✅ {N} passed
```

Cleanup: `rm -rf /tmp/3agents/`

---

## STOP
**Stop:** User decision | `CHANGES_REQUIRED` | Ambiguous requirements
**Continue:** Next step | Compile/test errors (fix or document) | `APPROVED_WITH_NOTES`

---

## CHECKLIST
- [ ] CLASSIFY: Determine feature/fix + slug + date → create workdir
- [ ] Create manifest.md
- [ ] PLAN: plan.md + task-{N}.md files
- [ ] EXECUTE: step-{N}.md + progress.md per task
- [ ] VALIDATE: full build + test
- [ ] REVIEW: final-review.md (if critical)
- [ ] SUMMARY: summary.md + cleanup

**Do not stop until summary.md exists.**

## INVOKE
```
/3agents Add JWT authentication          → ./docs/feature/20251129-jwt-authentication/
/3agents Fix the login page crash        → ./docs/fix/20251129-login-page-crash/
/3agents docs/feature/20251129-jwt-auth/plan.md   → Resume existing plan
```
```

Changes:

1. **Date format `YYYYMMDD`** — Added to PHASE 0 classification
2. **Workdir pattern** — Now `./docs/{type}/{date}-{slug}/`
3. **manifest.md** — Includes both the compact `YYYYMMDD` in the path and human-readable `YYYY-MM-DD` in the metadata
4. **Invoke examples** — Updated to show the date-prefixed paths

This gives you nice chronological sorting when listing directories, e.g.:
```
./docs/feature/
├── 20251125-user-profiles/
├── 20251127-api-rate-limiting/
└── 20251129-jwt-authentication/