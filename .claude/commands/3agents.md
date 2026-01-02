---
name: 3agents
description: Adversarial multi-agent loop - CORRECTNESS over SPEED
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
WORKDIR=".claude/workdir/$(date +%Y-%m-%d-%H%M)-$(echo "$ARGUMENTS" | tr ' ' '-' | cut -c1-40)"
mkdir -p "$WORKDIR"
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
└─────────────────────────────────────────────────────────────────┘
```

---

## QUAERO CODEBASE RULES

### OS Detection (MANDATORY before any shell command)

| Indicator | OS | Shell |
|-----------|-----|-------|
| `C:\...` or `D:\...` | Windows | PowerShell |
| `/home/...` or `/Users/...` | Unix/Linux/macOS | Bash |
| `/mnt/c/...` | WSL | Bash (but `powershell.exe` for Go) |

### Build & Test

| OS | Build | Test |
|----|-------|------|
| Windows | `.\scripts\build.ps1` | `go test -v ./test/...` |
| Linux/macOS | `./scripts/build.sh` | `go test -v ./test/...` |
| WSL | `powershell.exe -Command "cd C:\path; .\scripts\build.ps1"` | `powershell.exe -Command "cd C:\path; go test -v ./test/..."` |

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

## AGENTS

| Agent | Role | Stance |
|-------|------|--------|
| ARCHITECT | Requirements → step docs | Thorough |
| WORKER | Implements steps | Follow spec exactly |
| VALIDATOR | Reviews against requirements | **HOSTILE - default REJECT** |
| FINAL VALIDATOR | Reviews ALL changes together | **HOSTILE - catches cross-step issues** |
| DOCUMENTARIAN | Updates `docs/architecture` | Accurate |

---

## WORKFLOW

### PHASE 0: ARCHITECT

1. Read: `docs/architecture/*.md`, `docs/TEST_ARCHITECTURE.md`
2. Analyze existing patterns in target directories
3. Extract requirements → `$WORKDIR/requirements.md`
4. Create step docs → `$WORKDIR/step_N.md` for each step

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

**⟲ COMPACT after ARCHITECT phase**

**→ IMMEDIATELY proceed to PHASE 1 (no confirmation)**

---

### PHASE 1-3: IMPLEMENT (per step)

```
┌─────────────────────────────────────────────────────────────────┐
│ FOR EACH STEP (parallel if independent):                        │
│                                                                 │
│   WORKER: Implement → $WORKDIR/step_N_impl.md                   │
│      ↓                                                          │
│   VALIDATOR: Review → $WORKDIR/step_N_valid.md                  │
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
- Build must pass

**VALIDATOR must:**
- Default REJECT until proven correct
- Verify requirements with code line references
- Verify cleanup performed (no dead code left)
- Check codebase rule compliance

**VALIDATOR auto-REJECT:**
- Build fails
- Dead code left behind
- Old function alongside replacement
- Codebase rule violations
- Requirements not traceable to code

**⟲ COMPACT after each step PASS or at iteration 3+**

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
│ • Full build + test pass                                        │
│                                                                 │
│ REJECT → Back to relevant step for fix                          │
│ PASS → PHASE 5                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Write `$WORKDIR/final_validation.md`:**
```markdown
# Final Validation
## Build: PASS/FAIL
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
```

**⟲ COMPACT after COMPLETE**

**→ IMMEDIATELY proceed to PHASE 6 (no confirmation)**

---

### PHASE 6: DOCUMENTARIAN

Update `docs/architecture/*.md` to reflect changes.
Write `$WORKDIR/architecture-updates.md`.

**⟲ COMPACT at task end**

**→ TASK COMPLETE - output final summary only**

---

## COMPACTION POINTS

| When | Action |
|------|--------|
| After ARCHITECT | `/compact` |
| After step PASS | `/compact` |
| Iteration 3+ | `/compact` |
| After FINAL VALIDATION | `/compact` |
| Task complete | `/compact` |

**Recovery:** Read `$WORKDIR/*.md` artifacts to resume.

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

**This workflow runs AUTONOMOUSLY from start to finish.**