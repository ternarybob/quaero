---
name: 3agents
description: Adversarial multi-agent loop - CORRECTNESS over SPEED
---

Execute: $ARGUMENTS

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
└─────────────────────────────────────────────────────────────────┘
```

### Skills (read before applicable work)
| Skill | When | Path |
|-------|------|------|
| Refactoring | ALWAYS | `.claude/skills/refactoring/SKILL.md` |
| Go | Go changes | `.claude/skills/go/SKILL.md` |
| Frontend | UI changes | `.claude/skills/frontend/SKILL.md` |
| Monitoring | UI tests | `.claude/skills/monitoring/SKILL.md` |

### Config Parity
Changes to `./bin` MUST mirror to `./deployments/common` + `./test/config`

## AGENTS

| Agent | Role | Stance |
|-------|------|--------|
| ARCHITECT | Requirements → step docs | Thorough |
| WORKER | Implements steps | Follow spec exactly |
| VALIDATOR | Reviews against requirements/skills | **HOSTILE - default REJECT** |
| FINAL VALIDATOR | Reviews ALL changes together | **HOSTILE - catches cross-step issues** |
| DOCUMENTARIAN | Updates `docs/architecture` | Accurate |

## WORKFLOW

### PHASE 0: ARCHITECT

1. Read: `docs/architecture/*.md`, `docs/TEST_ARCHITECTURE.md`, applicable skills
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

---

### PHASE 1-3: IMPLEMENT (per step)

**Execution modes:**
- **Sequential:** Steps with dependencies execute in order
- **Parallel:** Independent steps (Deps: none) can batch-execute
```
┌─────────────────────────────────────────────────────┐
│ FOR EACH STEP (parallel if independent):           │
│                                                    │
│   WORKER: Implement → $WORKDIR/step_N_impl.md      │
│      ↓                                             │
│   VALIDATOR: Review → $WORKDIR/step_N_valid.md     │
│      ↓                                             │
│   PASS → next step    REJECT → iterate (max 5)    │
└─────────────────────────────────────────────────────┘
```

**WORKER must:**
- Follow step doc exactly
- Apply skills (EXTEND > MODIFY > CREATE)
- Perform cleanup listed in step doc
- Build must pass

**VALIDATOR must:**
- Default REJECT until proven correct
- Verify requirements with code line references
- Verify cleanup performed (no dead code left)
- Check skill compliance (READ skill files, don't rely on memory)

**VALIDATOR auto-REJECT:**
- Build fails
- Dead code left behind
- Old function alongside replacement
- Skill violations
- Requirements not traceable to code

**⟲ COMPACT after each step PASS or at iteration 3+**

---

### PHASE 4: FINAL VALIDATION (MANDATORY)

**Purpose:** Catch cross-step issues, verify holistic correctness.
```
┌─────────────────────────────────────────────────────┐
│ FINAL VALIDATOR reviews ALL changes together:      │
│                                                    │
│ • Re-read $WORKDIR/requirements.md                 │
│ • Verify ALL requirements satisfied                │
│ • Check for conflicts between steps                │
│ • Verify no dead code across ALL changes           │
│ • Verify consistent patterns across ALL changes    │
│ • Full build + test pass                           │
│                                                    │
│ REJECT → Back to relevant step for fix             │
│ PASS → PHASE 5                                     │
└─────────────────────────────────────────────────────┘
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

---

### PHASE 6: DOCUMENTARIAN

Update `docs/architecture/*.md` to reflect changes.
Write `$WORKDIR/architecture-updates.md`.

**⟲ COMPACT at task end**

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

## INVOKE
```
/3agents Fix the step icon mismatch
```