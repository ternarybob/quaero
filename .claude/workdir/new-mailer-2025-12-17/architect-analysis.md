# Architect Analysis: Mailer Integration

## Requirements Summary

1. **Add mailer service** - Send emails using user's Gmail/SMTP credentials
2. **Job integration** - Jobs like `web-search-asx.toml` can send results via email
3. **Credential storage** - Store mail credentials in database, NOT cleared by `reset_on_startup`
4. **Settings UI** - Secure credential management page

---

## CODEBASE ANALYSIS

### Existing Code to EXTEND (Not Create)

| Need | Existing Code | Extension Strategy |
|------|---------------|-------------------|
| Credential Storage | `KeyValueStorage` interface | Store SMTP creds as KV pairs with `smtp_` prefix |
| Service Pattern | `internal/services/kv/service.go` | Follow exact pattern for mailer service |
| Settings UI | `pages/settings.html` + partials | Add `settings-mail.html` partial + nav item |
| Job Type | `WorkerType` in `models/job_definition.go` | Add `WorkerTypeEmail` type |
| Handler Pattern | `internal/handlers/kv_handler.go` | Follow for mailer handler |
| DI/App Integration | `internal/app/app.go` | Add mailer service to App struct |

### Key Discovery: `reset_on_startup` Exemption

The `reset_on_startup` flag in `quaero.toml` deletes the **entire BadgerDB directory**.
There's NO selective exemption mechanism.

**Solution:** Store mail credentials in a **separate config file** (TOML) that persists outside the database:
- Location: `bin/variables/mail_config.toml`
- Pattern: Same as `bin/variables/variables.toml` for API keys
- Loaded at startup, NOT subject to database reset

---

## ARCHITECTURE DECISION

### Approach: EXTEND Existing Patterns

**Priority: EXTEND > MODIFY > CREATE** (per refactoring skill)

1. **EXTEND `variables.toml`** - Add mail credentials section (persists outside DB)
2. **EXTEND `KeyValueStorage`** - Mirror credentials to KV for runtime access
3. **EXTEND `settings.html`** - Add mail settings partial
4. **CREATE minimal mailer service** - Only because no notification service exists
5. **EXTEND job definition types** - Add `email` step type

### Files to Modify (NOT Create Where Possible)

| File | Action | Reason |
|------|--------|--------|
| `bin/variables/variables.toml` | EXTEND | Add mail credentials section |
| `internal/models/job_definition.go` | EXTEND | Add `WorkerTypeEmail` |
| `internal/services/config/config_service.go` | EXTEND | Load mail config |
| `pages/settings.html` | EXTEND | Add mail nav item |
| `internal/app/app.go` | EXTEND | Wire mailer service |
| `internal/handlers/routes.go` | EXTEND | Add mailer routes |

| File | Action | Reason |
|------|--------|--------|
| `internal/services/mailer/service.go` | CREATE | No existing notification service |
| `internal/handlers/mailer_handler.go` | CREATE | No existing mail handler |
| `pages/partials/settings-mail.html` | CREATE | No existing mail settings UI |
| `internal/queue/workers/email_worker.go` | CREATE | No existing email worker |

---

## DETAILED IMPLEMENTATION PLAN

### Phase 1: Credential Storage (Persists Outside DB)

**File: `bin/variables/variables.toml`** (EXTEND)
```toml
# Mail Configuration
# These credentials persist across database resets
[mail]
smtp_host = ""
smtp_port = "587"
smtp_username = ""
smtp_password = ""
from_email = ""
from_name = "Quaero"
use_tls = "true"
```

**Why variables.toml?**
- Already exists for storing API keys
- NOT subject to `reset_on_startup` (separate from BadgerDB)
- Loaded at startup by config service
- Can be injected into jobs via `{mail_from_email}` syntax

### Phase 2: Mailer Service (Minimal Creation)

**File: `internal/services/mailer/service.go`** (CREATE)

Following EXACT pattern from `internal/services/kv/service.go`:

```go
package mailer

type Service struct {
    configService interfaces.ConfigService  // For reading mail config
    logger        arbor.ILogger
}

func NewService(configSvc interfaces.ConfigService, logger arbor.ILogger) *Service

func (s *Service) SendEmail(ctx context.Context, to, subject, body string) error
func (s *Service) SendHTMLEmail(ctx context.Context, to, subject, htmlBody, textBody string) error
func (s *Service) GetConfig() (*MailConfig, error)
func (s *Service) SetConfig(ctx context.Context, config *MailConfig) error
```

### Phase 3: Email Worker (For Job Definitions)

**File: `internal/queue/workers/email_worker.go`** (CREATE)

Following pattern from `internal/queue/workers/agent_worker.go`:

```go
package workers

type EmailWorker struct {
    mailerService *mailer.Service
    logger        arbor.ILogger
}

func NewEmailWorker(mailerSvc *mailer.Service, logger arbor.ILogger) *EmailWorker

// Process handles email step in job definitions
func (w *EmailWorker) Process(ctx context.Context, job *models.QueueJobState) error
```

**Job Definition Usage:**
```toml
[step.send_results_email]
type = "email"
depends = "summarize_results"
to = "user@example.com"
subject = "ASX:GNP Analysis Complete"
body_from_step = "summarize_results"  # Use output from previous step
```

### Phase 4: Settings UI

**File: `pages/partials/settings-mail.html`** (CREATE)

Following pattern from `pages/partials/settings-kv.html`:

```html
<div x-data="mailSettings()" x-init="loadConfig()">
    <div class="card">
        <div class="card-header">
            <h3>Email Configuration</h3>
        </div>
        <div class="card-content">
            <!-- SMTP Settings Form -->
            <div class="field">
                <label class="label">SMTP Host</label>
                <input class="input" x-model="config.smtp_host" />
            </div>
            <!-- ... more fields -->
            <button class="button is-primary" @click="saveConfig()">Save</button>
            <button class="button" @click="testEmail()">Send Test Email</button>
        </div>
    </div>
</div>
```

**Modify: `pages/settings.html`**
- Add nav item for "Email" section
- Include `settings-mail.html` partial

### Phase 5: Handler & Routes

**File: `internal/handlers/mailer_handler.go`** (CREATE)

Following pattern from `internal/handlers/kv_handler.go`:

```go
type MailerHandler struct {
    mailerService *mailer.Service
    logger        arbor.ILogger
}

func (h *MailerHandler) GetConfigHandler(w http.ResponseWriter, r *http.Request)
func (h *MailerHandler) SetConfigHandler(w http.ResponseWriter, r *http.Request)
func (h *MailerHandler) SendTestHandler(w http.ResponseWriter, r *http.Request)
```

**Modify: Route Registration** (in server setup)
```go
mux.HandleFunc("/api/mail/config", mailHandler.GetConfigHandler)
mux.HandleFunc("/api/mail/test", mailHandler.SendTestHandler)
```

---

## ANTI-CREATION JUSTIFICATION

| New File | Justification |
|----------|---------------|
| `mailer/service.go` | No existing notification/mail service exists |
| `mailer_handler.go` | No existing mail handler exists |
| `settings-mail.html` | No existing mail settings UI exists |
| `email_worker.go` | No existing email job worker exists |

**Patterns Followed:**
- Service structure from `kv/service.go`
- Handler structure from `kv_handler.go`
- Worker structure from `agent_worker.go`
- Settings UI structure from `settings-kv.html`

---

## CREDENTIAL PERSISTENCE SOLUTION

**Problem:** `reset_on_startup = true` deletes BadgerDB including credentials.

**Solution:** Store credentials in `variables.toml` (file-based, not database):

1. `bin/variables/variables.toml` persists across restarts
2. Config service already loads this file at startup
3. Variables can be referenced in jobs as `{smtp_username}`, `{smtp_password}`
4. Settings UI saves to this file via config service

**Implementation:**
- Add `WriteVariables()` method to config service
- Handler uses config service to read/write mail settings
- File-based storage survives `reset_on_startup`

---

## BUILD VERIFICATION

Build command: `./scripts/build.sh`

**Dependencies to Add:**
- `net/smtp` (Go standard library - no external dependency)

**No New External Dependencies Required**

---

## SUMMARY

This implementation:
1. **EXTENDS** existing patterns wherever possible
2. **CREATES** only what's absolutely necessary (mailer service, worker, handler, UI)
3. **SOLVES** credential persistence via file-based storage (variables.toml)
4. **FOLLOWS** all existing codebase patterns

**Files to Create:** 4 (service, handler, worker, settings partial)
**Files to Modify:** 5 (variables.toml, job_definition.go, settings.html, app.go, routes)
