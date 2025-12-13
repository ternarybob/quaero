# Feature: Unified Logs API Endpoint
- Slug: unified-logs-api | Type: feature | Date: 2025-12-10
- Request: "But the point of having a single log api endpoint is to provide a filter for any requestor. ie. NOT /api/jobs/{stepJobId}/logs but /api/logs?jobid={jobsid}&step={stepJobId}&order=ASC&size=100"
- Prior: ./docs/fix/20251210-step-logs-inconsistent/

## User Intent
Create a single unified logs API endpoint (`/api/logs`) that supports query parameters for flexible filtering, replacing the job-specific `/api/jobs/{id}/logs` pattern. The new endpoint should:
1. Accept `jobid` parameter to filter logs by job
2. Accept `step` parameter to filter logs by step job ID
3. Accept `order` parameter (ASC/DESC) for sorting
4. Accept `size` parameter to limit results (default 100)

## Success Criteria
- [ ] New `/api/logs` endpoint exists and responds
- [ ] `jobid` query parameter filters logs to a specific job
- [ ] `step` query parameter filters logs to a specific step
- [ ] `order` query parameter controls sort direction (ASC/DESC)
- [ ] `size` query parameter limits result count (default 100)
- [ ] UI updated to use new unified endpoint
- [ ] Build succeeds

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | New API handler, route registration, storage queries |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | Update UI to use new endpoint |

**Active Skills:** go, frontend
