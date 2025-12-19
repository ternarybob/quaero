# Architect Analysis: Document ID Display in Documents Page

## Problem Statement

User cannot find a document by its ID (e.g., `doc_9624`) in the documents page because:
1. The ID is only visible in the expanded detail row (requires clicking first)
2. The search box doesn't search by document ID

From logs:
```
[INF] Summary completed: document saved with ID doc_9624
```

The user sees this ID in logs but cannot locate the document in the UI.

## Current Implementation Analysis

### Document Table (`pages/documents.html`)

**Table Structure (lines 90-109)**:
```html
<tr>
    <th>checkbox</th>
    <th>SOURCE</th>
    <th>DETAILS</th>
    <th>TAGS</th>
    <th>UPDATED</th>
</tr>
```

**Document ID Display (line 329)**:
The ID is shown ONLY in the expanded detail row:
```html
<span class="label label-primary">${doc.id}</span>
```

**Search Function (lines 249-262)**:
```javascript
filteredDocuments = allDocuments.filter(doc => {
    return (doc.title && doc.title.toLowerCase().includes(searchTerm)) ||
        (doc.content_markdown && doc.content_markdown.toLowerCase().includes(searchTerm)) ||
        (doc.source_id && doc.source_id.toLowerCase().includes(searchTerm));
});
```
**Problem**: Search does NOT include `doc.id`.

## Solution Options

### Option 1: Add ID to Search (Minimal Change) ✅ RECOMMENDED
Add `doc.id` to the client-side search filter:
```javascript
return (doc.id && doc.id.toLowerCase().includes(searchTerm)) ||
       (doc.title && doc.title.toLowerCase().includes(searchTerm)) || ...
```

**Pros**:
- Minimum change (1 line)
- User can search by ID
- No UI layout changes

**Cons**:
- ID still not visible until searched/expanded

### Option 2: Display Short ID in Table
Add ID column or show truncated ID in DETAILS column.

**Cons**:
- More intrusive change
- Table already dense

### Option 3: Both (Search + Display)
Add to search AND show ID prefix in table.

## Recommendation

**Option 1: Add ID to search filter**

This is the minimum viable change that solves the user's problem. They can:
1. Copy ID from logs (`doc_9624`)
2. Paste into search box
3. Document appears in results
4. Click to expand and see full details

## EXTEND > MODIFY > CREATE

- **EXTEND**: N/A - no interfaces to extend
- **MODIFY**: `pages/documents.html` - add `doc.id` to search filter (line ~256)
- **CREATE**: Nothing new needed

## Files to Modify

1. `pages/documents.html` - Line ~256 in `searchDocuments()` function

## Skill Compliance

- **Frontend Skill**: Using existing JavaScript patterns, no new frameworks
- **Refactoring Skill**: Minimum viable change, no new files
