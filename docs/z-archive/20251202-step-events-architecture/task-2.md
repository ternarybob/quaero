# Task 2: Audit and fix other workers (github, agent, places, web_search)

Depends: 1 | Critical: yes:architectural-change | Model: opus

## Addresses User Intent

Ensures ALL workers follow the same pattern - no direct EventService publishing.

## Do

1. Review github_log_worker.go - remove direct `document_saved` publishing
2. Review github_repo_worker.go - remove direct `document_saved` publishing
3. Review agent_worker.go - remove direct `job_error` publishing in `publishJobError()`
4. Review places_worker.go - replace `PublishSync` with Job Manager methods
5. Review web_search_worker.go - replace `PublishSync` with Job Manager methods
6. Ensure all workers use the uniform logging pattern

## Accept

- [ ] No direct EventService calls in any worker
- [ ] All workers use Job Manager's unified event publishing
- [ ] Code compiles without errors
