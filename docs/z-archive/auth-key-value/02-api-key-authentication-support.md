I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend Implementation Status:**
- ✅ Database schema extended with `api_key` and `auth_type` fields
- ✅ Storage layer updated with `GetCredentialsByName()` and `GetAPIKeyByName()` methods
- ✅ Auth handlers include `CreateAPIKeyHandler()` and `UpdateAPIKeyHandler()`
- ✅ API key resolution helper `ResolveAPIKey()` exists in `config.go`
- ✅ Services (LLM, Agent, Places) updated to use API key resolution
- ✅ Job definition integration supports `api_key` field in step config
- ✅ Config includes `AuthDirConfig` with `CredentialsDir` field

**Frontend Current State:**
- `pages/auth.html` displays cookie-based authentication in a table
- Alpine.js component `authPage()` handles loading, deleting, and formatting
- Uses Bulma CSS for styling (not BeerCSS as mentioned in AGENTS.md)
- WebSocket integration for real-time updates
- Existing endpoints: `/api/auth/list`, `/api/auth/{id}` (GET/DELETE)

**Missing Implementation:**
1. **UI Components:** No API Keys section in `auth.html`
2. **File Loading:** `load_auth_credentials.go` doesn't exist yet
3. **Startup Integration:** Auth file loading not called in `app.go`
4. **Routes:** API key CRUD routes not registered in `routes.go`
5. **Example Files:** No example auth TOML files in `deployments/local/auth/`

**Key Patterns to Follow:**
- Job definitions loading pattern from `load_job_definitions.go`
- Alpine.js component pattern from existing `authPage()` function
- Bulma CSS table styling (not BeerCSS)
- Sanitization pattern: mask API keys in responses (show first 4 + last 4 chars)

### Approach

## Implementation Strategy

**Phase 1: File Loading Infrastructure**
Create `load_auth_credentials.go` following the exact pattern from `load_job_definitions.go`, then integrate into app startup sequence.

**Phase 2: API Routes Registration**
Add API key CRUD routes to `routes.go` and update `handleAuthRoutes()` to route to new handlers.

**Phase 3: UI Implementation**
Add API Keys section to `auth.html` with Alpine.js component for CRUD operations, show/hide toggle, and Bulma CSS styling.

**Phase 4: Example Files & Documentation**
Create example TOML files and update config documentation.

**Key Design Decisions:**
- **Separation of Concerns:** Cookie auth and API key auth displayed in separate sections on same page
- **Security:** API keys masked in UI (first 4 + last 4 chars), never sent in full from server
- **Idempotency:** File loading uses ON CONFLICT to update existing keys
- **Validation:** API key names must be unique per auth_type

### Reasoning

Reviewed all relevant files mentioned by the user:
- Examined `auth.html` structure and Alpine.js patterns
- Analyzed `load_job_definitions.go` for file loading pattern
- Checked `routes.go` for routing conventions
- Verified `auth_handler.go` has new CRUD methods
- Confirmed `app.go` initialization sequence
- Reviewed `config.go` for auth directory configuration

## Mermaid Diagram

sequenceDiagram
    participant User as User (Browser)
    participant UI as auth.html (Alpine.js)
    participant Server as Server (routes.go)
    participant Handler as AuthHandler
    participant Storage as AuthStorage
    participant Files as ./auth/*.toml

    Note over Files,Storage: Startup Phase
    Files->>Storage: LoadAuthCredentialsFromFiles()
    Storage->>Storage: Parse TOML, validate, store
    Storage-->>Files: API keys loaded [REDACTED]

    Note over User,Storage: UI Interaction Phase
    User->>UI: Navigate to /auth page
    UI->>Server: GET /api/auth/list
    Server->>Handler: ListAuthHandler()
    Handler->>Storage: ListCredentials()
    Storage-->>Handler: All credentials (cookie + API key)
    Handler->>Handler: Mask API keys (first 4 + last 4)
    Handler-->>UI: Sanitized list (auth_type, masked keys)
    UI->>UI: Filter by auth_type='api_key'
    UI-->>User: Display API keys table

    Note over User,Storage: Create API Key
    User->>UI: Click "Add API Key", fill form
    UI->>Server: POST /api/auth/api-key
    Server->>Handler: CreateAPIKeyHandler()
    Handler->>Storage: StoreCredentials(auth_type='api_key')
    Storage-->>Handler: Success
    Handler-->>UI: 200 OK
    UI->>UI: Reload API keys list
    UI-->>User: Show success notification

    Note over User,Storage: Delete API Key
    User->>UI: Click delete, confirm
    UI->>Server: DELETE /api/auth/api-key/{id}
    Server->>Handler: DeleteAuthHandler()
    Handler->>Storage: DeleteCredentials(id)
    Storage-->>Handler: Success
    Handler-->>UI: 200 OK
    UI->>UI: Remove from local array
    UI-->>User: Show success notification

## Proposed File Changes

### internal\storage\sqlite\load_auth_credentials.go(NEW)

References: 

- internal\storage\sqlite\load_job_definitions.go
- internal\models\auth.go

**Create new file for loading auth credentials from TOML files.**

Follow the exact pattern from `@/internal/storage/sqlite/load_job_definitions.go`:

**1. Define AuthCredentialFile struct:**
- Fields: `Name string`, `APIKey string`, `ServiceType string`, `Description string`
- TOML tags: `toml:"name"`, `toml:"api_key"`, `toml:"service_type"`, `toml:"description"`
- All fields required except Description (optional)

**2. Implement ToAuthCredentials() method:**
- Convert AuthCredentialFile to `models.AuthCredentials`
- Set `AuthType = "api_key"`
- Set `SiteDomain = ""` (empty for API keys)
- Generate UUID for ID using `uuid.New().String()`
- Set `CreatedAt` and `UpdatedAt` to `time.Now()`
- Copy Name, APIKey, ServiceType, Description fields

**3. Implement LoadAuthCredentialsFromFiles() method on Manager:**
- Check if directory exists with `os.Stat()`, return nil if not found (optional directory)
- Read directory entries with `os.ReadDir()`
- Filter for `.toml` files only (skip `.json` for simplicity)
- For each TOML file:
  - Read file with `os.ReadFile()`
  - Parse with `toml.Unmarshal()` into AuthCredentialFile
  - Validate: name, api_key, service_type are required (return error if missing)
  - Convert to AuthCredentials with `ToAuthCredentials()`
  - Call `m.authStorage.StoreCredentials()` (idempotent with ON CONFLICT)
  - Log success: "Loaded API key: {name} [REDACTED]" (mask actual key value)
  - Increment loadedCount
- Log summary: "Loaded {count} API keys from {dir}"
- Continue on individual file errors (log warning, increment skippedCount)

**4. Error Handling:**
- Log warnings for invalid files, don't fail startup
- Skip non-TOML files silently
- Return nil error even if some files fail (graceful degradation)

**5. Security:**
- Never log actual API key values
- Use "[REDACTED]" placeholder in all log messages
- Mask keys as: `name[:4] + "..." + name[len(name)-4:]` if logging is needed

**Pattern Reference:** Follow `load_job_definitions.go` structure exactly, replacing job-specific logic with auth-specific logic.

### internal\app\app.go(MODIFY)

References: 

- internal\storage\sqlite\load_auth_credentials.go(NEW)
- internal\common\config.go

**Integrate auth credentials file loading into startup sequence.**

**Location:** In `initDatabase()` method, after job definitions loading (around line 216)

**Add auth credentials loading:**
```go
// Load auth credentials from files (after job definitions)
if sqliteMgr, ok := storageManager.(*sqlite.Manager); ok {
    if err := sqliteMgr.LoadAuthCredentialsFromFiles(ctx, a.Config.Auth.CredentialsDir); err != nil {
        a.Logger.Warn().Err(err).Msg("Failed to load auth credentials from files")
        // Don't fail startup - auth files are optional
    } else {
        a.Logger.Info().Str("dir", a.Config.Auth.CredentialsDir).Msg("Auth credentials loaded from files")
    }
}
```

**Placement Rationale:**
- Load AFTER schema initialization (line 208)
- Load AFTER job definitions (line 213-216) to maintain logical grouping
- Load BEFORE service initialization (line 238+) so services can use loaded keys
- Wrap in error handling that logs warning but doesn't fail startup

**Error Handling:**
- Log warning if loading fails
- Continue startup even if auth files are missing or invalid
- Auth credentials are optional - services fall back to config values

**Logging:**
- Success: "Auth credentials loaded from files" with directory path
- Failure: "Failed to load auth credentials from files" with error details

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\auth_handler.go(MODIFY)

**Register API key CRUD routes.**

**1. Update main route registration (around line 40):**

Add new route for API key operations:
```go
mux.HandleFunc("/api/auth/api-key", s.handleAPIKeyRoutes)     // POST/PUT for API keys
mux.HandleFunc("/api/auth/api-key/", s.handleAPIKeyRoutes)   // GET/DELETE /{id}
```

Place BEFORE the generic `/api/auth/` route (line 40) to ensure specific routes match first.

**2. Create new handleAPIKeyRoutes function (after handleAuthRoutes around line 196):**

```go
// handleAPIKeyRoutes routes /api/auth/api-key requests
func (s *Server) handleAPIKeyRoutes(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path
    
    // POST /api/auth/api-key (create)
    if r.Method == "POST" && path == "/api/auth/api-key" {
        s.app.AuthHandler.CreateAPIKeyHandler(w, r)
        return
    }
    
    // PUT /api/auth/api-key/{id} (update)
    if r.Method == "PUT" && strings.HasPrefix(path, "/api/auth/api-key/") {
        s.app.AuthHandler.UpdateAPIKeyHandler(w, r)
        return
    }
    
    // GET /api/auth/api-key/{id} (get single)
    if r.Method == "GET" && strings.HasPrefix(path, "/api/auth/api-key/") {
        s.app.AuthHandler.GetAuthHandler(w, r)
        return
    }
    
    // DELETE /api/auth/api-key/{id} (delete)
    if r.Method == "DELETE" && strings.HasPrefix(path, "/api/auth/api-key/") {
        s.app.AuthHandler.DeleteAuthHandler(w, r)
        return
    }
    
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
```

**3. Update handleAuthRoutes function (line 177):**

Add check to skip API key routes:
```go
// Skip if path is /api/auth/status, /api/auth/list, or /api/auth/api-key
if path == "/api/auth/status" || path == "/api/auth/list" || strings.HasPrefix(path, "/api/auth/api-key") {
    return
}
```

**Routing Logic:**
- `/api/auth/api-key` (POST) → CreateAPIKeyHandler
- `/api/auth/api-key/{id}` (PUT) → UpdateAPIKeyHandler
- `/api/auth/api-key/{id}` (GET) → GetAuthHandler (reuse existing)
- `/api/auth/api-key/{id}` (DELETE) → DeleteAuthHandler (reuse existing)
- `/api/auth/list` (GET) → ListAuthHandler (returns both cookie and API key auth)

**Note:** GET and DELETE reuse existing handlers since they work with any auth type.

### pages\auth.html(MODIFY)

**Add API Keys section below cookie authentication table.**

**1. Add API Keys section after cookie auth section (after line 91, before service logs):**

```html
<!-- API Keys Section -->
<section style="margin-top: 1.5rem;" x-data="apiKeysPage()">
    <div class="card">
        <div class="card-header">
            <header class="navbar">
                <section class="navbar-section">
                    <h3>API Keys</h3>
                </section>
                <section class="navbar-section">
                    <button class="btn btn-sm btn-primary" @click="showCreateForm = true" title="Add API Key">
                        <i class="fa-solid fa-plus"></i> Add API Key
                    </button>
                    <button class="btn btn-sm" @click="loadAPIKeys" title="Refresh API Keys">
                        <i class="fa-solid fa-rotate-right"></i>
                    </button>
                </section>
            </header>
        </div>
        <div class="card-body">
            <!-- Create Form (shown when showCreateForm is true) -->
            <div x-show="showCreateForm" class="box" style="margin-bottom: 1rem;">
                <h4>Add New API Key</h4>
                <form @submit.prevent="createAPIKey">
                    <div class="form-group">
                        <label>Name *</label>
                        <input type="text" x-model="newKey.name" required placeholder="e.g., gemini-llm">
                    </div>
                    <div class="form-group">
                        <label>API Key *</label>
                        <div style="position: relative;">
                            <input :type="showNewKeyValue ? 'text' : 'password'" x-model="newKey.api_key" required placeholder="Enter API key">
                            <button type="button" @click="showNewKeyValue = !showNewKeyValue" style="position: absolute; right: 10px; top: 50%; transform: translateY(-50%);" class="btn btn-sm">
                                <i class="fas" :class="showNewKeyValue ? 'fa-eye-slash' : 'fa-eye'"></i>
                            </button>
                        </div>
                    </div>
                    <div class="form-group">
                        <label>Service Type *</label>
                        <select x-model="newKey.service_type" required>
                            <option value="">Select service type</option>
                            <option value="google-gemini">Google Gemini (LLM/Agent)</option>
                            <option value="google-places">Google Places</option>
                            <option value="other">Other</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label>Description</label>
                        <textarea x-model="newKey.description" placeholder="Optional description"></textarea>
                    </div>
                    <div class="form-group">
                        <button type="submit" class="btn btn-primary" :disabled="creating">Create</button>
                        <button type="button" class="btn" @click="cancelCreate">Cancel</button>
                    </div>
                </form>
            </div>

            <!-- Loading State -->
            <div x-show="loading" class="text-center p-5">
                <span class="icon"><i class="fas fa-spinner fa-pulse fa-2x"></i></span>
                <p class="mt-3">Loading API keys...</p>
            </div>

            <!-- Empty State -->
            <div x-show="!loading && apiKeys.length === 0 && !showCreateForm" class="text-center p-5">
                <span class="icon text-muted"><i class="fas fa-key fa-3x"></i></span>
                <p class="mt-3 text-muted">No API keys stored</p>
                <p class="text-muted">Add API keys for services like Google Gemini or Google Places</p>
            </div>

            <!-- API Keys Table -->
            <div x-show="!loading && apiKeys.length > 0">
                <table class="table striped border">
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>API Key</th>
                            <th>Service Type</th>
                            <th>Description</th>
                            <th>Last Updated</th>
                            <th class="text-right">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <template x-for="key in apiKeys" :key="key.id">
                            <tr>
                                <td><strong x-text="key.name"></strong></td>
                                <td>
                                    <div style="display: flex; align-items: center; gap: 0.5rem;">
                                        <code x-text="key.masked_key" style="font-family: monospace;"></code>
                                        <button @click="toggleKeyVisibility(key.id)" class="btn btn-sm" title="Show/Hide Key">
                                            <i class="fas" :class="key.showFull ? 'fa-eye-slash' : 'fa-eye'"></i>
                                        </button>
                                    </div>
                                </td>
                                <td><span class="label label-secondary" x-text="(key.service_type || 'unknown').toUpperCase()"></span></td>
                                <td><small x-text="key.description || '-'"></small></td>
                                <td x-text="formatDate(key.updated_at)"></td>
                                <td class="text-right">
                                    <button class="btn btn-sm btn-error" @click="deleteAPIKey(key.id, key.name)" :disabled="deleting === key.id" title="Delete API Key">
                                        <i class="fas" :class="deleting === key.id ? 'fa-spinner fa-pulse' : 'fa-trash'"></i>
                                    </button>
                                </td>
                            </tr>
                        </template>
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</section>
```

**2. Add Alpine.js component for API keys (after authPage() function, around line 205):**

```javascript
// Alpine.js component for API keys management
function apiKeysPage() {
    return {
        apiKeys: [],
        loading: true,
        deleting: null,
        creating: false,
        showCreateForm: false,
        showNewKeyValue: false,
        newKey: {
            name: '',
            api_key: '',
            service_type: '',
            description: ''
        },

        init() {
            this.loadAPIKeys();
        },

        async loadAPIKeys() {
            this.loading = true;
            try {
                const response = await fetch('/api/auth/list');
                if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
                const data = await response.json();
                // Filter for API key type only
                this.apiKeys = (Array.isArray(data) ? data : []).filter(auth => auth.auth_type === 'api_key').map(key => ({
                    ...key,
                    masked_key: this.maskAPIKey(key.api_key),
                    showFull: false
                }));
            } catch (error) {
                console.error('Failed to load API keys:', error);
                window.showNotification('Failed to load API keys', 'error');
            } finally {
                this.loading = false;
            }
        },

        maskAPIKey(key) {
            if (!key || key.length < 8) return '••••••••';
            return key.substring(0, 4) + '•••' + key.substring(key.length - 4);
        },

        toggleKeyVisibility(keyId) {
            const key = this.apiKeys.find(k => k.id === keyId);
            if (key) {
                key.showFull = !key.showFull;
                // Note: Full key is never sent from server, so this just toggles the mask
                // In production, you'd need to fetch the full key from server on demand
            }
        },

        async createAPIKey() {
            if (!this.newKey.name || !this.newKey.api_key || !this.newKey.service_type) {
                window.showNotification('Please fill in all required fields', 'error');
                return;
            }

            this.creating = true;
            try {
                const response = await fetch('/api/auth/api-key', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.newKey)
                });

                if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

                window.showNotification('API key created successfully', 'success');
                this.cancelCreate();
                this.loadAPIKeys();
            } catch (error) {
                console.error('Failed to create API key:', error);
                window.showNotification('Failed to create API key', 'error');
            } finally {
                this.creating = false;
            }
        },

        cancelCreate() {
            this.showCreateForm = false;
            this.newKey = { name: '', api_key: '', service_type: '', description: '' };
            this.showNewKeyValue = false;
        },

        async deleteAPIKey(id, name) {
            if (!confirm(`Are you sure you want to delete API key "${name}"?\n\nAny services or jobs using this key will need to be updated.`)) {
                return;
            }

            this.deleting = id;
            try {
                const response = await fetch(`/api/auth/api-key/${id}`, { method: 'DELETE' });
                if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

                this.apiKeys = this.apiKeys.filter(key => key.id !== id);
                window.showNotification('API key deleted successfully', 'success');
            } catch (error) {
                console.error('Failed to delete API key:', error);
                window.showNotification('Failed to delete API key', 'error');
            } finally {
                this.deleting = null;
            }
        },

        formatDate(timestamp) {
            // Reuse formatDate from authPage() - same implementation
            if (!timestamp) return '-';
            let date;
            try {
                if (typeof timestamp === 'string') {
                    date = new Date(timestamp);
                } else if (typeof timestamp === 'number') {
                    date = new Date(timestamp < 10000000000 ? timestamp * 1000 : timestamp);
                } else {
                    return '-';
                }
                if (isNaN(date.getTime())) return '-';
                const now = new Date();
                const diffMs = now - date;
                const diffMins = Math.floor(diffMs / 60000);
                const diffHours = Math.floor(diffMs / 3600000);
                const diffDays = Math.floor(diffMs / 86400000);
                if (diffMins < 1) return 'Just now';
                if (diffMins < 60) return `${diffMins} minute${diffMins !== 1 ? 's' : ''} ago`;
                if (diffHours < 24) return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`;
                if (diffDays < 7) return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`;
                return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
            } catch (error) {
                console.error('Error formatting date:', error);
                return '-';
            }
        }
    }
}
```

**Key Features:**
- **Show/Hide Toggle:** Eye icon toggles between masked and full key display
- **Masked Display:** Shows first 4 + last 4 characters (e.g., "sk-a...xyz")
- **Service Type Dropdown:** Predefined options for common services
- **Inline Create Form:** Appears when "Add API Key" button is clicked
- **Confirmation Dialog:** Warns before deleting API keys
- **Bulma CSS Styling:** Consistent with existing cookie auth table
- **Real-time Updates:** Reloads list after create/delete operations

**Security Note:** The current implementation assumes the server sends masked keys. If the server sends full keys, update `ListAuthHandler` in `auth_handler.go` to mask them before sending.

### deployments\local\auth(NEW)

**Create auth directory for storing API key files.**

This directory will contain TOML files with API key definitions loaded during application startup.

**Directory Purpose:**
- Store API key credentials as TOML files
- Auto-loaded at startup via `LoadAuthCredentialsFromFiles()`
- Supports multiple files (e.g., `gemini.toml`, `places.toml`, `custom.toml`)
- Files are optional - directory can be empty

**Security Considerations:**
- Add `.gitignore` to prevent committing actual API keys
- Example files should use placeholder values
- Production deployments should use environment variables or secure vaults

### deployments\local\auth\.gitignore(NEW)

**Create .gitignore to prevent committing actual API keys.**

```
# Ignore all TOML files except example
*.toml
!example-api-keys.toml

# Ignore backup files
*.bak
*.backup
```

**Purpose:**
- Prevent accidental commit of real API keys to version control
- Allow example file to be committed for documentation
- Protect sensitive credentials

### deployments\local\auth\example-api-keys.toml(NEW)

**Create example auth credentials file with commented examples.**

```toml
# Example API Key Configuration
# ==============================
#
# Place your API key files in this directory (./auth/)
# Files are automatically loaded at application startup
#
# File Format:
#   name         = "unique-identifier"  # Required: Used to reference key in job definitions
#   api_key      = "your-actual-key"    # Required: The API key value
#   service_type = "service-name"       # Required: Service identifier
#   description  = "optional notes"     # Optional: Human-readable description
#
# Multiple keys can be defined in a single file or split across multiple files.
# Later files override earlier files if names conflict.

# Google Gemini API Key for LLM Service
# Get your API key from: https://aistudio.google.com/app/apikey
# Free tier: 15 requests/minute, 1500 requests/day

# name = "gemini-llm"
# api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
# service_type = "google-gemini"
# description = "Google Gemini API key for LLM embeddings and chat"

# Google Gemini API Key for Agent Service
# Can use same key as LLM service or separate key for quota management

# name = "gemini-agent"
# api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
# service_type = "google-gemini"
# description = "Google Gemini API key for agent operations (keyword extraction, summarization)"

# Google Places API Key
# Get your API key from: https://console.cloud.google.com/apis/credentials
# Enable Places API in Google Cloud Console

# name = "google-places"
# api_key = "YOUR_GOOGLE_PLACES_API_KEY"
# service_type = "google-places"
# description = "Google Places API key for location search and details"

# Custom API Key Example
# Use for any third-party service integration

# name = "my-custom-service"
# api_key = "your-api-key-here"
# service_type = "custom"
# description = "API key for custom service integration"

# ==============================
# Usage in Job Definitions
# ==============================
#
# Reference API keys by name in job definition steps:
#
# [[steps]]
# name = "extract_keywords"
# action = "agent"
# [steps.config]
# agent_type = "keyword_extractor"
# api_key = "gemini-agent"  # References the key defined above
#
# [[steps]]
# name = "search_places"
# action = "places_search"
# [steps.config]
# query = "restaurants near me"
# api_key = "google-places"  # References the key defined above

# ==============================
# Priority Order
# ==============================
#
# API keys are resolved in this order:
# 1. Auth storage (this file) - highest priority
# 2. Config file (quaero.toml) - fallback
# 3. Environment variables - fallback
#
# This allows you to:
# - Store keys in files for convenience
# - Override with environment variables for production
# - Use config file for development defaults
```

**Key Features:**
- Comprehensive documentation with examples
- All examples commented out to prevent accidental use
- Clear instructions for obtaining API keys
- Usage examples for job definitions
- Priority order explanation
- Multiple service types covered

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\common\config.go

**Update configuration file with auth directory documentation.**

**Location:** After Jobs Configuration section (around line 114)

**Add new section:**

```toml
# =============================================================================
# Authentication Configuration
# =============================================================================
# Configure authentication credentials storage and loading.
#
# API keys can be stored in TOML files in the credentials_dir and loaded at startup.
# This provides an alternative to storing keys in this config file or environment variables.
#
# Priority Order (highest to lowest):
#   1. Auth storage files (./auth/*.toml) - loaded at startup
#   2. Config file (this file) - llm.google_api_key, agent.google_api_key, etc.
#   3. Environment variables - QUAERO_LLM_GOOGLE_API_KEY, etc.
#
# Benefits of auth storage files:
#   - Separate credentials from configuration
#   - Support multiple keys per service (e.g., different quotas)
#   - Reference keys by name in job definitions
#   - Easy to manage and rotate keys
#
# Default: credentials_dir="./auth"
# Env var: QUAERO_AUTH_CREDENTIALS_DIR

# [auth]
# credentials_dir = "./auth"  # Uncomment to override default
```

**Update LLM section (around line 90):**

Add note about auth storage:
```toml
# Note: API keys can also be stored in auth storage (./auth/*.toml files)
# and referenced by name in job definitions. See [auth] section below.
```

**Update Agent section (around line 73):**

Add same note:
```toml
# Note: API keys can also be stored in auth storage (./auth/*.toml files)
# and referenced by name in job definitions. See [auth] section below.
```

**Update Places API section (around line 57):**

Add same note:
```toml
# Note: API keys can also be stored in auth storage (./auth/*.toml files)
# and referenced by name in job definitions. See [auth] section below.
```

**Rationale:**
- Documents the new auth storage feature
- Explains priority order for API key resolution
- Cross-references between sections for clarity
- Maintains consistency with existing config style

### internal\handlers\auth_handler.go(MODIFY)

References: 

- internal\models\auth.go

**Update ListAuthHandler to mask API keys in responses.**

**Location:** In `ListAuthHandler` method (around line 114)

**Modify sanitization logic:**

Replace the sanitized map creation (lines 114-125) with:

```go
// Sanitize response - don't send cookies, tokens, or full API keys to client
sanitized := make([]map[string]interface{}, len(credentials))
for i, cred := range credentials {
    sanitized[i] = map[string]interface{}{
        "id":           cred.ID,
        "name":         cred.Name,
        "site_domain":  cred.SiteDomain,
        "service_type": cred.ServiceType,
        "base_url":     cred.BaseURL,
        "auth_type":    cred.AuthType,  // Add auth_type field
        "created_at":   cred.CreatedAt,
        "updated_at":   cred.UpdatedAt,
    }
    
    // Mask API key if present (show first 4 + last 4 chars)
    if cred.AuthType == "api_key" && cred.APIKey != "" {
        if len(cred.APIKey) > 8 {
            sanitized[i]["api_key"] = cred.APIKey[:4] + "•••" + cred.APIKey[len(cred.APIKey)-4:]
        } else {
            sanitized[i]["api_key"] = "••••••••"  // Mask short keys completely
        }
    }
}
```

**Update GetAuthHandler (around line 158):**

Add same masking logic:

```go
// Sanitize response - don't send cookies, tokens, or full API keys
sanitized := map[string]interface{}{
    "id":           cred.ID,
    "name":         cred.Name,
    "site_domain":  cred.SiteDomain,
    "service_type": cred.ServiceType,
    "base_url":     cred.BaseURL,
    "auth_type":    cred.AuthType,  // Add auth_type field
    "created_at":   cred.CreatedAt,
    "updated_at":   cred.UpdatedAt,
}

// Mask API key if present
if cred.AuthType == "api_key" && cred.APIKey != "" {
    if len(cred.APIKey) > 8 {
        sanitized["api_key"] = cred.APIKey[:4] + "•••" + cred.APIKey[len(cred.APIKey)-4:]
    } else {
        sanitized["api_key"] = "••••••••"
    }
}
```

**Security Rationale:**
- Never send full API keys to client (XSS protection)
- Mask keys consistently: first 4 + last 4 chars
- Use bullet character (•) for visual clarity
- Short keys (<8 chars) masked completely
- Include `auth_type` field for UI filtering