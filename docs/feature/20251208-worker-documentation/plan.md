# Plan: Worker Documentation

Type: feature | Workdir: docs/feature/20251208-worker-documentation

## User Intent (from manifest)

Create comprehensive documentation for all queue workers in `internal/queue/workers/`. The documentation should:
1. Describe each worker's purpose
2. Document inputs and outputs for each worker
3. Document configuration required to activate/use each worker
4. Place the resulting documentation in `docs/architecture/`

## Workers to Document (16 total)

1. agent_worker.go
2. aggregate_summary_worker.go
3. analyze_build_worker.go
4. classify_worker.go
5. code_map_worker.go
6. crawler_worker.go
7. database_maintenance_worker.go
8. dependency_graph_worker.go
9. extract_structure_worker.go
10. github_git_worker.go
11. github_log_worker.go
12. github_repo_worker.go
13. local_dir_worker.go
14. places_worker.go
15. summary_worker.go
16. web_search_worker.go

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Analyze all 16 worker files to extract purpose, inputs, outputs, and config | - | no | sonnet |
| 2 | Write comprehensive workers.md documentation in docs/architecture/ | 1 | no | sonnet |

## Order

[1] → [2]
