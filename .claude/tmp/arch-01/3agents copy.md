---
name: 3agents-skills
description: Adversarial worker/validator loop - validator checks against docs/architecture
---

Execute: $ARGUMENTS

## OVERVIEW

Simple two-agent adversarial loop:
1. **WORKER** implements the request
2. **VALIDATOR** checks against `docs/architecture/*.md` requirements
3. **Iterate** until validator approves or max iterations reached

## CONFIG
```yaml
architecture_docs: docs/architecture/
max_iterations: 3
workdir: ./docs/{type}/{date}-{slug}/
```

## RULES
- Tests: `/test/api`, `/test/ui` only
- Binaries: `go build -o /tmp/` - never in root
- **Validator is adversarial** - actively looks for violations
- **Architecture docs are the requirements** - not suggestions
- **Iterate until correct** - don't accept partial compliance

---

## PHASE 0: SETUP

### Step 0.1: Create Workdir
```bash
# Type: feature | fix
# Slug: kebab-case from request
mkdir -p ./docs/{type}/{YYYYMMDD}-{slug}/
```

### Step 0.2: Load Architecture Requirements
```bash
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md
cat docs/architecture/workers.md
```

### Step 0.3: Write Manifest
**WRITE `{workdir}/manifest.md`:**
```markdown
# {Type}: {Title}
Date: {YYYY-MM-DD}
Request: "{original input}"

## User Intent
{What the user wants - this is the goal}

## Success Criteria
- [ ] {criterion 1}
- [ ] {criterion 2}

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| manager_worker_architecture.md | {section} | {key requirement} |
| QUEUE_LOGGING.md | {section} | {key requirement} |
| QUEUE_UI.md | {section} | {key requirement} |
```

---

## PHASE 1: WORKER IMPLEMENTS

**WORKER implements the request, documenting work in `step-{N}.md`**

### Step 1.1: Implement
1. Read manifest.md for requirements
2. Read relevant architecture docs
3. Implement changes
4. Run build and tests

**WRITE `{workdir}/step-{iteration}.md`:**
```markdown
# Step {N}: Implementation
Iteration: {N} | Status: complete | partial | failed

## Changes Made
| File | Action | Description |
|------|--------|-------------|
| `{path}` | created/modified | {what changed} |

## Build & Test
Build: Pass | Fail
Tests: Pass | Fail ({details})

## Architecture Compliance (self-check)
- [ ] {requirement from manifest} - {how addressed}
```

---

## PHASE 2: VALIDATOR CHECKS

**VALIDATOR adversarially checks against architecture docs. Be harsh.**

### Step 2.1: Load Requirements
```bash
cat {workdir}/manifest.md
cat docs/architecture/*.md
```

### Step 2.2: Validate Against Each Doc

**WRITE `{workdir}/validation-{iteration}.md`:**
```markdown
# Validation {N}
Validator: adversarial | Date: {timestamp}

## Architecture Compliance Check

### manager_worker_architecture.md
| Requirement | Compliant | Evidence |
|-------------|-----------|----------|
| Job hierarchy (Manager->Step->Worker) | Y/N | {proof} |
| Correct layer (orchestration/queue/execution) | Y/N | {proof} |

### QUEUE_LOGGING.md
| Requirement | Compliant | Evidence |
|-------------|-----------|----------|
| Uses AddJobLog variants correctly | Y/N | {proof} |
| Log lines start at 1, increment sequentially | Y/N | {proof} |

### QUEUE_UI.md
| Requirement | Compliant | Evidence |
|-------------|-----------|----------|
| Icon standards (fa-clock, fa-spinner, etc.) | Y/N | {proof} |
| Auto-expand behavior for running steps | Y/N | {proof} |
| API call count < 10 per step | Y/N | {proof} |

## Build & Test Verification
Build: Pass/Fail | Tests: Pass/Fail

## Verdict: PASS | FAIL

## Violations Found (if FAIL)
1. **Violation:** {specific issue}
   **Requirement:** {from which doc, which section}
   **Fix Required:** {what worker must do}
```

---

## PHASE 3: ITERATE OR COMPLETE

### If FAIL:
1. Worker reads `validation-{N}.md`
2. Worker implements fixes in `step-{N+1}.md`
3. Validator checks again in `validation-{N+1}.md`
4. Repeat until PASS or max_iterations (3)

### If PASS:
**WRITE `{workdir}/summary.md`:**
```markdown
# Complete: {task}
Iterations: {N}

## Result
{What was delivered}

## Architecture Compliance
All requirements from docs/architecture/ verified.

## Files Changed
- `{path}` - {description}
```

### If max_iterations reached:
**STOP and report remaining violations and what user action is required.**

---

## WORKFLOW DIAGRAM
```
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 0: SETUP                                                   │
│ - Create workdir                                                 │
│ - Load docs/architecture/*.md                                    │
│ - Write manifest.md with requirements                            │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: WORKER                                                  │◄─────┐
│ - Implement changes                                              │      │
│ - Write step-{N}.md                                              │      │
│ - Run build/tests                                                │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 2: VALIDATOR (adversarial)                                 │      │
│ - Check against ALL docs/architecture/*.md                       │      │
│ - Write validation-{N}.md                                        │      │
│ - Be harsh - find violations                                     │      │
├─────────────────────────────────────────────────────────────────┤      │
│ FAIL → List violations → Worker fixes ──────────────────────────┼──────┘
│ PASS → PHASE 3                                                   │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 3: COMPLETE                                                │
│ - Write summary.md                                               │
│ - All architecture requirements verified                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## VALIDATOR ADVERSARIAL RULES

The validator MUST be harsh. Do NOT approve unless:

1. **Every applicable requirement is MET** - not "close enough"
2. **Evidence is concrete** - not "probably compliant"
3. **Build passes** - no exceptions
4. **Tests pass** - no skipping

**Validator actively looks for:**
- Icon classes that don't match standard (QUEUE_UI.md)
- Log lines not starting at 1 (QUEUE_LOGGING.md)
- Steps not auto-expanding (QUEUE_UI.md)
- Wrong layer for code placement (manager_worker_architecture.md)
- Missing event publishing (QUEUE_SERVICES.md)
- Incorrect worker interface (workers.md)

**Validator NEVER:**
- Approves "mostly compliant" implementations
- Ignores small violations "because they're minor"
- Lets things slide to finish faster
- Trusts worker's self-assessment

---

## INVOKE
```
/3agents-skills Fix the step icon mismatch    → ./docs/fix/20251212-step-icons/
/3agents-skills Add log line numbering        → ./docs/fix/20251212-log-numbering/
```

