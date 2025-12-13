# Plan: DevOps Enrichment Pipeline for C/C++ Codebase Analysis

Type: feature | Workdir: ./docs/feature/20251207-devops-enrichment-pipeline/

## User Intent (from manifest)

Enable a DevOps engineer to understand a large, legacy C/C++ codebase (2GB, 30 years old) well enough to build CI/CD pipelines - without requiring C/C++ programming expertise. The pipeline should:

1. Extract structural information from C/C++ files (includes, defines, platform conditionals)
2. Analyze build systems (Makefiles, CMakeLists.txt, vcxproj) for targets, flags, dependencies
3. Classify files by role, component, test type using LLM
4. Build a dependency graph across all documents
5. Generate an actionable DevOps summary/guide for CI/CD

Each pass adds metadata tags to documents, building a queryable knowledge base.

## Tasks

| # | Description | Depends | Critical | Model |
|---|-------------|---------|----------|-------|
| 1 | Add DevOps worker type and interfaces | - | no | sonnet |
| 2 | Implement extract_structure action (regex-based) | 1 | no | sonnet |
| 3 | Implement analyze_build_system action (regex + LLM) | 1 | no | sonnet |
| 4 | Implement classify_devops action (LLM-based) | 1 | no | sonnet |
| 5 | Implement build_dependency_graph action | 1 | no | sonnet |
| 6 | Implement aggregate_devops_summary action (LLM synthesis) | 1 | no | sonnet |
| 7 | Create devops_enrich job definition | 2,3,4,5,6 | no | sonnet |
| 8 | Implement DevOps API handler and endpoints | 5,6 | no | sonnet |
| 9 | Create C/C++ test fixtures | - | no | sonnet |
| 10 | Write unit tests for all actions | 2,3,4,5,6 | no | sonnet |
| 11 | Write API integration tests | 8 | no | sonnet |
| 12 | Write long-running UI tests with progress monitoring | 7,9 | no | sonnet |
| 13 | Build verification and cleanup | 10,11,12 | no | sonnet |

## Order

[1, 9] → [2, 3, 4, 5, 6] → [7, 8] → [10, 11] → [12] → [13]

## Notes

- Tasks 1 and 9 can run in parallel (no dependencies)
- Tasks 2-6 can run in parallel after task 1 completes (all depend only on 1)
- Tasks 7 and 8 can run in parallel after their dependencies
- Following existing patterns: SummaryWorker for LLM calls, MetadataExtractor for regex
- KV storage for graph and summary aggregates
- API handlers follow document_handler.go pattern
