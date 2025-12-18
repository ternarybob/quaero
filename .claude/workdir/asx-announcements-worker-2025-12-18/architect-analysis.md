# Architect Analysis: ASX Announcements Worker

## Research Findings

### ASX Announcements Data Source
- **URL**: `https://www.asx.com.au/asx/v2/statistics/announcements.do?by=asxCode&timeframe=D&period=M6&asxCode={CODE}`
- **Format**: HTML (not JSON API)
- **Fields Available**:
  - Date/time
  - Price sensitivity flag
  - Headline
  - PDF document link
  - File metadata (pages, size)

### Official API Status
- Official ASX API requires paid subscription ($$$)
- Undocumented JSON APIs were blocked in Feb 2024 with anti-bot measures
- HTML page is still accessible

## Architecture Decision

### ANTI-CREATION BIAS Applied
Instead of creating complex PDF processing infrastructure, extend existing patterns:

1. **Use existing goquery** (already in go.mod) for HTML parsing
2. **Store announcement metadata** with PDF URLs
3. **Let Gemini web search** access PDF content during summarization
4. **Follow web_search_worker pattern** for document creation

### Implementation Approach

```
┌─────────────────────────────────────────┐
│ step.fetch_announcements                │
│ type = "asx_announcements"              │
│ asx_code = "GNP"                        │
│ period = "M6" (6 months)                │
│                                         │
│ For each announcement:                  │
│   → Parse date, headline, PDF URL       │
│   → Create document with tags:          │
│     ["asx-announcement", "gnp",         │
│      "date:YYYY-MM-DD", output_tags...] │
│   → Store PDF URL in metadata           │
└─────────────────────────────────────────┘
```

### Document Structure

Each announcement stored as individual document:
```markdown
# ASX Announcement: {Headline}

**Date**: {date}
**Company**: ASX:{code}
**Price Sensitive**: Yes/No
**Document**: [{filename}]({pdf_url})
**Pages**: {page_count}
**Size**: {file_size}

---
*Full announcement available at PDF link above*
```

### Tags Applied
- `asx-announcement` - Source type
- `{asx_code}` - Company code (lowercase)
- `date:YYYY-MM-DD` - Announcement date
- `price-sensitive` (if applicable)
- `output_tags` from step config

## Existing Patterns to EXTEND

| Component | Existing Code | Pattern |
|-----------|---------------|---------|
| Worker structure | `web_search_worker.go` | DefinitionWorker interface |
| HTML parsing | `goquery` (in go.mod) | Used by crawler |
| Document creation | `web_search_worker.go:469-556` | Same structure |
| Tag handling | `web_search_worker.go:517-538` | Date tag + output_tags |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/queue/workers/asx_announcements_worker.go` | New worker |
| `internal/models/worker_types.go` | Add WorkerTypeASXAnnouncements |

## Files to Modify

| File | Change |
|------|--------|
| `internal/app/worker_registry.go` | Register new worker |
| Job definition | Add fetch_announcements step |

## Future Enhancement Path

For PDF text extraction (later):
1. Add `github.com/ledongthuc/pdf` dependency
2. Download PDF to temp file
3. Extract text
4. Include in document content

For now: Store metadata + URL, let Gemini access PDFs via web.
