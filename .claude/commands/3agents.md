---
name: 3agents
description: Three-agent workflow - plan, implement, validate with user decision gates
---

Execute workflow for: $ARGUMENTS

## RULES
**Files:** All output is markdown (.md) in `docs/{folder-name}/`
**Tests:** Only `/test/api` and `/test/ui` - follow existing patterns strictly
**Binaries:** Never create in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Ignore backward compatibility, breaking changes allowed, DB rebuilds each run
**Auto-continue:** Run until user decision required

## CONFIG
```yaml
docs_root: docs
build_script: ./scripts/build.ps1
test_api: /test/api
test_ui: /test/ui

limits:
  max_validation_retries: 3  # Per step before user input required
  max_same_error_retries: 2  # Same error before escalation

agents:
  planner: claude-opus-4-20250514
  implementer: claude-sonnet-4-20250514
  validator: claude-sonnet-4-20250514
```

## SETUP
Create `docs/{lowercase-hyphenated-task}/` with tracking files

---

## AGENT 1 - PLANNER (Opus)

**Create:** `plan.md`
```markdown
---
task: "$ARGUMENTS"
complexity: low|medium|high
steps: N
---

# Plan

## Step 1: {Description}
**Why:** {Rationale}
**Depends:** {step numbers or 'none'}
**Validates:** {rule keys}
**Files:** {paths}
**Risk:** low|medium|high
**User decision required:** yes|no - {what decision}

## Step 2: {Description}
...

## User Decision Points
- Step {N}: {What requires user choice}
- {When to pause for input}

## Constraints
- {constraint}

## Success Criteria
- {criterion}
```

---

## AGENT 2 - IMPLEMENTER (Sonnet)

**Process:**
1. Read `plan.md`, `progress.md`, and last validation feedback
2. Check if step requires user decision - **PAUSE IF YES**
3. Implement current step:
   - Test with `go run` (never binaries in root)
   - Compile checks: `go build -o /tmp/test-binary`
   - Final builds: use `build_script`
   - Tests in `/test/api` or `/test/ui` only
   - Run: `cd /test/{api|ui} && go test -v`
4. Update `progress.md` with retry count

**Update:** `progress.md`
```markdown
# Progress: {task}

Current: Step {N} - awaiting validation (retry {X}/3)
Completed: {M} of {total}

- ✅ Step 1: {desc} (2025-11-08 14:32) - passed validation
- ⏳ Step 3: {desc} - awaiting validation (attempt {X})
- ⏸️ Step 4: {desc}

## Current Retry Status
Step {N}: Attempt {X}/3 - {error pattern if any}

{Brief implementation notes}

Updated: {ISO8601}
```

**IF max retries reached:** Create `escalation-step-{N}.md` and **STOP**

---

## AGENT 3 - VALIDATOR (Sonnet)

**Process:**
1. Read validation criteria from `plan.md`
2. Check: compilation, tests, artifacts, code quality
3. Track error patterns across retries
4. Document results

**Create:** `step-{N}-validation-attempt-{X}.md`
```markdown
# Validation: Step {N} - Attempt {X}

✅ code_compiles
✅ tests_must_pass
❌ follows_conventions - {issue}

Quality: {1-10}/10
Status: VALID | INVALID | BLOCKED

## Issues
- {issue with location and severity}

## Error Pattern Detection
Previous errors: {list if recurring}
Same error count: {N}/2
Recommendation: {auto-fix | user decision | escalate}

## Suggestions
- {improvement}

Validated: {ISO8601}
```

**IF same error 3+ times:** Set status to BLOCKED and **STOP**

---

## WORKFLOW
```
FOR each step:
  1. CHECK: Does step require user decision?
     → IF YES: Create decision-request.md and STOP
     → IF NO: Continue
  
  2. Agent 2 implements (reads previous validation feedback)
  
  3. Agent 3 validates
  
  4. IF BLOCKED (same error 3x):
       → Create escalation.md with analysis
       → STOP - user decision required
  
  5. IF INVALID (retry < 3):
       → Agent 2 fixes with validation feedback
       → Agent 3 re-validates
       → Increment retry counter
       → Repeat from step 2
  
  6. IF INVALID (retry = 3):
       → Create escalation.md
       → STOP - user decision required
  
  7. IF VALID:
       → Mark complete in progress.md
       → Reset retry counter
       → Next step

END FOR
```

---

## USER DECISION GATES

### Decision Request Format
**File:** `decision-step-{N}.md`
```markdown
# User Decision Required: Step {N}

## Context
{What's been done so far}

## Decision Needed
{Specific choice required}

## Options
1. **{Option 1}**
   - Pros: {list}
   - Cons: {list}
   - Implementation: {steps}

2. **{Option 2}**
   - Pros: {list}
   - Cons: {list}
   - Implementation: {steps}

## Recommendation
{Agent's suggested approach with reasoning}

## To Resume
Reply with: "Continue with option {N}" or provide custom direction

Created: {ISO8601}
```

### Escalation Format
**File:** `escalation-step-{N}.md`
```markdown
# Escalation: Step {N} - Assistance Needed

## Problem
{Clear description of blocker}

## Attempts Made
1. Attempt 1: {approach} - Result: {failure reason}
2. Attempt 2: {approach} - Result: {failure reason}
3. Attempt 3: {approach} - Result: {failure reason}

## Error Pattern
{Recurring issue analysis}

## Analysis
Root cause hypothesis: {analysis}
Blocking factor: {technical|design|unclear requirement}

## Options
1. **Modify approach:** {suggestion}
2. **Change requirement:** {alternative scope}
3. **Manual intervention:** {what needs user action}

## To Resume
- Provide guidance on approach, OR
- Approve modified scope, OR
- Perform manual action and reply "Continue"

Created: {ISO8601}
```

---

## AUTOMATIC TRIGGERS (No User Input)

✅ **Continue automatically:**
- Validation passes
- Fix attempt < 3 retries
- Clear path forward from validation feedback
- No architectural decisions needed

⛔ **Stop for user input:**
- Step marked "user decision required" in plan
- Validation failed 3 times
- Same error pattern 3 times
- Unclear requirement encountered
- Multiple valid implementation approaches
- Breaking change impacts existing features
- Security/data loss risk detected

---

## COMPLETION

When all steps complete, create `summary.md`:
```markdown
# Summary: {task}

## Models
Planner: Opus | Implementer: Sonnet | Validator: Sonnet

## Results
Steps: {N} completed | User decisions: {N} | Validation cycles: {N} | Avg quality: {X}/10

## User Interventions
- Step {N}: {Decision made}
- {List all decision points}

## Artifacts
- {file}

## Key Decisions
- {decision and rationale}

## Challenges & Solutions
- {challenge}: {solution} (automated|user-guided)

## Retry Statistics
- Total retries: {N}
- Escalations: {N}
- Auto-resolved: {N}

Completed: {ISO8601}
```

Update `progress.md`:
```markdown
# Progress: {task}

✅ COMPLETED

Steps: {N} | User decisions: {N} | Validation cycles: {N}

Completed: {ISO8601}
```

---

## VALIDATION RULES

- **no_root_binaries:** No executables in root (use `-o /tmp/`)
- **use_build_script:** Use build script for final builds
- **tests_in_correct_dir:** Tests ONLY in `/test/api` or `/test/ui`
- **tests_must_pass:** `cd /test/{api|ui} && go test -v` exit code = 0
- **code_compiles:** `go build -o /tmp/test-binary` succeeds
- **follows_conventions:** Formatting, naming, existing patterns

---

## EXECUTION SUMMARY

**Autonomous operation:** Runs steps automatically until:
- User decision explicitly required in plan
- 3 failed validation attempts on same step
- Same error repeats 3 times
- Unclear requirements or multiple valid approaches

**User provides:** Decisions, guidance on blockers, scope changes

**Resume command:** "Continue" or "Continue with option {N}"

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/`  
**Mode:** Auto-continue with decision gates