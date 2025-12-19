# Step 2: Fix Summary Mismatch - Order by Created At

## Problem

When multiple documents exist with the same output tag (from previous job runs), the email worker could retrieve the wrong document. The search was ordering by `updated_at DESC`, meaning an older summary that was recently accessed could be returned instead of the newest summary.

## Root Cause

1. Summary worker creates documents with `output_tags` (e.g., `["asx-gnp-summary"]`)
2. Previous summaries with the same tag still exist in the database
3. Search used `OrderBy: "updated_at"` by default
4. If an old document was recently read/updated, it would be returned first

## Solution

1. Added `OrderBy` and `OrderDir` fields to `SearchOptions` interface
2. Updated `AdvancedSearchService.executeListDocuments()` to use these fields
3. Updated `EmailWorker.resolveBody()` to use `OrderBy: "created_at"` when searching by tag

## Changes Made

### 1. `internal/interfaces/search_service.go`

Added ordering fields to SearchOptions:
```go
type SearchOptions struct {
    // ... existing fields ...

    // OrderBy specifies the field to order results by (created_at, updated_at)
    // Defaults to "updated_at" if not specified
    OrderBy string

    // OrderDir specifies the order direction (asc, desc)
    // Defaults to "desc" if not specified
    OrderDir string
}
```

### 2. `internal/services/search/advanced_search_service.go`

Updated `executeListDocuments()` to use SearchOptions.OrderBy/OrderDir:
```go
// Use OrderBy/OrderDir from SearchOptions if specified
orderBy := opts.OrderBy
if orderBy == "" {
    orderBy = "updated_at"  // Backward compatible default
}
orderDir := opts.OrderDir
if orderDir == "" {
    orderDir = "desc"
}

listOpts := &interfaces.ListOptions{
    OrderBy:  orderBy,
    OrderDir: orderDir,
    // ...
}
```

### 3. `internal/queue/workers/email_worker.go`

Updated `resolveBody()` to order by `created_at`:
```go
opts := interfaces.SearchOptions{
    Tags:     []string{tag},
    Limit:    1,
    OrderBy:  "created_at",  // Get newest created, not most recently updated
    OrderDir: "desc",
}
```

## Why This Fixes the Issue

Before:
- Search returns document most recently **updated**
- Old summary accessed = old summary returned in email
- New summary created but not emailed

After:
- Search returns document most recently **created**
- Always gets the newest summary from the current job run
- Old summaries are ignored (still exist but sorted lower)

## Build Verification

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Anti-Creation Compliance

- **EXTEND**: Extended SearchOptions with OrderBy/OrderDir fields
- **MODIFY**: Modified existing search service and email worker
- **CREATE**: No new files created
