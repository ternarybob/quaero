---
name: 3agents
description: Three-agent workflow - plan, implement, validate. Only stops for user decisions on implementation approach.
---

Execute workflow for: $ARGUMENTS

## RULES
**Files:** Output markdown in `docs/{folder-name}/`
**Tests:** Only `/test/api` and `/test/ui`
**Binaries:** Never in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Breaking changes allowed
**Skills:** Use @skill directives for each step
**Complete:** Run all steps to completion - only stop for design decisions

## CONFIG
```yaml
limits:
  max_retries: 2  # Quick retry, then move on

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
Create `docs/{lowercase-hyphenated-task}/`

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

## AGENT 2 - IMPLEMENTER (Sonnet)

**CRITICAL: DO NOT ask user for permission to continue with planned steps.**

**For each step:**
1. **Check for user decision:**
   - IF "User decision: yes" → Create `decision-step-{N}.md` and **STOP**
   - IF "User decision: no" → **CONTINUE AUTOMATICALLY**

2. **Implement with assigned @skill:**
   - @code-architect: Design, structure, interfaces
   - @go-coder: Implementation, following patterns
   - @test-writer: Tests following existing patterns
   - @none: Documentation

3. **Quick validation:**
   - Compile: `go build -o /tmp/test`
   - Run: `go run` if applicable
   - Tests: `cd /test/{api|ui} && go test -v` if test files exist

4. **Continue to next step automatically**
   - Do NOT ask "would you like me to continue"
   - Do NOT ask "shall I proceed"
   - Just do all remaining steps

**Update:** `progress.md`
```markdown
# Progress: {task}

- ✅ Step 1: {brief} [@{skill}] - Done
- ⚠️ Step 2: {brief} [@{skill}] - Done with issues (see below)
- ✅ Step 3: {brief} [@{skill}] - Done

## Issues Encountered
- Step 2: {issue} - Attempted fix, may need review in testing

Updated: {ISO8601}
```

---

## AGENT 3 - VALIDATOR (Sonnet)

**Quick validation per step:**
```markdown
# Validation: Step {N}

[@{skill}]

✅ Compiles
⚠️ Tests: {pass|fail|not applicable}
✅ Follows patterns

Quality: {X}/10
Status: DONE | DONE_WITH_ISSUES

Issues: {if any}
```

**Always continue - don't stop for validation failures**

---

## WORKFLOW
```
FOR each step in plan:
  
  IF step has "User decision: yes":
    Create decision-step-{N}.md
    STOP - wait for user
  
  ELSE:
    Agent 2: Implement with @skill (NO asking permission)
    Agent 3: Validate
    
    IF validation fails:
      Agent 2: Quick retry (1x)
      Agent 3: Re-validate
    
    Document status in progress.md
    AUTOMATICALLY continue to next step
  
END FOR

Create summary.md
DONE - report completion, do not ask to continue
```

---

## USER DECISION FORMAT

**Only created when plan says "User decision: yes"**

`decision-step-{N}.md`:
```markdown
# Decision Required: Step {N}

## Question
{What needs to be decided}

## Options
1. **{Option 1}**
   - {brief pro/con}
   
2. **{Option 2}**
   - {brief pro/con}

## Recommendation
{Suggested approach}

## Resume
Reply: "Option {N}" or provide direction
```

---

## COMPLETION

`summary.md`:
```markdown
# Done: {task}

## Results
Steps: {N} completed
Quality: {avg}/10

## Created/Modified
- {file} - {what}
- {file} - {what}

## Skills Used
- @code-architect: {N}
- @go-coder: {N}
- @test-writer: {N}

## Issues
{List any ⚠️ items from progress.md}

## Testing Status
- Compilation: {pass|issues}
- Tests run: {pass|fail|not run}

## Next Steps
Run 3agents-tester to validate implementation

Completed: {ISO8601}
```

---

## STOP CONDITIONS

**ONLY stop for:**
- ✋ User decision required in plan (architectural choice, multiple valid approaches)
- ✋ Ambiguous requirements (can't determine what to build)

**NEVER stop for:**
- ❌ Asking permission to continue ("would you like me to continue?")
- ❌ Asking if you should proceed with remaining steps
- ❌ Validation failures (document and continue)
- ❌ Test failures (document and continue)
- ❌ Compilation errors after retry (document and continue)
- ❌ Asking for confirmation on straightforward work

**Rule:** If the plan says what to do, DO IT. Don't ask. Just execute all steps.

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/`  
**Mode:** Run to completion, stop only for user decisions