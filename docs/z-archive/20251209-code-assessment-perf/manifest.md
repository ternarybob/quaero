# Fix: Code Assessment Performance and Filter Issues

- Slug: code-assessment-perf | Type: fix | Date: 2025-12-09
- Request: "1. Events need buffering - limiting service speed. 2. classify_files step processing ALL files instead of just unknown ones."
- Prior: none

## User Intent
1. **Event Buffering**: Events sent to clients are synchronous and slowing down the service. Events should be buffered/queued to not block worker execution.

2. **Category Filter Not Working**: The `classify_files` step has `filter_category = ["unknown"]` but is processing ~906 files instead of just the files the rule_classifier couldn't classify. The `rule_classify_files` step should classify most files, leaving only a small number with `category=unknown` for the LLM classifier.

## Success Criteria
- [ ] Events are buffered/async so workers aren't blocked by event publishing
- [ ] classify_files step only processes documents where `rule_classifier.category == "unknown"`
- [ ] Build compiles successfully
- [ ] Codebase assessment pipeline runs correctly

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend event service and filter logic |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ❌ | No frontend changes needed |

**Active Skills:** go
