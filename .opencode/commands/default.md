---
name: default
description: Smart router - decides between TDD (for test files) and Feature (for requirements) workflows.
allowed-tools:
  - Read
  - Edit
  - Write
  - Glob
  - Grep
  - Bash
  - Task
  - TodoWrite
---

Execute: $ARGUMENTS

## ROUTING ANALYSIS

1. **Analyze Input:**
   - Input: "$ARGUMENTS"
   - Check if input matches pattern `*_test.go`
   - Check if input file exists: `ls "$ARGUMENTS"` (if it looks like a file path)

2. **Decision Matrix:**
   - **IF** input matches `*_test.go` pattern:
     → **EXECUTE TDD WORKFLOW** (`.opencode/commands/3agents-tdd.md`)
   - **ELSE**:
     → **EXECUTE FEATURE WORKFLOW** (`.opencode/commands/3agents.md`)

## EXECUTION

### TDD WORKFLOW
1. **Load Instructions:** Read `.opencode/commands/3agents-tdd.md`
2. **Execute:** Adopt the persona and follow the instructions in that file EXACTLY, using "$ARGUMENTS" as your input.
   - Perform the Setup phase.
   - Enforce TDD rules.
   - Do not deviate.

### FEATURE WORKFLOW
1. **Load Instructions:** Read `.opencode/commands/3agents.md`
2. **Execute:** Adopt the persona and follow the instructions in that file EXACTLY, using "$ARGUMENTS" as your input.
   - Perform the Setup phase.
   - Enforce Multi-Agent/Architect rules.
   - Do not deviate.
