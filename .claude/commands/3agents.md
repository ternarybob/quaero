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
**Skills:** Planner MUST assign skill directive to each step - implementer MUST invoke assigned skill

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

skills:
  code-architect:
    use_for: [architecture, design, refactoring, structure, interfaces, patterns]
    context: "Analyze codebase structure and design patterns before implementation"
  
  go-coder:
    use_for: [implementation, coding, functions, methods, handlers, logic]
    context: "Write Go code following project conventions and idioms"
  
  test-writer:
    use_for: [tests, test coverage, test patterns, validation code]
    context: "Create tests following /test/api and /test/ui patterns"
  
  none:
    use_for: [documentation, planning, research, non-code tasks]
    context: "General implementation without specialized skill"
```

## SETUP
Create `docs/{lowercase-hyphenated-task}/` with tracking files

---

## AGENT 1 - PLANNER (Opus)

**Skill Assignment Rules:**
- Architecture/design decisions → @code-architect
- Go implementation tasks → @go-coder
- Writing tests → @test-writer
- Documentation/planning → @none

**Create:** `plan.md`
```markdown
---
task: "$ARGUMENTS"
complexity: low|medium|high
steps: N
---

# Plan

## Step 1: {Description}
**Skill:** @code-architect | @go-coder | @test-writer | @none
**Why:** {Rationale}
**Skill rationale:** {Why this skill is appropriate for this step}
**Depends:** {step numbers or 'none'}
**Validates:** {rule keys}
**Files:** {paths}
**Risk:** low|medium|high
**User decision required:** yes|no - {what decision}

## Step 2: {Description}
**Skill:** @code-architect | @go-coder | @test-writer | @none
**Why:** {Rationale}
**Skill rationale:** {Why this skill is appropriate for this step}
**Depends:** {step numbers or 'none'}
**Validates:** {rule keys}
**Files:** {paths}
**Risk:** low|medium|high
**User decision required:** yes|no - {what decision}

...

## User Decision Points
- Step {N}: {What requires user choice}
- {When to pause for input}

## Skill Distribution
- @code-architect: Steps {list}
- @go-coder: Steps {list}
- @test-writer: Steps {list}
- @none: Steps {list}

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
3. Read **Skill:** directive from current step in plan.md
4. Invoke assigned skill and implement:
   
   **IF Skill = @code-architect:**
   - Focus on structure, interfaces, architecture patterns
   - Analyze existing codebase patterns in context
   - Design before implementing
   - Document architectural decisions
   
   **IF Skill = @go-coder:**
   - Implement following Go idioms and project conventions
   - Follow existing code patterns strictly
   - Ensure clean, idiomatic Go code
   
   **IF Skill = @test-writer:**
   - Follow test patterns in /test/api or /test/ui
   - Ensure comprehensive coverage
   - Use table-driven tests where appropriate
   
   **IF Skill = @none:**
   - Standard implementation approach
   - Focus on documentation and clarity

5. Execute implementation:
   - Test with `go run` (never binaries in root)
   - Compile checks: `go build -o /tmp/test-binary`
   - Final builds: use `build_script`
   - Tests in `/test/api` or `/test/ui` only
   - Run: `cd /test/{api|ui} && go test -v`

6. Update `progress.md` with retry count and skill used

**Update:** `progress.md`
```markdown
# Progress: {task}

Current: Step {N} - awaiting validation (retry {X}/3)
Completed: {M} of {total}

- ✅ Step 1: {desc} [@code-architect] (2025-11-08 14:32) - passed validation
- ⏳ Step 3: {desc} [@go-coder] - awaiting validation (attempt {X})
- ⏸️ Step 4: {desc} [@test-writer]

## Current Retry Status
Step {N}: Attempt {X}/3 using @{skill} - {error pattern if any}

{Brief implementation notes}

Updated: {ISO8601}
```

**IF max retries reached:** Create `escalation-step-{N}.md` and **STOP**

---

## AGENT 3 - VALIDATOR (Sonnet)

**Process:**
1. Read validation criteria from `plan.md`
2. Note which skill was assigned for this step
3. Check: compilation, tests, artifacts, code quality
4. Validate skill-specific requirements:
   - @code-architect: Clean architecture, proper separation of concerns
   - @go-coder: Idiomatic Go, follows project conventions
   - @test-writer: Test coverage, follows test patterns
5. Track error patterns across retries
6. Document results

**Create:** `step-{N}-validation-attempt-{X}.md`
```markdown
# Validation: Step {N} - Attempt {X}

**Skill used:** @{skill}

## Validation Checks
✅ code_compiles
✅ tests_must_pass
❌ follows_conventions - {issue}
✅ skill_requirements_met - {skill-specific validation}

## Skill-Specific Validation
**For @{skill}:**
✅ {skill requirement 1}
✅ {skill requirement 2}
❌ {skill requirement 3} - {issue}

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
  
  2. READ: Skill directive from plan.md for current step
  
  3. Agent 2 implements using assigned skill
     → Invoke @{skill} with step context
     → Reads previous validation feedback
  
  4. Agent 3 validates
     → Checks general requirements
     → Validates skill-specific requirements
  
  5. IF BLOCKED (same error 3x):
       → Create escalation.md with analysis
       → Note if wrong skill was assigned
       → STOP - user decision required
  
  6. IF INVALID (retry < 3):
       → Agent 2 fixes with validation feedback
       → Uses SAME skill assignment
       → Agent 3 re-validates
       → Increment retry counter
       → Repeat from step 3
  
  7. IF INVALID (retry = 3):
       → Create escalation.md
       → Include skill assignment review
       → STOP - user decision required
  
  8. IF VALID:
       → Mark complete in progress.md
       → Note successful skill used
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
   - Suggested skill: @{skill}

2. **{Option 2}**
   - Pros: {list}
   - Cons: {list}
   - Implementation: {steps}
   - Suggested skill: @{skill}

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

## Skill Assignment
Assigned: @{skill}
Appropriate: yes|no - {reasoning}

## Attempts Made
1. Attempt 1 [@{skill}]: {approach} - Result: {failure reason}
2. Attempt 2 [@{skill}]: {approach} - Result: {failure reason}
3. Attempt 3 [@{skill}]: {approach} - Result: {failure reason}

## Error Pattern
{Recurring issue analysis}

## Analysis
Root cause hypothesis: {analysis}
Blocking factor: {technical|design|unclear requirement|wrong skill}

## Options
1. **Modify approach:** {suggestion}
2. **Change skill assignment:** Use @{different-skill} because {reason}
3. **Change requirement:** {alternative scope}
4. **Manual intervention:** {what needs user action}

## To Resume
- Provide guidance on approach, OR
- Approve skill change to @{skill}, OR
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
- Skill assignment is appropriate

⛔ **Stop for user input:**
- Step marked "user decision required" in plan
- Validation failed 3 times
- Same error pattern 3 times
- Skill assignment may be incorrect
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

## Skill Usage
- @code-architect: {N} steps
- @go-coder: {N} steps
- @test-writer: {N} steps
- @none: {N} steps

## User Interventions
- Step {N}: {Decision made}
- {List all decision points}

## Artifacts
- {file}

## Key Decisions
- {decision and rationale}

## Challenges & Solutions
- {challenge}: {solution} (automated|user-guided)
- Skill reassignments: {list any changed skill assignments}

## Retry Statistics
- Total retries: {N}
- Escalations: {N}
- Auto-resolved: {N}
- Skill-related issues: {N}

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
- **skill_requirements_met:** Skill-specific quality checks pass

### Skill-Specific Validation

**@code-architect:**
- Clean separation of concerns
- Proper interface definitions
- Follows architectural patterns
- Design decisions documented

**@go-coder:**
- Idiomatic Go code
- Follows project conventions
- Proper error handling
- No code smells

**@test-writer:**
- Tests in correct directory
- Follows existing test patterns
- Adequate coverage
- Table-driven where appropriate

---

## EXECUTION SUMMARY

**Autonomous operation:** Runs steps automatically until:
- User decision explicitly required in plan
- 3 failed validation attempts on same step
- Same error repeats 3 times
- Unclear requirements or multiple valid approaches
- Skill assignment appears incorrect

**User provides:** Decisions, guidance on blockers, scope changes, skill reassignments

**Resume command:** "Continue" or "Continue with option {N}"

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/`  
**Mode:** Auto-continue with decision gates  
**Skills:** Explicit routing via @skill directives