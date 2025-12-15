---
name: 3agents-skills
description: Adversarial 3-agent loop with Architect, Worker, and Validator - prioritizes reuse over creation
---

Execute: $ARGUMENTS

## OVERVIEW

**Three-agent adversarial loop that prioritizes CORRECTNESS over SPEED:**

1. **ARCHITECT** - Analyzes request, finds EXISTING code to reuse/extend, blocks new structures
2. **WORKER** - Implements by modifying existing code (NOT creating new files)
3. **VALIDATOR** - Harshly rejects anything non-compliant with architecture docs AND skills

**Core Philosophy:** The CORRECT result, not the QUICKEST or EASIEST.

## CONFIG
```yaml
architecture_docs: docs/architecture/
skills_docs: .claude/skills/
max_iterations: 5
workdir: ./docs/{type}/{date}-{slug}/
```

## FUNDAMENTAL RULES

### Anti-Creation Bias (CRITICAL)
```
┌─────────────────────────────────────────────────────────────────┐
│ BEFORE CREATING ANYTHING NEW, PROVE:                            │
│ 1. No existing function/type/pattern can be extended            │
│ 2. No existing code can be refactored to meet the need          │
│ 3. The new code follows EXACT patterns from existing codebase   │
│ 4. Creation was explicitly requested OR is absolutely necessary │
└─────────────────────────────────────────────────────────────────┘
```

### Hard Rules
- Tests: `/test/api`, `/test/ui` only - NEVER modify tests to make code pass
- Binaries: `go build -o /tmp/` - never in root
- **Agents are ADVERSARIAL** - they challenge each other, not agree
- **Architecture docs are LAW** - not suggestions, not guidelines
- **Skills docs define PATTERNS** - violating them is a FAIL
- **Existing code is PREFERRED** - new code is a last resort
- **Iterate until CORRECT** - not until "good enough"

---

## PHASE 0: ARCHITECT ANALYSIS (MANDATORY)

### Step 0.1: Create Workdir
```bash
mkdir -p ./docs/{type}/{YYYYMMDD}-{slug}/
```

### Step 0.2: Load ALL Reference Documents
```bash
# Architecture requirements
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md
cat docs/architecture/workers.md

# Skill patterns (MUST follow)
cat .claude/skills/go/SKILL.md
cat .claude/skills/frontend/SKILL.md
```

### Step 0.3: Architect Codebase Analysis
**ARCHITECT searches codebase BEFORE any implementation:**

1. **Find existing code that does similar things**
   - Search for functions with similar names/purposes
   - Search for types that could be extended
   - Search for patterns that could be reused

2. **Identify extension points**
   - What existing interfaces could this implement?
   - What existing services could this extend?
   - What existing patterns must this follow?

3. **Challenge the request**
   - Does this NEED new code, or can existing code be modified?
   - Is the user asking for the right thing?
   - What are the MINIMUM changes needed?

**WRITE `{workdir}/architect-analysis.md`:**
```markdown
# Architect Analysis
Date: {YYYY-MM-DD}
Request: "{original input}"

## User Intent
{What the user actually wants - not what they said}

## Existing Code Analysis
| Purpose | Existing Code | Can Extend? | Notes |
|---------|--------------|-------------|-------|
| {purpose} | `{file}:{function}` | YES/NO | {why} |

## Recommended Approach
**EXTEND** | **MODIFY** | **CREATE** (justify if CREATE)

### If EXTEND/MODIFY:
- File: `{path}`
- Function/Type: `{name}`
- Changes needed: {description}

### If CREATE (requires justification):
- **Why existing code cannot be used:** {specific reason}
- **Pattern to follow:** `{existing_file}` as template
- **Minimum viable change:** {description}

## Anti-Patterns Check
| Anti-Pattern | Risk | Mitigation |
|--------------|------|------------|
| Creating parallel structure | {YES/NO} | {how to avoid} |
| Duplicating existing logic | {YES/NO} | {how to avoid} |
| Ignoring existing patterns | {YES/NO} | {how to avoid} |

## Success Criteria (MEASURABLE)
- [ ] {criterion 1 - specific and testable}
- [ ] {criterion 2}

## Architecture Requirements (from docs)
| Doc | Section | Requirement | Applicable? |
|-----|---------|-------------|-------------|
| manager_worker_architecture.md | {section} | {requirement} | Y/N |
| QUEUE_LOGGING.md | {section} | {requirement} | Y/N |
| .claude/skills/go/SKILL.md | {section} | {requirement} | Y/N |
```

---

## PHASE 1: WORKER IMPLEMENTS

**WORKER implements ONLY what Architect approved, following EXACT patterns.**

### Step 1.1: Pre-Implementation Checklist
Before writing ANY code:
- [ ] Read architect-analysis.md completely
- [ ] Confirm approach is EXTEND/MODIFY (not CREATE unless justified)
- [ ] Identify the EXACT file(s) to modify
- [ ] Find the EXACT pattern to follow in existing code

### Step 1.2: Implement (Minimum Viable Change)
1. Modify EXISTING files when possible
2. Follow EXACT patterns from `.claude/skills/`
3. Run build BEFORE committing to approach
4. Run tests BEFORE claiming completion

**WRITE `{workdir}/step-{iteration}.md`:**
```markdown
# Step {N}: Implementation
Iteration: {N} | Status: complete | partial | failed

## Architect Compliance
- Recommended approach: {EXTEND/MODIFY/CREATE}
- Actual approach: {what I did}
- Deviation justification: {if different from architect}

## Changes Made
| File | Action | Lines Changed | Justification |
|------|--------|---------------|---------------|
| `{path}` | modified | +5/-2 | {why this change} |

## New Code Created (if any)
**REQUIRES JUSTIFICATION:**
- New file: `{path}` - WHY: {specific reason existing code couldn't work}

## Pattern Compliance
| Pattern Source | Pattern | Followed? | Evidence |
|----------------|---------|-----------|----------|
| .claude/skills/go/SKILL.md | Error handling | Y/N | `{code snippet}` |
| .claude/skills/go/SKILL.md | Logging with arbor | Y/N | `{code snippet}` |
| Existing code `{file}` | {pattern} | Y/N | `{code snippet}` |

## Build & Test
```
Build: Pass | Fail
Tests: Pass | Fail ({specific failures})
```

## Self-Critique (BE HARSH)
- What could I have done better?
- Did I create anything unnecessary?
- Did I follow the architect's recommendation?
```

---

## PHASE 2: VALIDATOR CHECKS (ADVERSARIAL)

**VALIDATOR's job is to REJECT. Finding approval is FAILURE to do the job.**

### Step 2.1: Load All Reference Material
```bash
cat {workdir}/architect-analysis.md
cat {workdir}/step-{N}.md
cat docs/architecture/*.md
cat .claude/skills/go/SKILL.md
cat .claude/skills/frontend/SKILL.md
```

### Step 2.2: Adversarial Validation

**VALIDATOR assumes implementation is WRONG until proven RIGHT.**

**WRITE `{workdir}/validation-{iteration}.md`:**
```markdown
# Validation {N}
Validator: ADVERSARIAL | Date: {timestamp}
Initial Stance: REJECT (must be convinced to approve)

## Architect Alignment Check
| Criterion | Expected | Actual | PASS/FAIL |
|-----------|----------|--------|-----------|
| Approach followed | {EXTEND/MODIFY/CREATE} | {what worker did} | ? |
| Files modified vs created | {expected} | {actual} | ? |
| Minimum viable change | {expected scope} | {actual scope} | ? |

## Anti-Creation Audit
| Question | Answer | FAIL if wrong |
|----------|--------|---------------|
| Were new files created? | Y/N | FAIL if Y without justification |
| Could existing code have been extended? | Y/N | FAIL if Y but new code created |
| Does new code duplicate existing patterns? | Y/N | FAIL if Y |
| Were new structures/types created unnecessarily? | Y/N | FAIL if Y |

## Architecture Compliance (STRICT)

### manager_worker_architecture.md
| Requirement | Compliant | Evidence (CONCRETE) |
|-------------|-----------|---------------------|
| Job hierarchy (Manager->Step->Worker) | Y/N | Line X in file Y shows... |
| Correct layer placement | Y/N | {proof with line numbers} |

### QUEUE_LOGGING.md
| Requirement | Compliant | Evidence (CONCRETE) |
|-------------|-----------|---------------------|
| Uses AddJobLog correctly | Y/N | {proof with code} |
| Log lines start at 1 | Y/N | {proof with code} |

### QUEUE_UI.md
| Requirement | Compliant | Evidence (CONCRETE) |
|-------------|-----------|---------------------|
| Icon standards | Y/N | {proof with classes used} |
| Auto-expand behavior | Y/N | {proof with code} |

### .claude/skills/go/SKILL.md
| Requirement | Compliant | Evidence (CONCRETE) |
|-------------|-----------|---------------------|
| Error wrapping with context | Y/N | {proof with code} |
| Arbor structured logging | Y/N | {proof with code} |
| Constructor injection DI | Y/N | {proof with code} |
| Interface-based dependencies | Y/N | {proof with code} |
| No global state | Y/N | {proof} |

## Build & Test Verification
```
Build: {actual output or PASS}
Tests: {actual output or PASS}
```

## Violations Found
| # | Severity | Violation | Requirement | Fix Required |
|---|----------|-----------|-------------|--------------|
| 1 | CRITICAL/MAJOR/MINOR | {issue} | {from doc} | {specific fix} |

## Verdict: PASS | FAIL

**FAIL reasons (if any):**
1. {reason 1}
2. {reason 2}

**PASS requires ALL of:**
- [ ] Zero CRITICAL violations
- [ ] Zero MAJOR violations
- [ ] Build passes
- [ ] Tests pass
- [ ] Architect approach followed
- [ ] No unnecessary new code
```

---

## PHASE 3: ITERATE OR COMPLETE

### If FAIL:
1. Worker reads `validation-{N}.md` CAREFULLY
2. Worker addresses EACH violation specifically
3. Worker implements fixes in `step-{N+1}.md`
4. Validator checks AGAIN in `validation-{N+1}.md` (equally harsh)
5. Repeat until PASS or max_iterations (5)

### If PASS (all checks satisfied):
**WRITE `{workdir}/summary.md`:**
```markdown
# Complete: {task}
Iterations: {N}

## Result
{What was delivered}

## Approach Taken
- Strategy: {EXTEND/MODIFY/CREATE}
- Files changed: {count}
- Lines changed: +{added}/-{removed}

## Architecture Compliance
All requirements from docs/architecture/ verified.
All patterns from .claude/skills/ followed.

## Files Changed
- `{path}` - {description}

## What Was NOT Created (good sign)
- {unnecessary code that was avoided}
```

### If max_iterations reached:
**STOP and report:**
1. Remaining violations
2. Why they cannot be fixed
3. What user decision is needed

---

## WORKFLOW DIAGRAM
```
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 0: ARCHITECT ANALYSIS (MANDATORY)                         │
│ - Search codebase for existing code to reuse                    │
│ - Challenge: Is new code actually needed?                       │
│ - Recommend: EXTEND > MODIFY > CREATE                           │
│ - Write architect-analysis.md                                   │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: WORKER IMPLEMENTS                                       │◄─────┐
│ - Follow architect's recommendation                              │      │
│ - Minimum viable change                                          │      │
│ - Must justify any new code                                      │      │
│ - Write step-{N}.md                                              │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 2: VALIDATOR (ADVERSARIAL)                                 │      │
│ - Assume REJECT until proven otherwise                           │      │
│ - Check anti-creation violations                                 │      │
│ - Check architecture doc compliance                              │      │
│ - Check skill pattern compliance                                 │      │
│ - Write validation-{N}.md                                        │      │
├─────────────────────────────────────────────────────────────────┤      │
│ FAIL → List ALL violations → Worker fixes ──────────────────────┼──────┘
│ PASS → PHASE 3 (requires ZERO critical/major violations)        │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 3: COMPLETE                                                │
│ - Write summary.md                                               │
│ - Confirm minimal change achieved                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## ADVERSARIAL BEHAVIOR RULES

### Architect MUST:
- **Challenge the request** - Is this really needed?
- **Find existing code first** - Never assume creation is needed
- **Block unnecessary structures** - No parallel hierarchies
- **Recommend minimum viable change** - Not the "proper" or "complete" solution

### Worker MUST:
- **Follow architect exactly** - Deviation requires written justification
- **Prefer modification** - Creating is a failure mode
- **Self-critique harshly** - Find your own mistakes first
- **Question your changes** - Are all of these necessary?

### Validator MUST:
- **Assume failure** - Start from REJECT position
- **Require concrete evidence** - Not "probably compliant"
- **Check anti-creation** - Did worker create unnecessarily?
- **Never approve "mostly correct"** - 100% or FAIL
- **Never trust self-assessment** - Verify everything independently

### All Agents MUST NOT:
- Agree with each other to finish faster
- Skip checks because "it's probably fine"
- Approve partial compliance
- Create new code without exhausting existing options
- Modify tests to make code pass
- Let minor violations slide

---

## SKILL PATTERN ENFORCEMENT

### Go Patterns (from .claude/skills/go/SKILL.md)
```go
// REQUIRED: Error wrapping
if err != nil {
    return fmt.Errorf("context: %w", err)
}

// REQUIRED: Arbor structured logging
logger.Info("message", "key", value)

// REQUIRED: Constructor injection
func NewService(dep Interface) *Service

// FORBIDDEN: Global state
var globalDB *badger.DB // ❌ FAIL

// FORBIDDEN: Panic
panic(err) // ❌ FAIL

// FORBIDDEN: fmt.Println
fmt.Println("debug") // ❌ FAIL
```

### Frontend Patterns (from .claude/skills/frontend/SKILL.md)
- Alpine.js for interactivity
- Bulma CSS for styling
- Server-side rendering with Go templates

---

## INVOKE
```
/3agents-skills Fix the step icon mismatch    → ./docs/fix/20251215-step-icons/
/3agents-skills Add log line numbering        → ./docs/fix/20251215-log-numbering/
```

