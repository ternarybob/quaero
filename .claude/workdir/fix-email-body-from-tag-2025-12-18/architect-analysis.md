# Architect Analysis: Fix Email body_from_tag Not Working

## Problem

Email contains "Job completed. No content was specified for this email." instead of the summary markdown.

## Root Cause

The `SummaryWorker.createDocument()` method (lines 442-495 in `summary_worker.go`) does NOT support the `output_tags` step config parameter.

Current tag sources (line 444-452):
```go
tags := []string{"summary"}
if jobDef != nil {
    jobNameTag := strings.ToLower(strings.ReplaceAll(jobDef.Name, " ", "-"))
    tags = append(tags, jobNameTag)
    if len(jobDef.Tags) > 0 {
        tags = append(tags, jobDef.Tags...)
    }
}
```

The step config `output_tags = ["asx-gnp-summary"]` is never read or applied.

## Evidence

1. Job definition specifies:
   ```toml
   [step.summarize_results]
   output_tags = ["asx-gnp-summary"]
   ```

2. Email step expects:
   ```toml
   [step.email_summary]
   body_from_tag = "asx-gnp-summary"
   ```

3. Summary document was saved with tags: `["summary", "web-search:-asx:gnp-company-info", "web-search", "asx", "stocks", "gnp"]`

4. Email worker searched for tag `"asx-gnp-summary"` → found 0 documents

## Solution

Modify `SummaryWorker.createDocument()` to support `output_tags` from step config, following the same pattern as other workers.

## Implementation

In `internal/queue/workers/summary_worker.go`:

1. Pass step config to `createDocument()`
2. Extract `output_tags` from step config
3. Append output_tags to the document tags

## Anti-Creation Verification

| Action | Type | Justification |
|--------|------|---------------|
| Modify summary_worker.go | MODIFY | Add support for existing `output_tags` pattern |

**No new code files needed** - extending existing worker functionality.
