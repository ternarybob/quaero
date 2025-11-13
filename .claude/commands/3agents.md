---
name: 3agents
description: Three-agent workflow - plan, implement, validate. Only stops for user decisions on implementation approach.
---

Execute workflow for: $ARGUMENTS

## INPUT HANDLING

**If $ARGUMENTS is a file path (e.g., `docs/fixes/01-plan-v1-xxx.md`):**
1. Extract filename without extension (e.g., `01-plan-v1-xxx`)
2. Create short folder name: `{number}-plan-{short-desc}` (e.g., `01-plan-xxx`)
3. Create working folder in same directory as the file: `docs/fixes/01-plan-xxx/`

**If $ARGUMENTS is a task description:**
1. Create lowercase-hyphenated folder name
2. Create working folder: `docs/features/{task-name}/`

**Output Location:** All markdown files (plan.md, step-*.md, progress.md, summary.md) go into the working folder determined above.

## RULES
**Tests:** Only `/test/api` and `/test/ui`
**Binaries:** Never in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Breaking changes allowed
**Skills:** Use @skill directives for each step
**Complete:** Run all steps to completion - only stop for design decisions

## CONFIG
```yaml
limits:
  max_retries: 2  # Quick retry per step, then move on

agents:
  planner: claude-opus-4-20250514
  implementer: claude-sonnet-4-20250514
  validator: claude-sonnet-4-20250514

skills:
  code-architect: [architecture, design, refactoring, structure]
  go-coder: [implementation, coding, functions, handlers]
  test-writer: [tests, test coverage, test patterns]
  none: [documentation, planning, non-code]
```

## SETUP

**Determine working folder from $ARGUMENTS:**
- If file path: Extract directory and create short folder name (e.g., `docs/fixes/01-plan-xxx/`)
- If task description: Create `docs/features/{lowercase-hyphenated-task}/`

**Create the working folder** and output all files there.

---

## AGENT 1 - PLANNER (Opus)

**Create:** `plan.md`
```markdown
# Plan: {task}

## Steps
1. **{Description}** 
   - Skill: @{skill}
   - Files: {paths}
   - User decision: {yes|no} - {if yes, what choice}

2. **{Description}**
   - Skill: @{skill}
   - Files: {paths}
   - User decision: {yes|no}

3. **{Description}**
   - Skill: @{skill}
   - Files: {paths}
   - User decision: {yes|no}

## Success Criteria
- {what defines done}
- {what defines done}
```

**User decision = yes when:**
- Multiple valid architectural approaches exist
- Design trade-offs require user preference
- Scope clarification needed
- NOT for technical issues - handle those automatically

---

## AGENT 2 & 3 - IMPLEMENT & VALIDATE LOOP

**For each step, create:** `step-{N}.md`

**CRITICAL: Each step iterates between Agent 2 (implement) and Agent 3 (validate) up to 2 times.**

### Step File Format: `step-{N}.md`

```markdown
# Step {N}: {Description}

**Skill:** @{skill}
**Files:** {paths}

---

## Iteration 1

### Agent 2 - Implementation
{What was implemented and why}

**Changes made:**
- `{file}`: {specific changes}
- `{file}`: {specific changes}

**Commands run:**
```bash
{compilation/test commands}
```

### Agent 3 - Validation
**Skill:** @{skill}

**Compilation:**
✅ Compiles cleanly | ⚠️ Compilation warnings | ❌ Does not compile

**Tests:**
✅ All tests pass | ⚠️ Some tests fail | ⚙️ No tests applicable | ❌ Tests error

**Code Quality:**
✅ Follows Go patterns
✅ Matches existing code style
✅ Proper error handling
⚠️ {any concerns}

**Quality Score:** {X}/10

**Issues Found:**
1. {issue description}
2. {issue description}

**Decision:** PASS | NEEDS_RETRY

---

## Iteration 2 (if NEEDS_RETRY)

### Agent 2 - Fixes
{What was fixed based on Agent 3 feedback}

**Changes made:**
- `{file}`: {fixes applied}

**Commands run:**
```bash
{verification commands}
```

### Agent 3 - Re-validation
**Skill:** @{skill}

**Compilation:**
✅ Compiles cleanly | ⚠️ Still has issues

**Tests:**
✅ Tests now pass | ⚠️ Still failing

**Code Quality:** {X}/10

**Remaining Issues:**
- {issue if any}

**Decision:** PASS | DONE_WITH_ISSUES

---

## Final Status

**Result:** ✅ COMPLETE | ⚠️ COMPLETE_WITH_ISSUES

**Quality:** {X}/10

**Notes:**
{Any important context for next steps or summary}

**→ Continuing to Step {N+1}**
```

---

## AGENT 2 - IMPLEMENTER RULES

**For each step:**

1. **Check for user decision:**
   - IF "User decision: yes" → Create `decision-step-{N}.md` and **STOP**
   - IF "User decision: no" → **CONTINUE AUTOMATICALLY**

2. **Create `step-{N}.md` and implement:**
   - Use assigned @skill from plan
   - @code-architect: Design, structure, interfaces
   - @go-coder: Implementation, following patterns
   - @test-writer: Tests following existing patterns
   - @none: Documentation

3. **Document implementation in step-{N}.md:**
   - What was done
   - Files modified
   - Commands run (compile/test)

4. **Wait for Agent 3 validation**

5. **If Agent 3 says NEEDS_RETRY:**
   - Implement fixes in Iteration 2
   - Document fixes in step-{N}.md
   - Wait for Agent 3 re-validation

6. **Move to next step:**
   - After PASS or DONE_WITH_ISSUES (iteration 2)
   - Update progress.md
   - **AUTOMATICALLY continue - NO asking permission**

---

## AGENT 3 - VALIDATOR RULES

**For each step:**

1. **Review Agent 2's implementation**

2. **Document validation in step-{N}.md under "Agent 3 - Validation":**
   - Check compilation
   - Run tests if applicable
   - Review code quality
   - Assign quality score (1-10)
   - List any issues found

3. **Make decision:**
   - **PASS:** If code works and quality is good (7+/10)
   - **NEEDS_RETRY:** If fixable issues found AND iteration 1
   - **DONE_WITH_ISSUES:** If iteration 2 OR issues are minor

4. **NEVER stop the workflow:**
   - Document issues but continue
   - Let progress.md track concerns
   - Full testing happens at the end

---

## PROGRESS TRACKING

**Update after each step:** `progress.md`

```markdown
# Progress: {task}

## Completed Steps

### Step 1: {brief description}
- **Skill:** @{skill}
- **Status:** ✅ Complete (9/10)
- **Iterations:** 1

### Step 2: {brief description}
- **Skill:** @{skill}
- **Status:** ⚠️ Complete with issues (6/10)
- **Iterations:** 2
- **Issues:** {brief issue description}

### Step 3: {brief description}
- **Skill:** @{skill}
- **Status:** ✅ Complete (8/10)
- **Iterations:** 1

## Current Step
Step 4: {description} - In progress

## Quality Average
{avg}/10 across {N} steps

**Last Updated:** {ISO8601}
```

---

## WORKFLOW

```
# Determine working folder from $ARGUMENTS
IF $ARGUMENTS is file path:
  Extract directory and filename
  Create short folder: {dir}/{number}-plan-{short}/
ELSE:
  Create folder: docs/features/{lowercase-hyphenated}/

Agent 1: Create plan.md in working folder

FOR each step in plan:
  
  IF step has "User decision: yes":
    Create decision-step-{N}.md
    STOP - wait for user
  
  ELSE:
    Create step-{N}.md
    
    # Iteration 1
    Agent 2: 
      - Implement with @skill
      - Document in step-{N}.md "Iteration 1 - Implementation"
      - Run compile/test commands
    
    Agent 3:
      - Review implementation
      - Document in step-{N}.md "Iteration 1 - Validation"
      - Decide: PASS | NEEDS_RETRY
    
    IF Agent 3 says NEEDS_RETRY:
      # Iteration 2
      Agent 2:
        - Fix issues
        - Document in step-{N}.md "Iteration 2 - Fixes"
      
      Agent 3:
        - Re-validate
        - Document in step-{N}.md "Iteration 2 - Re-validation"
        - Decide: PASS | DONE_WITH_ISSUES
    
    Mark final status in step-{N}.md
    Update progress.md
    
    AUTOMATICALLY continue to next step (NO asking permission)
  
END FOR

Create summary.md
DONE - report completion
```

---

## USER DECISION FORMAT

**Only created when plan says "User decision: yes"**

`decision-step-{N}.md`:
```markdown
# Decision Required: Step {N}

## Question
{What needs to be decided}

## Context
{Why this decision matters}

## Options

### Option 1: {Name}
**Approach:** {brief description}
**Pros:**
- {benefit}
- {benefit}
**Cons:**
- {drawback}
- {drawback}

### Option 2: {Name}
**Approach:** {brief description}
**Pros:**
- {benefit}
**Cons:**
- {drawback}

## Recommendation
**Suggested:** Option {N}
**Reasoning:** {why}

## To Resume
Reply with: "Option {N}" or provide your direction
```

---

## COMPLETION

`summary.md`:
```markdown
# Done: {task}

## Overview
**Steps Completed:** {N}
**Average Quality:** {avg}/10
**Total Iterations:** {count}

## Files Created/Modified
- `{path}` - {what changed}
- `{path}` - {what changed}
- `{path}` - {what changed}

## Skills Usage
- @code-architect: {N} steps
- @go-coder: {N} steps
- @test-writer: {N} steps
- @none: {N} steps

## Step Quality Summary
| Step | Description | Quality | Iterations | Status |
|------|-------------|---------|------------|--------|
| 1 | {brief} | 9/10 | 1 | ✅ |
| 2 | {brief} | 6/10 | 2 | ⚠️ |
| 3 | {brief} | 8/10 | 1 | ✅ |

## Issues Requiring Attention
{List any ⚠️ COMPLETE_WITH_ISSUES items from step files}

**Step 2:**
- {issue description}
- {why it needs review}

## Testing Status
**Compilation:** ✅ All files compile | ⚠️ Some warnings
**Tests Run:** ✅ Pass | ⚠️ Some failures | ⚙️ Not applicable
**Test Coverage:** {if applicable}

## Recommended Next Steps
1. Run `3agents-tester` to validate implementation
2. {any other recommendations}

## Documentation
All step details available in working folder:
- `plan.md`
- `step-{1..N}.md`
- `progress.md`

**Completed:** {ISO8601}
```

---

## STOP CONDITIONS

**ONLY stop workflow for:**
- ✋ User decision required (plan says "User decision: yes")
- ✋ Truly ambiguous requirements (cannot determine what to build)

**NEVER stop for:**
- ❌ Asking "would you like me to continue?"
- ❌ Asking "shall I proceed to step {N}?"
- ❌ Asking permission to implement planned steps
- ❌ Validation failures (document in step file, continue)
- ❌ Test failures (document in step file, continue)
- ❌ Compilation errors after retry (document, continue)
- ❌ "Let me know if you want me to continue"
- ❌ Confirmation requests on straightforward work

**Golden Rule:** 
If the plan says what to do, DO IT. Don't ask. Execute all steps. Only stop for architectural decisions marked in the plan.

---

## ANTI-PATTERNS TO AVOID

**❌ DON'T:**
```
Step 1 complete. Would you like me to continue to Step 2?
```

**✅ DO:**
```markdown
# Step 1: {Description}
[... implementation and validation ...]
Final Status: ✅ COMPLETE (8/10)

→ Continuing to Step 2
```

**❌ DON'T:**
```
I've finished the implementation. Should I run the tests now?
```

**✅ DO:**
```markdown
### Agent 2 - Implementation
Implementation complete.

**Commands run:**
```bash
go build -o /tmp/test
cd /test/api && go test -v
```

### Agent 3 - Validation
Tests: ✅ All pass
```

---

**Task:** $ARGUMENTS
**Mode:** Run to completion with Agent 2/3 iteration per step, stop only for user decisions

**Working Folder:** Determined from $ARGUMENTS (file path or task description)