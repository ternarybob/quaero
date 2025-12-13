# Feature: Job Logging Improvements
- Slug: job-logging-improvements | Type: feature | Date: 2025-12-12
- Request: "1. The 'Show xxx earlier logs' button does not actually do anything. Implement a load mechanism to show 200 of the latest logs. 2. The step logging needs to have a selectable level, and filter box (not 'Search logs...' -> 'Filter logs...'). 3. Show the log level in the event/logs, with 3 letter level eg. [INF], [DBG] and use the same colors as the Service Logs."
- Prior: none

## User Intent
The user wants three improvements to the job/step logging UI in queue.html:

1. **Fix "Show earlier logs" button**: Currently the button displays "Show X earlier logs" but clicking it doesn't work correctly. The user wants it to load and display 200 logs at a time (the latest/most recent logs).

2. **Add log level filter and rename search box**:
   - Change placeholder from "Search logs..." to "Filter logs..."
   - Add a selectable log level dropdown (like the existing filter in Flat view's step Events dropdown)

3. **Display log levels with colors**: Show log levels in the log lines using 3-letter format [INF], [DBG], [WRN], [ERR] with the same color scheme as Service Logs panel:
   - INF: #98C379 (soft sage green)
   - WRN: #E5C07B (soft yellow)
   - ERR: #E06C75 (soft red)
   - DBG: #61AFEF (soft sky blue)

## Success Criteria
- [ ] "Show X earlier logs" button loads 200 more logs when clicked
- [ ] Search input placeholder changed from "Search logs..." to "Filter logs..."
- [ ] Log level dropdown added next to filter input (All, Warn+, Error options)
- [ ] Log lines in tree view show [INF], [DBG], [WRN], [ERR] prefixes
- [ ] Log level prefixes use terminal-* CSS classes for colors matching Service Logs
- [ ] Build passes
- [ ] Existing functionality preserved

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | yes | no | No backend Go changes needed - this is frontend-only |
| frontend | .claude/skills/frontend/SKILL.md | yes | yes | All changes are in queue.html (Alpine.js + Spectre CSS) |

**Active Skills:** frontend
