# Plan: Codebase Assessment Implementation

Type: fix | Workdir: docs/fix/20251208-codebase-assess-impl/

## User Intent (from manifest)

Implement the codebase assessment pipeline redesign:
1. Remove redundant code (extract_structure_worker, devops_enrich.toml)
2. Create new language-agnostic codebase_assess.toml pipeline
3. Add multi-language test fixture
4. Implement TDD test for the new pipeline
5. Breaking changes are acceptable

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 0 | Delete extract_structure files and remove references | - | no | sonnet |
| 1 | Remove WorkerTypeExtractStructure from worker_type.go | 0 | no | sonnet |
| 2 | Remove extract_structure registration from app.go | 0,1 | no | sonnet |
| 3 | Create codebase_assess.toml job definition | - | no | sonnet |
| 4 | Create multi-language test fixture | - | no | sonnet |
| 5 | Replace devops_enrichment_test.go with codebase_assessment_test.go | 3,4 | no | sonnet |
| 6 | Update devops_enrich.toml to remove extract_structure step | 0 | no | sonnet |
| 7 | Verify build passes | 0-6 | no | sonnet |

## Order

[0] → [1,2,6] → [3,4] → [5] → [7]
