# Fix: Job Steps Collapse Toggle

- Slug: job-collapse | Type: fix | Date: 2025-12-10
- Request: "Update, when user clicks on the job (in queue) collapses/hides the steps"
- Prior: none

## User Intent

When the user clicks on a parent job card in the queue page (specifically the header/metadata area with timestamps like created, started, ended), the step rows that appear below that job should collapse (hide) or expand (show). Currently, clicking the job card navigates to job details - the user wants a way to toggle step visibility.

## Success Criteria

- [ ] Clicking on the job header/metadata area toggles visibility of step rows below
- [ ] Steps remain collapsed/expanded per user interaction (state preserved during session)
- [ ] The expand/collapse action does NOT trigger navigation to job details
- [ ] Visual indicator shows whether job's steps are expanded or collapsed

## Skills Assessment

| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ❌ | No backend changes needed - pure frontend change |
| frontend | .claude/skills/frontend/SKILL.md | ❌ | ❌ | Skill file doesn't exist (go/SKILL.md exists in frontend dir) |

**Active Skills:** none - proceeding without skills (frontend-only change using existing Alpine.js patterns)
