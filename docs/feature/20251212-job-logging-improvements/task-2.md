# Task 2: Rename placeholder and add log level filter dropdown for tree view
Workdir: ./docs/feature/20251212-job-logging-improvements/ | Depends: 1 | Critical: no
Model: opus | Skill: frontend

## Context
This task is part of: Job Logging Improvements - adding filtering capabilities to tree view step logs
Prior tasks completed: task-1 (fixed load more button)

## User Intent Addressed
"The step logging needs to have a selectable level, and filter box (not 'Search logs...' -> 'Filter logs...')"

## Input State
Files that exist before this task:
- `pages/queue.html` - Has search input with "Search logs..." placeholder (line ~588), no log level filter for tree view

## Output State
Files after this task completes:
- `pages/queue.html`:
  - Placeholder changed to "Filter logs..."
  - Log level dropdown added next to filter input
  - State added for tree log level filter
  - getFilteredTreeLogs updated to apply level filter

## Skill Patterns to Apply
### From frontend patterns:
- **DO:** Use Spectre CSS dropdown pattern (see existing Flat view filter dropdown lines 199-220)
- **DO:** Use Alpine.js reactive state with spread operator
- **DO:** Follow existing naming conventions (e.g., `treeLogLevelFilter`)
- **DON'T:** Create separate filter state per step - use per-job filter

## Implementation Steps
1. Change placeholder from "Search logs..." to "Filter logs..." at line ~588
2. Add `treeLogLevelFilter: {}` state to the Alpine.js data
3. Add getter function `getTreeLogLevelFilter(jobId)`
4. Add setter function `setTreeLogLevelFilter(jobId, level)`
5. Add dropdown next to the filter input in tree view header (after the input, before refresh button)
6. Update `getFilteredTreeLogs` function to apply level filtering

## Code Specifications
New state:
```javascript
treeLogLevelFilter: {},  // { jobId: 'all' | 'warn' | 'error' }
```

New functions:
```javascript
getTreeLogLevelFilter(jobId) {
    return this.treeLogLevelFilter[jobId] || 'all';
},
setTreeLogLevelFilter(jobId, level) {
    this.treeLogLevelFilter = { ...this.treeLogLevelFilter, [jobId]: level };
},
```

Dropdown HTML (Spectre CSS):
```html
<div class="dropdown dropdown-right" @click.stop>
    <a href="#" class="btn btn-sm" :class="getTreeLogLevelFilter(item.job.id) !== 'all' ? 'btn-primary' : ''" tabindex="0" style="padding: 0.2rem 0.5rem;">
        <i class="fas fa-filter" style="font-size: 0.8rem;"></i>
    </a>
    <ul class="menu" style="min-width: 120px;">
        <li class="menu-item">
            <a href="#" @click.prevent="setTreeLogLevelFilter(item.job.id, 'all')" :class="getTreeLogLevelFilter(item.job.id) === 'all' ? 'active' : ''">
                <i class="fas fa-list"></i> All
            </a>
        </li>
        <li class="menu-item">
            <a href="#" @click.prevent="setTreeLogLevelFilter(item.job.id, 'warn')" :class="getTreeLogLevelFilter(item.job.id) === 'warn' ? 'active' : ''">
                <i class="fas fa-exclamation-triangle" style="color: #E5C07B;"></i> Warn+
            </a>
        </li>
        <li class="menu-item">
            <a href="#" @click.prevent="setTreeLogLevelFilter(item.job.id, 'error')" :class="getTreeLogLevelFilter(item.job.id) === 'error' ? 'active' : ''">
                <i class="fas fa-times-circle" style="color: #E06C75;"></i> Error
            </a>
        </li>
    </ul>
</div>
```

## Accept Criteria
- [ ] Placeholder changed from "Search logs..." to "Filter logs..."
- [ ] Log level dropdown appears next to filter input
- [ ] Dropdown has All, Warn+, Error options
- [ ] Selecting a level filters logs in tree view
- [ ] Build passes

## Handoff
After completion, next task(s): task-3 (display log levels with colors)
