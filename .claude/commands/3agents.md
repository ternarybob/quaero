---
name: 3agent
description: Three-agent workflow - plan, implement with validation gates
---

Execute a three-agent workflow for: $ARGUMENTS

## GLOBAL RULE: ALL FILES ARE MARKDOWN
**Every file created in `docs/{folder-name}/` must be a .md file**
- Use markdown with YAML front-matter for structure
- Use emoji indicators for visual status (✅ ❌ ⏳ ⏸️)
- Human-readable format optimized for Claude CLI

## CONFIGURATION
Reference throughout workflow:
```yaml
project:
  docs_root: docs
  build_script: ./scripts/build.ps1
  test_api: ./test/api/
  test_ui: ./test/ui/

agents:
  planner: claude-opus-4-20250514
  implementer: claude-sonnet-4-20250514
  validator: claude-sonnet-4-20250514

validation_rules:
  - no_root_binaries: Binaries must not be in root directory
  - use_build_script: Use designated build script only
  - tests_in_correct_dir: Tests in appropriate test directory
  - tests_must_pass: All tests must pass
  - code_compiles: Code must compile without errors
  - follows_conventions: Follow project conventions
```

## SETUP
1. Generate folder: `lowercase-hyphenated-from-task`
2. Create: `docs/{folder-name}/`
3. Initialize tracking files

---

## AGENT 1 - PLANNER (Claude Opus)

**Model:** `claude-opus-4-20250514`

**Analyze and break down task into optimal sequence**

**Create:** `docs/{folder-name}/plan.md`

```markdown
---
task: "$ARGUMENTS"
folder: {folder-name}
complexity: low|medium|high
estimated_steps: N
---

# Implementation Plan

## Step 1: {Description}

**Why:** {Rationale for this step}
**Depends on:** {comma-separated step numbers or 'none'}
**Validation:** {comma-separated rule keys}
**Creates/Modifies:** {file paths}
**Risk:** low|medium|high

## Step 2: {Description}
...

---

## Constraints
- {constraint 1}
- {constraint 2}

## Success Criteria
- {criterion 1}
- {criterion 2}
```

---

## AGENT 2 - IMPLEMENTER (Claude Sonnet)

**Model:** `claude-sonnet-4-20250514`

**For current step only:**

1. Read `docs/{folder-name}/plan.md` → find current step
2. Read `docs/{folder-name}/progress.md` → check what's completed
3. Implement following rules:
   - Use `go run` with temp output for testing
   - Use `build_script` for final builds only
   - Put tests in correct test directory
   - Run: `cd {test_dir} && go test -v`

4. **Update:** `docs/{folder-name}/progress.md`

```markdown
# Progress: {task-name}

## Status
Current: Step {N} - awaiting validation
Completed: {M} of {total}

## Steps
- ✅ Step 1: {description} (2025-11-08 14:32)
- ✅ Step 2: {description} (2025-11-08 14:45)
- ⏳ Step 3: {description} - awaiting validation
- ⏸️ Step 4: {description}

## Implementation Notes
{Brief description of approach taken}

Last updated: {ISO8601}
```

**HALT** - Wait for validation before proceeding

---

## AGENT 3 - VALIDATOR (Claude Sonnet)

**Model:** `claude-sonnet-4-20250514`

**For current step only:**

1. Read step validation criteria from `plan.md`
2. Execute validation checklist
3. Test compilation/execution
4. Verify artifacts
5. Review code quality

**Create:** `docs/{folder-name}/step-{N}-validation.md`

```markdown
# Validation: Step {N}

## Validation Rules
✅ code_compiles
✅ tests_must_pass
❌ follows_conventions - {specific issue}

## Code Quality: {1-10}/10

## Status: VALID | INVALID

## Issues Found
- {Issue 1 with specific location}
- {Issue 2 with specific location}

## Suggestions
- {Improvement 1}
- {Improvement 2}

Validated: {ISO8601}
```

---

## GATE ENFORCEMENT (You, the orchestrator)

```
WHILE steps remain:
  1. Run Agent 2 (Sonnet) - implements current step
  2. Run Agent 3 (Sonnet) - validates current step
  3. IF validation shows "INVALID":
       → Agent 2 fixes (reads validation feedback)
       → Agent 3 re-validates
       → REPEAT until "VALID"
  4. IF validation shows "VALID":
       → Mark step complete in progress.md
       → Move to next step
```

**Critical:** Never advance without "Status: VALID"

---

## COMPLETION

When all steps show ✅ in progress.md:

**Create:** `docs/{folder-name}/summary.md`

```markdown
# Summary: {task-description}

## Models Used
- Planning: Claude Opus
- Implementation: Claude Sonnet  
- Validation: Claude Sonnet

## Results
- Steps completed: {N}
- Validation cycles: {total attempts}
- Average quality score: {X}/10

## Artifacts Created/Modified
- {file 1}
- {file 2}
- {file 3}

## Key Decisions
- {decision 1 and rationale}
- {decision 2 and rationale}

## Challenges Resolved
- {challenge 1 and solution}
- {challenge 2 and solution}

Completed: {ISO8601}
```

**Final progress.md update:**
```markdown
# Progress: {task-name}

## Status
✅ COMPLETED

All {N} steps completed
Total validation cycles: {count}

Completed: {ISO8601}
```

---

## VALIDATION RULES REFERENCE

Each rule is checked programmatically:

- **no_root_binaries:** Check root has no new executables
- **use_build_script:** Verify build script used (not manual builds)
- **tests_in_correct_dir:** Check test file paths
- **tests_must_pass:** Run `go test -v`, check exit code = 0
- **code_compiles:** Verify `go build` or `go run` succeeds
- **follows_conventions:** Check formatting, naming, structure

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/` (all files are markdown)  
**All agents:** Read plan.md and progress.md before acting
