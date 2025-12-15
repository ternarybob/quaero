# Refactor Codebase Assessment Pipeline for Large Codebases

## Problem Statement

The current `codebase_assess.toml` pipeline is ineffective for large codebases (5000+ files) due to:

1. **LLM Call Explosion**: Phase 2 makes 3 LLM calls per document (classify + enrich + recognize). For 5000 files = 15,000+ API calls.
2. **Hard Limit of 1000 Documents**: `agent_worker.go:629` has `Limit: 1000` - silently ignores files beyond this.
3. **Insufficient Timeout**: 4 hours is not enough for thousands of LLM calls with rate limiting.
4. **No Prioritization**: All files treated equally; critical files (entry points, APIs) not prioritized.

## Current Architecture

```
codebase_assess.toml
├── Phase 1: IMPORT (Parallel) - Efficient
│   ├── code_map (static regex analysis)
│   └── import_files (file import)
├── Phase 2: ANALYSIS (Sequential) - PROBLEMATIC
│   ├── classify_files (category_classifier) → 1 LLM call/file
│   ├── extract_build_info (metadata_enricher) → 1 LLM call/file
│   └── identify_components (entity_recognizer) → 1 LLM call/file
└── Phase 3: SYNTHESIS
    ├── generate_index (summary)
    ├── generate_summary (summary)
    └── generate_map (summary)
```

## Existing Assets

The codebase already has these scalable alternatives:

| Asset | Location | Description |
|-------|----------|-------------|
| `rule_classifier` agent | `internal/services/agents/rule_classifier.go` | Pattern-based classification, zero LLM calls |
| `batch_mode` option | `agent_worker.go:326` | Process all docs inline without child jobs |
| `codebase_classify.toml` | `deployments/local/job-definitions/` | Pipeline using rule_classifier |

## Refactoring Requirements

### 1. Replace LLM Classification with Rule-Based

**File**: `deployments/local/job-definitions/codebase_assess.toml`

Change:
```toml
[step.classify_files]
type = "agent"
agent_type = "category_classifier"  # LLM-based, slow
```

To:
```toml
[step.classify_files]
type = "agent"
agent_type = "rule_classifier"  # Pattern-based, fast
batch_mode = true               # Process inline, no child jobs
```

### 2. Remove Redundant LLM Steps

Remove or make optional:
- `extract_build_info` - Can be inferred from file patterns (package.json, Makefile, etc.)
- `identify_components` - Can be derived from code_map exports/imports

### 3. Increase Document Query Limit

**File**: `internal/queue/workers/agent_worker.go`

Change line ~629:
```go
opts.Limit = 1000 // Current limit
```

To:
```go
opts.Limit = 10000 // Support larger codebases
```

Or make configurable via step config:
```go
if limit, ok := filter["limit"].(int); ok && limit > 0 {
    opts.Limit = limit
} else {
    opts.Limit = 10000 // Default for large codebases
}
```

### 4. Add Selective LLM Enrichment (Optional)

Create a new step that only applies LLM analysis to high-value files:

```toml
[step.enrich_key_files]
type = "agent"
description = "LLM enrichment for critical files only"
depends = "classify_files"
agent_type = "metadata_enricher"
filter_tags = ["codebase", "test-project"]
filter_category = ["entry", "api", "core"]  # Only key files
batch_mode = true
```

### 5. Hierarchical Summarization

Instead of summarizing all files individually, summarize by directory then aggregate:

```toml
[step.summarize_directories]
type = "summary"
description = "Summarize each major directory"
depends = "classify_files"
filter_source_type = "code_map"
filter_node_type = "directory"  # Only directory documents
```

## Proposed New Pipeline Structure

```toml
# Phase 1: IMPORT (Parallel) - No changes needed
[step.code_map]
type = "code_map"
skip_summarization = true

[step.import_files]
type = "local_dir"

# Phase 2: FAST CLASSIFICATION (Rule-based)
[step.classify_files]
type = "agent"
agent_type = "rule_classifier"
batch_mode = true
depends = "import_files"

# Phase 3: SELECTIVE ENRICHMENT (LLM only for unknowns)
[step.enrich_unknown]
type = "agent"
agent_type = "metadata_enricher"
depends = "classify_files"
filter_category = ["unknown"]  # Only files rule_classifier couldn't categorize
batch_mode = true

# Phase 4: SYNTHESIS
[step.generate_summary]
type = "summary"
depends = "classify_files, code_map"
filter_category = ["source", "entry", "api", "config"]  # Key files only
```

## Implementation Tasks

| # | Task | File | Priority |
|---|------|------|----------|
| 1 | Update `classify_files` to use `rule_classifier` | `codebase_assess.toml` | High |
| 2 | Add `batch_mode = true` to agent steps | `codebase_assess.toml` | High |
| 3 | Remove `extract_build_info` step | `codebase_assess.toml` | Medium |
| 4 | Remove `identify_components` step | `codebase_assess.toml` | Medium |
| 5 | Increase default document limit to 10000 | `agent_worker.go` | High |
| 6 | Add selective LLM enrichment for unknowns | `codebase_assess.toml` | Low |
| 7 | Update summary steps to filter by category | `codebase_assess.toml` | Medium |
| 8 | Increase timeout to 8h for safety | `codebase_assess.toml` | Low |

## Acceptance Criteria

- [ ] Pipeline processes 5000+ files without hitting limits
- [ ] Classification completes in under 10 minutes (vs hours with LLM)
- [ ] LLM calls reduced by 90%+ (only for synthesis and unknown files)
- [ ] All existing tests pass
- [ ] Build succeeds: `go build -o /tmp/quaero ./cmd/quaero`

## Testing

```bash
# Build
go build -o /tmp/quaero ./cmd/quaero

# Run tests
go test ./internal/queue/workers/... -v

# Manual test with large codebase
# 1. Start quaero
# 2. Load codebase_assess job definition
# 3. Run against a 5000+ file codebase
# 4. Verify completion time < 30 minutes
```

## References

- Existing rule_classifier: `internal/services/agents/rule_classifier.go`
- Agent worker batch_mode: `internal/queue/workers/agent_worker.go:406-580`
- Alternative pipeline: `deployments/local/job-definitions/codebase_classify.toml`
- Code map worker: `internal/queue/workers/code_map_worker.go`
