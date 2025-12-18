# Summary: Fix Email body_from_tag Not Working

## Issue

Email contained "Job completed. No content was specified for this email." instead of the summary markdown from the previous step.

## Root Cause

The `SummaryWorker.createDocument()` method did not support the `output_tags` step config parameter. The job definition specified:

```toml
[step.summarize_results]
output_tags = ["asx-gnp-summary"]

[step.email_summary]
body_from_tag = "asx-gnp-summary"
```

But the summary document was saved without the `"asx-gnp-summary"` tag, so the email worker couldn't find it.

## Fix Applied

Modified `internal/queue/workers/summary_worker.go` to support `output_tags`:

1. **Extract stepConfig from metadata** (line 228):
   ```go
   stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})
   ```

2. **Updated createDocument signature** to accept stepConfig parameter

3. **Added output_tags processing** (lines 455-466):
   ```go
   if stepConfig != nil {
       if outputTags, ok := stepConfig["output_tags"].([]interface{}); ok {
           for _, tag := range outputTags {
               if tagStr, ok := tag.(string); ok && tagStr != "" {
                   tags = append(tags, tagStr)
               }
           }
       }
   }
   ```

4. **Updated call site** to pass stepConfig

## Build Verification

- Main build: PASS
- MCP server: PASS

## Testing

Restart the server and re-run the "Web Search: ASX:GNP Company Info" job. The email should now contain the summary markdown.

## Data Flow After Fix

```
[step.summarize_results]
├── Creates summary document
├── Tags: ["summary", "web-search-...", ..., "asx-gnp-summary"]  ← output_tags applied
└── Saves to document storage

[step.email_summary]
├── Searches documents with tag "asx-gnp-summary"
├── Finds summary document  ← NOW WORKS
├── Extracts ContentMarkdown
└── Sends email with summary content
```
