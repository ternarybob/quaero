# Adversarial Workflow Skill

## Purpose

This skill defines the adversarial multi-agent pattern for high-quality code changes. Use when executing `/3agents` command.

## Core Principle

```
┌─────────────────────────────────────────────────────────────────┐
│ ADVERSARIAL VALIDATION                                          │
│                                                                  │
│ The VALIDATOR agent is HOSTILE to the WORKER agent.             │
│ Default stance: REJECT until proven correct.                    │
│                                                                  │
│ This creates tension that produces higher quality code.         │
└─────────────────────────────────────────────────────────────────┘
```

## Agent Roles

### ARCHITECT (thinker + file-picker)

**Responsibility:** Analysis and planning

**Actions:**
1. Spawn multiple `file-picker` agents to explore codebase
2. Use `code-searcher` to find existing patterns
3. Spawn `thinker` to plan implementation
4. Write step documents with acceptance criteria

**Outputs:**
- `requirements.md` - Extracted requirements (LAW)
- `step_N.md` - Implementation specification per step
- `architect-analysis.md` - Patterns and decisions

### WORKER (editor)

**Responsibility:** Implementation

**Constraints:**
- Follow step doc EXACTLY
- No creative interpretation
- Match existing codebase patterns
- Perform mandatory cleanup

**Process:**
1. Read step doc before any code changes
2. Spawn `editor` agent to implement
3. Verify build passes
4. Document implementation

**Outputs:**
- `step_N_impl.md` - Implementation notes

### VALIDATOR (code-reviewer)

**Responsibility:** Adversarial review

**Stance:** HOSTILE - Default REJECT

**Checks:**
- [ ] Each requirement traceable to code (line references)
- [ ] No dead code left behind
- [ ] Old functions removed if replaced
- [ ] Codebase style matched
- [ ] Build passes

**Auto-REJECT triggers:**
- Build fails
- Dead code present
- Missing cleanup
- Requirement not implemented
- Pattern violation

**Outputs:**
- `step_N_valid.md` - Validation results with verdict

### FINAL VALIDATOR (code-reviewer)

**Responsibility:** Cross-step validation

**Additional checks:**
- [ ] No conflicts between steps
- [ ] Consistent patterns across ALL changes
- [ ] All requirements satisfied together
- [ ] Full test suite passes

**Outputs:**
- `final_validation.md` - Final verdict

### DOCUMENTARIAN (editor)

**Responsibility:** Update architecture docs

**Outputs:**
- `architecture-updates.md` - Changes made to docs

## Iteration Loop

```
WORKER implements
       ↓
VALIDATOR reviews
       ↓
   ┌───┴───┐
   │       │
REJECT   PASS
   │       │
   ↓       ↓
Iterate  Next Step
(max 5)
```

## Validation Template

When spawning `code-reviewer` for validation:

```
Review the implementation for Step N.

Your stance: HOSTILE - default REJECT.

Check against:
1. Requirements in $WORKDIR/requirements.md
2. Step spec in $WORKDIR/step_N.md
3. Acceptance criteria

Auto-REJECT if:
- Build fails
- Dead code present
- Missing cleanup
- Requirement not traceable to code
- Pattern violation

Write verdict to $WORKDIR/step_N_valid.md
```

## Step Document Template

```markdown
# Step N: <title>

## Dependencies
[none | step_1, step_2]

## Requirements Addressed
REQ-1, REQ-2

## Approach
### Files to Modify
- `path/to/file.go` - Change X

### Changes
1. Modify function Y to do Z
2. Add handler for W

### Patterns to Follow
- Match pattern in `existing/file.go`

## Cleanup Required
- Remove deprecated function `oldFunc()`
- Delete unused import

## Acceptance Criteria
- AC-1: X returns Y
- AC-2: Z handles error case
- AC-3: Build passes
```

## Quality Gates

### Per-Step Gates
1. Build passes
2. Step acceptance criteria met
3. Cleanup performed
4. No dead code

### Final Gates
1. ALL requirements satisfied
2. ALL tests pass
3. No cross-step conflicts
4. Documentation updated

## Anti-Patterns

```
❌ VALIDATOR rubber-stamps changes
❌ Skipping cleanup "for now"
❌ Proceeding despite build failure
❌ Implementing beyond step scope
❌ Leaving old code "just in case"
❌ Asking user for confirmation
```

## Success Criteria

The workflow succeeds when:
1. `$WORKDIR/summary.md` exists
2. All requirements have PASS status
3. Build and tests pass
4. No dead code in codebase
5. Architecture docs updated
