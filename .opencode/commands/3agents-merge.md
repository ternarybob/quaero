---
name: 3agents-merge
description: Git commit, update main, merge, and push.
allowed-tools:
  - Bash
  - Task
---

**INSTRUCTION:** Use the Task tool with `general-purpose` subagent for commit, push, and merge operations.

**PREREQUISITE:** Follow `/3agents-commit` command workflow first, then proceed with merge.

**CRITICAL RULES:**
- Inherit all rules from `/3agents-commit` command
- **AUTO-CONFIRM**: Proceed with all git operations without prompting user
- Handle merge conflicts by reporting them clearly

**Workflow:**

1. **Check Current Branch**
   - `git branch --show-current`
   - **If `main`**: Run `/3agents-commit` only, skip merge workflow, exit

2. **Execute Commit**
   - Run full `/3agents-commit` workflow (stage, format, commit, push)

3. **Update Main**
   - `git checkout main`
   - `git pull origin main`

4. **Merge Current Branch into Main**
   - `git merge <current-branch> --no-edit`
   - If conflicts: report and stop
   - If clean: continue

5. **Push Main**
   - `git push origin main`

6. **Stay on Main**
   - Remain on main branch

**Output:** Summary with:
- Commit hash and message
- Branch merged: `<current-branch>` → `main`
- Push status

**Context:** $ARGUMENTS
