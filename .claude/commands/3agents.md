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
WORKDIR = .claude/workdir/${DATE}-${TASK_SLUG}  # e.g., ./workdir/${DATE}-${TASK_SLUG}
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
2. **Extract ALL requirements from task** - explicit AND implicit
3. Write `$WORKDIR/requirements.md`:
   ```markdown
   ## Requirements
   - [ ] REQ-1: <requirement>
   - [ ] REQ-2: <requirement>
   ...
   ## Acceptance Criteria
   - [ ] AC-1: <criterion>
   ...
   ```
4. Search codebase for existing code to reuse
5. Challenge: Does this NEED new code?
6. Write `$WORKDIR/architect-analysis.md` referencing requirements

### PHASE 1: WORKER
1. **Read `$WORKDIR/requirements.md`** - understand ALL requirements
2. Follow architect's recommendation
3. Apply refactoring skill (EXTEND > MODIFY > CREATE)
4. **Run build - must pass**
5. Write `$WORKDIR/step-{N}.md` - note which REQs addressed

### PHASE 2: VALIDATOR
1. **Run build first** - FAIL = stop
2. **Read `$WORKDIR/requirements.md`**
3. **Verify EACH requirement implemented** - check boxes in requirements.md
4. **Review code against architect-analysis.md**
5. Check skill compliance with concrete evidence
6. Check anti-creation violations
7. Write `$WORKDIR/validation-{N}.md`:
   - Requirements: [x] REQ-1, [ ] REQ-2 (missing: reason)
   - Architecture compliance: PASS/FAIL
   - Skill compliance: PASS/FAIL

### PHASE 3: ITERATE (max 5)
```
VALIDATOR FAIL → WORKER reads validation + requirements.md
                      ↓
              Address ALL issues + missing REQs
                      ↓
              VALIDATOR re-checks ALL requirements
                      ↓
              PASS (all REQs checked) → Complete
              FAIL → Loop
```

### PHASE 4: COMPLETE
- Final build verification
- Update `$WORKDIR/requirements.md` - all boxes checked [x]
- Write `$WORKDIR/summary.md` with final requirement status

## INVOKE
```
/3agents Fix the step icon mismatch
# → ./workdir/fix-step-icon-mismatch-2024-12-17/

/3agents Add log line numbering
# → ./workdir/add-log-line-numbering-2024-12-17/
```