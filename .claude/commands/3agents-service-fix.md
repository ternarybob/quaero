---
name: 3agents-service-fix
description: Iterative service log review and fix loop. Reviews logs, implements fixes, deploys, and repeats up to 5 iterations.
context: fork
allowed-tools:
  - Read
  - Edit
  - Write
  - Glob
  - Grep
  - Bash
  - Task
  - TodoWrite
  - Skill
---

Execute: $ARGUMENTS

## EXECUTION MODE
```
┌─────────────────────────────────────────────────────────────────┐
│ ITERATIVE AUTONOMOUS EXECUTION - NO USER INTERACTION            │
│                                                                 │
│ • Max 5 iterations (configurable)                               │
│ • Each iteration: Review → Plan → Fix → Deploy → Summary        │
│ • Compacts conversation at start of each iteration              │
│ • Reads previous iteration summary to maintain context          │
│ • Do NOT stop for confirmation between phases                   │
│ • ONLY stop on unrecoverable errors or max iterations reached   │
│ • All output → $WORKDIR/iteration-{n}/logs/                     │
└─────────────────────────────────────────────────────────────────┘
```

## SETUP (MANDATORY - DO FIRST)

```bash
TASK_DESC="$ARGUMENTS"
TASK_SLUG=$(echo "$TASK_DESC" | tr ' ' '-' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]//g' | cut -c1-30)
DATE=$(date +%Y-%m-%d)
TIME=$(date +%H%M)
WORKDIR=".claude/workdir/${DATE}-${TIME}-service-fix-${TASK_SLUG}"
MAX_ITERATIONS=5

mkdir -p "$WORKDIR"
echo "Workdir: $WORKDIR"
echo "Max Iterations: $MAX_ITERATIONS"
```

## RULES

### Absolutes
```
┌─────────────────────────────────────────────────────────────────┐
│ • CORRECTNESS over SPEED                                        │
│ • LOCAL BUILD ONLY - use scripts/build.ps1 (not docker)         │
│ • CONTEXT COMPACT at start of each iteration                    │
│ • READ PREVIOUS SUMMARY before each iteration                   │
│ • EACH ITERATION gets own subdirectory: iteration-{n}/          │
│ • SUMMARY IS MANDATORY for each iteration                       │
│ • NO STOPPING - execute all phases without user prompts         │
│ • OUTPUT CAPTURE IS MANDATORY - all command output to log files │
│ • STOP ONLY when: service healthy OR max iterations reached     │
└─────────────────────────────────────────────────────────────────┘
```

### Output Capture (CRITICAL)
```
┌─────────────────────────────────────────────────────────────────┐
│ OUTPUT CAPTURE IS MANDATORY                                     │
│                                                                 │
│ • ALL build output → $ITER_DIR/logs/build.log                   │
│ • ALL service logs → $ITER_DIR/logs/service.log                 │
│ • ALL deploy output → $ITER_DIR/logs/deploy.log                 │
│ • Claude sees ONLY pass/fail + last 30 lines on failure         │
│ • NEVER let full command output into context                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## OS DETECTION (MANDATORY)

| Indicator | OS | Build Script |
|-----------|-----|--------------|
| `C:\...` or `D:\...` | Windows | `powershell.exe -File .\scripts\build.ps1` |
| `/home/...` or `/Users/...` | Unix/Linux/macOS | `./scripts/build.sh` |
| `/mnt/c/...` | WSL | `powershell.exe -File scripts/build.ps1` (Go build via PowerShell) |

**This workflow uses LOCAL builds only (scripts/build.ps1), NOT docker.**

---

## ITERATION LOOP

```
ITERATION = 1
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ ITERATION START                                                 │
│                                                                 │
│   ITER_DIR="$WORKDIR/iteration-$ITERATION"                      │
│   mkdir -p "$ITER_DIR/logs"                                     │
│                                                                 │
│   IF ITERATION > 1:                                             │
│     • COMPACT conversation context                              │
│     • READ $WORKDIR/iteration-$((ITERATION-1))/summary.md       │
│     • Carry forward: issues found, actions taken, status        │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: REVIEW SERVICE STATUS AND LOGS                         │
│                                                                 │
│   1.1 Check if service is running                               │
│       • pgrep -f "quaero" or process check                      │
│       • curl health endpoint: http://localhost:PORT/api/health  │
│                                                                 │
│   1.2 Review application logs                                   │
│       • Recent: bin/logs/quaero.*.log                           │
│       • Extract: FTL, ERR, error, panic, fatal (last 50 lines)  │
│       • Check for specific job/step failures                    │
│                                                                 │
│   1.3 Write status to: $ITER_DIR/status.md                      │
│       • Service: RUNNING/STOPPED                                │
│       • Health: OK/UNHEALTHY/UNREACHABLE                        │
│       • Issues found: [list with log references]                │
│                                                                 │
│   IF service healthy AND no issues:                             │
│     → SKIP to PHASE 5 (Summary) → END ITERATIONS                │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼ (if issues found)
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 2: CREATE ACTION PLAN                                     │
│                                                                 │
│   2.1 Analyze errors from status.md                             │
│       • Categorize: build, config, runtime, dependency          │
│       • Prioritize by severity                                  │
│                                                                 │
│   2.2 Write plan to: $ITER_DIR/action-plan.md                   │
│       ```markdown                                               │
│       # Action Plan - Iteration N                               │
│                                                                 │
│       ## Issues Identified                                      │
│       | # | Issue | Severity | Source |                         │
│       |---|-------|----------|--------|                         │
│       | 1 | Error message | HIGH/MED/LOW | log file:line |      │
│                                                                 │
│       ## Planned Actions                                        │
│       | # | Action | Files | Expected Outcome |                 │
│       |---|--------|-------|------------------|                 │
│       | 1 | Fix X | file.go | Resolves issue #1 |               │
│       ```                                                       │
│                                                                 │
│   2.3 If requires code exploration:                             │
│       → Use Task(subagent_type: Explore, model: opus)           │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 3: IMPLEMENT FIXES                                        │
│                                                                 │
│   3.1 Execute each action from plan                             │
│       • Edit files as needed                                    │
│       • Follow existing codebase patterns                       │
│                                                                 │
│   3.2 Build verification (MANDATORY with output capture):       │
│       ```bash                                                   │
│       BUILD_LOG="$ITER_DIR/logs/build.log"                      │
│                                                                 │
│       # WSL: Use PowerShell for Go build                        │
│       powershell.exe -File scripts/build.ps1 > "$BUILD_LOG" 2>&1│
│       BUILD_RESULT=$?                                           │
│                                                                 │
│       if [ $BUILD_RESULT -ne 0 ]; then                          │
│           echo "✗ BUILD FAILED"                                 │
│           tail -30 "$BUILD_LOG"                                 │
│           # Use /gofix skill for Go build errors                │
│       else                                                      │
│           echo "✓ BUILD PASSED"                                 │
│       fi                                                        │
│       ```                                                       │
│                                                                 │
│   3.3 If build fails:                                           │
│       → Use Skill(skill: "gofix") to auto-fix Go errors         │
│       → Re-run build                                            │
│                                                                 │
│   3.4 Write implementation notes: $ITER_DIR/implementation.md   │
│       • Files changed                                           │
│       • Actions completed                                       │
│       • Build result                                            │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 4: DEPLOY SERVICE                                         │
│                                                                 │
│   4.1 Stop existing service (if running)                        │
│       ```bash                                                   │
│       pkill -f "quaero" 2>/dev/null || true                     │
│       sleep 2                                                   │
│       ```                                                       │
│                                                                 │
│   4.2 Start service with output capture:                        │
│       ```bash                                                   │
│       DEPLOY_LOG="$ITER_DIR/logs/deploy.log"                    │
│                                                                 │
│       # WSL: Use PowerShell to run                              │
│       powershell.exe -File scripts/build.ps1 -run > "$DEPLOY_LOG" 2>&1 &│
│       DEPLOY_PID=$!                                             │
│                                                                 │
│       # Wait for startup                                        │
│       sleep 5                                                   │
│       ```                                                       │
│                                                                 │
│   4.3 Verify deployment:                                        │
│       • Check process running: pgrep -f "quaero"                │
│       • Check health endpoint: curl localhost:PORT/api/health   │
│       • Check for startup errors in new logs                    │
│                                                                 │
│   4.4 Write deploy result: $ITER_DIR/deploy-result.md           │
│       • Process: RUNNING/FAILED                                 │
│       • Health: OK/UNREACHABLE                                  │
│       • New errors (if any)                                     │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 5: ITERATION SUMMARY (MANDATORY)                          │
│                                                                 │
│   MUST write: $ITER_DIR/summary.md                              │
│                                                                 │
│   ```markdown                                                   │
│   # Iteration N Summary                                         │
│                                                                 │
│   ## Status                                                     │
│   - Service: RUNNING/STOPPED                                    │
│   - Health: OK/UNHEALTHY                                        │
│   - Iteration Result: SUCCESS/PARTIAL/FAILED                    │
│                                                                 │
│   ## Issues Found This Iteration                                │
│   | # | Issue | Severity | Status |                             │
│   |---|-------|----------|--------|                             │
│   | 1 | Description | HIGH | FIXED/PENDING |                    │
│                                                                 │
│   ## Actions Taken                                              │
│   | # | Action | File | Result |                                │
│   |---|--------|------|--------|                                │
│   | 1 | Description | file.go | SUCCESS/FAILED |                │
│                                                                 │
│   ## Files Changed                                              │
│   - file1.go: description                                       │
│   - file2.go: description                                       │
│                                                                 │
│   ## Build Result: PASS/FAIL                                    │
│   ## Deploy Result: SUCCESS/FAILED                              │
│                                                                 │
│   ## Remaining Issues for Next Iteration                        │
│   - Issue description (if any)                                  │
│                                                                 │
│   ## Recommendation                                             │
│   - CONTINUE: More issues to fix                                │
│   - STOP: Service healthy, all issues resolved                  │
│   - STOP: Max iterations reached, manual intervention needed    │
│                                                                 │
│   ## Log Files                                                  │
│   - logs/build.log                                              │
│   - logs/deploy.log                                             │
│   - logs/service.log (captured app logs)                        │
│   ```                                                           │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ ITERATION DECISION                                              │
│                                                                 │
│   IF service healthy AND no remaining issues:                   │
│     → STOP iterations → PHASE 6 (Final Summary)                 │
│                                                                 │
│   ELSE IF ITERATION >= MAX_ITERATIONS:                          │
│     → STOP iterations → PHASE 6 (Final Summary)                 │
│                                                                 │
│   ELSE:                                                         │
│     → ITERATION++ → Loop back to ITERATION START                │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 6: FINAL SUMMARY (MANDATORY)                              │
│                                                                 │
│   MUST write: $WORKDIR/summary.md                               │
│                                                                 │
│   ```markdown                                                   │
│   # Service Fix Summary                                         │
│                                                                 │
│   ## Task: $TASK_DESC                                           │
│   ## Final Status: SUCCESS/PARTIAL/FAILED                       │
│   ## Total Iterations: N                                        │
│                                                                 │
│   ## Service Status                                             │
│   - Running: YES/NO                                             │
│   - Health: OK/UNHEALTHY                                        │
│   - PID: XXXX (if running)                                      │
│                                                                 │
│   ## Issues Summary                                             │
│   | # | Issue | Initial State | Final State | Iteration Fixed | │
│   |---|-------|---------------|-------------|-----------------|  │
│   | 1 | Desc  | ERROR         | RESOLVED    | 2               |  │
│                                                                 │
│   ## All Changes Made                                           │
│   | File | Change | Iteration |                                 │
│   |------|--------|-----------|                                 │
│   | x.go | Fixed Y | 1        |                                 │
│                                                                 │
│   ## Iteration History                                          │
│   | # | Issues Found | Actions | Result | Duration |            │
│   |---|--------------|---------|--------|----------|            │
│   | 1 | 3            | 2       | PARTIAL| 5m       |            │
│   | 2 | 1            | 1       | SUCCESS| 3m       |            │
│                                                                 │
│   ## Remaining Issues (if any)                                  │
│   - Description (requires manual intervention)                  │
│                                                                 │
│   ## Artifacts                                                  │
│   - iteration-1/                                                │
│   - iteration-2/                                                │
│   - ...                                                         │
│   ```                                                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## CONTEXT COMPACTION

At the start of each iteration (after iteration 1):

```
┌─────────────────────────────────────────────────────────────────┐
│ CONTEXT COMPACTION PROTOCOL                                     │
│                                                                 │
│ 1. Read previous iteration summary:                             │
│    $WORKDIR/iteration-$((ITERATION-1))/summary.md               │
│                                                                 │
│ 2. Extract key information:                                     │
│    • Issues still pending                                       │
│    • Actions already taken (don't repeat)                       │
│    • Files already modified                                     │
│    • Current service status                                     │
│                                                                 │
│ 3. Start fresh with compacted context:                          │
│    • Don't re-read unchanged files                              │
│    • Focus on remaining issues                                  │
│    • Build on previous iteration's work                         │
│                                                                 │
│ Auto-compacting is instant (Claude Code 2.0.64+)                │
│ Forked context via `context: fork` provides isolation           │
└─────────────────────────────────────────────────────────────────┘
```

---

## LOG ANALYSIS HELPERS

```bash
# Extract errors from service logs
extract_service_errors() {
    local LOG_DIR=${1:-"bin/logs"}
    echo "--- Service Error Summary ---"

    # Find most recent log
    LATEST=$(ls -t "$LOG_DIR"/quaero.*.log 2>/dev/null | head -1)
    if [ -n "$LATEST" ]; then
        echo "Log: $LATEST"
        grep -E "FTL|ERR|error|panic|fatal" "$LATEST" | tail -20
    else
        echo "No log files found in $LOG_DIR"
    fi

    echo "--- End Summary ---"
}

# Check service health
check_service_health() {
    local PORT=${1:-8080}
    echo "--- Health Check ---"

    # Process check
    QUAERO_PID=$(pgrep -f "quaero" | head -1)
    if [ -n "$QUAERO_PID" ]; then
        echo "✓ Process running: PID $QUAERO_PID"
    else
        echo "✗ No quaero process found"
        return 1
    fi

    # Health endpoint
    if curl -s --connect-timeout 5 "http://localhost:$PORT/api/health" | grep -q "ok"; then
        echo "✓ Health endpoint: OK"
        return 0
    else
        echo "✗ Health endpoint: UNREACHABLE"
        return 1
    fi
}

# Copy recent logs to iteration directory
capture_service_logs() {
    local ITER_DIR=$1
    local LOG_DIR="bin/logs"

    # Copy last 500 lines of most recent log
    LATEST=$(ls -t "$LOG_DIR"/quaero.*.log 2>/dev/null | head -1)
    if [ -n "$LATEST" ]; then
        tail -500 "$LATEST" > "$ITER_DIR/logs/service.log"
        echo "Captured service logs to: $ITER_DIR/logs/service.log"
    fi
}
```

---

## SKILLS INTEGRATION

| Situation | Skill/Tool |
|-----------|------------|
| Go build errors | `Skill(skill: "gofix")` |
| Complex code changes | `Task(subagent_type: general-purpose, model: opus)` |
| Codebase exploration | `Task(subagent_type: Explore, model: opus)` |
| Pattern analysis | `Task(subagent_type: Plan, model: opus)` |

---

## OUTPUT LIMITS

| Output Type | Max Lines in Context | Action |
|-------------|---------------------|--------|
| Build stdout/stderr | 30 on failure, 0 on success | Redirect to $ITER_DIR/logs/build.log |
| Service logs | 20 (error extract) | Redirect to $ITER_DIR/logs/service.log |
| Deploy output | 30 on failure | Redirect to $ITER_DIR/logs/deploy.log |
| Health check | 5 | Direct output OK |

---

## FORBIDDEN ACTIONS

| Action | Result |
|--------|--------|
| Stop for user confirmation | VIOLATION |
| Use docker instead of local build | VIOLATION |
| Skip reading previous iteration summary | VIOLATION |
| Let full command output into context | VIOLATION |
| Skip writing iteration summary | VIOLATION |
| Skip final summary.md | VIOLATION |
| Continue past MAX_ITERATIONS without stop | VIOLATION |

## ALLOWED ACTIONS

| Action | Rationale |
|--------|-----------|
| Break existing APIs | If required for fix |
| Remove deprecated code | Cleanup is allowed |
| Read log files with tail/head/grep | Bounded output extraction |
| Use /gofix for build errors | Efficient error resolution |
| Proceed without confirmation | Autonomous execution |

---

## WORKDIR STRUCTURE

```
$WORKDIR/
├── iteration-1/
│   ├── status.md           # Service status review
│   ├── action-plan.md      # Planned fixes
│   ├── implementation.md   # Implementation notes
│   ├── deploy-result.md    # Deploy outcome
│   ├── summary.md          # Iteration summary (REQUIRED)
│   └── logs/
│       ├── build.log
│       ├── deploy.log
│       └── service.log
├── iteration-2/
│   └── ... (same structure)
├── iteration-N/
│   └── ...
└── summary.md              # Final summary (REQUIRED)
```

**Task is NOT complete until $WORKDIR/summary.md exists.**

---

## INVOKE

```
/3agents-service-fix review and fix job failures
/3agents-service-fix fix newsletter worker errors
/3agents-service-fix diagnose and repair service startup
```

Example run:
```
/3agents-service-fix fix portfolio-newsletter job failures
# → .claude/workdir/2026-01-20-0900-service-fix-fix-portfolio-news/
#    ├── iteration-1/
#    │   ├── status.md
#    │   ├── action-plan.md
#    │   ├── implementation.md
#    │   ├── deploy-result.md
#    │   ├── summary.md
#    │   └── logs/
#    ├── iteration-2/
#    │   └── ...
#    └── summary.md (final)
```

**This workflow runs AUTONOMOUSLY through up to 5 iterations until service is healthy.**
