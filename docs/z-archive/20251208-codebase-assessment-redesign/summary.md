# Complete: Codebase Assessment Pipeline Redesign

Type: fix | Tasks: 5 | Files: 1

## User Request

"Rethink the process for large codebase assessment to provide index, summary, and map. Review workers and provide actionable recommendations."

## Result

Created comprehensive recommendations document at `docs/fix/20251208-codebase-assessment-redesign/recommendations.md` containing:

### Analysis Completed

- **Current Pipeline Gaps**: Identified 5 critical gaps in `devops_enrich.toml`:
  - C/C++ only (hardcoded regex patterns)
  - No index, summary, or map artifacts
  - Underutilized workers (CodeMapWorker, SummaryWorker not used)

### Redesigned Pipeline

- **New `codebase_assess.toml`**: 9-step language-agnostic pipeline
- **3 Phases**:
  1. Import & Index (code_map, import_files)
  2. Analysis (classify_files, extract_build_info, identify_components)
  3. Synthesis (build_graph, generate_index, generate_summary, generate_map)

### Worker Recommendations

| Step | Worker | Purpose |
|------|--------|---------|
| code_map | CodeMapWorker | Hierarchical structure with LOC, languages |
| import_files | LocalDirWorker | Import file content as searchable documents |
| classify_files | AgentWorker | LLM-based file classification |
| generate_* | SummaryWorker | LLM synthesis of index/summary/map |

### TDD Test Specification

- `TestCodebaseAssessment_FullFlow` with 3-phase assertions
- Multi-language test fixture (Go + Python + JS)
- Artifact validation functions (assertIndexDocument, assertSummaryDocument, assertMapDocument)

### Implementation Tasks for 3agents

1. Create `bin/job-definitions/codebase_assess.toml`
2. Add `test/fixtures/multi_lang_project/` fixture
3. Implement `test/ui/codebase_assessment_test.go`
4. Enhance DependencyGraphWorker for language-agnostic detection
5. Add new agent types (build_extractor, architecture_mapper, file_indexer)
6. Run tests and iterate

## Validation: MATCHES

All success criteria met - comprehensive analysis with actionable recommendations.

## Review: N/A

No critical triggers (analysis/documentation task, no code changes)

## Verify

Build: N/A | Tests: N/A (documentation/analysis only - no code changes)

## Files Created

- `docs/fix/20251208-codebase-assessment-redesign/recommendations.md` - Full redesign specification (~400 lines)
