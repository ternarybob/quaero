# Quaero Development Standards

## ⛔ MANDATORY WORKFLOW ENFORCEMENT

**This project uses 3-agent workflow. Direct coding is PROHIBITED.**

---

## SESSION START (ONCE)

**On FIRST response, detect workflow:**
```powershell
dir .claude\commands\3agents-skills.md
dir .claude\skills\
```

**Announce:**
```
┌─────────────────────────────────────────────────┐
│ 🔧 3-AGENT WORKFLOW ACTIVE                      │
│ Skills: [list found]                            │
│ ⛔ Direct coding DISABLED                       │
└─────────────────────────────────────────────────┘
```

---

## ⛔ EVERY PROMPT - MANDATORY CHECK

**BEFORE responding to ANY prompt, you MUST assess:**

### 1. Is this a code change request?

Code change triggers:
- add, implement, create, build
- fix, resolve, correct, debug  
- refactor, clean, reorganize, improve
- update, change, modify (code implied)
- Any source file modification

### 2. If YES → STOP. Do NOT write code.

**You are NOT PERMITTED to:**
- Write new code directly
- Modify existing code directly
- Create new files directly
- Skip the workflow "just this once"

**You MUST:**
1. Announce: `📋 Code change detected. Starting 3-agent workflow...`
2. Execute `/3agents-skills` workflow from `.claude\commands\3agents-skills.md`
3. Create workdir, manifest, plan, tasks, steps, validation, summary

### 3. Is this a follow-up?

Follow-up indicators:
- "also", "and", "what about", "can you also"
- "actually", "instead", "change that to"
- "the same", "that file", "this feature"
- "last update", "last refactor", "last fix", "last change"
- "the previous", "what we just", "continue with"

If follow-up → continue in SAME workdir, don't restart workflow.

---

## DECISION MATRIX

| Code Change? | Follow-up? | Action |
|--------------|------------|--------|
| ✅ | ❌ | ⛔ STOP → New workdir → /3agents-skills |
| ✅ | ✅ | Continue SAME workdir |
| ❌ | - | Direct response OK |

---

## WORKFLOW EXECUTION

**New code change:**
```
📋 Code change detected. Starting 3-agent workflow...
   Workdir: .\docs\{type}\{date}-{slug}\
```
Then execute ALL phases from `.claude\commands\3agents-skills.md`

**Follow-up:**
```
📋 Continuing in: .\docs\{type}\{date}-{slug}\
```

**Direct response OK (no workflow needed):**
- Questions, explanations
- Read-only operations  
- Git commands
- Viewing files

---

## TOKEN EFFICIENCY

### Don't Re-read If Already In Context:
- Skills, task files, plan documents, source files

### Do Re-read If:
- File modified since last read
- Explicitly asked to refresh
- New session

### Compact Responses:
- Reference by name: "Updated per go/SKILL.md"
- Summarize, don't echo full content

---

## ⛔ VIOLATION CHECK

If you are about to write/modify code and have NOT:
- [ ] Created workdir in `.\docs\{type}\{date}-{slug}\`
- [ ] Written manifest.md with user intent
- [ ] Created plan.md and task files

**STOP. You are violating project rules. Start the workflow.**