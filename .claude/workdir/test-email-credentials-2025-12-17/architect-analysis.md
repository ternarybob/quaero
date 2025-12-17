# Architect Analysis: Email Credentials Test

## Task Summary

Create a UI test for adding email credentials via the Settings page Email section.

## Existing Code Analysis

### Relevant Files

| File | Purpose |
|------|---------|
| `test/ui/settings_test.go` | Existing settings page test pattern to EXTEND |
| `internal/services/mailer/service.go` | Mailer service (Config struct, SetConfig, GetConfig) |
| `internal/handlers/mailer_handler.go` | HTTP handlers (GetConfigHandler, SetConfigHandler, SendTestHandler) |
| `pages/partials/settings-mail.html` | UI form for email configuration |
| `internal/server/routes.go` | Routes: `/api/mail/config`, `/api/mail/test` |

### API Endpoints

- `GET /api/mail/config` - Get current mail configuration (password masked)
- `POST /api/mail/config` - Save mail configuration
- `POST /api/mail/test` - Send test email (requires configured settings)

### UI Elements (from settings-mail.html)

| Field ID | Description |
|----------|-------------|
| `smtp_host` | SMTP server hostname |
| `smtp_port` | SMTP port (default 587) |
| `smtp_username` | Email address/username |
| `smtp_password` | Password (masked in GET response) |
| `smtp_from` | From email address |
| `smtp_from_name` | From display name (default "Quaero") |
| `smtp_use_tls` | TLS checkbox |

### Navigation

The Email section is accessed via:
- Settings page (`/settings`)
- Nav item: `<a href="#" @click.prevent="selectSection('mail')">Email</a>`
- Section identifier: `mail`

## Implementation Decision

### EXTEND Existing Pattern

The test will EXTEND the pattern from `settings_test.go`:

1. **Setup** - Use `common.SetupTestEnvironment()`
2. **Navigation** - Navigate to `/settings`, click Email nav item
3. **Form Fill** - Fill SMTP configuration fields
4. **Save** - Submit form
5. **Verify** - Check configured status badge

### Test File Location

```
test/ui/settings_mail_test.go
```

**Justification:** Follows naming convention of other UI tests (`settings_test.go`, `logs_test.go`).

### Test Structure

```go
func TestSettingsMailConfiguration(t *testing.T) {
    // 1. Setup test environment
    // 2. Navigate to Settings > Email
    // 3. Fill in SMTP configuration (dummy values)
    // 4. Save configuration
    // 5. Verify "Email Configured" badge appears
    // 6. Verify form fields retained values
    // 7. Cleanup: Clear configuration
}
```

### Test Data

Use dummy SMTP values (won't actually connect):
- Host: `smtp.test.example.com`
- Port: `587`
- Username: `test@example.com`
- Password: `testpassword123`
- From: `test@example.com`
- From Name: `Quaero Test`
- Use TLS: `true`

### Verification Points

1. **Pre-save state**: "Email Not Configured" badge visible
2. **Post-save state**: "Email Configured" badge visible
3. **Form persistence**: Values retained after page reload
4. **Password masking**: Password field shows masked value (`********`)

## Anti-Creation Check

| Item | Existing Code | Action |
|------|---------------|--------|
| Test framework | `test/common/setup.go` | REUSE |
| Test patterns | `test/ui/settings_test.go` | EXTEND |
| New test file | None | CREATE (required for new feature) |

**Justification for CREATE:** No existing email configuration test exists. Must create new test file.

## Build Verification

Build command: `.\scripts\build.ps1` (Windows)

Test compilation: `go build ./test/...`
