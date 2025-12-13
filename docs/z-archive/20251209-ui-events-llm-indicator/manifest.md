# Fix: UI Events LLM Indicator and Remove Step Queue Stats

- Slug: ui-events-llm-indicator | Type: fix | Date: 2025-12-09
- Request: "1. Workers making LLM calls need to indicate in events whether it's an LLM call or local. 2. Remove step queue assessment stats - inaccurate and shows wrong information."
- Prior: none

## User Intent
1. **LLM Call Indicator**: Events in the UI should clearly indicate whether a worker is making an LLM call (uses Gemini API) or a local/rule-based operation (no LLM). Currently events show "AI: category_classifier" but don't distinguish between actual LLM calls and rule-based agents.

2. **Remove Step Queue Stats**: The "918 pending, 9 running, 1078 completed, 2 failed" stats shown on each step card are inaccurate and misleading. These stats show global queue counts, not step-specific counts. User wants these removed entirely.

## Success Criteria
- [ ] Events show "LLM: agent_type" for agents making LLM calls
- [ ] Events show "Rule: agent_type" (or similar) for rule-based agents not using LLM
- [ ] Step cards no longer display the inaccurate queue stats ("X pending, Y running, Z completed, W failed")
- [ ] Build compiles successfully
- [ ] Existing functionality remains intact

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend worker changes, agent service |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | HTML template changes to remove stats |

**Active Skills:** go, frontend
