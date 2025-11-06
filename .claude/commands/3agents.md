---
name: 3agent
description: Three-agent workflow - plan, implement with validation gates
---

Execute a three-agent workflow for: $ARGUMENTS

## SETUP
1. Create a folder name from the task (lowercase, hyphens, no spaces)
2. Create directory: `docs\{folder-name}\`
3. All planning and tracking files go in this directory

## PROJECT RULES (CRITICAL - Follow strictly)
1. **NO binaries in root directory** - if testing build, use `go run` or build to temp location
2. **Building**: ONLY use `.\scripts\build.ps1` to build the service
3. **Testing**: 
   - API tests go in `.\test\api\`
   - UI tests go in `.\test\ui\`
   - Add tests in the appropriate directory and run from there
   - Run with: `go test -v` from the test directory

## AGENT 1 - PLANNER (use subagent)
Create a detailed plan:
- Break down into discrete implementation steps
- Write to `docs\{folder-name}\plan.json`: 
```json
  {"task": "$ARGUMENTS", "folder": "{folder-name}", "steps": [{"id": 1, "task": "...", "validation": "..."}]}
```

## AGENT 2 & 3 - IMPLEMENTATION LOOP

**CRITICAL RULE: Agent 2 CANNOT proceed to next step until Agent 3 validates current step**

For EACH step in `docs\{folder-name}\plan.json`:

### Agent 2 - Implementer (use subagent)
- Implement ONLY the current step
- **Testing builds**: Use `go run` or specify output path outside root
- **Building service**: Use `.\scripts\build.ps1` ONLY
- **Adding tests**: Create in `.\test\api\` or `.\test\ui\` as appropriate
- **Running tests**: `cd` to test directory, then `go test -v`
- Write to `docs\{folder-name}\progress.json`:
```json
  {"current_step": 1, "completed": [], "status": "implementing"}
```

### Agent 3 - Validator (use different subagent) 
- Review Agent 2's implementation immediately
- **Check compliance**:
  - [ ] No binaries created in root directory
  - [ ] Build only via `.\scripts\build.ps1` if needed
  - [ ] Tests added to correct directory (`.\test\api\` or `.\test\ui\`)
  - [ ] Tests pass when run from test directory
- Check code quality, completeness, correctness
- For Go: verify it compiles and follows conventions
- Write to `docs\{folder-name}\step-{id}-validation.json`:
```json
  {"step": 1, "valid": true/false, "reason": "...", "checks": {"no_root_binaries": true, "tests_pass": true}, "timestamp": "..."}
```

### Gate Check (YOU enforce this)
- Read `docs\{folder-name}\step-{id}-validation.json`
- If valid=false: Agent 2 MUST fix issues, Agent 3 MUST revalidate
- If valid=true: Update progress.json and proceed to next step
- **DO NOT allow Agent 2 to continue until validation passes**

## COMPLETION
When all steps validated:
- Write final summary to `docs\{folder-name}\summary.md`
- Update progress.json: `{"status": "completed", "completed": [1,2,3...]}`

All artifacts stored in: `docs\{folder-name}\`

Requirements: $ARGUMENTS