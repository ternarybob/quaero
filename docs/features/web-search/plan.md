# Plan: Web Search Job Type

## Analysis

### Overview
Create a new "web_search" job type that uses Gemini SDK's GoogleSearch grounding feature to search the web for information based on natural language queries and store results as documents.

### Dependencies
- `google.golang.org/genai` - Gemini SDK with GoogleSearch grounding tool
- Existing job architecture (managers, workers, job definitions)
- Document storage system (Badger with FTS5)
- Queue system for job processing

### Approach
1. Create `WebSearchManager` - handles parent job creation, doesn't need child jobs (single search query)
2. Create `WebSearchWorker` - executes web search using Gemini SDK with GoogleSearch grounding
3. Register job type in app.go
4. Create job definition TOML with depth/breadth parameters
5. Add UI test for the new job type

### Key Design Decisions
- **Single job execution**: Unlike crawler which spawns child jobs, web search runs as a single job
- **Gemini GoogleSearch grounding**: Use `genai.GoogleSearch{}` tool for web search
- **Depth parameter**: Number of follow-up queries to explore related topics (max 10)
- **Breadth parameter**: Number of results to collect per query (max 5)
- **Result format**: Single document with markdown content containing all results with references
- **Tags field**: Store query, search date, and result count in document metadata

### Risks
- API rate limits on Gemini SDK
- Search result quality depends on Gemini model
- Long-running jobs need proper timeout handling

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Create WebSearchManager step manager | none | no | medium | sonnet |
| 2 | Create WebSearchWorker job worker | 1 | no | high | sonnet |
| 3 | Register job type in app.go | 2 | no | low | sonnet |
| 4 | Create job definition TOML | 3 | no | low | sonnet |
| 5 | Create UI test | 4 | no | medium | sonnet |
| 6 | Run test and fix issues | 5 | no | medium | sonnet |

## Order
Sequential: [1] → [2] → [3] → [4] → [5] → [6] → Validate
