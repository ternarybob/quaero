---
name: 2agents
description: Two-agent workflow - implement and review existing plan. Only stops for user decisions marked in plan.
---

Execute plan from: $ARGUMENTS

## INPUT HANDLING

**$ARGUMENTS must be a file path to an existing plan** (e.g., `docs/fixes/01-plan-v1.md` or document with plan content)

1. **Parse the plan document** to extract:
   - Task description
   - Steps with descriptions, skills, files, and user decision flags
   - Success criteria

2. **Determine working folder** from plan file path:
   - Extract directory and filename
   - Create short folder name: `{number}-impl-{short-desc}` (e.g., `01-impl-settings`)
   - Create working folder in same directory: `docs/fixes/01-impl-settings/`

3. **Copy original plan** to working folder as `plan-original.md`

**Output Location:** All markdown files (step-*.md, progress.md, summary.md) go into the working folder.

## RULES
**Tests:** Only `/test/api` and `/test/ui`
**Binaries:** Never in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Breaking changes allowed
**Skills:** Use @skill directives from plan for each step
**Complete:** Run all steps to completion - only stop for design decisions marked in plan

## CONFIG
```yaml
limits:
  max_retries: 2  # Quick retry per step, then move on

agents:
  implementer: claude-sonnet-4-20250514
  reviewer: claude-sonnet-4-20250514

skills:
  code-architect: [architecture, design, refactoring, structure]
  go-coder: [implementation, coding, functions, handlers]
  test-writer: [tests, test coverage, test patterns]
  none: [documentation, planning, non-code]
```

## SETUP

1. **Read and parse the plan file** from $ARGUMENTS
2. **Extract plan structure:**
   - Task/project name
   - List of steps with:
     - Description
     - Skill directive (@skill)
     - Files to modify
     - User decision flag (yes/no)
   - Success criteria

3. **Determine working folder:**
   - If plan path is `docs/fixes/01-plan-v1-settings.md`
   - Create: `docs/fixes/01-impl-settings/`

4. **Create working folder** and copy `plan-original.md` there

---

## AGENT 1 & 2 - IMPLEMENT & REVIEW LOOP

**For each step in the plan, create:** `step-{N}.md`

**CRITICAL: Each step iterates between Agent 1 (implement) and Agent 2 (review) up to 2 times.**

### Step File Format: `step-{N}.md`

```markdown
# Step {N}: {Description from plan}

**Skill:** @{skill from plan}
**Files:** {paths from plan}

---

## Iteration 1

### Agent 1 - Implementation
{What was implemented and why}

**Changes made:**
- `{file}`: {specific changes}
- `{file}`: {specific changes}

**Commands run:**
```bash
{compilation/test commands}
```

### Agent 2 - Review
**Skill:** @{skill from plan}

**Compilation:**
✅ Compiles cleanly | ⚠️ Compilation warnings | ❌ Does not compile

**Tests:**
✅ All tests pass | ⚠️ Some tests fail | ⚙️ No tests applicable | ❌ Tests error

**Code Quality:**
✅ Follows Go patterns
✅ Matches existing code style
✅ Proper error handling
⚠️ {any concerns}

**Alignment with Plan:**
✅ Implements plan requirements | ⚠️ Deviates from plan

**Quality Score:** {X}/10

**Issues Found:**
1. {issue description}
2. {issue description}

**Decision:** PASS | NEEDS_RETRY

---

## Iteration 2 (if NEEDS_RETRY)

### Agent 1 - Fixes
{What was fixed based on Agent 2 feedback}

**Changes made:**
- `{file}`: {fixes applied}

**Commands run:**
```bash
{verification commands}
```

### Agent 2 - Re-review
**Skill:** @{skill from plan}

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

## AGENT 1 - IMPLEMENTER RULES

**For each step:**

1. **Parse step from original plan:**
   - Extract description, skill, files, and user decision flag
   - Reference the detailed instructions in plan document

2. **Check for user decision:**
   - IF step has "User decision: yes" → Create `decision-step-{N}.md` and **STOP**
   - IF step has "User decision: no" or not marked → **CONTINUE AUTOMATICALLY**

3. **Create `step-{N}.md` and implement:**
   - Use assigned @skill from plan
   - Follow the approach and instructions from the plan document
   - @code-architect: Design, structure, interfaces
   - @go-coder: Implementation, following patterns
   - @test-writer: Tests following existing patterns
   - @none: Documentation

4. **Reference plan instructions:**
   - The plan document contains detailed instructions for each file
   - Follow the proposed changes verbatim
   - Trust the references and file paths in the plan
   - Only explore when absolutely necessary

5. **Document implementation in step-{N}.md:**
   - What was done (reference plan sections)
   - Files modified
   - Commands run (compile/test)

6. **Wait for Agent 2 review**

7. **If Agent 2 says NEEDS_RETRY:**
   - Implement fixes in Iteration 2
   - Document fixes in step-{N}.md
   - Wait for Agent 2 re-review

8. **Move to next step:**
   - After PASS or DONE_WITH_ISSUES (iteration 2)
   - Update progress.md
   - **AUTOMATICALLY continue - NO asking permission**

---

## AGENT 2 - REVIEWER RULES

**For each step:**

1. **Review Agent 1's implementation**

2. **Verify alignment with plan:**
   - Check if implementation matches plan instructions
   - Verify correct files were modified
   - Confirm approach follows plan recommendations

3. **Document review in step-{N}.md under "Agent 2 - Review":**
   - Check compilation
   - Run tests if applicable
   - Review code quality
   - Verify plan alignment
   - Assign quality score (1-10)
   - List any issues found

4. **Make decision:**
   - **PASS:** If code works, quality is good (7+/10), and aligns with plan
   - **NEEDS_RETRY:** If fixable issues found AND iteration 1
   - **DONE_WITH_ISSUES:** If iteration 2 OR issues are minor

5. **NEVER stop the workflow:**
   - Document issues but continue
   - Let progress.md track concerns
   - Full testing happens at the end

---

## PROGRESS TRACKING

**Update after each step:** `progress.md`

```markdown
# Progress: {task from plan}

## Plan Information
**Plan Source:** {original plan file path}
**Total Steps:** {N}
**Success Criteria:** {from plan}

## Completed Steps

### Step 1: {brief description}
- **Skill:** @{skill}
- **Status:** ✅ Complete (9/10)
- **Iterations:** 1
- **Plan Alignment:** ✅ Matches plan

### Step 2: {brief description}
- **Skill:** @{skill}
- **Status:** ⚠️ Complete with issues (6/10)
- **Iterations:** 2
- **Plan Alignment:** ✅ Matches plan
- **Issues:** {brief issue description}

### Step 3: {brief description}
- **Skill:** @{skill}
- **Status:** ✅ Complete (8/10)
- **Iterations:** 1
- **Plan Alignment:** ✅ Matches plan

## Current Step
Step 4: {description} - In progress

## Quality Average
{avg}/10 across {N} steps

**Last Updated:** {ISO8601}
```

---

## WORKFLOW

```
# Parse plan from $ARGUMENTS file
Read plan document
Extract: task, steps, skills, files, user decisions, success criteria

# Determine working folder
Extract directory and filename from plan path
Create short folder: {dir}/{number}-impl-{short}/
Create working folder
Copy plan to working folder as plan-original.md

FOR each step in parsed plan:
  
  IF step has "User decision: yes":
    Create decision-step-{N}.md
    STOP - wait for user
  
  ELSE:
    Create step-{N}.md
    
    # Iteration 1
    Agent 1: 
      - Implement using @skill from plan
      - Follow detailed instructions from plan document
      - Reference plan sections verbatim
      - Document in step-{N}.md "Iteration 1 - Implementation"
      - Run compile/test commands
    
    Agent 2:
      - Review implementation
      - Verify alignment with plan
      - Document in step-{N}.md "Iteration 1 - Review"
      - Decide: PASS | NEEDS_RETRY
    
    IF Agent 2 says NEEDS_RETRY:
      # Iteration 2
      Agent 1:
        - Fix issues
        - Document in step-{N}.md "Iteration 2 - Fixes"
      
      Agent 2:
        - Re-review
        - Document in step-{N}.md "Iteration 2 - Re-review"
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
{What needs to be decided - from plan or discovered during implementation}

## Context
{Why this decision matters}

## Options

### Option 1: {Name}
**Approach:** {brief description}
**Pros:**
- {benefit}
**Cons:**
- {drawback}

### Option 2: {Name}
**Approach:** {brief description}
**Pros:**
- {benefit}
**Cons:**
- {drawback}

## Plan Guidance
{What the original plan recommends, if specified}

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
# Done: {task from plan}

## Overview
**Plan Source:** {original plan file}
**Steps Completed:** {N}
**Average Quality:** {avg}/10
**Total Iterations:** {count}

## Plan Success Criteria
{Copy success criteria from plan}

## Verification Status
- ✅ Criterion met | ⚠️ Partially met | ❌ Not met
- ✅ Criterion met | ⚠️ Partially met | ❌ Not met

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
| Step | Description | Quality | Iterations | Plan Alignment | Status |
|------|-------------|---------|------------|----------------|--------|
| 1 | {brief} | 9/10 | 1 | ✅ | ✅ |
| 2 | {brief} | 6/10 | 2 | ✅ | ⚠️ |
| 3 | {brief} | 8/10 | 1 | ✅ | ✅ |

## Issues Requiring Attention
{List any ⚠️ COMPLETE_WITH_ISSUES items from step files}

**Step 2:**
- {issue description}
- {why it needs review}

## Testing Status
**Compilation:** ✅ All files compile | ⚠️ Some warnings
**Tests Run:** ✅ Pass | ⚠️ Some failures | ⚙️ Not applicable
**Test Coverage:** {if applicable}

## Plan Deviations
{List any deviations from the original plan, if any}
- None | {description of deviation and reasoning}

## Recommended Next Steps
1. Review changes against plan success criteria
2. Run comprehensive tests if needed
3. {any other recommendations from plan}

## Documentation
All step details available in working folder:
- `plan-original.md` (copied from source)
- `step-{1..N}.md`
- `progress.md`

**Completed:** {ISO8601}
```

---

## STOP CONDITIONS

**ONLY stop workflow for:**
- ✋ User decision required (plan says "User decision: yes")
- ✋ Cannot parse plan structure
- ✋ Plan file not found

**NEVER stop for:**
- ❌ Asking "would you like me to continue?"
- ❌ Asking "shall I proceed to step {N}?"
- ❌ Asking permission to implement planned steps
- ❌ Review failures (document in step file, continue)
- ❌ Test failures (document in step file, continue)
- ❌ Compilation errors after retry (document, continue)
- ❌ "Let me know if you want me to continue"
- ❌ Confirmation requests on straightforward work

**Golden Rule:** 
The plan says what to do, so DO IT. Don't ask. Execute all steps. Follow the plan verbatim. Only stop for architectural decisions marked in the plan.

---

## ANTI-PATTERNS TO AVOID

**❌ DON'T:**
```
Step 1 complete. Would you like me to continue to Step 2?
```

**✅ DO:**
```markdown
# Step 1: {Description}
[... implementation and review ...]
Final Status: ✅ COMPLETE (8/10)

→ Continuing to Step 2
```

**❌ DON'T:**
```
The plan says to modify settings.html. Should I verify the file exists first?
```

**✅ DO:**
```markdown
### Agent 1 - Implementation
Following plan instructions for settings.html modifications.
Implementing two-column grid layout as specified in plan.
```

**❌ DON'T:**
```
I've finished the implementation. Should I run the tests now?
```

**✅ DO:**
```markdown
### Agent 1 - Implementation
Implementation complete.

**Commands run:**
```bash
go build -o /tmp/test
cd /test/api && go test -v
```

### Agent 2 - Review
Tests: ✅ All pass
Plan Alignment: ✅ Matches plan specifications
```

---

## PLAN PARSING GUIDELINES

When parsing the plan document, extract:

1. **Task/Project Name:**
   - Look for headers like "# Plan:", "## Task:", or document title

2. **Steps:**
   - Look for numbered lists or step sections
   - Each step should have:
     - Description/title
     - Skill directive (may be inline like "@go-coder" or in a field)
     - Files to modify (list of paths)
     - User decision flag (look for "User decision: yes/no")

3. **Detailed Instructions:**
   - For each file modification, plan should contain:
     - What to change
     - How to change it
     - Code examples or specific line numbers
   - Use these as the implementation guide

4. **Success Criteria:**
   - Usually at the end or beginning of plan
   - Define what "done" looks like

**Trust the plan:** The user has explicitly stated "Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan."

---

## IMPLEMENTATION APPROACH

**For each step:**

1. **Read the plan section** for that step thoroughly
2. **Identify the proposed changes:**
   - Specific files to modify
   - Exact changes to make
   - Code snippets or patterns to follow

3. **Implement the changes:**
   - Make the exact modifications specified in the plan
   - Follow the approach and reasoning provided
   - Use the referenced files and line numbers

4. **Do NOT re-verify or second-guess:**
   - Trust that the plan author has already analyzed the codebase
   - Don't re-explore unless you encounter errors
   - Follow instructions verbatim

5. **Document what was done:**
   - Reference the plan section
   - List files modified
   - Note any deviations (if absolutely necessary)

**Example:**
```
Plan says: "Modify pages/settings.html, lines 23-101: Replace accordion structure with grid layout"

Implementation:
- Open pages/settings.html
- Locate lines 23-101
- Replace with grid structure as specified in plan
- Document: "Replaced accordion structure (lines 23-101) with two-column grid layout per plan section 'HTML Restructure'"
```

---

**Task:** Execute plan from $ARGUMENTS
**Mode:** Run to completion with Agent 1/2 iteration per step, stop only for user decisions marked in plan

**Working Folder:** Determined from plan file path
**Plan Trust:** Follow plan verbatim, trust references, implement as specified