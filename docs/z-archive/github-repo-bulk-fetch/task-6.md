# Task 6: Performance Testing and Validation

- Group: 6 | Mode: sequential | Model: sonnet
- Skill: @performance-engineer | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-6/ | Source: C:/development/quaero/ | Output: docs/fixes/github-repo-bulk-fetch/

## Files
- `test/config/job-definitions/github-repo-collector.toml` - Enable batch mode
- `test/config/job-definitions/github-repo-collector-batch.toml` - NEW: Batch mode test config

## Requirements

### 1. Create Batch Mode Job Definition
```toml
id = "github-repo-collector-batch"
name = "GitHub Repository Collector (Batch Mode)"
type = "fetch"
timeout = "30m"
enabled = true

[[steps]]
name = "fetch_repo_content_batch"
action = "github_repo_fetch"

[steps.config]
connector_id = "{github_connector_id}"
owner = "ternarybob"
repo = "quaero"
branches = ["main"]
extensions = [".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"]
exclude_paths = ["vendor/", "node_modules/", ".git/", "dist/", "build/"]
max_files = 100
batch_mode = true
batch_size = 50
```

### 2. Run Performance Comparison
Execute both job definitions and compare:
```bash
# Sequential mode (existing)
# Start time, end time, document count

# Batch mode (new)
# Start time, end time, document count
```

### 3. Validation Checklist
- [ ] Documents created match between modes
- [ ] Content is identical
- [ ] Metadata is complete
- [ ] No duplicate documents

### 4. Performance Metrics to Capture
```
| Metric | Sequential | Batch | Improvement |
|--------|------------|-------|-------------|
| Total time | X sec | Y sec | Z% |
| API calls | N | M | (N-M)/N% |
| Documents/sec | A | B | B/A x |
```

### 5. Edge Case Testing
- Empty repository
- Repository with only binary files
- Repository with very large files (>1MB)
- Repository with 500+ files
- Network interruption during batch

### 6. Update Default Configuration
If performance validates, update default job definition:
```toml
batch_mode = true  # Enable by default
```

## Acceptance
- [ ] Performance test shows 5x+ improvement
- [ ] All documents match between modes
- [ ] Edge cases handled correctly
- [ ] No regressions in existing functionality
- [ ] Build passes: `go build -o /tmp/quaero ./...`
- [ ] All tests pass: `go test ./...`
