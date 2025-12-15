# Refactoring Skill

## Purpose
Core patterns for ALL code modifications in Quaero. Referenced by commands and other skills.

## ANTI-CREATION BIAS
```
┌─────────────────────────────────────────────────────────────────┐
│ Before creating ANYTHING, prove existing code can't be extended │
│                                                                  │
│ PRIORITY: EXTEND > MODIFY > CREATE                              │
│                                                                  │
│ CREATE requires written justification:                          │
│ • Why existing code cannot be extended                          │
│ • What pattern from existing codebase it follows                │
└─────────────────────────────────────────────────────────────────┘
```

## BUILD REQUIREMENT
```
OS Detection:
- C:\... or D:\... = Windows
- /home/... or /Users/... = Unix/Linux/macOS

Build Commands:
- Windows:     .\scripts\build.ps1
- Linux/macOS: ./scripts/build.sh

BUILD FAIL = TASK FAIL (no exceptions)
```

## ARCHITECTURE COMPLIANCE

Always read before modifying code:
- `docs/architecture/*.md` - Architecture requirements (LAW)
- `.claude/skills/go/SKILL.md` - Go patterns
- `.claude/skills/frontend/SKILL.md` - Frontend patterns

## CODE MODIFICATION RULES

### Before ANY Change:
1. Search codebase for existing similar code
2. Identify extension points (interfaces, services, patterns)
3. Challenge: Does this NEED new code?

### When Modifying:
1. Follow EXACT patterns from existing codebase
2. Minimum viable change (not "proper" or "complete")
3. Build must pass before completion

### Forbidden:
- Creating parallel structures
- Duplicating existing logic
- Ignoring existing patterns
- Modifying tests to make code pass