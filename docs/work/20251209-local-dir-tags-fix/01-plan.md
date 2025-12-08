# Implementation Plan: Fix local_dir worker ignoring step-level tags

## Problem Analysis

The codebase assessment job (`codebase_assess.toml`) is failing at steps 6, 7, and 9 with the error:
```
Init failed: worker init failed: no documents found matching tags: [codebase quaero]
```

### Root Cause

The `LocalDirWorker.CreateJobs` method in `internal/queue/workers/local_dir_worker.go:494-498` reads tags from `jobDef.Tags` (job definition level) instead of `step.Config["tags"]` (step level):

```go
// Current (buggy) code
baseTags := jobDef.Tags
if baseTags == nil {
    baseTags = []string{}
}
```

However, the TOML job definition specifies tags at the **step level**:

```toml
[step.import_files]
tags = ["codebase", "quaero"]  # <-- This is in step.Config["tags"]
```

This means documents imported by `import_files` step don't receive the tags, so subsequent steps like `generate_summary` that filter by `filter_tags = ["codebase", "quaero"]` find zero documents.

## Skills Required
- [x] go - for service/worker implementation

## Work Packages

### WP1: Fix LocalDirWorker tag extraction [PARALLEL-SAFE]
**Skills:** go
**Files:** internal/queue/workers/local_dir_worker.go
**Description:** Update CreateJobs to read tags from step.Config["tags"] with fallback to jobDef.Tags
**Acceptance:**
- Tags from step config are used when available
- Falls back to job definition tags when step tags not specified
- Existing tests pass
- Documents created by local_dir worker have correct tags

### WP2: Add unit test for step-level tags [PARALLEL-SAFE]
**Skills:** go
**Files:** internal/queue/workers/local_dir_worker_test.go
**Description:** Add test case ensuring step-level tags are applied to created documents
**Acceptance:** Test verifies tags from step.Config are passed through batch job config

## Execution Order
1. WP1, WP2 (parallel - independent changes)

## Validation Checklist
- [ ] Build passes: `go build -o /tmp/quaero ./cmd/quaero`
- [ ] Tests pass: `go test ./internal/queue/workers/... -v`
- [ ] Run codebase_assess job and verify documents are tagged
- [ ] Verify generate_summary step finds documents with correct tags

## Implementation Details

### WP1: Code Changes

In `local_dir_worker.go`, update CreateJobs to:

```go
// Get tags for documents - prefer step config, fallback to job definition
var baseTags []string
if stepTags, ok := step.Config["tags"].([]interface{}); ok {
    for _, tag := range stepTags {
        if tagStr, ok := tag.(string); ok {
            baseTags = append(baseTags, tagStr)
        }
    }
} else if stepTags, ok := step.Config["tags"].([]string); ok {
    baseTags = stepTags
}

// Fallback to job definition tags if no step tags specified
if len(baseTags) == 0 && len(jobDef.Tags) > 0 {
    baseTags = jobDef.Tags
}
```

This follows the same pattern used by other workers (e.g., `summary_worker.go:84-92`) for extracting slice configs from TOML.
