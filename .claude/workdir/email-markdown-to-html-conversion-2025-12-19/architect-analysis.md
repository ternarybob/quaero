# ARCHITECT ANALYSIS - Email Markdown to HTML Conversion

## Task
1. Email worker should convert markdown to HTML (not send raw markdown)
2. Update test to assert HTML/text conversion

## Current State Analysis

### Email Worker (`internal/queue/workers/email_worker.go`)
The worker **already has markdown-to-HTML conversion** implemented:

1. **Line 263-264**: `body_from_tag` path already calls `convertMarkdownToHTML()`
```go
textBody = doc.ContentMarkdown
htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
```

2. **Lines 282-323**: `convertMarkdownToHTML()` uses goldmark library to convert markdown
3. **Lines 336-387**: `wrapInEmailTemplate()` wraps HTML in styled email template
4. **Lines 166-172**: `SendHTMLEmail` is called when htmlBody is not empty

### Mailer Service (`internal/services/mailer/service.go`)
Also **already supports HTML emails**:

1. **Lines 174-201**: Creates multipart MIME message with HTML and text parts
2. Uses base64 encoding to handle large HTML content safely

## Finding: Code is Already Correct

The email worker and mailer service **already convert markdown to HTML and send proper HTML emails**.

The issue might be one of:
1. The test doesn't verify HTML conversion is happening
2. Some edge case where conversion isn't triggered

## Recommendation

### Task 1: Verify Email Worker
No code changes needed - the conversion is already implemented and working.

### Task 2: Update Test
Add assertions to `test/ui/job_definition_web_search_asx_test.go` to verify:
1. The email step was executed
2. The email contained HTML content (via job logs or API verification)

Since this is an E2E UI test, we should verify via job logs that indicate HTML was sent.

## Files to Modify
1. `test/ui/job_definition_web_search_asx_test.go` - Add assertion for HTML email

## Anti-Creation Compliance
- No new files created
- Extending existing test file
- No changes to production code (already correct)
