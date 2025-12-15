# Plan: Job Logging Improvements
Type: feature | Workdir: ./docs/feature/20251212-job-logging-improvements/ | Date: 2025-12-12

## Context
Project: Quaero
Related files:
- `pages/queue.html` - Main queue page with tree view and step logging

## User Intent (from manifest)
The user wants three improvements to the job/step logging UI in queue.html:
1. Fix "Show earlier logs" button to actually load 200 more logs when clicked
2. Change "Search logs..." placeholder to "Filter logs..." and add log level filter dropdown
3. Display log levels in log lines with 3-letter format and Service Logs colors

## Success Criteria (from manifest)
- [ ] "Show X earlier logs" button loads 200 more logs when clicked
- [ ] Search input placeholder changed from "Search logs..." to "Filter logs..."
- [ ] Log level dropdown added next to filter input (All, Warn+, Error options)
- [ ] Log lines in tree view show [INF], [DBG], [WRN], [ERR] prefixes
- [ ] Log level prefixes use terminal-* CSS classes for colors matching Service Logs
- [ ] Build passes
- [ ] Existing functionality preserved

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| frontend | Alpine.js reactive data, Spectre CSS components, existing terminal-* color classes |

## Technical Approach
All changes are in `pages/queue.html`:

1. **Fix "Show earlier logs"**: The `loadMoreStepLogs` function exists but the button uses the wrong increment (100). Change default step log limit from 50 to 200, and ensure the "show more" button increments by 200.

2. **Rename placeholder + add level dropdown**:
   - Change `placeholder="Search logs..."` to `placeholder="Filter logs..."` on line 588
   - Add a log level filter dropdown next to the filter input, similar to the Flat view's step Events filter dropdown (lines 199-220)
   - Add state for tree log level filter (`treeLogLevelFilter`)
   - Implement getter/setter functions for tree log level filter

3. **Display log levels with colors**:
   - Modify the tree log line template (line 680-685) to include a log level span
   - Use `getLogLevelBadge()` function (already exists, returns [INF], [DBG], etc.)
   - Use `getTerminalLevelClass()` function (already exists, returns terminal-info, etc.)
   - The CSS classes already exist in `pages/static/quaero.css`

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| pages/queue.html | modify | All three features - UI and JS logic |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Fix "Show earlier logs" button to load 200 logs | - | no | opus | frontend | 1 |
| 2 | Rename placeholder and add log level filter dropdown for tree view | 1 | no | opus | frontend | 1 |
| 3 | Display log levels with colored prefixes in tree log lines | 2 | no | opus | frontend | 1 |

## Execution Order
[1] -> [2] -> [3]

## Risks/Decisions
- The loadMoreStepLogs function already exists and makes API calls - we need to ensure it works correctly
- Tree view filter state needs to be separate from Flat view step event filter state
- getFilteredTreeLogs function needs to be updated to apply level filtering
