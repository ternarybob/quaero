# Feature: Job Queue UI Optimization

- Slug: codebase-assessment-optimization | Type: feature | Date: 2025-12-11
- Request: "Fix job/tree not showing all steps, running icon on completed jobs, clean services/frontend from context-specific code, expand tree view as events are received with 100-item limit, switch to light view, maintain div rather than scrollable text box"
- Prior: docs/feature/20251211-codebase-assessment-optimization (continuation - manifest already existed)

## User Intent

The user wants to fix multiple issues in the Job Queue UI:

1. **Steps Not Showing**: The tree view only shows 1 step when there should be 3 (as indicated by "Steps: 3" in the log)
2. **Running Icon Bug**: The step icon shows a spinner (running) even though the job status is "Completed"
3. **Clean Architecture**: Remove context-specific code from services and frontend - logs should be standard with key/value context, UI uses context to create views
4. **Live Tree Expansion**: Tree view should expand as events are received, with a 100-item limit on step events/logs, ordered earliest to latest, showing '...' at top when there are earlier logs
5. **Light Theme**: Switch to a light view - black text, light gray background for the tree view
6. **Div vs Scrollable**: Use div elements rather than scrollable text boxes where possible

## Success Criteria

- [ ] All steps (3) from a job definition are displayed in the tree view
- [ ] Step icon correctly reflects actual status (checkmark for completed, spinner for running)
- [ ] Services emit standard logs with key/value context (no context-specific formatting)
- [ ] Frontend uses log context to create appropriate views
- [ ] Tree view expands automatically as new events arrive
- [ ] Step logs limited to 100 items, ordered earliest-to-latest
- [ ] '...' indicator shown when earlier logs exist beyond the 100-item limit
- [ ] Tree view uses light theme (black text, light gray background)
- [ ] Log display uses div elements instead of scrollable text boxes

## Skills Assessment

| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Y | Y | Backend services emit logs, need to review log emission patterns |
| frontend | .claude/skills/frontend/SKILL.md | Y | Y | UI tree view, theming, Alpine.js reactive updates |

**Active Skills:** go, frontend
