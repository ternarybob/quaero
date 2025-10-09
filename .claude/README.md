# Claude Code Configuration

This directory contains Claude Code configuration and hooks for the Quaero project.

## Files

### settings.json

Contains hook configuration for Claude Code. Hooks enforce coding standards and provide reminders.

**Configured hooks:**
- `user_prompt_submit` - Displays reminders for build/test commands
- `pre_write` / `pre_edit` - Validates Go code before modifications
- `post_write` / `post_edit` - Updates function index after modifications

### hooks/project-standards.js

Node.js script that enforces project standards:

**Validations:**
- Directory structure rules (common/ vs services/)
- Logging standards (arbor only, no fmt.Println)
- Error handling (no ignored errors)
- Duplicate function detection
- File/function size limits
- Forbidden patterns (TODO, FIXME)

**Build/Test Reminders:**
- Displays critical reminders when build/test keywords detected
- Prevents accidental use of `go build` or `go test`

## Function Index

The hook maintains `.claude/go-function-index.json` to track all Go functions and prevent duplicates.

**Rebuild index manually:**
```bash
node .claude/hooks/project-standards.js rebuild-index
```

## Hook Behavior

### User Prompt Submit

When you mention "build" or "test" in your message, displays:
```
═══════════════════════════════════════════════════════════
🚨 CRITICAL REMINDERS:

  TESTS: MUST use ./test/run-tests.ps1
         NEVER: go test, cd test && go test
         Usage: ./test/run-tests.ps1 -Type [all|unit|api|ui]

  BUILD: MUST use ./scripts/build.ps1
         NEVER: go build directly
         Usage: ./scripts/build.ps1 [-Clean] [-Release]

═══════════════════════════════════════════════════════════
```

### Pre-Write/Edit Validation

Before modifying Go files, checks for:

**Blockers (operation prevented):**
- Receiver methods in `internal/common/`
- Use of `fmt.Println` or `log.Printf` (must use arbor logger)
- Ignored errors (`_ = someFunc()`)
- Duplicate function names

**Warnings (operation allowed):**
- Files over 500 lines
- Functions over 80 lines
- TODO/FIXME comments

**Example blocked operation:**
```
═══════════════════════════════════════════════════════════
❌ WRITE OPERATION BLOCKED
═══════════════════════════════════════════════════════════
❌ BLOCKED: Receiver methods not allowed in internal/common/
   Move to internal/services/ for stateful services
❌ BLOCKED: Use arbor logger instead of /fmt\.Println/
═══════════════════════════════════════════════════════════
```

### Post-Write/Edit Indexing

After successful Go file modifications:
- Rebuilds function index
- Indexes all exported functions
- Enables duplicate detection for future operations

## Disabling Hooks

To temporarily disable hooks, rename or delete `.claude/settings.json`.

**Note:** Hooks enforce critical project standards. Disabling them may lead to code quality issues.

## Troubleshooting

### Hook Not Running

1. Check Node.js is installed: `node --version`
2. Verify settings.json syntax is valid JSON
3. Check hook script is executable
4. Review Claude Code logs for hook errors

### False Positives

If the hook blocks a valid operation:
1. Review the specific error message
2. Check if code violates project standards
3. If standards need adjustment, update hook configuration

### Index Issues

If duplicate detection is inaccurate:
```bash
# Rebuild the function index
node .claude/hooks/project-standards.js rebuild-index
```

## Configuration Reference

### Max File Lines
Default: 500 lines

Warns when files exceed this limit. Encourages splitting large files into smaller modules.

### Max Function Lines
Default: 80 lines (ideal: 20-40)

Warns when functions exceed this limit. Encourages single responsibility principle.

### Forbidden Patterns
- `TODO` - Incomplete work markers
- `FIXME` - Known issues markers

These should be tracked in issue tracker, not code comments.

### Required Libraries

**Logging:** `github.com/ternarybob/arbor` (ONLY)
- Blocks: `fmt.Println`, `fmt.Printf`, `log.Println`, `log.Printf`

**Banner:** `github.com/ternarybob/banner`

**Config:** `github.com/pelletier/go-toml/v2`

## See Also

- [CLAUDE.md](../CLAUDE.md) - Complete development guide
- [README.md](../README.md) - Project overview
