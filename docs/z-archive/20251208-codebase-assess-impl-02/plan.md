# Plan: Codebase Assessment Implementation

Type: fix | Workdir: docs/fix/20251208-codebase-assess-impl-02/

## User Intent (from manifest)

Implement the codebase assessment pipeline redesign:
1. Remove redundant C/C++-specific code (extract_structure worker, devops_enrich.toml step)
2. Create new language-agnostic codebase_assess.toml pipeline
3. Add multi-language test fixture
4. Implement TDD test for the new pipeline
5. Breaking changes are acceptable - do NOT preserve backward compatibility

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Delete extract_structure files (worker, action, test) | - | no | sonnet |
| 2 | Remove WorkerTypeExtractStructure from worker_type.go | 1 | no | sonnet |
| 3 | Remove extract_structure worker registration from app.go | 1,2 | no | sonnet |
| 4 | Update devops_enrich.toml to remove extract_structure step | 1 | no | sonnet |
| 5 | Create codebase_assess.toml job definition | - | no | sonnet |
| 6 | Create multi-language test fixture | - | no | sonnet |
| 7 | Create codebase_assessment_test.go test file | 5,6 | no | sonnet |
| 8 | Verify build passes | 1-7 | no | sonnet |

## Order

[1] → [2,3,4] → [5,6] → [7] → [8]
