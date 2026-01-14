# 3agents - Adversarial Multi-Agent Workflow

**Execute:** $ARGUMENTS

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

## SETUP (MANDATORY - DO FIRST)

```bash
WORKDIR=".codebuff/workdir/$(date +%Y-%m-%d-%H%M)-$(echo "$ARGUMENTS" | tr ' ' '-' | cut -c1-40)"
mkdir -p "$WORKDIR"
mkdir -p "$WORKDIR/logs"
echo "Workdir: $WORKDIR"
```

## FUNDAMENTAL RULES

```
┌─────────────────────────────────────────────────────────────────┐
│ • CORRECTNESS over SPEED                                        │
│ • Requirements are LAW - no interpretation                      │
│ • EXISTING PATTERNS ARE LAW - match codebase style              │
│ • BACKWARD COMPATIBILITY NOT REQUIRED - break if needed         │
│ • CLEANUP IS MANDATORY - remove dead/redundant code             │
│ • STEPS ARE MANDATORY - no implementation without step docs     │
│ • SUMMARY IS MANDATORY - task incomplete without summary.md     │
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
│ • See ONLY pass/fail + last 30 lines on failure                 │
│ • NEVER let full command output into context                    │
│ • Reference log files by path, don't paste contents             │
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

## SKILLS (Read before applicable work)

| Skill | Path | When |
|-------|------|------|
| Refactoring | `.codebuff/skills/refactoring/SKILL.md` | ALL changes |
| Go | `.codebuff/skills/go/SKILL.md` | Go code changes |
| Frontend | `.codebuff/skills/frontend/SKILL.md` | Frontend changes |
| Adversarial | `.codebuff/skills/adversarial-workflow/SKILL.md` | This workflow |

---

## AGENT ROLES

| Agent | Role | Codebuff Agent | Stance |
|-------|------|----------------|--------|
| ARCHITECT | Requirements → step docs | `thinker` + `file-picker` | Thorough |
| WORKER | Implements steps | `editor` | Follow spec exactly |
| VALIDATOR | Reviews against requirements | `code-reviewer` | **HOSTILE - default REJECT** |
| FINAL VALIDATOR | Reviews ALL changes together | `code-reviewer` | **HOSTILE - catches cross-step issues** |
| DOCUMENTARIAN | Updates `docs/architecture` | `editor` | Accurate |

---

## WORKFLOW

### PHASE 0: ARCHITECT

**Use agents:** `thinker` for planning, `file-picker` for finding files, `code-searcher` for patterns

**Steps:**
1. Spawn `file-picker` agents (2-5 in parallel) to find relevant files
2. Read architecture docs: `docs/architecture/*.md`
3. Read applicable skills from `.codebuff/skills/`
4. Analyze existing patterns in target directories using `code-searcher`
5. Spawn `thinker` to analyze requirements and plan implementation

**Create artifacts:**
```bash
# Write requirements
cat > "$WORKDIR/requirements.md" << 'EOF'
# Requirements
## REQ-1: <requirement>
## REQ-2: <requirement>
...
EOF

# Write step docs (one per step)
cat > "$WORKDIR/step_1.md" << 'EOF'
# Step 1: <title>
## Deps: [none | step_1, step_2]  # Enables parallelization
## Requirements: REQ-1, REQ-2
## Approach: <files, changes, patterns>
## Cleanup: <functions/code to remove>
## Acceptance: AC-1, AC-2
EOF

# Write architect analysis
cat > "$WORKDIR/architect-analysis.md" << 'EOF'
# Architect Analysis
## Patterns Found
## Decisions Made
## Cleanup Candidates
EOF
```

**→ IMMEDIATELY proceed to PHASE 1 (no confirmation)**

---

### PHASE 1-3: IMPLEMENT (per step)

```
┌─────────────────────────────────────────────────────────────────┐
│ FOR EACH STEP:                                                  │
│                                                                 │
│   WORKER: Spawn `editor` agent                                  │
│      → Implement → $WORKDIR/step_N_impl.md                      │
│      ↓                                                          │
│   BUILD CHECK (output to $WORKDIR/logs/build_stepN_iterM.log)   │
│      ↓                                                          │
│   VALIDATOR: Spawn `code-reviewer` agent                        │
│      → Review → $WORKDIR/step_N_valid.md                        │
│      ↓                                                          │
│   PASS → next step    REJECT → iterate (max 5)                  │
│                                                                 │
│ DO NOT STOP BETWEEN STEPS - continue automatically              │
└─────────────────────────────────────────────────────────────────┘
```

**WORKER must:**
- Read step doc before implementing
- Follow step doc exactly
- Apply codebase rules (logging, error handling, structure)
- Perform cleanup listed in step doc
- Build must pass (verified with output capture)

**Build verification (MANDATORY with output capture):**
```bash
STEP=N
ITER=M
BUILD_LOG="$WORKDIR/logs/build_step${STEP}_iter${ITER}.log"

# OS Detection
if [[ "$PWD" == /mnt/c/* ]]; then
    # WSL - use PowerShell for Go
    powershell.exe -Command "cd C:\path; .\scripts\build.ps1" > "$BUILD_LOG" 2>&1
elif [[ -f ./scripts/build.sh ]]; then
    # Unix/Linux/macOS
    ./scripts/build.sh > "$BUILD_LOG" 2>&1
else
    # Windows
    powershell.exe -Command ".\scripts\build.ps1" > "$BUILD_LOG" 2>&1
fi

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

**VALIDATOR must (spawn `code-reviewer` with HOSTILE stance):**
- Default REJECT until proven correct
- Verify requirements with code line references
- Verify cleanup performed (no dead code left)
- Check codebase rule compliance
- Verify build passed

**VALIDATOR auto-REJECT:**
- Build fails
- Dead code left behind
- Old function alongside replacement
- Codebase rule violations
- Requirements not traceable to code

**Write implementation notes:**
```bash
cat > "$WORKDIR/step_${STEP}_impl.md" << 'EOF'
# Step N Implementation
## Files Changed
## Key Decisions
## Cleanup Performed
EOF
```

**Write validation results:**
```bash
cat > "$WORKDIR/step_${STEP}_valid.md" << 'EOF'
# Step N Validation
## Requirements Check
| REQ | Status | Code Reference |
## Cleanup Check
## Build Status
## Verdict: PASS/REJECT
EOF
```

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
│ Spawn `code-reviewer` with prompt:                              │
│ "Review ALL changes for final validation. Be HOSTILE."          │
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

Spawn `editor` agent to update `docs/architecture/*.md` to reflect changes.

Write `$WORKDIR/architecture-updates.md`:
```markdown
# Architecture Updates
## Files Updated
## Changes Made
```

**→ TASK COMPLETE - output final summary only**

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
# → .codebuff/workdir/2024-12-17-1430-implement-feature-X-for-componen/
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
