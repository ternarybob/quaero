---
name: 3agents
description: Three-agent workflow with parallel execution. Opus plans, Sonnet workers execute in parallel sandboxes, Sonnet validates incrementally.
---

Execute workflow for: $ARGUMENTS

## CLAUDE CLI CONFIGURATION

```yaml
# Agent model assignments - HYBRID APPROACH
agents:
  planner:
    model: claude-opus-4-5-20251101
    mode: interactive  # Extended thinking for planning
    flags: ["--verbose"]
    purpose: "Deep analysis, dependency mapping, parallelization strategy"
  
  implementer:
    model: claude-sonnet-4-5-20250929
    mode: print  # Non-interactive, parallel-safe
    flags: ["--print", "--output-format", "json"]
    purpose: "Fast parallel execution of bounded tasks"
  
  validator:
    model: claude-sonnet-4-5-20250929
    mode: print
    flags: ["--print"]
    purpose: "Quick incremental validation (compile, test, basic review)"
  
  final_reviewer:
    model: claude-opus-4-5-20251101
    mode: interactive
    flags: ["--verbose"]
    purpose: "Deep review of critical paths before completion"
    triggers:
      - security
      - authentication
      - authorization
      - payments
      - data-migration
      - crypto
      - api-breaking
      - database-schema

# Step complexity overrides - use Opus for complex implementations
step_complexity:
  high:
    model: claude-opus-4-5-20251101
    indicators:
      - "architectural change"
      - "breaking change"
      - "security sensitive"
      - "complex algorithm"
      - "state machine"
      - "concurrent/parallel logic"
  medium:
    model: claude-sonnet-4-5-20250929
    indicators:
      - "standard implementation"
      - "add endpoint"
      - "add tests"
  low:
    model: claude-sonnet-4-5-20250929
    indicators:
      - "rename"
      - "move file"
      - "update config"
      - "fix typo"

# Execution settings
execution:
  max_parallel_workers: 3
  sandbox: true
  working_dir: /tmp/3agents-work
  final_review: auto  # auto | always | never | critical-only
  
limits:
  max_retries: 2
  step_timeout: 300  # 5 min per step
  opus_timeout: 600  # 10 min for Opus steps

skills:
  code-architect: [architecture, design, refactoring, structure]
  go-coder: [implementation, coding, functions, handlers]
  test-writer: [tests, test coverage, test patterns]
  none: [documentation, planning, non-code]
```

---

## INPUT HANDLING

**If $ARGUMENTS is a file path (e.g., `docs/fixes/01-plan-v1-xxx.md`):**
```bash
DIR=$(dirname "$ARGUMENTS")
BASE=$(basename "$ARGUMENTS" .md)
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
WORKDIR="${DIR}/${TIMESTAMP}-${BASE}/"
mkdir -p "$WORKDIR"
```

**If $ARGUMENTS is a task description:**
```bash
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
SLUG=$(echo "$ARGUMENTS" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | cut -c1-30)
WORKDIR="docs/features/${TIMESTAMP}-${SLUG}/"
mkdir -p "$WORKDIR"
```

---

## RULES

- **Tests:** Only `/test/api` and `/test/ui`
- **Binaries:** Never in root - use `go build -o /tmp/` or `go run`
- **Beta mode:** Breaking changes allowed
- **Skills:** Use @skill directives for each step
- **Complete:** Run all steps to completion - only stop for design decisions
- **Parallel:** Independent steps execute simultaneously

---

## PHASE 1: PLANNER (Opus)

**Purpose:** Deep analysis and parallelization planning

**Opus must think before planning.** The planner uses extended thinking to:
1. Analyze the full scope
2. Identify dependencies between steps
3. Group independent steps for parallel execution
4. Flag genuine decision points

### Planner Prompt

```
You are the PLANNER agent. Before creating the plan, think deeply about:

<planning_analysis>
1. What are ALL the discrete tasks needed?
2. Which tasks depend on others? Draw the dependency graph mentally.
3. Which tasks are INDEPENDENT and can run in parallel?
4. What decisions genuinely require user input vs. technical choices you can make?
5. What's the critical path?
</planning_analysis>

After thinking, create plan.md with parallel groups identified.
```

### Output: `plan.md`

```markdown
# Plan: {task}

## Dependency Analysis
{Brief explanation of what depends on what}

## Critical Path Flags
{List any steps that trigger Opus final review}
- Step 2a: security (triggers final review)
- Step 3: api-breaking (triggers final review)

## Execution Groups

### Group 1 (Sequential - Foundation)
These must run first, in order:

1. **{Description}**
   - Skill: @{skill}
   - Files: {paths}
   - Complexity: low | medium | high
   - Critical: no | yes:{trigger}
   - Depends on: none
   - User decision: no

### Group 2 (Parallel - Independent Work)
These can run simultaneously after Group 1:

2a. **{Description}**
    - Skill: @{skill}
    - Files: {paths}
    - Complexity: high  # Uses Opus for implementation
    - Critical: yes:security  # Triggers final review
    - Depends on: Step 1
    - User decision: no
    - Sandbox: worker-a

2b. **{Description}**
    - Skill: @{skill}
    - Files: {paths}
    - Complexity: medium
    - Critical: no
    - Depends on: Step 1
    - User decision: no
    - Sandbox: worker-b

2c. **{Description}**
    - Skill: @{skill}
    - Files: {paths}
    - Complexity: low
    - Critical: no
    - Depends on: Step 1
    - User decision: no
    - Sandbox: worker-c

### Group 3 (Sequential - Integration)
Runs after Group 2 completes:

3. **{Description}**
   - Skill: @{skill}
   - Files: {paths}
   - Complexity: high
   - Critical: yes:api-breaking
   - Depends on: 2a, 2b, 2c
   - User decision: yes - {what choice needed}

## Parallel Execution Map
```
[Step 1] ──┬──> [Step 2a*] ──┐
           ├──> [Step 2b]   ──┼──> [Step 3*] ──> [Final Review*]
           └──> [Step 2c]  ──┘
           
* = Opus (high complexity or critical)
```

## Final Review Triggers
Steps flagged for Opus final review:
- 2a: security
- 3: api-breaking

## Success Criteria
- {what defines done}
- {what defines done}
```

---

## PHASE 2: SPAWN PARALLEL WORKERS

**Orchestrator creates sandboxed workers for parallel groups.**

### Spawning Pattern

```bash
#!/bin/bash
# orchestrate.sh - Run from main agent

WORKDIR="$1"
PLAN="$WORKDIR/plan.md"

# Model selection based on complexity
select_model() {
    local complexity=$1
    case "$complexity" in
        high)   echo "claude-opus-4-5-20251101" ;;
        medium) echo "claude-sonnet-4-5-20250929" ;;
        low)    echo "claude-sonnet-4-5-20250929" ;;
        *)      echo "claude-sonnet-4-5-20250929" ;;
    esac
}

# Timeout based on model
select_timeout() {
    local model=$1
    if [[ "$model" == *"opus"* ]]; then
        echo "600"  # 10 min for Opus
    else
        echo "300"  # 5 min for Sonnet
    fi
}

spawn_worker() {
    local step_id=$1
    local step_desc=$2
    local skill=$3
    local files=$4
    local sandbox_name=$5
    local complexity=${6:-medium}
    local critical=${7:-no}
    
    # Select model based on complexity
    local MODEL=$(select_model "$complexity")
    local TIMEOUT=$(select_timeout "$MODEL")
    
    # Create isolated workspace
    SANDBOX_DIR="/tmp/3agents-sandbox-${sandbox_name}"
    mkdir -p "$SANDBOX_DIR"
    
    # Copy relevant source files
    cp -r $files "$SANDBOX_DIR/" 2>/dev/null || true
    
    # Log model selection
    echo "[Worker ${sandbox_name}] Using $MODEL (complexity: $complexity, critical: $critical)"
    
    # Spawn worker with appropriate model
    timeout "$TIMEOUT" claude --model "$MODEL" \
           --print \
           --output-format json \
           --allowedTools "Edit,Write,Bash" \
           "Execute step ${step_id}: ${step_desc}
            
            Skill: @${skill}
            Files: ${files}
            Working directory: ${SANDBOX_DIR}
            
            Instructions:
            1. Implement the step completely
            2. Run compilation check: go build -o /tmp/test ./...
            3. Run relevant tests if they exist
            4. Output JSON with: {status, files_changed, errors, output}
            
            Do not ask questions. Make reasonable technical decisions.
            " > "${WORKDIR}/step-${step_id}-result.json" 2>&1 &
    
    echo $!  # Return PID for tracking
}

# Example: Spawn parallel group with complexity-based model selection
PIDS=()

# High complexity = Opus, others = Sonnet
PIDS+=($(spawn_worker "2a" "Implement auth handler" "go-coder" "internal/handlers/" "worker-a" "high" "security"))
PIDS+=($(spawn_worker "2b" "Add tests" "test-writer" "test/" "worker-b" "medium" "no"))
PIDS+=($(spawn_worker "2c" "Update types" "go-coder" "internal/types/" "worker-c" "low" "no"))

# Wait for all parallel workers
for pid in "${PIDS[@]}"; do
    wait $pid
done

echo "Parallel group complete"
```

### Worker Agent Prompt Template

```markdown
You are WORKER agent in sandbox: {sandbox_name}

## Task
{step_description}

## Constraints
- Skill: @{skill}
- Files to modify: {files}
- Sandbox directory: {sandbox_dir}
- Timeout: 5 minutes

## Instructions
1. Read existing code to understand patterns
2. Implement the required changes
3. Compile to verify: `go build -o /tmp/test ./...`
4. Run tests if applicable: `go test ./...`
5. Document what you changed

## Output Format
Respond with ONLY this JSON:
```json
{
  "status": "success" | "partial" | "failed",
  "files_changed": ["path1", "path2"],
  "compilation": "pass" | "fail",
  "tests": "pass" | "fail" | "skipped",
  "changes_summary": "Brief description of changes",
  "errors": ["error1", "error2"] | null,
  "needs_retry": true | false
}
```

Do NOT ask questions. Make reasonable technical decisions.
Do NOT stop for confirmation. Complete the task.
```

---

## PHASE 3: INCREMENTAL VALIDATION

**Validator runs continuously, checking completed work as it arrives.**

### Validator Loop

```bash
#!/bin/bash
# validate.sh - Runs alongside workers

WORKDIR="$1"
VALIDATED=()

validate_step() {
    local result_file=$1
    local step_id=$(echo $result_file | grep -oP 'step-\K[^-]+')
    
    claude --model claude-sonnet-4-5-20250929 \
           --print \
           "You are VALIDATOR agent.
            
            Review this implementation result:
            $(cat $result_file)
            
            Check:
            1. Did it compile? 
            2. Did tests pass?
            3. Are the changes correct for the stated goal?
            4. Code quality score (1-10)?
            
            Output JSON:
            {
              \"step\": \"${step_id}\",
              \"valid\": true|false,
              \"quality\": N,
              \"issues\": [...],
              \"verdict\": \"PASS\" | \"NEEDS_RETRY\" | \"DONE_WITH_ISSUES\"
            }
           " > "${WORKDIR}/validation-${step_id}.json"
}

# Watch for completed work
while true; do
    for result in "$WORKDIR"/step-*-result.json; do
        [ -f "$result" ] || continue
        step_id=$(basename "$result" -result.json)
        
        # Skip if already validated
        [[ " ${VALIDATED[@]} " =~ " ${step_id} " ]] && continue
        
        echo "Validating $step_id..."
        validate_step "$result"
        VALIDATED+=("$step_id")
    done
    
    # Check if all steps validated
    # Exit condition...
    
    sleep 2
done
```

---

## PHASE 4: MERGE & INTEGRATE

**After parallel work completes, merge changes back.**

### Merge Strategy

```bash
#!/bin/bash
# merge.sh - Combine sandbox work

WORKDIR="$1"
MAIN_BRANCH=$(pwd)

for sandbox in /tmp/3agents-sandbox-*; do
    [ -d "$sandbox" ] || continue
    
    worker_name=$(basename "$sandbox" | sed 's/3agents-sandbox-//')
    
    echo "Merging changes from $worker_name..."
    
    # Get list of changed files from result JSON
    changed_files=$(jq -r '.files_changed[]' "${WORKDIR}/step-*-${worker_name}*.json" 2>/dev/null)
    
    for file in $changed_files; do
        if [ -f "${sandbox}/${file}" ]; then
            # Copy back to main workspace
            cp "${sandbox}/${file}" "${MAIN_BRANCH}/${file}"
            echo "  Merged: $file"
        fi
    done
done

# Final compilation check
echo "Running final compilation..."
go build -o /tmp/final-test ./...

# Run full test suite
echo "Running full test suite..."
cd /test/api && go test -v ./...
```

---

## PHASE 5: FINAL REVIEW (Opus 4.5)

**Purpose:** Deep review of critical paths before completion.

**Triggers automatically when plan contains:**
- `Critical: yes:security`
- `Critical: yes:authentication`
- `Critical: yes:authorization`
- `Critical: yes:payments`
- `Critical: yes:data-migration`
- `Critical: yes:crypto`
- `Critical: yes:api-breaking`
- `Critical: yes:database-schema`

### Final Review Script

```bash
#!/bin/bash
# final-review.sh - Opus deep review of critical changes

WORKDIR="$1"

# Collect all critical steps from plan
CRITICAL_STEPS=$(grep -E "Critical: yes:" "$WORKDIR/plan.md" | sed 's/.*Critical: yes:\([a-z-]*\).*/\1/')

if [ -z "$CRITICAL_STEPS" ]; then
    echo "No critical steps flagged - skipping final review"
    exit 0
fi

echo "=== PHASE 5: FINAL REVIEW (Opus 4.5) ==="
echo "Critical triggers found: $CRITICAL_STEPS"

# Gather all changes for review
CHANGES=""
for step_file in "$WORKDIR"/step-*.md; do
    CHANGES+="$(cat "$step_file")\n\n---\n\n"
done

# Gather current state of modified files
MODIFIED_FILES=$(grep -h "files_changed" "$WORKDIR"/*.json 2>/dev/null | jq -r '.[]' | sort -u)

FILE_CONTENTS=""
for f in $MODIFIED_FILES; do
    if [ -f "$f" ]; then
        FILE_CONTENTS+="### $f\n\`\`\`go\n$(cat "$f")\n\`\`\`\n\n"
    fi
done

# Opus final review
claude --model claude-opus-4-5-20251101 \
       --verbose \
       "You are the FINAL REVIEWER - a senior engineer doing a thorough security and architecture review.

## Critical Triggers
These aspects require deep review: $CRITICAL_STEPS

## Changes Made
$CHANGES

## Current File State
$FILE_CONTENTS

## Your Review Must Cover

### 1. Security Review (if security/auth/crypto triggered)
- Authentication bypass vulnerabilities
- Authorization logic flaws
- Input validation gaps
- Injection vulnerabilities (SQL, command, etc.)
- Sensitive data exposure
- Cryptographic weaknesses

### 2. Architecture Review (if api-breaking/database-schema triggered)
- Breaking change impact assessment
- Migration path for existing clients
- Backward compatibility concerns
- Database migration safety
- Data integrity risks

### 3. Correctness Review
- Logic errors
- Edge cases not handled
- Race conditions
- Resource leaks
- Error handling gaps

## Output Format

Create: $WORKDIR/final-review.md

\`\`\`markdown
# Final Review: {task}

## Review Scope
- Critical triggers: {list}
- Files reviewed: {count}
- Steps reviewed: {count}

## Security Findings

### Critical Issues
{List any critical security issues that MUST be fixed}

### Warnings  
{List security concerns that should be addressed}

### Passed Checks
{List security aspects that look good}

## Architecture Findings

### Breaking Changes
{Impact assessment of any breaking changes}

### Migration Notes
{Required migration steps if any}

## Code Quality

### Issues Found
1. {issue + file + line if possible}
2. {issue}

### Recommendations
1. {recommendation}
2. {recommendation}

## Verdict

**Status:** ✅ APPROVED | ⚠️ APPROVED_WITH_NOTES | ❌ CHANGES_REQUIRED

**Blocking Issues:** {count}
**Warnings:** {count}

{If CHANGES_REQUIRED, list specific fixes needed}

## Sign-off
Reviewed by: Opus 4.5 Final Reviewer
Timestamp: {ISO8601}
\`\`\`
" > "$WORKDIR/final-review.md"

# Check verdict
VERDICT=$(grep -oP 'Status:\*\* \K[^\n]+' "$WORKDIR/final-review.md" | head -1)

if [[ "$VERDICT" == *"CHANGES_REQUIRED"* ]]; then
    echo "❌ FINAL REVIEW FAILED - Changes required"
    echo "See: $WORKDIR/final-review.md"
    exit 1
fi

echo "✅ Final review complete: $VERDICT"
```

### Final Review Output: `final-review.md`

```markdown
# Final Review: {task}

## Review Scope
- Critical triggers: security, api-breaking
- Files reviewed: 8
- Steps reviewed: 4

## Security Findings

### Critical Issues
None found.

### Warnings
1. **Rate limiting not implemented** - `internal/handlers/auth.go:45`
   - Login endpoint should have rate limiting to prevent brute force
   - Severity: Medium
   - Recommendation: Add rate limiter middleware

### Passed Checks
✅ Password hashing uses bcrypt with cost 12
✅ JWT tokens have appropriate expiry (15 min access, 7 day refresh)
✅ No secrets in code or config
✅ Input validation present on all endpoints
✅ SQL queries use parameterized statements

## Architecture Findings

### Breaking Changes
- API endpoint `/api/v1/login` response shape changed
- Added `refresh_token` field (additive, non-breaking)
- Removed `session_id` field (BREAKING)

### Migration Notes
1. Clients must update to handle new response format
2. Provide deprecation warning in v1, remove in v2
3. Consider versioned endpoint `/api/v2/login`

## Code Quality

### Issues Found
1. Missing error wrap context - `internal/handlers/auth.go:78`
2. TODO comment should be tracked - `internal/auth/jwt.go:23`

### Recommendations
1. Add structured logging for auth events
2. Consider extracting token logic to separate package

## Verdict

**Status:** ⚠️ APPROVED_WITH_NOTES

**Blocking Issues:** 0
**Warnings:** 3

The implementation is sound. Address warnings before production deployment.

## Sign-off
Reviewed by: Opus 4.5 Final Reviewer
Timestamp: 2025-05-25T14:32:00Z
```

---

## STEP FILE FORMAT (Updated)

`step-{N}.md`:

```markdown
# Step {N}: {Description}

**Skill:** @{skill}
**Files:** {paths}
**Sandbox:** {sandbox_name} | main
**Parallel Group:** {group_number}

---

## Worker Execution

**Started:** {ISO8601}
**Sandbox:** /tmp/3agents-sandbox-{name}

### Implementation
{Auto-generated from worker JSON output}

**Files Changed:**
- `{file}`: {from changes_summary}

**Compilation:** ✅ Pass | ❌ Fail
**Tests:** ✅ Pass | ⚠️ Fail | ⚙️ Skipped

**Worker Status:** {status from JSON}

---

## Validation

**Validator Result:**
- Quality Score: {N}/10
- Issues Found: {count}
- Verdict: PASS | NEEDS_RETRY | DONE_WITH_ISSUES

**Issues:**
1. {issue from validation JSON}

---

## Retry (if needed)

### Retry Attempt 1
**Changes:** {what was fixed}
**Result:** {new status}

---

## Final Status

**Result:** ✅ COMPLETE | ⚠️ COMPLETE_WITH_ISSUES
**Quality:** {N}/10
**Duration:** {seconds}s

→ {Next step or "Waiting for parallel siblings"}
```

---

## PROGRESS FILE (Real-time)

`progress.md` - Updated by orchestrator continuously:

```markdown
# Progress: {task}

**Started:** {ISO8601}
**Working Directory:** {workdir}

## Execution Status

### Group 1 (Sequential)
| Step | Description | Status | Quality | Duration |
|------|-------------|--------|---------|----------|
| 1 | {desc} | ✅ Complete | 9/10 | 45s |

### Group 2 (Parallel) - IN PROGRESS
| Step | Worker | Status | Quality | Duration |
|------|--------|--------|---------|----------|
| 2a | worker-a | ✅ Complete | 8/10 | 62s |
| 2b | worker-b | 🔄 Running | - | 34s... |
| 2c | worker-c | ✅ Complete | 7/10 | 58s |

### Group 3 (Sequential)
| Step | Description | Status | Quality | Duration |
|------|-------------|--------|---------|----------|
| 3 | {desc} | ⏳ Waiting | - | - |

## Live Stats
- **Completed:** 3/5 steps
- **In Progress:** 1 step (worker-b)
- **Waiting:** 1 step
- **Average Quality:** 8.0/10
- **Elapsed Time:** 2m 15s

## Worker Status
```
worker-a: ✅ Idle (completed step 2a)
worker-b: 🔄 Active (step 2b - 34s)
worker-c: ✅ Idle (completed step 2c)
```

**Last Updated:** {ISO8601}
```

---

## ORCHESTRATOR MAIN LOOP

```bash
#!/bin/bash
# 3agents.sh - Main entry point

set -e

ARGUMENTS="$*"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Determine working directory
if [[ -f "$ARGUMENTS" ]]; then
    DIR=$(dirname "$ARGUMENTS")
    BASE=$(basename "$ARGUMENTS" .md | cut -c1-20)
    WORKDIR="${DIR}/${TIMESTAMP}-${BASE}"
else
    SLUG=$(echo "$ARGUMENTS" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | cut -c1-30)
    WORKDIR="docs/features/${TIMESTAMP}-${SLUG}"
fi

mkdir -p "$WORKDIR"
echo "Working directory: $WORKDIR"

# PHASE 1: Planning (Opus 4.5 - interactive, extended thinking)
echo "=== PHASE 1: PLANNING (Opus 4.5) ==="
claude --model claude-opus-4-5-20251101 \
       --verbose \
       "You are the PLANNER agent. 

THINK DEEPLY before responding. Consider:
- Full scope of: $ARGUMENTS
- Dependencies between tasks
- What can run in parallel
- Critical path

Create a detailed plan.md with parallel execution groups.
Working directory: $WORKDIR

$(cat << 'PLAN_TEMPLATE'
# Plan format required:
## Dependency Analysis
## Execution Groups (with parallel grouping)  
## Success Criteria
PLAN_TEMPLATE
)
"

# Wait for plan.md
while [[ ! -f "$WORKDIR/plan.md" ]]; do
    sleep 1
done

# PHASE 2: Parse plan and spawn workers
echo "=== PHASE 2: SPAWNING WORKERS ==="

# Start validator in background
./validate.sh "$WORKDIR" &
VALIDATOR_PID=$!

# Parse groups from plan and execute
# (This would be more sophisticated in practice)

# Sequential Group 1
for step in $(parse_sequential_steps "$WORKDIR/plan.md" 1); do
    execute_step "$step" "$WORKDIR"
    wait_for_validation "$step" "$WORKDIR"
done

# Parallel Group 2
PIDS=()
for step in $(parse_parallel_steps "$WORKDIR/plan.md" 2); do
    spawn_worker "$step" "$WORKDIR" &
    PIDS+=($!)
done
wait "${PIDS[@]}"

# Sequential Group 3 (integration)
for step in $(parse_sequential_steps "$WORKDIR/plan.md" 3); do
    # Check for user decisions
    if step_needs_decision "$step"; then
        create_decision_file "$step" "$WORKDIR"
        echo "USER DECISION REQUIRED - see $WORKDIR/decision-${step}.md"
        exit 0
    fi
    execute_step "$step" "$WORKDIR"
done

# PHASE 3: Merge
echo "=== PHASE 3: MERGING ==="
./merge.sh "$WORKDIR"

# PHASE 4: Final Review (Opus - if critical steps exist)
echo "=== PHASE 4: FINAL REVIEW ==="
./final-review.sh "$WORKDIR"
REVIEW_EXIT=$?

if [ $REVIEW_EXIT -ne 0 ]; then
    echo "❌ Final review requires changes - see $WORKDIR/final-review.md"
    exit 1
fi

# PHASE 5: Summary
echo "=== PHASE 5: SUMMARY ==="
kill $VALIDATOR_PID 2>/dev/null || true

create_summary "$WORKDIR"

echo "COMPLETE - see $WORKDIR/summary.md"
```

---

## CLAUDE CLI INVOCATION PATTERNS

### Model Selection Helper
```bash
# Select model based on step complexity
select_worker_model() {
    local complexity=$1
    case "$complexity" in
        high)   echo "claude-opus-4-5-20251101" ;;    # Complex = Opus
        *)      echo "claude-sonnet-4-5-20250929" ;;  # Default = Sonnet
    esac
}
```

### For Opus 4.5 Planner (extended thinking)
```bash
claude --model claude-opus-4-5-20251101 \
       --verbose \
       --system-prompt "You are a meticulous planner. Think deeply about dependencies, parallelization, and flag critical paths." \
       "$PROMPT"
```

### For Sonnet 4.5 Workers (parallel, non-interactive) - Default
```bash
claude --model claude-sonnet-4-5-20250929 \
       --print \
       --output-format json \
       --max-turns 1 \
       --allowedTools "Edit,Write,Bash" \
       "$PROMPT" > output.json &
```

### For Opus 4.5 Workers (high complexity steps)
```bash
# Used when plan marks step as Complexity: high
claude --model claude-opus-4-5-20251101 \
       --print \
       --output-format json \
       --max-turns 3 \
       --allowedTools "Edit,Write,Bash" \
       "$PROMPT" > output.json &
```

### For Sonnet 4.5 Validator (incremental, streaming)
```bash
claude --model claude-sonnet-4-5-20250929 \
       --print \
       --output-format json \
       "$VALIDATION_PROMPT"
```

### For Opus 4.5 Final Reviewer (critical paths)
```bash
# Triggered when plan contains Critical: yes:{trigger}
claude --model claude-opus-4-5-20251101 \
       --verbose \
       --system-prompt "You are a senior engineer doing security and architecture review." \
       "$REVIEW_PROMPT"
```

---

## USER DECISION HANDLING

**Only stop for genuine architectural decisions.**

`decision-step-{N}.md`:
```markdown
# Decision Required: Step {N}

## Context
{Why this decision matters - from Opus analysis}

## Options Analyzed

### Option 1: {Name}
**Trade-offs:**
- Pro: {benefit}
- Con: {drawback}
**Opus Assessment:** {planner's analysis}

### Option 2: {Name}
**Trade-offs:**
- Pro: {benefit}
- Con: {drawback}
**Opus Assessment:** {planner's analysis}

## Recommendation
**Suggested:** Option {N}
**Reasoning:** {from Opus thinking}

## Resume Command
```bash
# After deciding, resume with:
./3agents.sh --resume "$WORKDIR" --decision "option-1"
```
```

---

## SUMMARY FORMAT

`summary.md`:
```markdown
# Complete: {task}

## Execution Stats
| Metric | Value |
|--------|-------|
| Total Steps | {N} |
| Parallel Steps | {N} |
| Sequential Steps | {N} |
| Opus Steps | {N} (high complexity) |
| Sonnet Steps | {N} (standard) |
| Total Duration | {time} |
| Parallel Efficiency | {saved_time} |

## Model Usage
| Phase | Model | Duration | Purpose |
|-------|-------|----------|---------|
| Planning | Opus 4.5 | 45s | Dependency analysis |
| Step 1 | Sonnet 4.5 | 30s | Low complexity |
| Step 2a | Opus 4.5 | 90s | High complexity (security) |
| Step 2b | Sonnet 4.5 | 45s | Medium complexity |
| Step 2c | Sonnet 4.5 | 35s | Low complexity |
| Step 3 | Opus 4.5 | 75s | High complexity (breaking) |
| Validation | Sonnet 4.5 | 60s | Incremental checks |
| Final Review | Opus 4.5 | 120s | Security/architecture |

## Quality Summary
| Group | Steps | Avg Quality | Status |
|-------|-------|-------------|--------|
| 1 | 2 | 8.5/10 | ✅ |
| 2 (parallel) | 3 | 7.8/10 | ✅ |
| 3 | 1 | 9/10 | ✅ |

**Overall Quality:** {weighted_avg}/10

## Final Review Results
**Reviewer:** Opus 4.5
**Status:** ⚠️ APPROVED_WITH_NOTES
**Critical Triggers:** security, api-breaking

### Findings Summary
| Severity | Count | Status |
|----------|-------|--------|
| Critical | 0 | ✅ |
| Warning | 3 | ⚠️ Noted |
| Info | 2 | ℹ️ |

### Action Items (from final review)
1. [ ] Add rate limiting to login endpoint
2. [ ] Track TODO in jwt.go:23
3. [ ] Document breaking change in CHANGELOG

## Files Modified
```
{tree of changed files}
```

## Parallel Execution Map
```
[1: Setup] ─────────────────────────────────────┐
                                                │
[2a: Auth*] ──┐                                 │
[2b: Tests]   ├── 90s (parallel) ──────────────│── Total: 4m 30s
[2c: Types]  ──┘                                │
                                                │
[3: Integrate*] ────────────────────────────────│
                                                │
[Final Review*] ────────────────────────────────┘

* = Opus 4.5 (high complexity or critical review)
```

## Verification
```bash
# Final compilation
go build -o /tmp/final ./...  # ✅ Pass

# Test results
go test ./test/api/...        # ✅ 42 passed, 0 failed
go test ./test/ui/...         # ✅ 18 passed, 0 failed

# Final review
./final-review.sh             # ⚠️ APPROVED_WITH_NOTES
```

## Documentation
- `plan.md` - Original plan with complexity/critical flags
- `step-*.md` - Individual step details
- `progress.md` - Execution timeline
- `final-review.md` - Opus security/architecture review
- `validation-*.json` - Raw validation results

**Completed:** {ISO8601}
```

---

## STOP CONDITIONS

**ONLY stop for:**
- ✋ User decision marked in plan (`User decision: yes`)
- ✋ Unrecoverable merge conflict between parallel workers
- ✋ Ambiguous requirements (cannot determine what to build)
- ✋ Final review verdict: `CHANGES_REQUIRED` (critical security/architecture issues)

**NEVER stop for:**
- ❌ Asking to continue to next step
- ❌ Asking to spawn workers
- ❌ Final review verdict: `APPROVED_WITH_NOTES` (continue with warnings logged)
- ❌ Validation failures (retry, then document)
- ❌ Compilation errors after retry (document, continue)
- ❌ Test failures (document, continue)
- ❌ Permission to merge

**Parallel-specific rules:**
- Workers NEVER ask questions - make technical decisions
- Workers have 5-minute timeout - partial results accepted
- Merge conflicts resolved automatically (last-write-wins for non-overlapping changes)
- Overlapping file conflicts → flag for integration step

---

## ANTI-PATTERNS

**❌ DON'T:** Worker asking for clarification
```
I'm not sure if I should use a pointer or value receiver. 
Which would you prefer?
```

**✅ DO:** Worker making a decision
```json
{
  "status": "success",
  "changes_summary": "Used pointer receiver for Handler to match existing patterns in codebase",
  "files_changed": ["internal/handlers/auth.go"]
}
```

**❌ DON'T:** Sequential when parallel possible
```
Step 2a complete. Starting step 2b...
Step 2b complete. Starting step 2c...
```

**✅ DO:** Parallel execution
```
Spawning workers for parallel group 2...
  worker-a: step 2a (handlers)
  worker-b: step 2b (tests)  
  worker-c: step 2c (types)
All workers complete. Merging results...
```

---

## TASK INVOCATION

```bash
# From task description
./3agents.sh "Add user authentication with JWT"

# From existing plan file
./3agents.sh docs/fixes/01-plan-v1-auth.md

# Resume after decision
./3agents.sh --resume docs/features/20250525-auth/ --decision "option-2"
```

**Task:** $ARGUMENTS
**Mode:** Parallel execution with sandboxed workers
**Thinking:** Enabled for Opus planner