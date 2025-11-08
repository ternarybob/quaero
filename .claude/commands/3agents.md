---
name: 3agent
description: Four-agent workflow - plan, implement, validate, test in parallel
---

Execute workflow for: $ARGUMENTS

## RULES
**Files:** All output is markdown (.md) in `docs/{folder-name}/`
**Tests:** Only `/test/api` and `/test/ui` - follow existing patterns strictly
**Binaries:** Never create in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Ignore backward compatibility, breaking changes allowed, DB rebuilds each run

## CONFIG
```yaml
docs_root: docs
build_script: ./scripts/build.ps1
test_api: /test/api
test_ui: /test/ui

agents:
  planner: claude-opus-4-20250514
  implementer: claude-sonnet-4-20250514
  validator: claude-sonnet-4-20250514
  test_updater: claude-sonnet-4-20250514
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

## Step 2: {Description}
...

## Constraints
- {constraint}

## Success Criteria
- {criterion}
```

---

## AGENT 2 - IMPLEMENTER (Sonnet)

**Process:**
1. Read `plan.md` and `progress.md`
2. Implement current step:
   - Test with `go run` (never binaries in root)
   - Compile checks: `go build -o /tmp/test-binary`
   - Final builds: use `build_script`
   - Tests in `/test/api` or `/test/ui` only
   - Run: `cd /test/{api|ui} && go test -v`
3. Update `progress.md`

**Update:** `progress.md`

```markdown
# Progress: {task}

Current: Step {N} - awaiting validation
Completed: {M} of {total}

- ✅ Step 1: {desc} (2025-11-08 14:32)
- ⏳ Step 3: {desc} - awaiting validation
- ⏸️ Step 4: {desc}

{Brief implementation notes}

Updated: {ISO8601}
```

---

## AGENT 3 - VALIDATOR (Sonnet)

**Process:**
1. Read validation criteria from `plan.md`
2. Check: compilation, tests, artifacts, code quality
3. Document results

**Create:** `step-{N}-validation.md`

```markdown
# Validation: Step {N}

✅ code_compiles
✅ tests_must_pass
❌ follows_conventions - {issue}

Quality: {1-10}/10
Status: VALID | INVALID

## Issues
- {issue with location}

## Suggestions
- {improvement}

Validated: {ISO8601}
```

---

## AGENT 4 - TEST UPDATER (Sonnet)

**Critical:** Study existing test patterns first - follow them strictly, don't invent new ones

**Process:**
1. Read `plan.md` to understand changes
2. Find relevant tests in `/test/api` and `/test/ui`
3. Update tests following existing patterns
4. Run: `cd /test/api && go test -v` and `cd /test/ui && go test -v`

**Create:** `step-{N}-tests.md`

```markdown
# Tests: Step {N}

## Modified
- {file} - {reason}

## Added  
- {file} - {coverage}

## Results

### API (/test/api)
```
{go test output}
```

### UI (/test/ui)
```
{go test output}
```

Total: {N} | Passed: {N} | Failed: {N}
Status: PASS | FAIL

Updated: {ISO8601}
```

---

## WORKFLOW

```
FOR each step:
  1. Agent 2 implements
  2. Run parallel: Agent 3 validates + Agent 4 tests
  3. IF INVALID or FAIL:
       → Agent 2 fixes (reads both feedbacks)
       → Re-validate and re-test
       → Repeat until both pass
  4. IF VALID and PASS:
       → Mark complete in progress.md
       → Next step
```

---

## COMPLETION

When all steps complete, create `summary.md`:

```markdown
# Summary: {task}

## Models
Planner: Opus | Implementer: Sonnet | Validator: Sonnet | Tests: Sonnet

## Results
Steps: {N} | Validation cycles: {N} | Avg quality: {X}/10
Tests run: {N} | Pass rate: {%}

## Artifacts
- {file}

## Key Decisions
- {decision and rationale}

## Challenges
- {challenge and solution}

Completed: {ISO8601}
```

Update `progress.md`:
```markdown
# Progress: {task}

✅ COMPLETED

Steps: {N} | Validation cycles: {N} | Tests: {N}

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

**Task:** $ARGUMENTS  
**Docs:** `docs/{folder-name}/`