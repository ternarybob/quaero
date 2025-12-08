---
name: 3agents-skills
description: Three-agent workflow with skills. Opus plans, Sonnet implements with skill context, Sonnet validates.
---

Execute task: $ARGUMENTS

---

## SKILLS CONFIGURATION

Skills provide domain-specific context to each agent phase. Load skills before execution.

```yaml
skills:
  go:
    path: .claude/skills/go/SKILL.md
    applies_to: [implementation, refactoring, handlers, services, tests]
  
  frontend:
    path: .claude/skills/frontend/SKILL.md  
    applies_to: [templates, alpine, css, ui]
  
  architecture:
    path: .claude/skills/architecture/SKILL.md
    applies_to: [design, structure, patterns]

# Skill selection rules
skill_routing:
  - pattern: "*.go"
    skills: [go]
  - pattern: "*.html"
    skills: [frontend, go]  # Templates need both
  - pattern: "*_test.go"
    skills: [go]
  - pattern: "internal/handlers/*"
    skills: [go, frontend]
  - pattern: "internal/services/*"
    skills: [go]
  - pattern: "internal/queue/*"
    skills: [go]
```

---

## AGENT MODELS

```yaml
agents:
  planner:
    model: claude-opus-4-5-20251101
    role: Architect - deep analysis, skill selection, parallelization
    
  implementer:
    model: claude-sonnet-4-20250514
    role: Builder - executes with skill context, writes code
    instances: 1-3  # Parallel for independent tasks
    
  validator:
    model: claude-sonnet-4-20250514
    role: Reviewer - verifies against skills, runs tests
```

---

## EXECUTION RULES

- **Skill Loading:** Each agent loads relevant skills before work
- **Tests:** Only in `/test/api` and `/test/ui` directories
- **Binaries:** Never in root - use `go build -o /tmp/` or `go run`
- **Complete:** Run all phases to completion - only pause for design decisions
- **Parallel:** Independent implementation steps can run simultaneously

---

## PHASE 1: PLANNER (Opus)

**Purpose:** Deep analysis, skill selection, work decomposition

### Step 1.1: Skill Discovery
```bash
# List available skills
ls -la .claude/skills/
```

Load and analyze relevant skills based on $ARGUMENTS:
- If task involves Go code → load `go/SKILL.md`
- If task involves templates/UI → load `frontend/SKILL.md`
- If task involves architecture → load `architecture/SKILL.md`

### Step 1.2: Analysis (Extended Thinking)

Before creating the plan, think deeply:

1. **Scope Analysis**
   - What files/packages are affected?
   - What are the dependencies between changes?
   - Which skills apply to each component?

2. **Risk Assessment**
   - What could break?
   - What tests need updating?
   - Are there migration concerns?

3. **Parallelization Opportunities**
   - Which tasks are independent?
   - What's the critical path?
   - How many implementer instances needed?

### Step 1.3: Generate Implementation Plan

Create `{workdir}/01-plan.md`:

```markdown
# Implementation Plan: {task_summary}

## Skills Required
- [ ] go - for {specific reasons}
- [ ] frontend - for {specific reasons}

## Work Packages

### WP1: {name} [PARALLEL-SAFE]
**Skills:** go
**Files:** internal/services/foo.go
**Description:** {what to do}
**Acceptance:** {how to verify}

### WP2: {name} [DEPENDS: WP1]
**Skills:** go, frontend
**Files:** internal/handlers/foo.go, web/templates/foo.html
**Description:** {what to do}
**Acceptance:** {how to verify}

### WP3: {name} [PARALLEL-SAFE]
**Skills:** go
**Files:** internal/services/foo_test.go
**Description:** {what to do}
**Acceptance:** {how to verify}

## Execution Order
1. WP1, WP3 (parallel)
2. WP2 (after WP1)

## Validation Checklist
- [ ] All tests pass: `go test ./...`
- [ ] Lint clean: `golangci-lint run`
- [ ] Follows skill patterns
```

---

## PHASE 2: IMPLEMENTER (Sonnet)

**Purpose:** Execute work packages with skill context

### For Each Work Package:

#### Step 2.1: Load Required Skills
```bash
# Example for WP with go skill
cat .claude/skills/go/SKILL.md
```

#### Step 2.2: Implement
Apply skill patterns while implementing:
- Follow code patterns from skill
- Use recommended error handling
- Match project structure

#### Step 2.3: Self-Check Against Skill
Before completing, verify:
- [ ] Matches skill's recommended patterns
- [ ] Avoids skill's listed anti-patterns
- [ ] Uses correct package structure
- [ ] Error handling follows guidelines

#### Step 2.4: Document Changes
Append to `{workdir}/02-changes.md`:
```markdown
## WP{n}: {name}

### Files Modified
- `path/to/file.go` - {summary}

### Skill Compliance
- [x] Error wrapping with context
- [x] Structured logging
- [x] Table-driven tests
- [ ] N/A - no handlers in this WP

### Ready for Validation
```

---

## PHASE 3: VALIDATOR (Sonnet)

**Purpose:** Verify implementation against skills and requirements

### Step 3.1: Load All Applicable Skills
```bash
cat .claude/skills/go/SKILL.md
cat .claude/skills/frontend/SKILL.md  # if applicable
```

### Step 3.2: Run Automated Checks
```bash
# Build
go build -o /tmp/quaero ./cmd/quaero

# Test
go test -v ./...

# Lint
golangci-lint run

# Check for common issues
grep -r "panic(" internal/  # Should be empty
grep -r "log\." internal/   # Should use slog instead
```

### Step 3.3: Skill Compliance Review

For each modified file, verify against loaded skills:

**Go Skill Checklist:**
- [ ] Context passed to DB/network calls
- [ ] Errors wrapped with `%w`
- [ ] Structured logging with slog
- [ ] No global state
- [ ] Handlers are thin (logic in services)
- [ ] Tests are table-driven

**Frontend Skill Checklist (if applicable):**
- [ ] Alpine.js data binding correct
- [ ] JSON helper used for template data
- [ ] No inline scripts

### Step 3.4: Generate Report

Create `{workdir}/03-validation.md`:
```markdown
# Validation Report

## Automated Checks
- Build: ✅ PASS
- Tests: ✅ PASS (coverage: X%)
- Lint: ✅ PASS

## Skill Compliance

### go/SKILL.md
| Pattern | Status | Notes |
|---------|--------|-------|
| Error wrapping | ✅ | All errors use %w |
| Structured logging | ✅ | slog throughout |
| Table-driven tests | ✅ | New tests follow pattern |

### Issues Found
{none or list}

### Recommendations
{optional improvements}

## Result: ✅ APPROVED / ⚠️ NEEDS REVISION
```

---

## PHASE 4: COMPLETION

If validation passes:
1. Summarize changes to user
2. List files modified
3. Note any follow-up tasks

If validation fails:
1. Return to Phase 2 with specific fixes
2. Re-validate after fixes
3. Max 2 revision cycles before escalating to user

---

## WORKDIR STRUCTURE

```
docs/work/{timestamp}-{slug}/
├── 01-plan.md           # Planner output
├── 02-changes.md        # Implementer log
├── 03-validation.md     # Validator report
└── 04-summary.md        # Final summary (if complete)
```

---

## EXAMPLE INVOCATION

```bash
# From project root
claude "/3agents-skills add rate limiting to crawl jobs"

# With a plan file
claude "/3agents-skills docs/features/rate-limiting-spec.md"
```