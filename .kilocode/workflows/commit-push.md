**INSTRUCTION:** Use the Task tool with the `general-purpose` subagent to handle commit and push operations.

**Task Agent Instructions:**
Create a commit message, commit the staged changes, and push to remote repository. Follow these strict rules:

**CRITICAL RULES:**
- NO AI attribution in commit message
- NO mention of Claude, AI, automated, or assistant
- NO co-author attribution to AI tools
- Write as human developer

**Process:**
1. Check current branch with `git branch --show-current`
2. Stage all modified files with `git add .` (if no staged changes exist)
3. **Go Library Check**: If working in a Go project (has go.mod file), run `gofmt -s -w .` to format all Go files before committing
4. Generate conventional commit message (type(scope): description)
5. Validate message contains no AI references
6. Execute `git commit -m "your message"`
7. Push to current branch with `git push`
8. Confirm both commit and push were successful

**Commit Message Format:**
- `feat(scope): add new functionality`
- `fix(scope): resolve specific issue`
- `docs(scope): update documentation`
- `refactor(scope): improve code structure`
- `test(scope): add or update tests`
- `chore(scope): maintenance tasks`

**Safety checks:**
- Check for existing staged changes, or stage all modified files if none exist
- Check current branch name
- For Go projects: Detect go.mod file presence and auto-format with gofmt
- Verify remote repository is accessible
- Handle any push conflicts gracefully

**Agent Task:**
Execute the complete commit-push workflow autonomously, including all git operations, message generation, and validation. Return a summary of the commit hash, message, and push status.

Context/Description: $ARGUMENTS