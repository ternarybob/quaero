# Plan: Category-Based Filtering for Agent Steps

- **Type:** feature
- **Workdir:** ./docs/feature/20251209-category-filter/
- **Skills:** go

## Overview

Add support for filtering documents by nested metadata fields (specifically `rule_classifier.category`) in agent_worker. This allows pipeline steps to only process files matching specific categories, reducing LLM calls from ~3000 to ~300.

## Tasks

| # | Description | Depends | Skill | Critical | Files |
|---|-------------|---------|-------|----------|-------|
| 1 | Add nested metadata filtering to search common.go | - | go | no | internal/services/search/common.go |
| 2 | Add filter_category support to agent_worker queryDocuments | 1 | go | no | internal/queue/workers/agent_worker.go |
| 3 | Update codebase_assess.toml pipeline with category filters | 2 | go | no | bin/job-definitions/codebase_assess.toml |

## Execution Order
[1] → [2] → [3] sequential

## Design

### Filter Syntax
Support dot-notation for nested metadata:
- `rule_classifier.category=source` - matches docs where metadata["rule_classifier"]["category"] == "source"
- `rule_classifier.category=build,config,docs` - matches any of these categories

### Pipeline Config
```toml
[step.classify_files]
filter_category = ["unknown"]  # Only LLM-classify files rule_classifier couldn't

[step.extract_build_info]
filter_category = ["build", "config", "docs"]  # Only build-related files

[step.identify_components]
filter_category = ["source"]  # Only source code files
```

## Validation Checklist
- [ ] Nested metadata filtering works: `rule_classifier.category=source`
- [ ] Multi-value filtering works: `category=build,config,docs`
- [ ] Pipeline steps filtered correctly
- [ ] Build passes: `go build -o /tmp/quaero ./cmd/quaero`
- [ ] Tests pass
