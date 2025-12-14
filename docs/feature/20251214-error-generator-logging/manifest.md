# Feature: Error Generator Logging Enhancements
Date: 2025-12-14
Request: "Error generator worker should capture failed jobs as ERR logs, add WRN level support, display log counts (shown/total), add log level filter, standardize refresh button"

## User Intent
Enhance the error_generator worker and queue UI to:
1. Capture failed jobs and log them as ERR entries
2. Support WRN (warning) level log entries
3. Show log counts (displayed/total) in the step header
4. Add log level filter dropdown matching the settings page filter style
5. Standardize the refresh logs button across the app

## Success Criteria
- [ ] error_generator worker creates ERR log entries for failed jobs
- [ ] error_generator worker creates WRN log entries
- [ ] Log API response includes total count and filtered count
- [ ] Step logs display shows "logs: X/Y" format
- [ ] Log filter dropdown shows ERR/WRN/INF/DBG checkboxes
- [ ] Selected filter levels filter API calls with level parameter
- [ ] API response log count matches displayed items
- [ ] Refresh logs button uses standard refresh icon (fa-sync-alt)
- [ ] Test assertions verify filter behavior
- [ ] No free text filter in logs (removed)

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_LOGGING.md | Log Entry Schema | Log levels: debug, info, warn, error |
| QUEUE_LOGGING.md | Log Retrieval API | GET /api/jobs/{id}/logs supports `level` query param |
| QUEUE_UI.md | Icon Standards | Status icons must match standard (fa-sync-alt for refresh) |
| QUEUE_UI.md | Log Display | Log line numbering starts at 1 |
| QUEUE_UI.md | API Calls | Minimize API calls < 10 per step |
| workers.md | Worker Interfaces | Workers use jobMgr.AddJobLog for logging |
