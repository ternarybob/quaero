---
name: gofix
description: Automatically fix all Go build errors
allowed-tools:
  - Read
  - Edit
  - Write
  - Bash
  - Glob
  - Grep
---

## Task
Fix all Go compilation errors until `go build ./...` passes.

## Process

1. Run `go build ./... 2>&1` and capture errors
2. For each error:
   - Read the file and line referenced
   - Identify the root cause (missing method, type mismatch, undefined, etc.)
   - Fix the issue
3. Re-run build, repeat until clean
4. Run `go vet ./...` for additional issues
5. Report summary of fixes

## Interface Mismatch Fix Pattern
When you see "cannot use X as Y" for interfaces:
1. Find the interface definition with `grep -r "type Y interface" --include="*.go"`
2. Compare required methods vs implemented methods
3. Add missing methods or fix signatures

## Rules
- Max 10 iterations (fail if not fixed by then)
- Fix root causes, not symptoms
- Preserve existing code style
- No `fmt.Println` - use existing logger

## Output
Brief summary: files changed, errors fixed, iterations needed.