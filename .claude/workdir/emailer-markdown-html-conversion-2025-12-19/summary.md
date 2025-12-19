# Summary: Email Worker Fixes

## Issues Resolved

### Issue 1: Markdown sent instead of HTML (size-related)

**Problem**: Large HTML emails were being corrupted because:
- Static MIME boundary could conflict with content
- No Content-Transfer-Encoding header
- Long lines exceeded RFC 5322 limits (998 chars max)

**Solution**: Updated `internal/services/mailer/service.go`:
- Generate unique MIME boundary using `crypto/rand`
- Add `Content-Transfer-Encoding: base64` header
- Base64 encode content with 76-char line breaks per RFC 2045

### Issue 2: Saved summary doesn't match what's sent

**Problem**: When multiple documents had the same tag (from previous job runs), the wrong one could be emailed because search used `OrderBy: "updated_at"`.

**Solution**:
1. Added `OrderBy` and `OrderDir` fields to `interfaces.SearchOptions`
2. Updated `AdvancedSearchService` to use these options
3. Updated `EmailWorker.resolveBody()` to use `OrderBy: "created_at"` when fetching by tag

## Files Modified

| File | Changes |
|------|---------|
| `internal/services/mailer/service.go` | Added base64 encoding, unique boundary |
| `internal/interfaces/search_service.go` | Added OrderBy/OrderDir to SearchOptions |
| `internal/services/search/advanced_search_service.go` | Use OrderBy from options |
| `internal/queue/workers/email_worker.go` | Use created_at ordering for tag search |

## Build Verification

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Testing

To verify the fixes:
1. Restart the Quaero server
2. Run the ASX investment analysis job
3. Verify email arrives with:
   - Properly formatted HTML (tables, headers, etc.)
   - Content matching the saved summary document

## Technical Details

### MIME Encoding (RFC Compliance)

```
Before:
Content-Type: text/html; charset="UTF-8"
<raw HTML with long lines>

After:
Content-Type: text/html; charset="UTF-8"
Content-Transfer-Encoding: base64

PHN0eWxlPmJvZHl7Zm9udC1mYW1pbHk6LWFwcGxlLXN5c3RlbSxC...
```

### Search Ordering

```
Before:
opts := interfaces.SearchOptions{
    Tags:  []string{tag},
    Limit: 1,
}  // Orders by updated_at DESC

After:
opts := interfaces.SearchOptions{
    Tags:     []string{tag},
    Limit:    1,
    OrderBy:  "created_at",
    OrderDir: "desc",
}  // Orders by created_at DESC
```
