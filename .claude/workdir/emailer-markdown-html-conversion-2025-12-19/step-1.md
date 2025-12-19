# Step 1: Fix MIME Encoding in Mailer Service

## Problem

When sending large HTML emails (e.g., investment analysis reports), the HTML content was being corrupted or displayed as raw markdown. This is due to:

1. **Static MIME boundary** - Could conflict with content
2. **Missing Content-Transfer-Encoding** - Large HTML content violates RFC 5322 line length limits (998 chars max)
3. **No encoding of content** - Special characters and long lines could be mangled by mail servers

## Solution

Modified `internal/services/mailer/service.go` to:

1. **Generate unique MIME boundary** using `crypto/rand` for each email
2. **Add Content-Transfer-Encoding: base64** header for both text and HTML parts
3. **Base64 encode content** with 76-character line breaks per RFC 2045

## Changes Made

### 1. Added imports
```go
import (
    "crypto/rand"
    "encoding/base64"
    // ... existing imports
)
```

### 2. Updated SendHTMLEmail() multipart construction
```go
// Before:
boundary := "boundary123456789"
msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
msg.WriteString(htmlBody)

// After:
boundary := generateBoundary()  // Unique per message
msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
msg.WriteString("Content-Transfer-Encoding: base64\r\n")
msg.WriteString(encodeBase64WithLineBreaks(htmlBody))
```

### 3. Added helper functions
```go
// generateBoundary creates a unique MIME boundary using crypto/rand
func generateBoundary() string

// encodeBase64WithLineBreaks encodes content with 76-char line breaks per RFC 2045
func encodeBase64WithLineBreaks(content string) string
```

## Build Verification

```
[0;32mMain executable built: /mnt/c/development/quaero/bin/quaero.exe[0m
[0;32mMCP server built: /mnt/c/development/quaero/bin/quaero-mcp/quaero-mcp.exe[0m
```

## Why This Fixes the Issue

### Before
- Long HTML lines (e.g., inline CSS, table data) exceeded mail server limits
- Some servers would wrap lines incorrectly, breaking HTML structure
- Content might appear as raw text/markdown

### After
- Base64 encoding produces predictable 76-char lines
- All special characters safely encoded
- Unique boundary prevents content collision
- Email clients properly decode and render HTML

## Anti-Creation Compliance

- **EXTEND**: Extended existing mailer service functionality
- **MODIFY**: Modified `SendHTMLEmail()` method
- **CREATE**: Only added helper functions (no new files)
