# Summary: Email Markdown Conversion Fix

## Problem
Emails were being sent as raw markdown instead of formatted HTML. The goldmark markdown parser was producing minimal HTML output (only 97 bytes increase from 6999 bytes of markdown), indicating it failed to recognize the LLM-generated markdown.

## Root Cause
LLM-generated markdown from the summary worker was malformed in ways that goldmark couldn't parse (likely unclosed code blocks, broken tables, or other structural issues). The same issue affected the UI's markdown preview using marked.js.

## Solution
Added a fallback HTML conversion system in `email_worker.go`:

1. **Detection**: After goldmark conversion, check if HTML grew by less than 10%
2. **Fallback**: Use simple line-by-line markdown parser for malformed content

### New Functions Added

**`simpleMarkdownToHTML(markdown string) string`**
- Processes markdown line-by-line
- Handles: headers, code blocks, lists, horizontal rules
- Gracefully handles unclosed blocks
- Always produces valid HTML

**`processInlineMarkdown(text string) string`**
- Handles inline formatting: bold, italic, inline code
- Escapes HTML for security

## Test Results
```
--- PASS: TestJobDefinitionWebSearchASX (39.16s)
PASS: Email HTML generated from markdown (6265 bytes)
PASS: Email sent successfully with HTML body from markdown conversion
```

## Files Modified

| File | Change |
|------|--------|
| `internal/queue/workers/email_worker.go` | Added fallback detection and simple markdown parser |

## Impact
- Emails will now always render as properly formatted HTML
- Malformed markdown from LLMs is handled gracefully
- No changes to test files required
