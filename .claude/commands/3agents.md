---
name: 3agent
description: Three-agent workflow - plan, implement with validation gates
---

Execute a three-agent workflow for: $ARGUMENTS

## CONFIGURATION
Define once, reference throughout:
```json
{
  "project": {
    "docs_root": "docs",
    "build_script": "./scripts/build.ps1",
    "test_directories": {
      "api": "./test/api/",
      "ui": "./test/ui/"
    }
  },
  "validation_rules": {
    "no_root_binaries": "Binaries must not be created in root directory",
    "use_build_script": "Building must only use designated build script",
    "tests_in_correct_dir": "Tests must be in appropriate test directory",
    "tests_must_pass": "All tests must pass before validation",
    "code_compiles": "Code must compile without errors",
    "follows_conventions": "Code must follow project conventions"
  },
  "file_templates": {
    "plan": "plan.json",
    "progress": "progress.json",
    "validation": "step-{id}-validation.json",
    "summary": "summary.md"
  }
}
```

## SETUP
1. Generate folder name: `lowercase-hyphenated-from-task`
2. Create directory: `{docs_root}/{folder-name}/`
3. Initialize progress tracking

## AGENT 1 - PLANNER (subagent)

**Output**: `{docs_root}/{folder-name}/plan.json`
```json
{
  "task": "$ARGUMENTS",
  "folder": "{folder-name}",
  "steps": [
    {
      "id": 1,
      "task": "Description",
      "validation_criteria": ["rule1", "rule2"],
      "artifacts": ["files created or modified"]
    }
  ]
}
```

## AGENT 2 - IMPLEMENTER (subagent)

**For current step only:**

1. Read: `{docs_root}/{folder-name}/plan.json` → Get current step
2. Implement step following:
   - Use `go run` for testing builds (specify temp output path)
   - Use `{build_script}` for final builds only
   - Create tests in `{test_directories.api}` or `{test_directories.ui}`
   - Run tests from test directory: `cd {test_dir} && go test -v`

3. Update: `{docs_root}/{folder-name}/progress.json`
```json
{
  "current_step": 1,
  "status": "awaiting_validation",
  "completed": [],
  "timestamp": "ISO8601"
}
```

**HALT**: Wait for Agent 3 validation before proceeding.

## AGENT 3 - VALIDATOR (different subagent)

**For current step only:**

1. Read step validation criteria from `plan.json`
2. Execute validation checklist (from `validation_rules` config):
   - Check each rule applicable to current step
   - Test compilation/execution
   - Verify artifacts created correctly

3. Output: `{docs_root}/{folder-name}/step-{id}-validation.json`
```json
{
  "step_id": 1,
  "valid": true,
  "failed_rules": [],
  "passed_rules": ["rule1", "rule2"],
  "notes": "Optional feedback",
  "timestamp": "ISO8601"
}
```

## GATE ENFORCEMENT (Orchestrator)
```
WHILE steps remain:
  1. Agent 2 implements current step
  2. Agent 3 validates current step
  3. IF validation fails:
       - Agent 2 fixes issues (using validation feedback)
       - Agent 3 re-validates
       - REPEAT until valid=true
  4. IF validation passes:
       - Update progress.json: move step to completed[]
       - INCREMENT current_step
       - CONTINUE to next iteration
```

**Critical**: No step advancement without `valid: true` in validation file.

## COMPLETION

When `completed.length == steps.length`:

1. Generate `{docs_root}/{folder-name}/summary.md`:
```markdown
# Task: {task_description}
## Steps Completed: {count}
## Total Validation Cycles: {count}
## Artifacts: 
- List all files created/modified
## Notes:
- Key decisions
- Challenges resolved
```

2. Final `progress.json`: 
```json
{
  "status": "completed",
  "completed": [all step ids],
  "total_validation_cycles": N,
  "timestamp": "ISO8601"
}
```

## VALIDATION RULES REFERENCE

Use validation_rules keys in step definitions. Rules are checked programmatically:

- `no_root_binaries`: Check root dir has no new executables
- `use_build_script`: Verify build script usage via logs/history
- `tests_in_correct_dir`: Verify test file paths match conventions
- `tests_must_pass`: Execute `go test -v` and check exit code
- `code_compiles`: Verify `go build` or `go run` succeeds
- `follows_conventions`: Check formatting, naming, structure

**Extensibility**: Add new rules to config without modifying workflow logic.

---

**Requirements**: $ARGUMENTS
**Working Directory**: `{docs_root}/{folder-name}/`
**All agents**: Reference configuration above, not hard-coded values