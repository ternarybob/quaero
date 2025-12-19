# Step 1: Add Document ID to Search Filter

## Problem

User sees document ID in logs (e.g., `doc_9624`) but cannot locate the document in the Documents page because:
1. The search box didn't search by document ID
2. The ID was only visible after expanding the detail row

## Solution

Added `doc.id` to the client-side search filter in `pages/documents.html`.

## Change Made

**File**: `pages/documents.html`, line 256

```javascript
// Before:
filteredDocuments = allDocuments.filter(doc => {
    return (doc.title && doc.title.toLowerCase().includes(searchTerm)) ||
        (doc.content_markdown && doc.content_markdown.toLowerCase().includes(searchTerm)) ||
        (doc.source_id && doc.source_id.toLowerCase().includes(searchTerm));
});

// After:
filteredDocuments = allDocuments.filter(doc => {
    return (doc.id && doc.id.toLowerCase().includes(searchTerm)) ||
        (doc.title && doc.title.toLowerCase().includes(searchTerm)) ||
        (doc.content_markdown && doc.content_markdown.toLowerCase().includes(searchTerm)) ||
        (doc.source_id && doc.source_id.toLowerCase().includes(searchTerm));
});
```

## How It Works

1. User copies document ID from logs (e.g., `doc_9624`)
2. User pastes ID into the Documents page search box
3. The document is filtered and shown in results
4. User clicks row to expand and see full details

## Build Verification

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Anti-Creation Compliance

- **EXTEND**: N/A
- **MODIFY**: Modified existing search filter in `documents.html`
- **CREATE**: No new files
