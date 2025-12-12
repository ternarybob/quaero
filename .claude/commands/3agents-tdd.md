---
name: test-iterate
description: Execute Go test file and iterate until all tests pass
---

Execute: $ARGUMENTS

## INPUT VALIDATION

**GATE: Must provide a Go test file**

```
IF $ARGUMENTS does not end with "_test.go":
  STOP with error: "ERROR: Must provide a Go test file (*_test.go)"
  Example: /test-iterate test/ui/job_definition_test.go
```

## CONFIG
```yaml
max_iterations: 5
architecture_docs: docs/architecture/
workdir: ./docs/test-fix/{YYYYMMDD}-{test-name}/
```

## RULES
- **Only fix code to pass tests** - tests define requirements
- **Do NOT modify tests** - tests are the specification
- **Validate fixes against architecture docs** - fixes must be compliant
- **Iterate until all tests pass** - no partial success

---

## PHASE 0: SETUP

### Step 0.1: Validate Input
```bash
# Check file exists and is a test file
test -f "$ARGUMENTS" || STOP "File not found: $ARGUMENTS"
echo "$ARGUMENTS" | grep -q "_test.go$" || STOP "Not a test file"
```

### Step 0.2: Create Workdir
```bash
# Extract test name from file
TEST_NAME=$(basename "$ARGUMENTS" _test.go)
mkdir -p ./docs/test-fix/{YYYYMMDD}-{TEST_NAME}/
```

### Step 0.3: Load Architecture Requirements
```bash
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md
cat docs/architecture/workers.md
```

---

## PHASE 1: EXECUTE TEST

### Step 1.1: Run Test
```bash
go test -v -run "Test.*" {test_file_path} 2>&1
```

### Step 1.2: Capture Results
**WRITE `{workdir}/test-run-{iteration}.md`:**
```markdown
# Test Run {N}
File: {test_file}
Date: {timestamp}

## Result: PASS | FAIL

## Test Output
```
{full test output}
```

## Failures (if any)
| Test | Error | Location |
|------|-------|----------|
| {TestName} | {error message} | {file:line} |
```

### If PASS:
Go to PHASE 4: COMPLETE

### If FAIL:
Continue to PHASE 2

---

## PHASE 2: ANALYZE & FIX

### Step 2.1: Analyze Failures
For each failing test:
1. Read the test code to understand what it expects
2. Read the implementation code being tested
3. Identify the root cause of failure

### Step 2.2: Check Architecture Compliance
Before fixing, verify the fix will comply with:
- `docs/architecture/manager_worker_architecture.md`
- `docs/architecture/QUEUE_LOGGING.md`
- `docs/architecture/QUEUE_UI.md`
- `docs/architecture/QUEUE_SERVICES.md`

### Step 2.3: Implement Fix
**WRITE `{workdir}/fix-{iteration}.md`:**
```markdown
# Fix {N}
Iteration: {N}

## Failures Addressed
| Test | Root Cause | Fix |
|------|------------|-----|
| {TestName} | {why it failed} | {what we're changing} |

## Architecture Compliance
| Doc | Requirement | How Fix Complies |
|-----|-------------|------------------|
| {doc} | {requirement} | {compliance evidence} |

## Changes Made
| File | Change |
|------|--------|
| `{path}` | {description} |

## NOT Changed (tests are spec)
- {test_file} - Tests define requirements, not modified
```

---

## PHASE 3: VALIDATE & ITERATE

### Step 3.1: Re-run Tests
Return to PHASE 1: EXECUTE TEST

### Step 3.2: Check Iteration Count
```
IF iteration > max_iterations (5):
  STOP with report of remaining failures
  User action required
```

---

## PHASE 4: COMPLETE

**WRITE `{workdir}/summary.md`:**
```markdown
# Test Fix Complete
File: {test_file}
Iterations: {N}

## Result: ALL TESTS PASS

## Fixes Applied
| Iteration | Files Changed | Tests Fixed |
|-----------|---------------|-------------|
| 1 | {files} | {tests} |
| 2 | {files} | {tests} |

## Architecture Compliance Verified
All fixes comply with docs/architecture/ requirements.

## Final Test Output
```
{passing test output}
```
```

---

## WORKFLOW DIAGRAM
```
┌─────────────────────────────────────────────────────────────────┐
│ INPUT VALIDATION                                                 │
│ - Must be *_test.go file                                         │
│ - STOP if not a test file                                        │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 0: SETUP                                                   │
│ - Create workdir                                                 │
│ - Load docs/architecture/*.md                                    │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: EXECUTE TEST                                            │◄─────┐
│ - Run: go test -v {file}                                         │      │
│ - Write test-run-{N}.md                                          │      │
├─────────────────────────────────────────────────────────────────┤      │
│ PASS → PHASE 4                                                   │      │
│ FAIL → PHASE 2                                                   │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 2: ANALYZE & FIX                                           │      │
│ - Analyze test failures                                          │      │
│ - Check architecture compliance                                  │      │
│ - Implement fix (DO NOT modify tests)                            │      │
│ - Write fix-{N}.md                                               │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 3: VALIDATE & ITERATE                                      │      │
│ - Check iteration count                                          │      │
│ - If < 5: return to PHASE 1 ────────────────────────────────────┼──────┘
│ - If >= 5: STOP with remaining failures                          │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 4: COMPLETE                                                │
│ - Write summary.md                                               │
│ - All tests pass                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## CRITICAL RULES

**Tests are the specification:**
- NEVER modify test files to make them pass
- Tests define what the code SHOULD do
- If a test fails, the implementation is wrong

**Architecture compliance is mandatory:**
- Every fix must comply with docs/architecture/
- Check QUEUE_UI.md for frontend fixes
- Check QUEUE_LOGGING.md for logging fixes
- Check manager_worker_architecture.md for job system fixes

**Iteration limits:**
- Max 5 iterations before stopping
- If stuck, report remaining failures to user
- User may need to clarify requirements

---

## INVOKE
```
/test-iterate test/ui/job_definition_codebase_classify_test.go
/test-iterate test/api/queue_test.go
```

