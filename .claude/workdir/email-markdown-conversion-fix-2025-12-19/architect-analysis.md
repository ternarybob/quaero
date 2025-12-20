# ARCHITECT ANALYSIS - Email Markdown Conversion Fix

## Problem Statement

User reports:
1. Last 3 emails were sent as raw markdown (not HTML)
2. `doc_1a57bd18-52e3-4d53-bbf3-870be1aec2df` does NOT show as markdown preview (shows raw)
3. `doc_7f5dedcc-92bd-4727-875f-3c68a2422707` DOES show as markdown preview correctly

## Log Analysis

From the logs, the email worker IS converting markdown to HTML:

```
markdown_len=6999
html_len=7096   (only 97 bytes increase!)
final_len=9111  (after template wrap adds ~2000 bytes)
has_html=true
```

The problem: **goldmark only added 97 bytes** when converting 6999 bytes of markdown.

This means goldmark is treating most of the content as plain text (not markdown).

## Root Cause Hypothesis

The LLM-generated markdown from the summary worker is malformed. Possible issues:

1. **Unbalanced code blocks** - Missing closing backticks cause entire document to be treated as code
2. **Broken tables** - Pipe characters or formatting errors
3. **Mixed line endings** - CRLF vs LF confusion
4. **HTML-like content** - Content that goldmark escapes rather than parses

The UI also uses `marked.js` for rendering (documents.html:447-452), and the user reports the same document doesn't render there either - confirming it's a malformed markdown issue, not a goldmark-specific problem.

## Verification Needed

Need to examine actual content of:
- `doc_1a57bd18-52e3-4d53-bbf3-870be1aec2df` (bad rendering)
- `doc_7f5dedcc-92bd-4727-875f-3c68a2422707` (good rendering)

Compare structure to identify what makes one parse correctly and the other fail.

## Potential Fixes

### Option 1: Pre-process Markdown Before Conversion
Add markdown sanitization in email_worker before goldmark conversion:
- Fix unbalanced code blocks (count backticks)
- Ensure tables have proper formatting
- Normalize line endings

### Option 2: Update Summary Prompt to Enforce Valid Markdown
Modify summary_worker prompt to explicitly require:
- Complete code blocks (always close with matching backticks)
- Valid table formatting
- No raw HTML

### Option 3: Use Fallback HTML Generation
If goldmark produces minimal HTML (low increase ratio), fall back to a simpler formatting:
- Convert `#` headers to `<h1>`, `<h2>`, etc.
- Convert `**bold**` to `<strong>`
- Preserve line breaks with `<br>`

## Recommended Approach

**Option 3** is the most robust - it handles malformed markdown gracefully without requiring LLM prompt changes.

Detect when goldmark's output is suspiciously small compared to input, and apply a simple line-by-line conversion.

## Files to Modify

| File | Change |
|------|--------|
| `internal/queue/workers/email_worker.go` | Add markdown sanitization or fallback HTML generation |

## Anti-Creation Compliance

- **EXTEND > MODIFY**: Modifying existing `convertMarkdownToHTML` function
- **No new files**: All changes within existing email_worker.go
