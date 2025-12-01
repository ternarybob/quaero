# Task 4: Simplify UI to event/log display model
Depends: 3 | Critical: yes:architectural-change | Model: opus

## Context
Current UI has complex multi-level expansion (parent → steps → children). Need to simplify to flat event/log display that shows all activity under parent job.

## Current UI Structure
```
Parent Job Card
├── Step 1 Row [expand button]
│   └── Child Job 1, Child Job 2...
└── Step 2 Row [expand button]
    └── Child Job 3, Child Job 4...
```

## Target UI Structure
```
Parent Job Card [expand]
├── Summary Header
│   - "2 steps scheduled"
│   - Step list with types
├── Status badge (Pending/Running/Completed/Failed)
├── Progress bar (optional)
└── Events Panel (collapsible)
    └── Live log stream (newest at bottom, auto-scroll)
```

## Do
- Remove step row expansion logic from queue.html
- Remove child job rendering under steps
- Add events panel component with log display
- Connect to WebSocket for real-time updates
- Subscribe to job logs when panel expanded
- Unsubscribe when collapsed
- Display step summary at top of expanded card
- Show progress as "Step X of Y"
- Auto-scroll to newest logs
- Support filtering by level (info/warn/error)

## UI Components to Modify
- `pages/queue.html`: Main job list and card rendering
- Job card template: Simplify to summary + events
- WebSocket connection: Add subscription logic

## Accept
- [ ] Job cards show summary header with step list
- [ ] Expanding shows events panel with live logs
- [ ] Logs update in real-time via WebSocket
- [ ] No nested child job rows (flat structure)
- [ ] Build compiles without errors
