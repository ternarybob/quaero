# Global Development Standards

## SESSION START - WORKFLOW DETECTION (MANDATORY)

**On first response only, run:**
```powershell
dir .claude\commands\3agents-skills.md
dir .claude\skills\
```

**Then announce based on ACTUAL results:**

### If BOTH exist:
```
┌─────────────────────────────────────────────────┐
│ 🔧 3-AGENT WORKFLOW ACTIVE                      │
│ Skills: [list found]                            │
│ Direct coding disabled.                         │
└─────────────────────────────────────────────────┘
```

### If missing:
```
┌─────────────────────────────────────────────────┐
│ ⚡ STANDARD MODE - Direct coding enabled        │
└─────────────────────────────────────────────────┘
```

---

## CONTEXT CONTINUITY

### Follow-up Detection
If the next prompt relates to the previous request:
- **Continue in same workdir** - don't create new
- **Don't re-announce** workflow status
- **Don't re-read** skills/docs already in context

### Related Prompt Indicators:
- "also", "and", "what about", "can you also"
- "actually", "instead", "change that to"
- "the same", "that file", "this feature"
- "last update", "last refactor", "last fix", "last change"
- "the previous", "what we just", "continue with"
- Refers to files/tasks just discussed
- Continuation of same feature/fix

### New Request Indicators:
- Completely different topic
- "new feature", "different thing"
- Explicit: "start fresh", "new task"

---

## TOKEN EFFICIENCY

### Don't Re-read If Already In Context:
- Skills (go/SKILL.md, frontend/SKILL.md)
- Task files (task-N.md, step-N.md)
- Plan documents
- Source files already viewed

### Do Re-read If:
- File was modified since last read
- Explicitly asked to refresh
- New session started

### Compact Responses:
- Skip repeating file contents back
- Reference by name: "Updated `handler.go` per go/SKILL.md patterns"
- Summarize changes, don't echo full diffs

---

## WORKFLOW MODE

### Use /3agents-skills for:
- add, implement, create, build
- fix, resolve, correct, debug
- refactor, clean, reorganize, improve
- Any source file modification

### Before code change:
```
📋 3-agent workflow: .\docs\{type}\{date}-{slug}\
```

### Direct Response OK:
- Questions, explanations
- Read-only operations
- Git commands

---

## STANDARD MODE

- Direct coding permitted
- Normal behavior