# Plan: GitHub Collector Jobs

## Analysis

### Problem Statement
Create two GitHub collector job types:
1. **Repo Content Collector**: Import repository files (by branch) as documents with folder path tracking
2. **Actions Log Collector**: Import GitHub Actions logs with metadata (time/date, workflow info)

### Current State
- GitHub connector exists (`internal/connectors/github/connector.go`)
- GitHubLogWorker exists (`internal/queue/workers/github_log_worker.go`)
- GitHub log utilities exist (`internal/githublogs/`) - **needs consolidation**
- No repo content fetching capability

### Architecture Approach
Following existing patterns:
- **Manager** (StepManager): Creates parent job, spawns children for each file/log
- **Worker** (JobWorker): Processes individual items from queue
- **TOML job definitions**: Configure jobs via file

### New Job Types
| Job Type | Manager Action | Worker Type | Source Type |
|----------|---------------|-------------|-------------|
| GitHub Repo | `github_repo_fetch` | `github_repo_file` | `github_repo` |
| GitHub Actions | `github_actions_fetch` | `github_action_log` | `github_action_log` |

### Document Fields
- **Tags**: From job definition + auto-generated (e.g., `["github", "repo-name", "branch"]`)
- **Metadata.folder**: File path within repo (e.g., `src/components/`)
- **Metadata.branch**: Branch name
- **Metadata.commit_sha**: Current commit
- **Metadata.workflow_name**: For action logs
- **Metadata.run_date**: For action logs

### Dependencies
- `github.com/google/go-github/v60` - Already in use
- Existing connector service
- Existing document storage

### Risks
- Rate limiting: GitHub API has limits (use connector token)
- Large repos: Need pagination and file type filtering
- Binary files: Should be excluded

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Add new job/source type constants | none | no | low | sonnet |
| 2 | Extend GitHub connector with repo API methods | 1 | no | medium | sonnet |
| 3 | Create GitHubRepoManager (spawns file jobs) | 2 | no | medium | sonnet |
| 4 | Create GitHubRepoWorker (processes files) | 2 | no | medium | sonnet |
| 5 | Refactor GitHubLogManager from githublogs | 2 | no | medium | sonnet |
| 6 | Update GitHubLogWorker with metadata | 2 | no | low | sonnet |
| 7 | Register managers and workers in app.go | 3,4,5,6 | no | low | sonnet |
| 8 | Create TOML job definitions | 7 | no | low | sonnet |
| 9 | Add API endpoints for GitHub jobs | 7 | no | medium | sonnet |
| 10 | Create tests for GitHub collectors | 7 | no | medium | sonnet |

## Order
Sequential: [1] → [2] → Concurrent: [3,4,5,6] → Sequential: [7] → Concurrent: [8,9,10] → Review

## Files to Modify/Create

### New Files
- `internal/connectors/github/repo.go` - Repo content fetching
- `internal/queue/managers/github_repo_manager.go` - Repo job orchestration
- `internal/queue/managers/github_actions_manager.go` - Actions log orchestration
- `internal/queue/workers/github_repo_worker.go` - File processing worker
- `deployments/local/job-definitions/github-repo-collector.toml`
- `deployments/local/job-definitions/github-actions-collector.toml`
- `test/api/github_jobs_test.go`

### Modified Files
- `internal/models/job_model.go` - Add job type constants
- `internal/models/document.go` - Add source type constants
- `internal/connectors/github/connector.go` - Add repo methods
- `internal/queue/workers/github_log_worker.go` - Enhance metadata
- `internal/app/app.go` - Register new managers/workers
- `internal/api/handlers/jobs_handler.go` - Add GitHub job endpoints

### Deprecated/Removed
- `internal/githublogs/` - Consolidate into connector package
