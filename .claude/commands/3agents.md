---
name: 3agent
description: Four-agent workflow - plan, implement, validate, and test with parallel execution
---

Execute a four-agent workflow for: $ARGUMENTS

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
  test_api: /test/api
  test_ui: /test/ui
  development_mode: beta
  deployment: local_only

test_rules:
  - only_two_test_locations: "Tests ONLY in /test/api and /test/ui - no other locations"
  - follow_existing_structure: "Strictly follow existing test patterns and structure - do not invent new patterns"
  - unit_tests_with_code: "Unit tests are co-located with source code"

principles:
  - ignore_backward_compatibility: "This is beta development - database rebuilds on each run, no migration concerns"
  - breaking_changes_allowed: "Breaking changes are acceptable, optimize for clean design over compatibility"

agents:
  planner: claude-opus-4-20250514
  implementer: claude-sonnet-4-20250514
  validator: claude-sonnet-4-20250514
  test_updater: claude-sonnet-4-20250514

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

**Consider:**
- This is beta development - ignore backward compatibility
- Breaking changes are acceptable - optimize for clean design
- Database rebuilds every time - no migration concerns

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
   - Use `go run` with temp output for testing (never create binaries in root)
   - For compile checks: `go build -o /tmp/test-binary` or discard output
   - Use `build_script` for final builds only
   - Put tests in `/test/api` or `/test/ui` only
   - Run tests: `cd /test/api && go test -v` or `cd /test/ui && go test -v`

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

## AGENT 4 - TEST UPDATER (Claude Sonnet)

**Model:** `claude-sonnet-4-20250514`

**Can run in parallel with Agent 3 validation**

**CRITICAL TEST RULES:**
- Tests ONLY in `/test/api` and `/test/ui` - no other locations
- STRICTLY follow existing test patterns and structure
- Do NOT invent new test patterns or conventions
- Study existing tests first before making changes

**For current step only:**

1. Read `docs/{folder-name}/plan.md` → understand changes made in current step
2. Find relevant existing tests:
   - API tests: `/test/api`
   - UI tests: `/test/ui`
3. Study existing test structure and patterns
4. Update tests to match new functionality (following existing patterns)
5. Execute tests from test directories:
   - `cd /test/api && go test -v`
   - `cd /test/ui && go test -v`
6. Document results

**Create:** `docs/{folder-name}/step-{N}-tests.md`

```markdown
# Test Updates: Step {N}

## Tests Modified
- {test_file_1} - {reason}
- {test_file_2} - {reason}

## Tests Added
- {new_test_file} - {coverage}

## Test Execution Results

### API Tests (/test/api)
```
{output from: cd /test/api && go test -v}
```

### UI Tests (/test/ui)
```
{output from: cd /test/ui && go test -v}
```

## Summary
- Total tests run: {N}
- Passed: {N}
- Failed: {N}
- Coverage: {percentage if available}

## Status: PASS | FAIL

Updated: {ISO8601}
```

**Note:** Agent 4 can run while Agent 3 validates, results feed into final step validation

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
  2. Run in parallel:
     → Agent 3 (Sonnet) - validates current step
     → Agent 4 (Sonnet) - updates and runs tests
  3. Wait for both to complete
  4. IF validation shows "INVALID" OR tests show "FAIL":
       → Agent 2 fixes issues (reads validation + test feedback)
       → Agent 3 re-validates
       → Agent 4 re-runs tests
       → REPEAT until both "VALID" and "PASS"
  5. IF validation shows "VALID" AND tests show "PASS":
       → Mark step complete in progress.md
       → INCREMENT current_step
       → CONTINUE to next iteration
```

**Critical:** Never advance without both "Status: VALID" and "Status: PASS"

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
- Test Updates: Claude Sonnet

## Results
- Steps completed: {N}
- Validation cycles: {total attempts}
- Average code quality score: {X}/10
- Total tests run: {N}
- Test pass rate: {percentage}

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
Total tests run: {test_count}

Models used:
- Planner: claude-opus-4-20250514
- Implementer: claude-sonnet-4-20250514
- Validator: claude-sonnet-4-20250514
- Test Updater: claude-sonnet-4-20250514

Completed: {ISO8601}
```

---

## VALIDATION RULES REFERENCE

Each rule is checked programmatically:

- **no_root_binaries:** Check root has no new executables (compile tests must use `-o /tmp/` or discard output)
- **use_build_script:** Verify build script used (not manual builds)
- **tests_in_correct_dir:** Tests ONLY in `/test/api` or `/test/ui` - no other locations
- **tests_must_pass:** Run `cd /test/api && go test -v` and `cd /test/ui && go test -v`, check exit code = 0
- **code_compiles:** Verify code compiles with `go build -o /tmp/test-binary` or `go run` (never create binaries in root)
- **follows_conventions:** Check formatting, naming, structure, and that existing test patterns are followed

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/` (all files are markdown)  
**All agents:** Read plan.md and progress.md before acting