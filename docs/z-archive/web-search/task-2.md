# Task 2: Create WebSearchWorker

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans\web-search

## Files
- `internal/queue/workers/web_search_worker.go` - Create new file

## Requirements

Create a JobWorker implementation that executes web searches using Gemini SDK with GoogleSearch grounding.

### Execution Flow:
1. Extract config: query, depth, breadth, resolved_api_key
2. Initialize Gemini client with GoogleSearch tool
3. Execute initial search query
4. If depth > 0, extract follow-up questions and search iteratively
5. Collect all results with source URLs
6. Create Document with:
   - Title: "Web Search: {query}"
   - ContentMarkdown: Formatted markdown with all results and sources
   - SourceType: "web_search"
   - Tags: from job definition + ["web_search"]
   - Metadata: query, search_date, result_count, depth, breadth

### Gemini SDK Usage:
```go
searchTool := genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
config := genai.GenerateContentConfig{Tools: []genai.Tool{searchTool}}
resp, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Configure(config), genai.Text(query))
// Access grounding metadata: resp.Candidates[0].GroundingMetadata.WebSearchQueries
// Access grounding chunks: resp.Candidates[0].GroundingMetadata.GroundingChunks
```

### Error Handling:
- Timeout after 5 minutes per search
- Retry on transient errors (up to 3 times)
- Store partial results if some searches fail
- Record errors in document metadata

## Acceptance
- [ ] Implements JobWorker interface
- [ ] Uses Gemini SDK with GoogleSearch grounding
- [ ] Handles depth/breadth iteration
- [ ] Creates Document with markdown content
- [ ] Includes source URLs in results
- [ ] Proper error handling
- [ ] Compiles
