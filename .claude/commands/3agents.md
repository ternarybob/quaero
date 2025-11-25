---
name: 3agents
description: Three-agent parallel workflow. Opus plans, executes, validates.
---

Execute workflow for: $ARGUMENTS

## CONFIG

```yaml
model: claude-opus-4-5-20251101
timeout: 600
max_parallel: 3
working_dir: /tmp/3agents-work

skills:
  code-architect: [architecture, design, refactoring]
  go-coder: [implementation, handlers, functions]
  test-writer: [tests, coverage]
  none: [documentation, planning]

critical_triggers:
  - security
  - authentication
  - authorization
  - payments
  - data-migration
  - crypto
  - api-breaking
  - database-schema
```

## RULES

- **Tests:** Only `/test/api` and `/test/ui`
- **Binaries:** `go build -o /tmp/` or `go run` - never in root
- **Workers:** Never ask questions - make technical decisions
- **Complete:** Run all steps - only stop for design decisions

---

## PHASE 1: PLAN

```bash
claude --model claude-opus-4-5-20251101 --verbose "$PROMPT"
```

**Planner thinks about:**
1. All discrete tasks needed
2. Dependency graph
3. Parallel groupings
4. Critical path flags

### Output: `plan.md`

```markdown
# Plan: {task}

## Dependency Analysis
{what depends on what}

## Execution Groups

### Group 1 (Sequential)
1. **{Description}**
   - Skill: @{skill} | Files: {paths}
   - Critical: no | Depends: none

### Group 2 (Parallel)
2a. **{Description}**
    - Skill: @{skill} | Files: {paths}
    - Critical: yes:security | Depends: 1 | Sandbox: worker-a

2b. **{Description}**
    - Skill: @{skill} | Files: {paths}
    - Critical: no | Depends: 1 | Sandbox: worker-b

### Group 3 (Sequential)
3. **{Description}**
   - Skill: @{skill} | Files: {paths}
   - Critical: yes:api-breaking | Depends: 2a,2b
   - User decision: yes - {choice needed}

## Execution Map
[1] ──┬──> [2a] ──┬──> [3] ──> [Final Review]
      └──> [2b] ──┘

## Success Criteria
- {done condition}
```

---

## PHASE 2: EXECUTE

### Spawn Workers

```bash
spawn_worker() {
    local step_id=$1 step_desc=$2 skill=$3 files=$4 sandbox=$5
    
    SANDBOX_DIR="/tmp/3agents-sandbox-${sandbox}"
    mkdir -p "$SANDBOX_DIR"
    cp -r $files "$SANDBOX_DIR/" 2>/dev/null || true
    
    timeout 600 claude --model claude-opus-4-5-20251101 \
           --print --output-format json \
           --allowedTools "Edit,Write,Bash" \
           "Execute step ${step_id}: ${step_desc}
            Skill: @${skill} | Files: ${files} | Dir: ${SANDBOX_DIR}
            
            1. Implement completely
            2. Compile: go build -o /tmp/test ./...
            3. Test if applicable
            4. Output JSON result with embedded markdown summary
            " > "${WORKDIR}/step-${step_id}-result.json" 2>&1 &
    echo $!
}

# Parallel execution
PIDS=()
PIDS+=($(spawn_worker "2a" "Auth handler" "go-coder" "internal/handlers/" "worker-a"))
PIDS+=($(spawn_worker "2b" "Add tests" "test-writer" "test/" "worker-b"))
for pid in "${PIDS[@]}"; do wait $pid; done
```

### Worker Output Format

Workers output JSON with embedded markdown summary:

```json
{
  "status": "success|partial|failed",
  "files_changed": ["path1", "path2"],
  "compilation": "pass|fail",
  "tests": "pass|fail|skipped",
  "errors": null,
  "needs_retry": false,
  "summary": "## Step 2a: Auth Handler\n\n### Actions\n1. Created JWT middleware in auth.go\n2. Added login/logout handlers\n\n### Files\n- `internal/handlers/auth.go` - new handlers\n- `internal/middleware/jwt.go` - token validation\n\n### Decisions\n- Pointer receiver to match patterns\n- bcrypt cost 12\n\n### Verify\n✅ Compiled\n✅ Tests pass"
}
```

### Worker Summary Template

```markdown
## Step {N}: {Description}

### Actions
1. {what was done}
2. {what was done}

### Files
- `{path}` - {change}

### Decisions
- {choice}: {rationale}

### Verify
{✅|❌} Compiled | {✅|❌|⚙️} Tests
```

---

## PHASE 3: VALIDATE

```bash
validate_step() {
    claude --model claude-opus-4-5-20251101 --print \
           "Review: $(cat $result_file)
            
            Check: compile, tests, correctness, quality (1-10)
            
            JSON: {\"step\": \"N\", \"valid\": bool, \"quality\": N, \"verdict\": \"PASS|NEEDS_RETRY|DONE_WITH_ISSUES\"}" \
           > "${WORKDIR}/validation-${step_id}.json"
}
```

---

## PHASE 4: MERGE

```bash
for sandbox in /tmp/3agents-sandbox-*; do
    changed=$(jq -r '.files_changed[]' "${WORKDIR}"/step-*.json 2>/dev/null)
    for file in $changed; do
        [ -f "${sandbox}/${file}" ] && cp "${sandbox}/${file}" "./${file}"
    done
done

go build -o /tmp/final ./...
go test ./test/api/... ./test/ui/...
```

---

## PHASE 5: FINAL REVIEW

**Triggers:** When plan contains `Critical: yes:{trigger}`

```bash
claude --model claude-opus-4-5-20251101 --verbose \
       "FINAL REVIEW - Security/architecture review.
        Triggers: $CRITICAL_STEPS
        Changes: $CHANGES
        
        Output final-review.md"
```

### Output: `final-review.md`

```markdown
# Final Review: {task}

## Scope
Triggers: {list} | Files: {N}

## Security
**Critical:** {issues or "None"}
**Warnings:** {list}

## Architecture  
**Breaking:** {assessment}
**Migration:** {notes}

## Verdict
**Status:** ✅ APPROVED | ⚠️ APPROVED_WITH_NOTES | ❌ CHANGES_REQUIRED

## Actions
1. [ ] {item}
```

---

## OUTPUT FILES

| File | Content |
|------|---------|
| `plan.md` | Execution plan |
| `step-{N}-result.json` | Worker output + summary |
| `validation-{N}.json` | Validator results |
| `final-review.md` | Review verdict |
| `summary.md` | Final summary |

### `summary.md`

```markdown
# Complete: {task}

## Stats
Steps: {N} | Parallel: {N} | Duration: {time} | Quality: {avg}/10

## Worker Summaries
{extracted from each step JSON summary field}

## Review
**Status:** {verdict}
**Actions:** {list}

## Verify
go build ./...     # ✅
go test ./test/... # ✅ {N} passed

**Done:** {ISO8601}
```

---

## STOP CONDITIONS

**Stop:** User decision | Merge conflict | Ambiguous requirements | `CHANGES_REQUIRED`

**Continue:** Next step | Spawning | `APPROVED_WITH_NOTES` | Retryable failures

---

## INVOKE

```bash
./3agents.sh "Add JWT authentication"
./3agents.sh docs/fixes/01-plan.md
./3agents.sh --resume $WORKDIR --decision "option-1"
```