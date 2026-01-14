---
name: 3agents
description: Adversarial multi-agent loop - CORRECTNESS over SPEED. Output captured to files to prevent context overflow.
allowed-tools:
  - Read
  - Edit
  - Write
  - Glob
  - Grep
  - Bash
  - Task
  - TodoWrite
---

Execute: $ARGUMENTS

## EXECUTION MODE
```
┌─────────────────────────────────────────────────────────────────┐
│ AUTONOMOUS BATCH EXECUTION - NO USER INTERACTION               │
│                                                                 │
│ • Do NOT stop for confirmation between phases                   │
│ • Do NOT ask "should I proceed?" or "continue?"                 │
│ • Do NOT pause after completing steps                           │
│ • Do NOT wait for user input at any point                       │
│ • ONLY stop on unrecoverable errors (missing files, no access)  │
│ • Execute ALL phases sequentially until $WORKDIR/summary.md     │
└─────────────────────────────────────────────────────────────────┘
```

## SETUP
```bash
WORKDIR=".opencode/workdir/$(date +%Y-%m-%d-%H%M)-$(echo "$ARGUMENTS" | tr ' ' '-' | cut -c1-40)"
mkdir -p "$WORKDIR"
mkdir -p "$WORKDIR/logs"
echo "Workdir: $WORKDIR"
```

## RULES

### Absolutes
```
┌─────────────────────────────────────────────────────────────────┐
│ • CORRECTNESS over SPEED                                        │
│ • Requirements are LAW - no interpretation                      │
│ • EXISTING PATTERNS ARE LAW - match codebase style              │
│ • BACKWARD COMPATIBILITY NOT REQUIRED - break if needed         │
│ • CLEANUP IS MANDATORY - remove dead/redundant code             │
│ • STEPS ARE MANDATORY - no implementation without step docs     │
│ • SUMMARY IS MANDATORY - task incomplete without $WORKDIR/summary.md │
│ • NO STOPPING - execute all phases without user prompts         │
│ • OUTPUT CAPTURE IS MANDATORY - all command output to log files │
└─────────────────────────────────────────────────────────────────┘
```

### Output Capture (CRITICAL - prevents context overflow)
```
┌─────────────────────────────────────────────────────────────────┐
│ OUTPUT CAPTURE IS MANDATORY                                     │
│                                                                 │
│ • ALL build output → $WORKDIR/logs/build_*.log                  │
│ • ALL test output → $WORKDIR/logs/test_*.log                    │
│ • ALL lint output → $WORKDIR/logs/lint_*.log                    │
│ • Agent sees ONLY pass/fail + last 30 lines on failure          │
│ • NEVER let full command output into context                    │
│ • Reference log files by path, don't paste contents             │
│                                                                 │
│ This prevents context overflow during long-running operations.  │
└─────────────────────────────────────────────────────────────────┘
```

### Output Limits
| Output Type | Max Lines in Context | Action |
|-------------|---------------------|--------|
| Build stdout/stderr | 30 on failure, 0 on success | Redirect to $WORKDIR/logs/*.log |
| Test stdout/stderr | 30 on failure, 0 on success | Redirect to $WORKDIR/logs/*.log |
| Lint output | 20 | Redirect to $WORKDIR/logs/*.log |
| File reads | 500 | Use grep/head/tail for large files |
| Error extraction | 20 | Use grep to extract relevant lines |

---

## QUAERO CODEBASE RULES

### OS Detection (MANDATORY before any shell command)

| Indicator | OS | Shell |
|-----------|-----|-------|
| `C:\...` or `D:\...` | Windows | PowerShell |
| `/home/...` or `/Users/...` | Unix/Linux/macOS | Bash |
| `/mnt/c/...` | WSL | Bash |

### Build & Test (WITH OUTPUT CAPTURE)

**Unix/Linux/macOS:**
```bash
# Build - capture to file
BUILD_LOG="$WORKDIR/logs/build_step${STEP}.log"
./scripts/build.sh > "$BUILD_LOG" 2>&1
BUILD_RESULT=$?
if [ $BUILD_RESULT -ne 0 ]; then
    echo "✗ BUILD FAILED"
    tail -30 "$BUILD_LOG"
else
    echo "✓ BUILD PASSED"
fi

# Test - capture to file
TEST_LOG="$WORKDIR/logs/test_step${STEP}.log"
go test -v ./test/... > "$TEST_LOG" 2>&1
TEST_RESULT=$?
if [ $TEST_RESULT -ne 0 ]; then
    echo "✗ TESTS FAILED"
    tail -30 "$TEST_LOG"
else
    echo "✓ TESTS PASSED"
fi
```

**Windows:**
```powershell
# Build - capture to file
$BUILD_LOG = "$WORKDIR/logs/build_step${STEP}.log"
.\scripts\build.ps1 > $BUILD_LOG 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ BUILD FAILED"
    Get-Content $BUILD_LOG -Tail 30
} else {
    Write-Host "✓ BUILD PASSED"
}
```

**WSL:**
```bash
# Same as Unix/Linux/macOS
BUILD_LOG="$WORKDIR/logs/build_step${STEP}.log"
./scripts/build.sh > "$BUILD_LOG" 2>&1
# ... see Unix block above for full pattern
```

### Architecture

```
cmd/quaero/           → Entry point, CLI
internal/app/         → DI & orchestration (composition root)
internal/server/      → HTTP server & routing
internal/handlers/    → HTTP/WebSocket handlers
internal/services/    → Business logic (stateful, WITH receivers)
internal/common/      → Utilities (stateless, NO receivers)
internal/jobs/
  ├── manager/        → StepManager implementations
  ├── worker/         → JobWorker implementations
  └── monitor/        → JobMonitor implementations
internal/storage/     → BadgerDB persistence
internal/interfaces/  → All interface definitions
```

### Architecture Docs (read before applicable work)

| Doc | Path |
|-----|------|
| Manager/Worker | `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` |
| Test | `docs/TEST_ARCHITECTURE.md` |

### Go Rules

**Logging (github.com/ternarybob/arbor):**
```go
logger.Info().Str("field", value).Msg("Message")
logger.Error().Err(err).Msg("Error occurred")
```

**Error handling:**
```go
if err != nil {
    return fmt.Errorf("context: %w", err)
}
```

**Structure:**
- `internal/common/` — Stateless functions ONLY (no receivers)
- `internal/services/` — Stateful services (WITH receivers)

### Forbidden

```go
fmt.Println("message")           // ❌ Use logger
log.Printf("message")            // ❌ Use logger
_ = someFunction()               // ❌ Handle all errors
// TODO: fix later               // ❌ No deferred TODOs
func (c *Config) Method() {}     // ❌ No receivers in common/
```

### Config Parity

Changes to `./bin` MUST mirror to `./deployments/common` + `./test/config`

### Frontend

Alpine.js + Bulma CSS. No React/Vue/SPA/HTMX.

---

## AGENTS (Task Tool Configuration)

Use the `Task` tool with `subagent_type` parameter.

| Agent | Role | Stance | Task Config |
|-------|------|--------|-------------|
| ARCHITECT | Requirements → step docs | Thorough | `subagent_type: general` |
| WORKER | Implements steps | Follow spec exactly | `subagent_type: general` |
| VALIDATOR | Reviews against requirements | **HOSTILE - default REJECT** | `subagent_type: general` |
| FINAL VALIDATOR | Reviews ALL changes together | **HOSTILE - catches cross-step issues** | `subagent_type: general` |
| DOCUMENTARIAN | Updates `docs/architecture` | Accurate | `subagent_type: general` |

### Parallel Step Execution
For independent steps, launch multiple Task agents in parallel using `run_in_background: true` (if supported) or sequential execution:
```
Task(subagent_type: general)
  → Step 1 implementation
Task(subagent_type: general)
  → Step 2 implementation (if no deps on Step 1)
```

---

## WORKFLOW

### PHASE 0: ARCHITECT

**Use Task tool with Plan agent:** `Task(subagent_type: general)` (Prompt: "Act as Architect...")

1. Read: `docs/architecture/*.md`, `docs/TEST_ARCHITECTURE.md`
2. For market worker tests: Read `.opencode/skills/test-architecture/SKILL.md` (MANDATORY)
3. Analyze existing patterns in target directories (use `subagent_type: explore` for codebase exploration)
4. Extract requirements → `$WORKDIR/requirements.md`
5. Create step docs → `$WORKDIR/step_N.md` for each step

**Step doc template (`$WORKDIR/step_N.md`):**
```markdown
# Step N: <title>
## Deps: [none | step_1, step_2]  # REQUIRED - enables parallelization
## Requirements: REQ-1, REQ-2
## Approach: <files, changes, patterns>
## Cleanup: <functions/code to remove>
## Acceptance: AC-1, AC-2
```

5. Write `$WORKDIR/architect-analysis.md` (patterns, decisions, cleanup candidates)

**→ IMMEDIATELY proceed to PHASE 1 (no confirmation)**

---

### PHASE 1-3: IMPLEMENT (per step)

```
┌─────────────────────────────────────────────────────────────────┐
│ FOR EACH STEP (parallel if independent using Task tool):        │
│                                                                 │
│   WORKER: Task(subagent_type: general)                          │
│      → Implement → $WORKDIR/step_N_impl.md                      │
│      ↓                                                          │
│   BUILD CHECK (output to $WORKDIR/logs/build_stepN_iterM.log)   │
│      ↓                                                          │
│   VALIDATOR: Task(subagent_type: general)                       │
│      → Review → $WORKDIR/step_N_valid.md                        │
│      ↓                                                          │
│   PASS → next step    REJECT → iterate (max 5)                  │
│                                                                 │
│ DO NOT STOP BETWEEN STEPS - continue automatically              │
└─────────────────────────────────────────────────────────────────┘
```

**WORKER must:**
- Follow step doc exactly
- Apply codebase rules (logging, error handling, structure)
- Perform cleanup listed in step doc
- Build must pass (verified with output capture)

**Build verification (MANDATORY with output capture):**
```bash
STEP=N
ITER=M
BUILD_LOG="$WORKDIR/logs/build_step${STEP}_iter${ITER}.log"

# Run build with full capture
./scripts/build.sh > "$BUILD_LOG" 2>&1
BUILD_RESULT=$?

if [ $BUILD_RESULT -ne 0 ]; then
    echo "✗ BUILD FAILED - Step $STEP Iteration $ITER"
    echo "Log: $BUILD_LOG"
    echo "=== Last 30 lines ==="
    tail -30 "$BUILD_LOG"
    echo "=== End ==="
else
    echo "✓ BUILD PASSED - Step $STEP Iteration $ITER"
fi
```

**Error extraction helper:**
```bash
# Extract compilation errors from build log
extract_build_errors() {
    local LOG_FILE=$1
    echo "--- Build Error Summary ---"
    grep -E "error:|undefined:|cannot|invalid" "$LOG_FILE" | head -20
    echo "--- End Summary ---"
    echo "Full log: $LOG_FILE"
}
```

**VALIDATOR must:**
- Default REJECT until proven correct
- Verify requirements with code line references
- Verify cleanup performed (no dead code left)
- Check codebase rule compliance
- Verify build passed (check log exists and result)

**VALIDATOR auto-REJECT:**
- Build fails
- Dead code left behind
- Old function alongside replacement
- Codebase rule violations
- Requirements not traceable to code

**→ IMMEDIATELY proceed to next step or PHASE 4 (no confirmation)**

---

### PHASE 4: FINAL VALIDATION (MANDATORY)

```
┌─────────────────────────────────────────────────────────────────┐
│ FINAL VALIDATOR reviews ALL changes together:                   │
│                                                                 │
│ • Re-read $WORKDIR/requirements.md                              │
│ • Verify ALL requirements satisfied                             │
│ • Check for conflicts between steps                             │
│ • Verify no dead code across ALL changes                        │
│ • Verify consistent patterns across ALL changes                 │
│ • Full build + test pass (with output capture)                  │
│                                                                 │
│ REJECT → Back to relevant step for fix                          │
│ PASS → PHASE 5                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Final build and test (with output capture):**
```bash
# Final build
FINAL_BUILD_LOG="$WORKDIR/logs/build_final.log"
./scripts/build.sh > "$FINAL_BUILD_LOG" 2>&1
BUILD_OK=$?

# Final test (if applicable)
FINAL_TEST_LOG="$WORKDIR/logs/test_final.log"
go test -v ./... > "$FINAL_TEST_LOG" 2>&1
TEST_OK=$?

if [ $BUILD_OK -eq 0 ] && [ $TEST_OK -eq 0 ]; then
    echo "✓ FINAL BUILD AND TEST PASSED"
else
    echo "✗ FINAL VALIDATION FAILED"
    [ $BUILD_OK -ne 0 ] && tail -30 "$FINAL_BUILD_LOG"
    [ $TEST_OK -ne 0 ] && tail -30 "$FINAL_TEST_LOG"
fi
```

**Write `$WORKDIR/final_validation.md`:**
```markdown
# Final Validation
## Build: PASS/FAIL
## Build Log: $WORKDIR/logs/build_final.log
## Test: PASS/FAIL
## Test Log: $WORKDIR/logs/test_final.log
## All Requirements: [table with status]
## Cross-step Issues: [none or list]
## Cleanup Verified: ✓/✗
## Verdict: PASS/REJECT
```

**→ IMMEDIATELY proceed to PHASE 5 (no confirmation)**

---

### PHASE 5: COMPLETE (MANDATORY)

**MUST write `$WORKDIR/summary.md`:**
```markdown
# Summary
## Build: PASS
## Requirements: [table - REQ | Status | Implemented In]
## Steps: [table - Step | Iterations | Key Decisions]
## Breaking Changes: [list]
## Cleanup: [table - Type | Item | File | Reason]
## Files Changed: [list]
## Log Files
| File | Purpose |
|------|---------|
| logs/build_step*.log | Per-step build output |
| logs/build_final.log | Final build verification |
| logs/test_final.log | Final test run |
```

**→ IMMEDIATELY proceed to PHASE 6 (no confirmation)**

---

### PHASE 6: DOCUMENTARIAN

Update `docs/architecture/*.md` to reflect changes.
Write `$WORKDIR/architecture-updates.md`.

**→ TASK COMPLETE - output final summary only**

---

## CONTEXT MANAGEMENT

### Automatic Context Optimization
- Output truncation: Commands should truncate to ~30K chars with file path reference
- Background agents: Use `run_in_background: true` to isolate agent context (if supported)

### Recovery Protocol
If context issues occur:
1. Read `$WORKDIR/*.md` artifacts to resume state
2. Use `TaskOutput(task_id)` to retrieve background agent results (if supported)
3. If needed: `/clear` and restart from last completed phase

---

## OUTPUT CAPTURE QUICK REFERENCE

```bash
# CORRECT: Build output to file, summary to Agent
./scripts/build.sh > "$WORKDIR/logs/build.log" 2>&1
if [ $? -ne 0 ]; then tail -30 "$WORKDIR/logs/build.log"; fi

# CORRECT: Test output to file, summary to Agent
go test -v ./... > "$WORKDIR/logs/test.log" 2>&1
if [ $? -ne 0 ]; then tail -30 "$WORKDIR/logs/test.log"; fi

# CORRECT: Extract specific errors from log
grep -E "error:|FAIL" "$WORKDIR/logs/build.log" | head -20

# CORRECT: Check test results
grep "^--- FAIL:" "$WORKDIR/logs/test.log"

# WRONG: Direct output to Agent (will overflow context)
./scripts/build.sh
go test -v ./...

# WRONG: Cat entire log file
cat "$WORKDIR/logs/build.log"

# WRONG: Read large sections of log
tail -500 "$WORKDIR/logs/test.log"
```

---

## FORBIDDEN ACTIONS

| Action | Result |
|--------|--------|
| Stop for user confirmation | VIOLATION |
| Ask questions expecting response | VIOLATION |
| Let full build/test output into context | VIOLATION |
| Paste log file contents (>30 lines) | VIOLATION |
| Cat entire log files | VIOLATION |
| Run commands without output capture | VIOLATION |
| Skip writing summary.md | VIOLATION |
| Leave dead code | VIOLATION |

## ALLOWED ACTIONS

| Action | Rationale |
|--------|-----------|
| Break existing APIs | Backward compat not required |
| Remove deprecated code | Cleanup is mandatory |
| Read log files with tail/head/grep | Bounded output extraction |
| Reference log paths without pasting | Preserves context |
| Proceed without confirmation | Autonomous execution |

---

## FORBIDDEN PHRASES
```
┌─────────────────────────────────────────────────────────────────┐
│ NEVER OUTPUT THESE:                                             │
│                                                                 │
│ • "Should I proceed?"                                           │
│ • "Ready to continue?"                                          │
│ • "Let me know when..."                                         │
│ • "Would you like me to..."                                     │
│ • "Shall I..."                                                  │
│ • "Do you want me to..."                                        │
│ • "I'll wait for..."                                            │
│ • "Before I continue..."                                        │
│ • Any question expecting user response                          │
│                                                                 │
│ INSTEAD: Just do it. Document in $WORKDIR. Keep moving.         │
└─────────────────────────────────────────────────────────────────┘
```

---

## WORKDIR ARTIFACTS

| File | Purpose | When Created | Required |
|------|---------|--------------|----------|
| `requirements.md` | Extracted requirements | Phase 0 | **YES** |
| `architect-analysis.md` | Patterns, decisions | Phase 0 | **YES** |
| `step_N.md` | Step specifications | Phase 0 | **YES** |
| `step_N_impl.md` | Implementation notes | Phase 1-3 | **YES** |
| `step_N_valid.md` | Validation results | Phase 1-3 | **YES** |
| `final_validation.md` | Final review | Phase 4 | **YES** |
| `summary.md` | Final summary | Phase 5 | **YES** |
| `architecture-updates.md` | Doc changes | Phase 6 | **YES** |
| `logs/` | All command output | Throughout | **YES** |
| `logs/build_*.log` | Build output | Phase 1-4 | **YES** |
| `logs/test_*.log` | Test output | Phase 4 | If tests run |

**Task is NOT complete until `summary.md` exists in workdir.**

---

## INVOKE
```
/3agents implement feature X for component Y
# → .opencode/workdir/2024-12-17-1430-implement-feature-X-for-componen/
#    ├── requirements.md
#    ├── architect-analysis.md
#    ├── step_1.md, step_1_impl.md, step_1_valid.md
#    ├── step_2.md, step_2_impl.md, step_2_valid.md
#    ├── final_validation.md
#    ├── summary.md
#    ├── architecture-updates.md
#    └── logs/
#        ├── build_step1_iter1.log
#        ├── build_step2_iter1.log
#        ├── build_final.log
#        └── test_final.log
```

**This workflow runs AUTONOMOUSLY from start to finish with all output captured to files.**
