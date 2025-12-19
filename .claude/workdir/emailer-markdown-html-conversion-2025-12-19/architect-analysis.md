# Architect Analysis: Email Worker Markdown/HTML Issues

## Problems Reported

1. **Markdown sent instead of HTML** - When markdown is "too large" it loses formatting
2. **Saved summary doesn't match what's sent** - Summary document content differs from email

## Code Flow Analysis

### Email Worker Flow (`email_worker.go`)

```
1. resolveBody() called with stepConfig
2. If body_from_tag is set:
   a. Search for documents with tag (Limit: 1)
   b. Get first result document
   c. Read doc.ContentMarkdown
   d. Call convertMarkdownToHTML(doc.ContentMarkdown)
   e. Returns (textBody, htmlBody)
3. CreateJobs sends email:
   - If htmlBody != "" → SendHTMLEmail (multipart HTML + text)
   - Else → SendEmail (plain text only)
```

### Markdown to HTML Conversion (`email_worker.go:277-320`)

```go
func (w *EmailWorker) convertMarkdownToHTML(markdown string) string {
    if markdown == "" {
        return ""  // ← Problem 1: Returns empty for empty input
    }

    md := goldmark.New(...)

    var buf bytes.Buffer
    if err := md.Convert([]byte(markdown), &buf); err != nil {
        // Fallback to preformatted
        return w.wrapInEmailTemplate("<pre>..." + escapeHTML(markdown) + "</pre>")
    }

    htmlContent := buf.String()
    if htmlContent == "" {
        // Fallback for empty conversion result
        return w.wrapInEmailTemplate("<pre>..." + escapeHTML(markdown) + "</pre>")
    }

    return w.wrapInEmailTemplate(htmlContent)
}
```

## Root Cause Analysis

### Issue 1: Markdown Being Sent Instead of HTML

**Hypothesis**: The goldmark conversion is working fine, but the HTML is being constructed correctly but the email **appears** to show markdown because email clients may not render the HTML part properly.

Looking at `mailer/service.go:172-195`:
```go
if htmlBody != "" {
    boundary := "boundary123456789"  // ← PROBLEM: Static boundary
    msg.WriteString("MIME-Version: 1.0\r\n")
    msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
    ...
}
```

**PROBLEM FOUND**: The MIME boundary is static (`boundary123456789`).

This is NOT a size issue. The issue is likely:
1. **Missing Content-Transfer-Encoding headers** for large messages with special characters
2. **Missing base64 encoding** for HTML content that may contain long lines or special chars

Email RFCs (RFC 2045, 5322) have line length limits (998 chars max, 78 recommended). Large HTML content without proper encoding can be corrupted by mail servers.

### Issue 2: Saved Summary Not Matching Email

Looking at the search in `resolveBody()`:
```go
if tag, ok := stepConfig["body_from_tag"].(string); ok && tag != "" {
    opts := interfaces.SearchOptions{
        Tags:  []string{tag},
        Limit: 1,  // ← Gets "first" result
    }
    results, err := w.searchService.Search(ctx, "", opts)
    if err == nil && len(results) > 0 {
        if doc, err := w.documentStorage.GetDocument(results[0].ID); err == nil && doc != nil {
            // Use doc.ContentMarkdown
        }
    }
}
```

The search uses `ListDocuments` (empty query) with `OrderBy: "updated_at"`, `OrderDir: "desc"`.

**POTENTIAL ISSUE**: If there are multiple documents with the tag `asx-gnp-summary`:
- The search returns the most recently **updated** document
- This might be an OLD summary if it was recently updated for some reason
- Or a different summary from a previous job run

The user should only have ONE document with the specific output_tag, but if old summaries aren't cleaned up, the wrong one could be selected.

## Investigation Needed

1. Check if there are multiple documents with tag `asx-gnp-summary`
2. Check if Content-Transfer-Encoding is properly set for large HTML

## Recommended Fixes

### Fix 1: Add Content-Transfer-Encoding to MIME parts

In `mailer/service.go`, the HTML part should include:
```go
msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")  // ADD THIS
msg.WriteString("\r\n")
msg.WriteString(quotedPrintableEncode(htmlBody))  // ENCODE HTML
```

### Fix 2: Use unique boundary per message

Replace static `boundary123456789` with a unique boundary:
```go
boundary := fmt.Sprintf("boundary_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
```

## EXTEND > MODIFY > CREATE

- **EXTEND**: Use existing mailer service patterns
- **MODIFY**: `internal/services/mailer/service.go` - add proper MIME encoding
- **CREATE**: No new files needed

## Files to Modify

1. `internal/services/mailer/service.go` - Add Content-Transfer-Encoding, unique boundary
