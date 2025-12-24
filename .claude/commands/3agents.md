---
name: 3agents
description: Adversarial 4-agent loop (3 core + documentarian) - CORRECTNESS over SPEED. Steps are MANDATORY.
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

1. **ARCHITECT** - Assesses requirements, reads architecture docs, creates step documentation with clear acceptance criteria (STEPS ARE MANDATORY)
2. **WORKER** - Implements steps according to step documentation, following skills and architecture
3. **VALIDATOR** - Adversarial reviewer: validates against requirements, architecture, AND skills. Default: REJECT
4. **DOCUMENTARIAN** - Updates `docs/architecture` after all steps complete to reflect new patterns, decisions, and configuration

**ADVERSARIAL RELATIONSHIPS:**
```
WORKER ←→ VALIDATOR    : Hostile opposition - VALIDATOR assumes bugs exist
ARCHITECT → WORKER     : Clear requirements - WORKER must follow precisely
ARCHITECT → VALIDATOR  : Requirements are LAW - VALIDATOR enforces compliance
DOCUMENTARIAN ← ALL    : Documents decisions from all phases into architecture docs
```

## RULES

### Core Principles
- **CORRECTNESS over SPEED** - Take time to get it right
- **ADVERSARIAL by default** - Challenge, don't agree
- **Requirements are LAW** - No interpretation, no "good enough"
- **Skills are enforceable** - Violations = automatic REJECT
- **STEPS ARE MANDATORY** - Every task MUST have step documentation with validation against architecture and requirements. No exceptions.

### Configuration Directory Rules
**`./bin` is UNTRACKED (UAT environment):**
- Changes CAN be made directly to `./bin` for testing/UAT purposes
- **HOWEVER** - all configuration changes MUST be mirrored to:
  - `./deployments/common` - Production-ready configuration
  - `./test/config` - Test configuration
- VALIDATOR must verify configuration parity between these directories
- Any change to `./bin` without corresponding changes to deployments/test = **REJECT**

### Skill Compliance (MANDATORY)
- **Refactoring**: `.claude/skills/refactoring/SKILL.md` - ALWAYS applies
- **Go**: `.claude/skills/go/SKILL.md` - for any Go code changes
- **Frontend**: `.claude/skills/frontend/SKILL.md` - for any frontend changes
- **Monitoring**: `.claude/skills/monitoring/SKILL.md` - for UI test changes

### Validation Rules
- VALIDATOR must READ skill files, not rely on memory
- VALIDATOR must verify EACH checklist item in applicable skills
- VALIDATOR must trace requirements to code (with line numbers)
- VALIDATOR must document architecture compliance with evidence
- **Update skills** where patterns are missing or outdated

### Prohibitions
- NEVER modify tests to make code pass
- NEVER skip skill compliance checks
- NEVER approve without requirements traceability
- NEVER iterate on assumptions - verify against docs

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

**Purpose:** Adversarial review of completed step against requirements, architecture, and skills. **Default stance: REJECT until proven correct.**

**CRITICAL:** The VALIDATOR must be hostile to the WORKER's implementation. Challenge every decision. Assume bugs exist until proven otherwise.

For the current step:

#### Step 2.1: Build Verification
1. **Run build first** - FAIL = immediate REJECT (no exceptions)
2. Run any tests specified in step documentation

#### Step 2.2: Requirements Verification
1. **Re-read `$WORKDIR/requirements.md`** - the original requirements
2. **Cross-reference step requirements** from `$WORKDIR/step_N.md`
3. **For EACH requirement addressed by this step:**
   - Find the specific code that implements it
   - Verify behavior matches requirement intent (not just letter)
   - Challenge: Does implementation handle edge cases?

#### Step 2.3: Architecture Compliance Check
1. **Re-read architecture documents:**
   - `docs/architecture/ARCHITECTURE.md`
   - Any domain-specific architecture docs from architect-analysis.md
2. **Verify against architecture patterns:**
   - Does code follow established architectural boundaries?
   - Are dependencies injected correctly?
   - Does it use existing extension points?
   - Challenge: Could this break existing functionality?

#### Step 2.4: Skill Compliance Audit
**Read and verify against EACH applicable skill:**

**Refactoring Skill (`.claude/skills/refactoring/SKILL.md`):**
- [ ] EXTEND > MODIFY > CREATE priority followed?
- [ ] If new file created: Written justification exists?
- [ ] Follows EXACT patterns from existing codebase?
- [ ] Minimum viable change (not over-engineered)?
- [ ] No parallel structures or duplicated logic?

**Go Skill (`.claude/skills/go/SKILL.md`) - if Go code changed:**
- [ ] Error handling: All errors wrapped with context (`%w`)?
- [ ] Logging: Uses arbor, never fmt.Println/log.Printf?
- [ ] DI: Constructor injection, no global state?
- [ ] Handlers: Thin handlers, logic in services?
- [ ] Context: ctx passed to all I/O operations?
- [ ] Build: Used scripts, not `go build` directly?

**Frontend Skill (`.claude/skills/frontend/SKILL.md`) - if frontend changed:**
- [ ] Templates: Server-side Go templates only?
- [ ] JS: Alpine.js only, no other frameworks?
- [ ] CSS: Bulma only, no inline styles?
- [ ] No direct DOM manipulation (use Alpine)?
- [ ] WebSockets for real-time (not polling)?

**Monitoring Skill (`.claude/skills/monitoring/SKILL.md`) - if UI tests changed:**
- [ ] Uses UITestContext with defer Cleanup()?
- [ ] Screenshots at key moments?
- [ ] Structured logging with utc.Log()?
- [ ] All chromedp errors checked?
- [ ] No hardcoded waits without purpose?

#### Step 2.5: Adversarial Challenges
**VALIDATOR must ask these questions and find evidence:**
1. What could break due to this change?
2. Is there a simpler way to achieve the same result?
3. Are there any hidden assumptions in the implementation?
4. Would a new developer understand this code?
5. Does this match how similar features are implemented elsewhere?

#### Step 2.6: Write Validation Report
**Write `$WORKDIR/step_N_validation.md`:**
```markdown
# Step N Validation

## Build Status
- Command: `<command>`
- Result: PASS/FAIL

## Requirements Traceability
| REQ | Requirement Text | Code Location | Verified | Notes |
|-----|------------------|---------------|----------|-------|
| REQ-1 | <text> | `file.go:42` | ✓/✗ | <evidence or issue> |
| REQ-2 | <text> | `file.go:78` | ✓/✗ | <evidence or issue> |

## Acceptance Criteria
- [x] AC-1: PASS - <concrete evidence with code reference>
- [ ] AC-2: FAIL - <specific failure reason>

## Architecture Compliance
- Status: PASS/FAIL
- Docs verified: <list of architecture docs checked>
- Patterns followed: <specific patterns confirmed>
- Violations: <any architectural violations found>

## Skill Compliance Audit

### Refactoring Skill
- Status: PASS/FAIL
- EXTEND > MODIFY > CREATE: <evidence>
- Pattern compliance: <evidence>
- Violations: <list any anti-creation violations>

### Go/Frontend/Monitoring Skill (as applicable)
- Status: PASS/FAIL
- Checklist results: <reference completed checklist above>
- Anti-patterns found: <list specific violations>

## Adversarial Findings
1. **Challenge:** <question asked>
   - **Finding:** <what was discovered>
   - **Action:** None required / MUST FIX

## Issues Found (BLOCKING)
1. <Issue description>
   - Requirement violated: <REQ-N or AC-N>
   - Expected: <what should be based on requirements/skills>
   - Actual: <what the code does>
   - Evidence: `<file:line>` - <code snippet>
   - Fix required: <specific action WORKER must take>

## Verdict: PASS/REJECT

### If REJECT:
WORKER must address ALL blocking issues before re-validation.
Maximum iterations remaining: <5 - current_iteration>

## Skill Updates Identified
- <Skill path>: <pattern that should be documented>
```

### PHASE 3: ITERATE (per step, max 5 iterations)

**ADVERSARIAL ITERATION PROTOCOL:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ITERATION LOOP (max 5 per step)                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  VALIDATOR REJECT                                                   │
│       ↓                                                             │
│  WORKER reads step_N_validation.md                                  │
│       ↓                                                             │
│  WORKER must address EVERY blocking issue (no partial fixes)        │
│       ↓                                                             │
│  WORKER updates step_N_implementation.md with:                      │
│       - Iteration number                                            │
│       - Each issue addressed with evidence                          │
│       - Code changes made                                           │
│       ↓                                                             │
│  VALIDATOR re-reads requirements.md and step_N.md (fresh eyes)      │
│       ↓                                                             │
│  VALIDATOR performs FULL validation (not just checking fixes)       │
│       ↓                                                             │
│  PASS → Move to next step     REJECT → Loop (max 5)                 │
│                                                                     │
│  ⚠ ITERATION 5 REJECT = TASK FAILURE (escalate to architect)       │
└─────────────────────────────────────────────────────────────────────┘
```

**WORKER Iteration Response Template:**
Update `$WORKDIR/step_N_implementation.md`:
```markdown
## Iteration <N> Response

### Issues Addressed
1. **Issue:** <issue from validation>
   - **Root cause:** <why this happened>
   - **Fix applied:** <what was changed>
   - **Evidence:** `<file:line>` - <code snippet>
   - **Verification:** <how to verify fix works>

### Additional Changes
- <any other changes made during fix>

### Ready for Re-validation
```

**VALIDATOR Re-validation Rules:**
1. **Start fresh** - Re-read requirements and step docs (don't rely on memory)
2. **Verify ALL previous issues fixed** - not just marked as fixed
3. **Look for regression** - Did fixes break something else?
4. **Look for NEW issues** - Fresh review may find issues missed before
5. **Increase scrutiny each iteration** - If iteration 3+, be MORE critical

**Escalation at Iteration 5:**
If VALIDATOR rejects at iteration 5:
1. **Write `$WORKDIR/step_N_escalation.md`:**
   ```markdown
   # Step N Escalation

   ## Iteration History
   - Iteration 1: <summary of issues>
   - Iteration 2: <summary of issues>
   ...

   ## Persistent Issues
   - <issues that keep reappearing>

   ## Root Cause Analysis
   - <why worker cannot satisfy requirements>

   ## Recommendation
   - [ ] Requirements unclear - needs ARCHITECT clarification
   - [ ] Architecture issue - needs redesign
   - [ ] Skill gap - pattern not documented
   - [ ] Other: <explanation>
   ```
2. Return to ARCHITECT for reassessment

**After each step PASS:**
- WORKER proceeds to next step (`step_N+1.md`)
- Iteration count resets to 0 for new step
- If no more steps, proceed to PHASE 4

### PHASE 4: COMPLETE

1. **Final build verification** - must pass
2. **Final requirements verification:**
   - Re-read `$WORKDIR/requirements.md`
   - Verify ALL requirements marked complete with evidence
3. **Skill updates:**
   - Review all `step_N_validation.md` for "Skill Updates Identified"
   - Apply updates to `.claude/skills/*/SKILL.md` files
4. **Write `$WORKDIR/summary.md`:**
   ```markdown
   # Task Summary

   ## Final Build
   - Command: `<build command>`
   - Result: PASS

   ## Requirements Traceability Matrix
   | REQ | Requirement | Status | Implemented In | Validated In |
   |-----|-------------|--------|----------------|--------------|
   | REQ-1 | <text> | ✓ | step_1 | step_1_validation.md |
   | REQ-2 | <text> | ✓ | step_2 | step_2_validation.md |

   ## Acceptance Criteria Summary
   | AC | Criterion | Status | Evidence |
   |----|-----------|--------|----------|
   | AC-1 | <text> | ✓ | `file.go:42` - <description> |

   ## Steps Completed
   | Step | Title | Iterations | Key Decisions |
   |------|-------|------------|---------------|
   | 1 | <title> | 2 | <decision made> |
   | 2 | <title> | 1 | <decision made> |

   ## Skill Compliance Summary
   | Skill | Applied | Violations Fixed | Updates Made |
   |-------|---------|------------------|--------------|
   | Refactoring | ✓ | 0 | None |
   | Go | ✓ | 1 (bare error) | Added error pattern |
   | Frontend | N/A | - | - |
   | Monitoring | N/A | - | - |

   ## Architecture Compliance
   - Docs verified: <list of architecture docs>
   - Patterns followed: <key patterns>
   - No violations

   ## Files Changed
   - `path/to/file.go`: <summary of changes>
   - `path/to/file.html`: <summary of changes>

   ## Skills Updated
   - `.claude/skills/go/SKILL.md`: Added pattern for <X>
   - (or "No updates required")
   ```

### PHASE 5: ARCHITECTURE DOCUMENTATION

**Purpose:** Update `docs/architecture` to reflect any architectural decisions, patterns, or changes discovered during implementation.

**Trigger:** Executes automatically after PHASE 4 (COMPLETE) finishes successfully.

#### Step 5.1: Review Implementation Artifacts
1. **Re-read all workdir artifacts:**
   - `$WORKDIR/architect-analysis.md` - Original architecture decisions
   - `$WORKDIR/step_*_implementation.md` - Implementation details
   - `$WORKDIR/step_*_validation.md` - Validation findings
   - `$WORKDIR/summary.md` - Final summary

2. **Identify documentation updates needed:**
   - New patterns introduced
   - Architecture decisions made
   - Integration points added
   - Configuration changes

#### Step 5.2: Update Architecture Documents
1. **Review existing architecture docs:**
   - `docs/architecture/ARCHITECTURE.md` - Main architecture document
   - `docs/architecture/README.md` - Architecture overview
   - Domain-specific docs (WORKERS.md, QUEUE_*.md, etc.)

2. **For each significant change, update the appropriate doc:**
   - Add new sections for new components/patterns
   - Update existing sections if behavior changed
   - Add cross-references between related sections
   - Document configuration requirements (especially deployments/common and test/config)

#### Step 5.3: Write Architecture Update Summary
**Write `$WORKDIR/architecture-updates.md`:**
```markdown
# Architecture Documentation Updates

## Documents Modified
| Document | Section | Change Type | Description |
|----------|---------|-------------|-------------|
| ARCHITECTURE.md | <section> | Added/Updated | <description> |
| WORKERS.md | <section> | Added/Updated | <description> |

## New Patterns Documented
- <Pattern name>: <Brief description>
- <Pattern name>: <Brief description>

## Configuration Documentation
- `deployments/common`: <What was documented>
- `test/config`: <What was documented>

## Cross-References Added
- <Doc A> ↔ <Doc B>: <Relationship documented>

## Deferred Documentation
- <Item>: <Reason deferred, e.g., "needs further discussion">
```

#### Step 5.4: Verification
1. **Ensure consistency:**
   - Architecture docs match actual implementation
   - Configuration paths are accurate
   - No contradictions between documents
2. **Update timestamps/version notes** if the project uses them

## INVOKE
```
/3agents Fix the step icon mismatch
# → ./workdir/2024-12-17-fix-step-icon-mismatch/

/3agents Add log line numbering
# → ./workdir/2024-12-17-add-log-line-numbering/
```