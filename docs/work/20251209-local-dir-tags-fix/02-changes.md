# Changes Log

## WP1: Fix LocalDirWorker tag extraction

### Files Modified
- `internal/queue/workers/local_dir_worker.go` - Fixed tag extraction to read from step.Config["tags"] with fallback to jobDef.Tags

### Changes Made
Updated the tag extraction logic in `CreateJobs` method (lines 494-509):

**Before:**
```go
// Get tags for documents
baseTags := jobDef.Tags
if baseTags == nil {
    baseTags = []string{}
}
```

**After:**
```go
// Get tags for documents - prefer step config tags, fallback to job definition tags
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

### Skill Compliance
- [x] Error wrapping with context - N/A (no new errors introduced)
- [x] Structured logging - N/A (existing logging unchanged)
- [x] Table-driven tests - new test uses table-driven pattern
- [x] Follows existing patterns - matches filter_tags extraction in other workers

### Ready for Validation

---

## WP2: Add unit test for step-level tags

### Files Modified
- `internal/queue/workers/local_dir_worker_test.go` - Added comprehensive table-driven test

### Test Cases Added
1. Step tags as interface slice (TOML parsing)
2. Step tags as string slice (direct Go usage)
3. Fallback to job definition tags when step tags missing
4. Step tags override job definition tags
5. Empty step tags fallback to job definition

### Skill Compliance
- [x] Table-driven tests - uses Go testing table pattern
- [x] Named subtests - uses t.Run() for clear test naming
- [x] Follows existing patterns - matches other tests in the file

### Ready for Validation
