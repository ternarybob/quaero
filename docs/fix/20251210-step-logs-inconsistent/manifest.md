# Fix: Step Logs Inconsistent After Logging Refactor
- Slug: step-logs-inconsistent | Type: fix | Date: 2025-12-10
- Request: "After the last logging refactor, the steps logging is not consistent. Shows logs in the console, however 0 in the step."
- Prior: none

## User Intent
Fix the inconsistency where job step logs appear in the console (terminal) but show "Events (0)" and "No events yet for this step" in the UI. The logs are being generated but not propagated to the step events display in the queue page.

## Success Criteria
- [ ] Step events show correctly in UI when logs are generated
- [ ] Console logs match UI events count
- [ ] Events aggregate properly per step

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend event service, WebSocket handlers |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | Queue page UI, event display (same as go skill) |

**Active Skills:** go (for backend event flow investigation and fix)
