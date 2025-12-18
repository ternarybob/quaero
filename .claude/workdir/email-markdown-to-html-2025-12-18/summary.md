# Summary: Convert Markdown to HTML in Email

## Issue

Emails sent raw markdown text instead of formatted HTML. User wanted emails to look like the beautifully rendered document shown in the Quaero UI.

## Solution

Added markdown-to-HTML conversion using `goldmark` library in the email worker.

## Changes Made

### 1. Added goldmark dependency
- `github.com/yuin/goldmark` - Standard Go markdown library with GFM support

### 2. Modified `internal/queue/workers/email_worker.go`

Added two new functions:

**`convertMarkdownToHTML()`** - Converts markdown to HTML using goldmark with:
- GitHub Flavored Markdown extensions (tables, strikethrough, etc.)
- Hard line breaks
- XHTML compliant output

**`wrapInEmailTemplate()`** - Wraps HTML in a professional email template with:
- Clean typography (system fonts, proper line-height)
- Styled headings (h1, h2, h3 with colors and spacing)
- Formatted lists with proper indentation
- Code blocks with monospace font and background
- Table styling with borders
- Blockquote styling
- Light gray background with white content card
- Responsive design
- Footer attribution

### 3. Updated `resolveBody()` to convert markdown
Both `body_from_document` and `body_from_tag` now convert markdown to HTML.

## Build Verification

- Main build: PASS
- MCP server: PASS

## Testing

Restart the server and re-run the "Web Search: ASX:GNP Company Info" job. The email should now contain a beautifully formatted HTML email that looks similar to the Quaero UI document viewer.

## Email Preview

The email will have:
- White content card on light gray background
- Proper heading hierarchy with styling
- Bulleted/numbered lists with indentation
- Bold text highlighted
- Professional footer
