# Code Assessment: Consolidate Enrichment Workers Under Agent Worker

## Manifest

| Field | Value |
|-------|-------|
| Type | assessment |
| Slug | consolidate-enrichment-workers |
| Date | 2025-12-08 |
| Status | completed |

## User Intent

Assess whether the 5 enrichment pipeline workers can be replaced by the existing `agent` worker, since each step appears to be simply an AI prompt with file/files input.

**From prompt_1.md:**
> "Review the code, and assess if the agent worker can replace the specific test. The worker will require updating, to enable markdown insert into the document, or creation of a new document (ie. summary)."

## Scope

### Workers Under Assessment

1. `extract_structure` - Extracts C/C++ code structure (includes, defines, conditionals)
2. `analyze_build` - Parses build files (CMake, Makefile) for targets and dependencies
3. `classify` - LLM-based classification of file roles and components
4. `dependency_graph` - Builds dependency graph from extracted metadata
5. `aggregate_summary` - Generates summary of all enrichment metadata

### Comparison Target

- `agent` worker - Unified worker that processes documents with AI agents

## Files Analyzed

### Workers
- `internal/queue/workers/agent_worker.go`
- `internal/queue/workers/extract_structure_worker.go`
- `internal/queue/workers/analyze_build_worker.go`
- `internal/queue/workers/classify_worker.go`
- `internal/queue/workers/dependency_graph_worker.go`
- `internal/queue/workers/aggregate_summary_worker.go`
- `internal/queue/workers/summary_worker.go`

### Actions
- `internal/jobs/actions/extract_structure.go`
- `internal/jobs/actions/analyze_build_system.go`
- `internal/jobs/actions/classify_devops.go`
- `internal/jobs/actions/build_dependency_graph.go`
- `internal/jobs/actions/aggregate_devops_summary.go`

### Job Definition
- `test/results/ui/dev-20251208-150723/TestDevOpsEnrichmentPipeline_FullFlow/devops_enrich.toml`

## Deliverables

- [x] manifest.md - This file
- [x] assessment.md - Detailed analysis and findings

## Conclusion

**The enrichment workers CANNOT be fully replaced by the agent worker.**

### Key Findings:

| Worker | Uses LLM? | Can Replace with Agent? |
|--------|-----------|------------------------|
| `extract_structure` | No (regex) | **NO** - More efficient as regex |
| `analyze_build` | Yes | Partially - needs modifications |
| `classify` | Yes | Partially - needs modifications |
| `dependency_graph` | No (aggregation) | **NO** - Multi-document operation |
| `aggregate_summary` | Yes | **NO** - Creates new documents |

### Recommendation

**Keep the specialized workers.** The current architecture provides:
- Efficient regex-based processing (no LLM overhead)
- Multi-document aggregation support
- Document creation capability
- Appropriate separation of concerns

See `assessment.md` for detailed analysis.
