I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Authentication Storage:**
- `auth_credentials` table stores cookie-based authentication with unique constraint on `site_domain`
- Current fields: `id`, `name`, `site_domain`, `service_type`, `data`, `cookies`, `tokens`, `base_url`, `user_agent`, `created_at`, `updated_at`
- Storage interface provides CRUD operations via `AuthStorage` interface
- Auth service (`internal/services/auth/service.go`) is Atlassian-centric but supports generic auth

**API Key Usage Pattern:**
- LLM service: `config.LLM.GoogleAPIKey` → `genai.NewClient()`
- Agent service: `config.Agent.GoogleAPIKey` → `gemini.NewModel()`
- Places service: `config.PlacesAPI.APIKey` → HTTP request parameter
- All services read API keys from config during initialization
- Environment variables override config file values

**Job Definition Pattern:**
- Job definitions loaded from `./job-definitions/*.toml` files via `LoadJobDefinitionsFromFiles()`
- Steps have `Config map[string]interface{}` for flexible parameters
- Managers extract config values and pass to services
- Example: `CrawlerManager` uses `jobDef.AuthID` to reference auth credentials

**File Loading Pattern:**
- `load_job_definitions.go` provides template for loading TOML files
- Reads directory, parses TOML/JSON, validates, saves to database
- Called during `initDatabase()` in `app.go`
- Idempotent with ON CONFLICT handling

### Approach

## Implementation Strategy

**1. Extend Database Schema (Breaking Change Acceptable)**
- Add `api_key` TEXT field to `auth_credentials` table
- Add `auth_type` TEXT field with values 'cookie' or 'api_key'
- Modify unique index: Remove `idx_auth_site_domain`, add `idx_auth_name_type` on `(name, auth_type)`
- Keep `site_domain` for cookie auth, allow NULL for API keys
- Default `auth_type='cookie'` for backward compatibility

**2. Storage Layer Updates**
- Add `GetCredentialsByName(ctx, name)` to `AuthStorage` interface
- Add `GetAPIKeyByName(ctx, name)` convenience method
- Update all SQL queries in `auth_storage.go` to include new fields
- Implement unique name validation for API keys
- Maintain backward compatibility for cookie-based auth

**3. API Key Resolution Helper**
- Create `ResolveAPIKey(ctx, authStorage, name, configFallback)` in `common/config.go`
- Resolution order: auth storage by name → config fallback → error
- Return resolved key string or error
- Log resolution source for debugging

**4. Service Integration**
- Modify service constructors to accept `AuthStorage` parameter
- Update `NewGeminiService()` to resolve API key via helper
- Update `agents.NewService()` to resolve API key via helper
- Update `places.NewService()` to resolve API key via helper
- Maintain backward compatibility: if key not in storage, use config

**5. Job Definition Integration**
- Add `api_key` string field support in `JobStep.Config`
- Update managers to resolve API key from storage when present
- Pass resolved key to service methods (requires service API changes)
- Add validation in `job_definition_handler.go` to check API key existence

**6. Auth Handler Extensions**
- Add `POST /api/auth/api-key` for creating API keys
- Add `PUT /api/auth/api-key/{id}` for updating API keys
- Sanitize responses: never return `api_key` value in GET responses
- Reuse existing `ListAuthHandler` and `DeleteAuthHandler`

**7. File Loading System**
- Create `load_auth_credentials.go` following `load_job_definitions.go` pattern
- Support TOML format: `name`, `api_key`, `service_type`, `description`
- Call `LoadAuthCredentialsFromFiles()` in `initDatabase()` after schema init
- Add `auth_dir` config option (default: `./auth`)
- Mask API keys in logs: "Loaded API key: gemini-llm [REDACTED]"

### Reasoning

Explored the codebase to understand:
- Current auth storage schema and unique constraints
- How services initialize with API keys from config
- Job definition loading pattern from files
- Manager pattern for extracting step config
- Service constructor signatures and dependencies
- Auth handler structure and routing

## Mermaid Diagram

sequenceDiagram
    participant App as Application Startup
    participant DB as Database/Schema
    participant Loader as Auth File Loader
    participant Storage as Auth Storage
    participant Service as LLM/Agent/Places Service
    participant Manager as Job Manager
    participant Worker as Job Worker

    Note over App,Worker: Startup Phase
    App->>DB: Initialize schema with api_key, auth_type fields
    App->>Loader: LoadAuthCredentialsFromFiles("./auth")
    Loader->>Loader: Parse TOML files
    Loader->>Storage: StoreCredentials(api_key, auth_type='api_key')
    Storage->>DB: INSERT with unique constraint on (name, auth_type)
    
    Note over App,Worker: Service Initialization
    App->>Service: NewService(config, authStorage, logger)
    Service->>Service: ResolveAPIKey("gemini-llm", config.GoogleAPIKey)
    Service->>Storage: GetAPIKeyByName("gemini-llm")
    alt API key in storage
        Storage-->>Service: Return API key
        Service->>Service: Use resolved key
    else Fallback to config
        Service->>Service: Use config.GoogleAPIKey
    end
    
    Note over App,Worker: Job Execution Phase
    Manager->>Manager: CreateParentJob(step, jobDef)
    Manager->>Manager: Extract api_key from step.Config
    alt api_key specified
        Manager->>Storage: GetAPIKeyByName(api_key_name)
        Storage-->>Manager: Return resolved key
        Manager->>Worker: Pass resolved key in job config
    else No api_key
        Manager->>Worker: Use service default
    end
    
    Worker->>Service: Execute with resolved API key
    Service->>Service: Call external API with key

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

**Extend auth_credentials table schema:**

1. Add `api_key TEXT` field after `tokens` field
2. Add `auth_type TEXT NOT NULL DEFAULT 'cookie'` field after `api_key`
3. Remove `CREATE UNIQUE INDEX idx_auth_site_domain` (line 30)
4. Add `CREATE UNIQUE INDEX idx_auth_name_type ON auth_credentials(name, auth_type)` for unique API key names
5. Modify `site_domain` to allow NULL: change `site_domain TEXT NOT NULL` to `site_domain TEXT`
6. Add comment explaining auth_type values: 'cookie' for cookie-based auth, 'api_key' for API key storage

**Rationale:** API keys don't have site domains, so we need a different uniqueness constraint. Name + auth_type ensures unique API key names while allowing duplicate names across auth types.

### internal\models\auth.go(MODIFY)

**Extend AuthCredentials model:**

1. Add `APIKey string` field after `Tokens` field with JSON tag `json:"api_key"`
2. Add `AuthType string` field after `APIKey` with JSON tag `json:"auth_type"` and comment explaining values: 'cookie' or 'api_key'
3. Update struct comment to document both authentication types

**Example:**
```go
type AuthCredentials struct {
    // ... existing fields ...
    Tokens      map[string]string      `json:"tokens"`       // Auth tokens
    APIKey      string                 `json:"api_key"`      // API key for service authentication
    AuthType    string                 `json:"auth_type"`    // Authentication type: 'cookie' or 'api_key'
    BaseURL     string                 `json:"base_url"`     // Service base URL
    // ... rest of fields ...
}
```

### internal\storage\sqlite\auth_storage.go(MODIFY)

References: 

- internal\storage\sqlite\schema.go(MODIFY)
- internal\models\auth.go(MODIFY)

**Update all SQL queries and methods:**

1. **StoreCredentials method (line 35):**
   - Add `api_key` and `auth_type` to INSERT statement
   - Add `api_key` and `auth_type` to UPDATE statement
   - Default `auth_type` to 'cookie' if empty
   - For API keys, allow `site_domain` to be NULL

2. **GetCredentialsByID method (line 136):**
   - Add `api_key` and `auth_type` to SELECT statement
   - Add scan targets for new fields

3. **GetCredentialsBySiteDomain method (line 174):**
   - Add `api_key` and `auth_type` to SELECT statement
   - Add scan targets for new fields

4. **ListCredentials method (line 233):**
   - Add `api_key` and `auth_type` to SELECT statement
   - Add scan targets for new fields

5. **GetCredentials method (line 278):**
   - Add `api_key` and `auth_type` to SELECT statement
   - Add scan targets for new fields

**Add new methods:**

6. **GetCredentialsByName method:**
   - Query: `SELECT ... FROM auth_credentials WHERE name = ?`
   - Return first match (name is unique per auth_type)
   - Include all fields including `api_key` and `auth_type`

7. **GetAPIKeyByName method:**
   - Query: `SELECT api_key FROM auth_credentials WHERE name = ? AND auth_type = 'api_key'`
   - Return only the API key string
   - Return error if not found or if auth_type is not 'api_key'
   - Log warning if API key is empty

### internal\interfaces\storage.go(MODIFY)

References: 

- internal\models\auth.go(MODIFY)

**Extend AuthStorage interface:**

Add two new methods to the `AuthStorage` interface (after line 21):

```go
// GetCredentialsByName retrieves authentication credentials by name
// Returns first match (name is unique per auth_type)
GetCredentialsByName(ctx context.Context, name string) (*models.AuthCredentials, error)

// GetAPIKeyByName retrieves an API key by name
// Returns error if not found or if auth_type is not 'api_key'
GetAPIKeyByName(ctx context.Context, name string) (string, error)
```

**Rationale:** These methods enable lookup by friendly name for API key resolution in services and job definitions.

### internal\common\config.go(MODIFY)

References: 

- internal\interfaces\storage.go(MODIFY)

**Add API key resolution helper function:**

1. Add new function after `ApplyFlagOverrides` (around line 583):

```go
// ResolveAPIKey resolves an API key by name with fallback to config value
// Resolution order: auth storage by name → config fallback → error
// Returns the resolved API key string or error if not found
func ResolveAPIKey(ctx context.Context, authStorage interfaces.AuthStorage, name string, configFallback string) (string, error)
```

**Implementation logic:**
- If `authStorage` is not nil, call `GetAPIKeyByName(ctx, name)`
- If found and non-empty, log "Resolved API key from auth storage: {name}" and return
- If not found or empty, check `configFallback`
- If `configFallback` is non-empty, log "Using API key from config for: {name}" and return
- Otherwise, return error: "API key '{name}' not found in auth storage or config"

**Add auth_dir config field:**

2. Add to `Config` struct (after `Jobs` field around line 22):
```go
Auth        AuthDirConfig    `toml:"auth"`
```

3. Add new config struct (after `JobsConfig` around line 91):
```go
// AuthDirConfig contains configuration for authentication file loading
type AuthDirConfig struct {
    CredentialsDir string `toml:"credentials_dir"` // Directory containing auth credential files (TOML)
}
```

4. Add default value in `NewDefaultConfig()` (around line 207):
```go
Auth: AuthDirConfig{
    CredentialsDir: "./auth", // Default directory for auth files
},
```

5. Add environment variable override in `applyEnvOverrides()` (around line 527):
```go
// Auth configuration
if authDir := os.Getenv("QUAERO_AUTH_CREDENTIALS_DIR"); authDir != "" {
    config.Auth.CredentialsDir = authDir
}
```

### internal\services\llm\gemini_service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go(MODIFY)

**Update NewGeminiService to support API key resolution:**

1. **Change function signature (line 103):**
   - Add `authStorage interfaces.AuthStorage` parameter after `config`
   - Update function comment to document new parameter

2. **Add API key resolution logic (after line 105):**
   - Call `common.ResolveAPIKey(ctx, authStorage, "gemini-llm", config.LLM.GoogleAPIKey)`
   - Store resolved key in local variable `apiKey`
   - Use `apiKey` instead of `config.LLM.GoogleAPIKey` in `genai.NewClient()` call (line 131)

3. **Update error message (line 106):**
   - Change to: "Google API key is required for LLM service (set via auth storage, QUAERO_LLM_GOOGLE_API_KEY, or llm.google_api_key in config)"

4. **Update logging (line 146):**
   - Add log field indicating resolution source: "api_key_source" ("auth_storage" or "config")

**Rationale:** Enables LLM service to resolve API keys from auth storage with config fallback, maintaining backward compatibility.

### internal\services\agents\service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go(MODIFY)

**Update NewService to support API key resolution:**

1. **Change function signature (line 56):**
   - Add `authStorage interfaces.AuthStorage` parameter after `config`
   - Update function comment to document new parameter

2. **Add API key resolution logic (after line 58):**
   - Call `common.ResolveAPIKey(ctx, authStorage, "gemini-agent", config.GoogleAPIKey)`
   - Store resolved key in local variable `apiKey`
   - Use `apiKey` instead of `config.GoogleAPIKey` in `gemini.NewModel()` call (line 75)

3. **Update error message (line 59):**
   - Change to: "Google API key is required for agent service (set via auth storage, QUAERO_AGENT_GOOGLE_API_KEY, or agent.google_api_key in config)"

4. **Update logging (line 95):**
   - Add log field indicating resolution source: "api_key_source" ("auth_storage" or "config")

**Rationale:** Enables agent service to resolve API keys from auth storage with config fallback, maintaining backward compatibility.

### internal\services\places\service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go(MODIFY)

**Update NewService to support API key resolution:**

1. **Change function signature (line 28):**
   - Add `authStorage interfaces.AuthStorage` parameter after `config`
   - Update function comment to document new parameter

2. **Add API key resolution logic (before line 33):**
   - Create context: `ctx := context.Background()`
   - Call `common.ResolveAPIKey(ctx, authStorage, "google-places", config.APIKey)`
   - Store resolved key in local variable `apiKey`
   - Store `apiKey` in service struct instead of `config.APIKey`

3. **Update Service struct (line 19):**
   - Add `apiKey string` field
   - Remove direct reference to `config.APIKey` in methods

4. **Update textSearch method (line 125):**
   - Change `params.Set("key", s.config.APIKey)` to `params.Set("key", s.apiKey)`

5. **Update nearbySearch method (line 205):**
   - Change `params.Set("key", s.config.APIKey)` to `params.Set("key", s.apiKey)`

**Rationale:** Enables places service to resolve API keys from auth storage with config fallback, maintaining backward compatibility.

### internal\app\app.go(MODIFY)

References: 

- internal\services\llm\gemini_service.go(MODIFY)
- internal\services\agents\service.go(MODIFY)
- internal\services\places\service.go(MODIFY)
- internal\storage\sqlite\load_auth_credentials.go(NEW)

**Update service initialization calls:**

1. **LLM service initialization (line 258):**
   - Change `llm.NewGeminiService(a.Config, a.Logger)` to `llm.NewGeminiService(a.Config, a.StorageManager.AuthStorage(), a.Logger)`

2. **Agent service initialization (line 397):**
   - Change `agents.NewService(&a.Config.Agent, a.Logger)` to `agents.NewService(&a.Config.Agent, a.StorageManager.AuthStorage(), a.Logger)`

3. **Places service initialization (line 370):**
   - Change `places.NewService(&a.Config.PlacesAPI, a.EventService, a.Logger)` to `places.NewService(&a.Config.PlacesAPI, a.StorageManager.AuthStorage(), a.EventService, a.Logger)`

4. **Load auth credentials from files (in initDatabase method after line 216):**
   - Add call to `sqliteMgr.LoadAuthCredentialsFromFiles(ctx, a.Config.Auth.CredentialsDir)`
   - Wrap in error handling: log warning if fails, don't fail startup
   - Add log message: "Auth credentials loaded from files"

**Rationale:** Pass AuthStorage to services for API key resolution and load auth files during startup.

### internal\models\job_definition.go(MODIFY)

**Document api_key support in JobStep config:**

Add documentation comment in the config keys section (around line 73) explaining the new `api_key` field:

```go
//   - api_key (string): Name of API key from auth storage to use for this step. Optional.
//     If specified, the step will resolve the API key by name from auth_credentials table.
//     Example: "gemini-llm", "google-places", "my-custom-key"
```

**No code changes needed** - `JobStep.Config` is already `map[string]interface{}` which supports arbitrary keys. Managers will extract the `api_key` value when present.

**Rationale:** Document the convention without changing the flexible config structure.

### internal\jobs\manager\agent_manager.go(MODIFY)

References: 

- internal\interfaces\storage.go(MODIFY)
- internal\models\job_definition.go(MODIFY)

**Add API key resolution support:**

1. **Add authStorage field to AgentManager struct (line 16):**
   ```go
   authStorage    interfaces.AuthStorage
   ```

2. **Update NewAgentManager constructor (line 27):**
   - Add `authStorage interfaces.AuthStorage` parameter
   - Store in struct: `authStorage: authStorage`

3. **Add API key resolution in CreateParentJob (after line 54):**
   - Check if `api_key` is present in `stepConfig`:
     ```go
     if apiKeyName, ok := stepConfig["api_key"].(string); ok && apiKeyName != "" {
         // Resolve API key from auth storage
         apiKey, err := m.authStorage.GetAPIKeyByName(ctx, apiKeyName)
         if err != nil {
             return "", fmt.Errorf("failed to resolve API key '%s': %w", apiKeyName, err)
         }
         // Store resolved key in job config for worker to use
         jobConfig["resolved_api_key"] = apiKey
         m.logger.Debug().Str("api_key_name", apiKeyName).Msg("Resolved API key for agent job")
     }
     ```

**Note:** This enables job definitions to reference API keys by name. The worker will need to use the resolved key when calling the agent service.

**Rationale:** Allows agent steps to specify which API key to use via `api_key` field in step config.

### internal\jobs\manager\places_search_manager.go(MODIFY)

References: 

- internal\interfaces\storage.go(MODIFY)
- internal\models\job_definition.go(MODIFY)

**Add API key resolution support:**

1. **Add authStorage field to PlacesSearchManager struct (line 16):**
   ```go
   authStorage     interfaces.AuthStorage
   ```

2. **Update NewPlacesSearchManager constructor (line 27):**
   - Add `authStorage interfaces.AuthStorage` parameter
   - Store in struct: `authStorage: authStorage`

3. **Add API key resolution in CreateParentJob (after line 48):**
   - Check if `api_key` is present in `stepConfig`:
     ```go
     if apiKeyName, ok := stepConfig["api_key"].(string); ok && apiKeyName != "" {
         // Resolve API key from auth storage
         apiKey, err := m.authStorage.GetAPIKeyByName(ctx, apiKeyName)
         if err != nil {
             return "", fmt.Errorf("failed to resolve API key '%s': %w", apiKeyName, err)
         }
         // Override places service config with resolved key
         // Note: This requires PlacesService to accept per-request API keys
         // For now, log warning that API key override is not yet supported
         m.logger.Warn().Str("api_key_name", apiKeyName).Msg("API key override not yet supported for places service - using config value")
     }
     ```

**Note:** Full implementation requires PlacesService API changes to accept per-request API keys. This change documents the pattern for future implementation.

**Rationale:** Establishes the pattern for API key resolution in managers, even if full support requires service API changes.

### internal\handlers\auth_handler.go(MODIFY)

References: 

- internal\models\auth.go(MODIFY)
- internal\interfaces\storage.go(MODIFY)

**Add API key CRUD endpoints:**

1. **Add CreateAPIKeyHandler method (after DeleteAuthHandler around line 199):**
   - Method: POST
   - Parse request body: `name`, `api_key`, `service_type`, `description`
   - Validate: name and api_key are required
   - Create `AuthCredentials` with `AuthType="api_key"`, `SiteDomain=NULL`
   - Call `authStorage.StoreCredentials()`
   - Return sanitized response (without api_key value)
   - Log: "Created API key: {name}"

2. **Add UpdateAPIKeyHandler method:**
   - Method: PUT
   - Extract ID from path
   - Parse request body: `name`, `api_key`, `service_type`, `description`
   - Validate: ID exists and is api_key type
   - Update credentials
   - Return sanitized response (without api_key value)
   - Log: "Updated API key: {name}"

3. **Update ListAuthHandler (line 101):**
   - Modify sanitization to mask `api_key` field: show only first 4 and last 4 characters
   - Example: "sk-abc...xyz" instead of full key
   - Add `auth_type` to response

4. **Update GetAuthHandler (line 131):**
   - Add `auth_type` to sanitized response
   - Mask `api_key` field if present

**Rationale:** Provides API endpoints for managing API keys while ensuring keys are never exposed in full in responses.

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\auth_handler.go(MODIFY)

**Add API key routes:**

1. **Update handleAuthRoutes function (line 177):**
   - Add route for `POST /api/auth/api-key` → `CreateAPIKeyHandler`
   - Add route for `PUT /api/auth/api-key/{id}` → `UpdateAPIKeyHandler`
   - Keep existing routes for GET/DELETE

2. **Add route pattern matching:**
   ```go
   // POST /api/auth/api-key
   if r.Method == "POST" && path == "/api/auth/api-key" {
       s.app.AuthHandler.CreateAPIKeyHandler(w, r)
       return
   }
   
   // PUT /api/auth/api-key/{id}
   if r.Method == "PUT" && strings.HasPrefix(path, "/api/auth/api-key/") {
       s.app.AuthHandler.UpdateAPIKeyHandler(w, r)
       return
   }
   ```

**Rationale:** Routes API key CRUD operations to new handlers.

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\interfaces\storage.go(MODIFY)
- internal\models\job_definition.go(MODIFY)

**Add API key validation:**

1. **Update validateRuntimeDependencies method (around line 518):**
   - After agent service validation, add API key validation loop
   - For each step in `jobDef.Steps`:
     - Check if `step.Config["api_key"]` exists
     - If present, call `h.authStorage.GetAPIKeyByName(ctx, apiKeyName)`
     - If not found, set `jobDef.RuntimeStatus = "disabled"` and `jobDef.RuntimeError = "API key '{name}' not found in auth storage"`
     - Log warning: "Job definition references missing API key: {name}"

2. **Update CreateJobDefinitionHandler (around line 128):**
   - After step validation, add API key existence check
   - For each step with `api_key` in config:
     - Verify API key exists in auth storage
     - Return 400 error if not found: "API key '{name}' referenced in step '{step.Name}' does not exist"

**Rationale:** Validates that API keys referenced in job definitions actually exist, preventing runtime failures.

### internal\storage\sqlite\load_auth_credentials.go(NEW)

References: 

- internal\storage\sqlite\load_job_definitions.go
- internal\models\auth.go(MODIFY)

**Create new file for loading auth credentials from TOML files:**

Follow the pattern from `load_job_definitions.go` with these adaptations:

1. **Define AuthCredentialFile struct:**
   ```go
   type AuthCredentialFile struct {
       Name        string `toml:"name"`         // Required: unique name
       APIKey      string `toml:"api_key"`      // Required: API key value
       ServiceType string `toml:"service_type"` // Required: service identifier
       Description string `toml:"description"`  // Optional: human-readable description
   }
   ```

2. **Implement ToAuthCredentials method:**
   - Convert file struct to `models.AuthCredentials`
   - Set `AuthType = "api_key"`
   - Set `SiteDomain = NULL` (empty string)
   - Generate UUID for ID
   - Set timestamps

3. **Implement LoadAuthCredentialsFromFiles method on Manager:**
   - Read directory entries
   - Filter for `.toml` files
   - Parse each file with `toml.Unmarshal`
   - Validate: name, api_key, service_type are required
   - Call `authStorage.StoreCredentials()` (idempotent with ON CONFLICT)
   - Log: "Loaded API key: {name} [REDACTED]" (mask the actual key)
   - Count loaded/skipped files

4. **Error handling:**
   - Log warnings for invalid files, don't fail startup
   - Skip non-TOML files
   - Continue on individual file errors

**Rationale:** Enables loading API keys from files during startup, following established pattern.

### deployments\local\auth\example-api-keys.toml(NEW)

**Create example auth credentials file:**

Provide template with commented examples:

```toml
# Example API Key Configuration
# Place your API key files in this directory (./auth/)
# Each file can contain one or more API key definitions

# Google Gemini API Key for LLM Service
# Get your API key from: https://aistudio.google.com/app/apikey
# [[credentials]]
# name = "gemini-llm"
# api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
# service_type = "google-gemini"
# description = "Google Gemini API key for LLM embeddings and chat"

# Google Gemini API Key for Agent Service
# [[credentials]]
# name = "gemini-agent"
# api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
# service_type = "google-gemini"
# description = "Google Gemini API key for agent operations"

# Google Places API Key
# Get your API key from: https://console.cloud.google.com/apis/credentials
# [[credentials]]
# name = "google-places"
# api_key = "YOUR_GOOGLE_PLACES_API_KEY"
# service_type = "google-places"
# description = "Google Places API key for location search"

# Custom API Key Example
# [[credentials]]
# name = "my-custom-service"
# api_key = "your-api-key-here"
# service_type = "custom"
# description = "API key for custom service integration"
```

**Rationale:** Provides users with clear examples and instructions for configuring API keys via files.

### deployments\local\auth(NEW)

**Create auth directory for storing API key files.**

This directory will contain TOML files with API key definitions that are loaded during application startup.

**Note:** Add `.gitignore` file to prevent committing actual API keys:
```
# Ignore all TOML files except example
*.toml
!example-api-keys.toml
```

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\common\config.go(MODIFY)

**Add auth configuration section:**

Add new section after `[jobs]` section (around line 90):

```toml
# Authentication Configuration
[auth]
# Directory containing authentication credential files (TOML format)
# API keys can be stored in files and loaded at startup
# Example: ./auth/my-keys.toml
credentials_dir = "./auth"
```

**Update LLM and Agent sections with auth storage note:**

Add comment to `[llm]` section:
```toml
# Note: API keys can also be stored in auth storage (./auth/*.toml files)
# and referenced by name in job definitions
```

Add same comment to `[agent]` and `[places_api]` sections.

**Rationale:** Documents the new auth storage feature and provides configuration option for auth directory.