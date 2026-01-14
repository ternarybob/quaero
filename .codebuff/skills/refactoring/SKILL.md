# Refactoring Skill

## Purpose
Core patterns for ALL code modifications. Referenced by commands and other skills.

## FUNDAMENTAL RULES

```
┌─────────────────────────────────────────────────────────────────┐
│ ANTI-CREATION BIAS                                              │
│                                                                  │
│ PRIORITY: EXTEND > MODIFY > CREATE                              │
│                                                                  │
│ Before creating ANYTHING, prove existing code can't be extended │
│ CREATE requires written justification:                          │
│ • Why existing code cannot be extended                          │
│ • What pattern from existing codebase it follows                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ BACKWARD COMPATIBILITY IS NOT REQUIRED                          │
│                                                                  │
│ • Requirements define the ONLY valid behavior                   │
│ • Legacy code behavior is IRRELEVANT if requirements differ     │
│ • Breaking changes are ALLOWED to satisfy requirements          │
│ • NEVER add compatibility shims for old behavior                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ CLEANUP IS MANDATORY                                            │
│                                                                  │
│ • Remove redundant functions when replacing with new impl       │
│ • Delete dead code paths made obsolete by changes               │
│ • Remove unused parameters, variables, and imports              │
│ • Consolidate duplicate logic - don't leave old version         │
│                                                                  │
│ Leave the codebase CLEANER than you found it.                   │
└─────────────────────────────────────────────────────────────────┘
```

## BUILD REQUIREMENT

```
BUILD FAIL = TASK FAIL (no exceptions)

OS Detection:
- C:\... or D:\... = Windows (PowerShell)
- /home/... or /Users/... = Unix/Linux/macOS (Bash)
- /mnt/c/... = WSL (Bash, but PowerShell for Go builds)

Build Commands:
- Windows:     .\scripts\build.ps1
- Linux/macOS: ./scripts/build.sh
- WSL:         powershell.exe -Command "cd C:\path; .\scripts\build.ps1"
```

## ARCHITECTURE COMPLIANCE

Read before modifying code:
- `docs/architecture/*.md` - Architecture requirements (LAW)
- `.codebuff/skills/go/SKILL.md` - Go patterns
- `.codebuff/skills/frontend/SKILL.md` - Frontend patterns

## CODE MODIFICATION RULES

### Before ANY Change
1. Search codebase for existing similar code
2. Identify extension points (interfaces, services, patterns)
3. Challenge: Does this NEED new code?

### When Modifying
1. Follow EXACT patterns from existing codebase
2. Minimum viable change (not "proper" or "complete")
3. Remove any code made redundant by changes
4. Build must pass before completion

### Forbidden
- Creating parallel structures
- Duplicating existing logic
- Ignoring existing patterns
- Modifying tests to make code pass
- Leaving dead code behind
- Adding backward compatibility shims

## Pattern Discovery

**Use these agents to find patterns:**

1. `file-picker` - Find similar files
   ```
   Prompt: "Find files similar to X that show pattern Y"
   ```

2. `code-searcher` - Find pattern usage
   ```
   Pattern: "func.*Handler.*http.ResponseWriter"
   Flags: "-g *.go"
   ```

3. `thinker` - Analyze patterns
   ```
   Prompt: "Analyze the pattern in file X and how to apply it to Y"
   ```

## Cleanup Checklist

After ANY modification:

- [ ] Old function removed if replaced
- [ ] Unused imports removed
- [ ] Unused variables removed
- [ ] Dead code paths removed
- [ ] Duplicate logic consolidated
- [ ] Comments updated or removed if obsolete

## Anti-Patterns (AUTO-FAIL)

```go
// ❌ Creating new file when existing can be extended
// New file: internal/services/new_helper.go
// Should extend: internal/services/existing_helper.go

// ❌ Duplicating logic
func newParseConfig() { /* same as existing parseConfig */ }

// ❌ Leaving old alongside new
func oldHelper() { }  // REMOVE THIS
func newHelper() { }  // Keep only this

// ❌ Adding backward compatibility
if legacyMode {
    return oldBehavior()
}
return newBehavior()

// ❌ Unused code
import "unused/package"  // Remove!
var unusedVar = "x"      // Remove!
```

## Success Criteria

1. Build passes
2. No new parallel structures created
3. All cleanup performed
4. Existing patterns followed
5. Codebase is cleaner than before
