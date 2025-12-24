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
TIME = $(date +%H%M)
WORKDIR = .claude/workdir/${DATE}-${TIME}-${TASK_SLUG}
mkdir -p $WORKDIR
```

## AGENTS

1. **ARCHITECT** - Assesses requirements, architecture, creates step documentation
2. **WORKER** - Implements steps according to step documentation
3. **VALIDATOR** - Reviews completed step and code against step documentation

**WORKER and VALIDATOR are ADVERSARIAL** - WORKER must rework if VALIDATOR finds misaligned code.

## RULES

- Agents are ADVERSARIAL - challenge, don't agree
- Follow ALL patterns in `.claude/skills/refactoring/SKILL.md`
- Apply `.claude/skills/go/SKILL.md` for Go changes
- Apply `.claude/skills/frontend/SKILL.md` for frontend changes
- Apply `.claude/skills/monitoring/SKILL.md` for UI test changes (screenshots, monitoring, results)
- **Update skills** where they don't match requirements or discovered patterns
- NEVER modify tests to make code pass
- Iterate until CORRECT, not "good enough"

## WORKFLOW

### PHASE 0: ARCHITECT

**Purpose:** Assess requirements, review architecture, create step-by-step implementation plan.

1. **Read architecture docs:**
   - `docs/architecture/ARCHITECTURE.md`
   - `docs/architecture/README.md`
   - Relevant architecture docs for the task domain
2. **Read applicable skills:**
   - `.claude/skills/refactoring/SKILL.md`
   - Domain-specific skills (go, frontend, monitoring)
3. **Extract ALL requirements** - explicit AND implicit
4. **Write `$WORKDIR/requirements.md`:**
   ```markdown
   ## Requirements
   - [ ] REQ-1: <requirement>
   - [ ] REQ-2: <requirement>
   ...
   ## Acceptance Criteria
   - [ ] AC-1: <criterion>
   ...
   ```
5. **Search codebase** for existing code to reuse
6. **Challenge:** Does this NEED new code?
7. **Create step documentation** - Break work into discrete steps:

   **Write `$WORKDIR/step_1.md`:**
   ```markdown
   # Step 1: <step title>

   ## Objective
   <What this step accomplishes>

   ## Requirements Addressed
   - REQ-1: <requirement>
   - REQ-2: <requirement>

   ## Architecture Context
   - Relevant patterns from docs/architecture
   - Skills to apply

   ## Implementation Approach
   - Files to modify/create
   - Specific changes needed
   - Code patterns to follow

   ## Acceptance Criteria
   - [ ] AC-1: <specific, testable criterion>
   - [ ] AC-2: <specific, testable criterion>

   ## Build/Test Requirements
   - Commands to run
   - Expected outcomes
   ```

   Repeat for `step_2.md`, `step_3.md`, etc.

8. **Write `$WORKDIR/architect-analysis.md`:**
   ```markdown
   ## Overview
   <High-level analysis>

   ## Steps Summary
   1. Step 1: <title> - REQs: 1, 2
   2. Step 2: <title> - REQs: 3
   ...

   ## Architecture Decisions
   - <Decision and rationale>

   ## Skill Updates Required
   - <Skill path>: <proposed update reason>
   ```

### PHASE 1: WORKER

**Purpose:** Implement ONE step at a time according to step documentation.

For each step (starting with `step_1.md`):

1. **Read the step documentation** (`$WORKDIR/step_N.md`)
2. **Understand scope** - ONLY implement what's in this step
3. **Apply refactoring skill** (EXTEND > MODIFY > CREATE)
4. **Follow architecture patterns** specified in step doc
5. **Run build - must pass**
6. **Document work in `$WORKDIR/step_N_implementation.md`:**
   ```markdown
   # Step N Implementation

   ## Files Changed
   - `path/to/file.go`: <what changed>

   ## Implementation Details
   - <Key decisions made>
   - <Patterns followed>

   ## Acceptance Criteria Status
   - [x] AC-1: <evidence>
   - [x] AC-2: <evidence>

   ## Build Status
   - Command: `<build command>`
   - Result: PASS

   ## Ready for Validation
   ```

### PHASE 2: VALIDATOR

**Purpose:** Review completed step against step documentation. **Default stance: REJECT until proven correct.**

For the current step:

1. **Run build first** - FAIL = immediate REJECT
2. **Read step documentation** (`$WORKDIR/step_N.md`)
3. **Read implementation doc** (`$WORKDIR/step_N_implementation.md`)
4. **Verify EACH acceptance criterion** with concrete evidence
5. **Review code against:**
   - Step documentation requirements
   - Architecture patterns from `architect-analysis.md`
   - Applicable skills
6. **Check for violations:**
   - Anti-creation violations (unnecessary new files)
   - Skill non-compliance
   - Architecture misalignment
7. **Write `$WORKDIR/step_N_validation.md`:**
   ```markdown
   # Step N Validation

   ## Build Status
   - Command: `<command>`
   - Result: PASS/FAIL

   ## Acceptance Criteria
   - [x] AC-1: PASS - <evidence>
   - [ ] AC-2: FAIL - <reason>

   ## Architecture Compliance
   - Status: PASS/FAIL
   - Evidence: <specific code references>

   ## Skill Compliance
   - Status: PASS/FAIL
   - Evidence: <specific patterns checked>

   ## Issues Found
   1. <Issue description>
      - Expected: <what should be>
      - Actual: <what is>
      - Fix required: <specific action>

   ## Verdict: PASS/REJECT

   ## Skill Updates Identified
   - <Skill path>: <pattern that should be documented>
   ```

### PHASE 3: ITERATE (per step, max 5 iterations)

```
VALIDATOR REJECT → WORKER reads step_N_validation.md
                        ↓
                Address ALL issues listed
                        ↓
                Update step_N_implementation.md
                        ↓
                VALIDATOR re-validates
                        ↓
                PASS → Move to next step (back to WORKER)
                REJECT → Loop (max 5)
```

**After each step PASS:**
- WORKER proceeds to next step (`step_N+1.md`)
- If no more steps, proceed to PHASE 4

### PHASE 4: COMPLETE

1. **Final build verification**
2. **Update `$WORKDIR/requirements.md`** - all boxes checked [x]
3. **Update skills** based on identified patterns:
   - Review all `step_N_validation.md` for "Skill Updates Identified"
   - Modify `.claude/skills/*/SKILL.md` files as needed
4. **Write `$WORKDIR/summary.md`:**
   ```markdown
   # Task Summary

   ## Requirements Status
   - [x] REQ-1: <requirement>
   - [x] REQ-2: <requirement>

   ## Steps Completed
   1. Step 1: <title> - <iterations to pass>
   2. Step 2: <title> - <iterations to pass>

   ## Files Changed
   - <list of all files>

   ## Skills Updated
   - <skill path>: <what was added/changed>

   ## Architecture Compliance
   - All changes align with: <architecture docs referenced>
   ```

## INVOKE
```
/3agents Fix the step icon mismatch
# → ./workdir/2024-12-17-fix-step-icon-mismatch/

/3agents Add log line numbering
# → ./workdir/2024-12-17-add-log-line-numbering/
```