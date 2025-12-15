# Feature: Job Page Logs Pagination with Unified API
- Slug: job-logs-pagination | Type: feature | Date: 2025-12-10
- Request: "Review the Job page. The total logs needs to be paged, and limited to 1000 per page. Hence also uses the same log api end point."
- Prior: ./docs/feature/20251210-unified-logs-api/

## User Intent
Update the Job page (`/job/{id}`) to:
1. Use the unified `/api/logs` endpoint instead of `/api/jobs/{id}/logs` and `/api/jobs/{id}/logs/aggregated`
2. Implement pagination with 1000 logs per page
3. Add pagination controls (next/prev) for navigating large log sets

## Success Criteria
- [ ] Job page uses `/api/logs?scope=job&job_id={id}` endpoint
- [ ] Logs are limited to 1000 per page
- [ ] Pagination controls allow navigating between pages
- [ ] Build succeeds

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ❌ | No backend changes needed - API already supports pagination |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | Update UI to use unified endpoint with pagination |

**Active Skills:** frontend
