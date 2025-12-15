# Feature: Log Pagination and Display Improvements
Date: 2025-12-15
Request: "Create a document addressing: 1) 'Show N earlier logs' button not working, propose pagination for 1000+ logs; 2) Propose updates to queue.html for log display (min 100 logs); 3) Propose test updates to job_definition_general_test.go"

## User Intent
The user wants a design document that proposes solutions for:
1. The "Show X earlier logs" button/link on the queue page doesn't function properly - clicking should expand with 100 more logs
2. Log display improvements - jobs with 1000+ logs need pagination or other methods to view all logs
3. Initial log display should show at least 100 logs when job has > 100 logs
4. Tests to verify the proposed behavior

## Success Criteria
- [ ] Document proposes working solution for "Show earlier logs" button functionality
- [ ] Document proposes pagination or alternative for jobs with 1000+ logs
- [ ] Document proposes minimum 100 logs display for initial load
- [ ] Document proposes test assertions for job_definition_general_test.go
- [ ] Proposed solutions align with QUEUE_UI.md architecture requirements
- [ ] Proposed solutions align with QUEUE_LOGGING.md architecture requirements

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | Log Fetching Strategy | Initial fetch via REST API when step expanded |
| QUEUE_UI.md | Log Fetching Strategy | "Show earlier logs" button fetches with offset |
| QUEUE_UI.md | Minimize API Calls | Step log API calls should be < 10 per job execution |
| QUEUE_UI.md | API Endpoints | GET /api/jobs/{id}/logs with limit/offset params |
| QUEUE_LOGGING.md | Log Retrieval API | GET /api/jobs/{id}/logs supports limit, offset, level params |
| QUEUE_LOGGING.md | Log Retrieval API | GET /api/jobs/{id}/logs/aggregated supports cursor pagination |
| QUEUE_LOGGING.md | Log Line Numbering | Logs MUST start at line 1, increment sequentially |
| QUEUE_LOGGING.md | UI Log Display | Pagination via "Show earlier logs" button with offset |
