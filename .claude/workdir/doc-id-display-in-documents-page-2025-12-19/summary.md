# Summary: Document ID Search in Documents Page

## Problem

User could not find documents by their ID (e.g., `doc_9624`) in the Documents page because:
- The search box only searched title, content, and source_id
- The document ID was only visible after expanding the row

## Solution

Added `doc.id` to the client-side search filter in `pages/documents.html`.

## Change

**File**: `pages/documents.html`, line 256

```javascript
// Added doc.id to the filter:
filteredDocuments = allDocuments.filter(doc => {
    return (doc.id && doc.id.toLowerCase().includes(searchTerm)) ||  // ← ADDED
        (doc.title && doc.title.toLowerCase().includes(searchTerm)) ||
        (doc.content_markdown && doc.content_markdown.toLowerCase().includes(searchTerm)) ||
        (doc.source_id && doc.source_id.toLowerCase().includes(searchTerm));
});
```

## Usage

1. Copy document ID from logs: `doc_9624`
2. Go to Documents page
3. Paste ID into search box
4. Document appears in results
5. Click to expand and see full details

## Build Verification

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Files Modified

| File | Change |
|------|--------|
| `pages/documents.html` | Added `doc.id` to search filter |
