# Task 3: Display log levels with colored prefixes in tree log lines
Workdir: ./docs/feature/20251212-job-logging-improvements/ | Depends: 2 | Critical: no
Model: opus | Skill: frontend

## Context
This task is part of: Job Logging Improvements - showing log levels visually in tree view
Prior tasks completed: task-1 (fixed load more), task-2 (added level filter)

## User Intent Addressed
"Show the log level in the event/logs, with 3 letter level eg. [INF], [DBG] and use the same colors as the Service Logs."

## Input State
Files that exist before this task:
- `pages/queue.html` - Tree log lines only show line number and message (lines ~680-685)
- Existing functions: `getLogLevelBadge(level)` returns [INF], [DBG], [WRN], [ERR]
- Existing functions: `getTerminalLevelClass(level)` returns terminal-info, terminal-warning, etc.
- CSS classes exist in `pages/static/quaero.css`:
  - `.terminal-info { color: #98C379; }`
  - `.terminal-warning { color: #E5C07B; }`
  - `.terminal-error { color: #E06C75; font-weight: 600; background-color: rgba(224, 108, 117, 0.1); }`
  - `.terminal-debug { color: #61AFEF; }`

## Output State
Files after this task completes:
- `pages/queue.html` - Tree log lines show [INF], [DBG], [WRN], [ERR] with matching colors

## Skill Patterns to Apply
### From frontend patterns:
- **DO:** Use existing helper functions (getLogLevelBadge, getTerminalLevelClass)
- **DO:** Use existing CSS classes (terminal-info, terminal-warning, etc.)
- **DO:** Keep log line layout clean and readable
- **DON'T:** Create new CSS - reuse existing terminal-* classes
- **DON'T:** Change the existing log level functions

## Implementation Steps
1. Find the tree log line template (around line 680)
2. Add a span between line number and log text for the log level badge
3. Apply :class binding using getTerminalLevelClass(log.level)
4. Apply x-text using getLogLevelBadge(log.level)

## Code Specifications
Current log line template (lines ~680-685):
```html
<div class="tree-log-line" :class="{ 'log-error': log.level === 'error' }">
    <span class="tree-log-num" x-text="getStepLogStartIndex(item.job.id, step.name, stepIndex) + logIdx + 1"></span>
    <span class="tree-log-text" x-text="log.text || log.message"></span>
</div>
```

Updated template:
```html
<div class="tree-log-line" :class="{ 'log-error': log.level === 'error' }">
    <span class="tree-log-num" x-text="getStepLogStartIndex(item.job.id, step.name, stepIndex) + logIdx + 1"></span>
    <span :class="getTerminalLevelClass(log.level)" x-text="getLogLevelBadge(log.level)" style="margin-right: 0.5rem; font-weight: 500;"></span>
    <span class="tree-log-text" x-text="log.text || log.message"></span>
</div>
```

## Accept Criteria
- [ ] Tree log lines show [INF], [DBG], [WRN], [ERR] prefixes
- [ ] Level prefix uses terminal-* CSS class matching Service Logs colors
- [ ] Info logs: #98C379 (sage green)
- [ ] Warning logs: #E5C07B (soft yellow)
- [ ] Error logs: #E06C75 (soft red)
- [ ] Debug logs: #61AFEF (sky blue)
- [ ] Build passes

## Handoff
After completion, next task(s): validation
