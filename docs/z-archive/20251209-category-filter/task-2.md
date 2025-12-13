# Task 2: Add filter_category support to agent_worker queryDocuments

- **Depends:** 1
- **Skill:** go
- **Critical:** no
- **Files:** internal/queue/workers/agent_worker.go

## Actions
- Add `filter_category` extraction from step config
- Convert `filter_category: ["source", "build"]` to MetadataFilters
- Pass to SearchOptions.MetadataFilters

## Config Format
```toml
[step.extract_build_info]
filter_category = ["build", "config", "docs"]
```

Translates to:
```go
opts.MetadataFilters = map[string]string{
    "rule_classifier.category": "build,config,docs",
}
```

## Acceptance
- [ ] filter_category extracted from step config
- [ ] Converted to MetadataFilters with nested key
- [ ] Documents filtered correctly
