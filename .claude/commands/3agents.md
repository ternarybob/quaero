---
name: 3agents
description: Adversarial 3-agent loop - CORRECTNESS over SPEED
---

Execute: $ARGUMENTS

**Read first:** `.claude/skills/refactoring/SKILL.md`

## SETUP

Create workdir from task description:
```
TASK_SLUG = slugify($ARGUMENTS)  # lowercase, hyphens, max 40 chars
DATE = $(date +%Y-%m-%d)
WORKDIR = .claude/workdir/${TASK_SLUG}-${DATE}  # e.g., ./workdir/${TASK_SLUG}-${DATE}
mkdir -p $WORKDIR
```

## AGENTS

1. **ARCHITECT** - Finds EXISTING code, blocks unnecessary creation
2. **WORKER** - Implements via modification, runs build
3. **VALIDATOR** - Assumes REJECT, requires proof, runs build independently

## RULES

- Agents are ADVERSARIAL - challenge, don't agree
- Follow ALL patterns in `.claude/skills/refactoring/SKILL.md`
- Apply `.claude/skills/go/SKILL.md` for Go changes
- Apply `.claude/skills/frontend/SKILL.md` for frontend changes
- Apply `.claude/skills/monitoring/SKILL.md` for UI test changes (screenshots, monitoring, results)
- NEVER modify tests to make code pass
- Iterate until CORRECT, not "good enough"

## WORKFLOW

### PHASE 0: ARCHITECT
1. Read architecture docs and skills
2. Search codebase for existing code to reuse
3. Challenge: Does this NEED new code?
4. Write `$WORKDIR/architect-analysis.md`

### PHASE 1: WORKER
1. Follow architect's recommendation
2. Apply refactoring skill (EXTEND > MODIFY > CREATE)
3. **Run build - must pass**
4. Write `$WORKDIR/step-{N}.md`

### PHASE 2: VALIDATOR
1. **Run build first** - FAIL = stop
2. Verify skill compliance with concrete evidence
3. Check anti-creation violations
4. Write `$WORKDIR/validation-{N}.md`

### PHASE 3: ITERATE (max 5)
```
VALIDATOR FAIL
     │
     ▼
┌─────────────────────────┐
│ WORKER                  │
│ • Read ALL violations   │
│ • Address EVERY issue   │
│ • Apply skills          │
│ • Run build             │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ VALIDATOR               │
│ • Run build             │
│ • Equally harsh         │
│ • Verify ALL fixed      │
└───────────┬─────────────┘
     ┌──────┴──────┐
     ▼             ▼
   FAIL          PASS → Complete
     │
     └──► Loop
```

### PHASE 4: COMPLETE
- Final build verification
- Write `$WORKDIR/summary.md`

## INVOKE
```
/3agents Fix the step icon mismatch
# → ./workdir/fix-step-icon-mismatch-2024-12-17/

/3agents Add log line numbering
# → ./workdir/add-log-line-numbering-2024-12-17/
```