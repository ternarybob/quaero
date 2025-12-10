# Global Development Standards

## SESSION START (ONCE)

**On FIRST response of session, detect workflow capability:**
```powershell
dir .claude\commands\3agents-skills.md
dir .claude\skills\
```

**Announce result:**
```
┌─────────────────────────────────────────────────┐
│ 🔧 3-AGENT WORKFLOW ACTIVE                      │
│ Skills: [list found]                            │
└─────────────────────────────────────────────────┘
```
or
```
┌─────────────────────────────────────────────────┐
│ ⚡ STANDARD MODE                                │
└─────────────────────────────────────────────────┘
```

---

## EVERY PROMPT - REQUEST ASSESSMENT

**On EVERY prompt, assess:**

### 1. Is workflow active? (from session start check)
### 2. Is this a code change request?

Code change keywords:
- add, implement, create, build
- fix, resolve, correct, debug  
- refactor, clean, reorganize, improve
- update (when code implied)
- Any source file modification

### 3. Is this a follow-up to previous work?

Follow-up indicators:
- "also", "and", "what about", "can you also"
- "actually", "instead", "change that to"
- "the same", "that file", "this feature"
- "last update", "last refactor", "last fix", "last change"
- "the previous", "what we just", "continue with"

---

## DECISION MATRIX

| Workflow Active? | Code Change? | Follow-up? | Action |
|------------------|--------------|------------|--------|
| ✅ | ✅ | ❌ | New workdir, run /3agents-skills |
| ✅ | ✅ | ✅ | Continue in SAME workdir |
| ✅ | ❌ | - | Direct response |
| ❌ | - | - | Direct response (standard mode) |

---

## WORKFLOW EXECUTION

**New code change (not follow-up):**
```
📋 3-agent workflow: .\docs\{type}\{date}-{slug}\
```
Then execute phases from `.claude\commands\3agents-skills.md`

**Follow-up to previous:**
```
📋 Continuing in: .\docs\{type}\{date}-{slug}\
```
Continue in existing workdir, don't re-read skills already in context.

**Direct response OK:**
- Questions, explanations
- Read-only operations
- Git commands

---

## TOKEN EFFICIENCY

### Don't Re-read If Already In Context:
- Skills, task files, plan documents, source files already viewed

### Do Re-read If:
- File was modified since last read
- Explicitly asked to refresh
- New session started

### Compact Responses:
- Reference by name: "Updated per go/SKILL.md"
- Summarize changes, don't echo full content