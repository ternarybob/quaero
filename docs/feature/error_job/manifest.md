# Feature: Error Generator Worker

Date: 2025-12-13
Request: "Create a new worker, error_generator_worker.go, which generates log items, with delays, and then randomly generates warnings and error logs. The worker recursively creates new workers, some of which fail. Create tests asserting error tolerance, UI status display (INF 1000, WRN 100, ERR 50), and error block display above logs."

## User Intent

Create an error generator worker that:
1. Generates log items with delays
2. Randomly generates warnings and error logs
3. Recursively creates child workers (some fail)
4. Tests error tolerance configuration to stop jobs when failure threshold is exceeded
5. Tests UI displays log counts (INF/WRN/ERR) in step card headers
6. Tests errors are maintained as a separate block above ongoing logs

## Success Criteria

- [x] error_generator_worker.go implements both DefinitionWorker and JobWorker interfaces
- [x] Worker generates logs with configurable delays
- [x] Worker randomly generates INF, WRN, and ERR level logs
- [x] Worker recursively creates child workers with configurable failure rates
- [x] error_generator.toml job definition with error_tolerance config
- [x] Test asserts job stops when max_child_failures (50) exceeded with failure_action="continue"
- [ ] Test asserts UI step card header shows status counts: INF 1000, WRN 100, ERR 50 (NOT IMPLEMENTED - test skipped)
- [x] Test asserts errors display as separate block above ongoing logs
- [x] Build passes
- [x] Tests pass (2 pass, 1 skip)

## Applicable Architecture Requirements

| Doc | Section | Requirement |
|-----|---------|-------------|
| manager_worker_architecture.md | Worker Interfaces | Workers implement JobWorker (GetWorkerType, Validate, Execute) and/or DefinitionWorker (GetType, Init, CreateJobs, ReturnsChildJobs, ValidateConfig) |
| manager_worker_architecture.md | Data Flow | Workers create queue jobs via JobManager, StepManager routes to workers |
| QUEUE_LOGGING.md | Logging Methods | Use AddJobLog(ctx, jobID, level, msg) for context-aware logging |
| QUEUE_LOGGING.md | Log Entry Schema | LogEntry: Index, Timestamp, Level (debug/info/warn/error), Message, StepName, Originator |
| QUEUE_LOGGING.md | Log Line Numbering | CRITICAL: Lines start at 1, increment sequentially |
| QUEUE_UI.md | Icon Standards | Status icons: pending=fa-clock, running=fa-spinner fa-spin, completed=fa-check-circle, failed=fa-times-circle |
| QUEUE_UI.md | Auto-Expand | CRITICAL: Steps auto-expand when running |
| QUEUE_UI.md | API Calls | CRITICAL: < 10 API calls per step |
| QUEUE_SERVICES.md | Error Handling | Job failure flow: Worker returns error → JobProcessor catches → SetJobError → EventJobStatusChange → StepMonitor checks tolerance → StopAllChildJobs if exceeded |
| QUEUE_SERVICES.md | Event Publishing | Workers must publish events via EventService for UI updates |
| workers.md | DefinitionWorker Interface | GetType(), Init(), CreateJobs(), ReturnsChildJobs(), ValidateConfig() |
| workers.md | JobWorker Interface | GetWorkerType(), Validate(), Execute() |
| workers.md | Worker Classification | Parallel processing workers create child jobs; inline workers process synchronously |
