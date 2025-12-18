# Architect Analysis: Convert Markdown to HTML in Email

## Problem

Email sends raw markdown text instead of rendered HTML. User wants emails to look like the formatted document in the UI.

## Current Behavior

In `email_worker.go:246-247`:
```go
if doc.ContentMarkdown != "" {
    textBody = doc.ContentMarkdown  // Sends raw markdown as plain text
}
```

## Solution

Convert markdown to HTML before sending email. Use `goldmark` - the most popular Go markdown library.

## Implementation

1. **Add goldmark dependency** to go.mod
2. **Create markdown-to-HTML helper** in email_worker.go
3. **Modify resolveBody()** to convert markdown content to HTML
4. **Wrap HTML in email template** with basic styling

## Existing Patterns

The codebase already uses `github.com/JohannesKaufmann/html-to-markdown` for HTML→Markdown.
For Markdown→HTML, `goldmark` is the standard choice.

## Anti-Creation Verification

| Action | Type | Justification |
|--------|------|---------------|
| Modify email_worker.go | MODIFY | Add markdown-to-HTML conversion |
| Add goldmark dependency | EXTEND | Standard library for markdown rendering |

## HTML Email Template

Wrap converted HTML in a styled template:
```html
<!DOCTYPE html>
<html>
<head>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 800px; margin: 0 auto; padding: 20px; }
    h1, h2, h3 { color: #1a1a1a; }
    ul, ol { padding-left: 20px; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
    pre { background: #f4f4f4; padding: 16px; border-radius: 6px; overflow-x: auto; }
  </style>
</head>
<body>
  {{.Content}}
</body>
</html>
```
