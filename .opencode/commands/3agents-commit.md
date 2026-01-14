---
name: 3agents-commit
description: Git commit and push with auto-formatting and convention enforcement.
allowed-tools:
  - Bash
  - Task
---

**INSTRUCTION:** Use the Task tool with `general-purpose` subagent for commit and push operations.

**CRITICAL RULES:**
- Write as human developer - NO AI attribution (Claude, AI, automated, assistant, co-author)
- Use conventional commit format: `type(scope): description`
- Auto-format Go projects before committing
- **AUTO-CONFIRM**: Proceed with all git operations without prompting user

**Workflow:**

1. **OS Detection (MANDATORY)**
   - Detect OS to determine shell and script paths:
     - Windows: PowerShell, `.\scripts\build.ps1`
     - Unix/Linux/macOS/WSL: Bash, `./scripts/build.sh`

2. **Pre-commit**
   - Set git user: `git config user.name "bobmcallan"`
   - Set credential: `git config --global credential.github.com.username bobmcallan`
   - Get current branch: `git branch --show-current`
   - **Normalize line endings**: Remove Windows CRLF → `git config core.autocrlf input`
   - **Go projects only**: Detect `go.mod` → run `gofmt -s -w .`
   - Stage changes: `git add .`

3. **Commit**
   - Generate conventional commit message
   - Validate no AI references
   - Execute: `git commit -m "your message"`

4. **Push**
   - Push to current branch: `git push`
   - **If prompted for confirmation**: Automatically select "Yes, and don't ask again" option
   - Confirm success or handle conflicts

**Commit Types:**
- `feat`: new functionality
- `fix`: bug resolution
- `docs`: documentation
- `refactor`: code improvement
- `test`: testing updates
- `chore`: maintenance

**Output:** Summary with commit hash, message, and push status.

**Context:** $ARGUMENTS
