I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend Infrastructure (Already Complete - Phases 1-6):**
- ✅ KV storage layer exists: `KeyValueStorage` interface with `Get`, `Set`, `Delete`, `List`, `GetAll` methods
- ✅ KV service exists: `kv.Service` with business logic and validation
- ✅ KV service initialized in `app.go` (line 352-356) and accessible via `App.KVService`
- ✅ Data model: `KeyValuePair` struct with `key`, `value`, `description`, `created_at`, `updated_at`
- ✅ API key migration complete: old `/api/auth/api-key/*` endpoints removed from `auth_handler.go` (line 181 comment)

**Frontend Current State:**
- ❌ UI still references old API key endpoints in `settings-components.js` (lines 593-788)
- ❌ `authApiKeys` component uses `/api/auth/api-key` and `/api/auth/list` endpoints (lines 627, 664, 698)
- ❌ No `/api/kv` routes registered in `routes.go`
- ❌ No KV handler exists yet

**Key Architectural Patterns Observed:**
- Handler pattern: `auth_handler.go` provides template (constructor injection, method-based routing, JSON responses)
- Route registration: `routes.go` uses `handleAuthRoutes` pattern for sub-routing
- Alpine.js components: Use reactive data, fetch API, global notification system (`window.showNotification`)
- Value masking: Current implementation shows first 4 + last 4 chars (line 778-786 in settings-components.js)

### Approach

## Implementation Strategy

**Three-Layer Approach (Backend → Routes → Frontend):**

1. **Create KV Handler** (`internal/handlers/kv_handler.go`)
   - Follow `auth_handler.go` pattern: constructor with service injection, logger, helper methods
   - Implement 5 HTTP handlers: List (GET /api/kv), Get (GET /api/kv/{key}), Create (POST /api/kv), Update (PUT /api/kv/{key}), Delete (DELETE /api/kv/{key})
   - Add value sanitization: mask values in responses (show first 4 + last 4 chars, or "••••••••" if < 8 chars)
   - Use existing helpers: `RequireMethod`, `WriteJSON`, `WriteError` from handlers package

2. **Register Routes** (`internal/server/routes.go`)
   - Add `/api/kv` route for list/create operations (line ~91, near other API routes)
   - Add `/api/kv/` route with sub-router function `handleKVRoutes` (similar to `handleAuthRoutes` pattern)
   - Wire handler in `app.go` initialization (line ~563, after other handlers)

3. **Update Frontend** (`pages/static/settings-components.js`)
   - Refactor `authApiKeys` component (lines 593-788) to use new `/api/kv` endpoints
   - Simplify data model: remove `service_type` field (no longer needed), use `key` instead of `name`
   - Update form fields in `settings-auth-apikeys.html` to match new structure
   - Maintain existing UX: masking, modals, validation, caching

**Trade-offs & Decisions:**
- **Value Masking Strategy**: Implement server-side masking in handler (not service layer) to keep service pure and testable
- **Backward Compatibility**: None needed - old endpoints already removed, migration complete
- **Service Type Removal**: Simplify to generic key/value (no service_type field) - users can encode type in key name (e.g., "gemini-llm-key")
- **Route Pattern**: Use sub-router pattern (`handleKVRoutes`) for consistency with existing codebase, easier to extend later

### Reasoning

Explored the codebase structure by listing directories and reading key files. Examined `auth_handler.go` to understand handler patterns, `routes.go` for routing conventions, `app.go` for service initialization, and `settings-components.js` for frontend Alpine.js patterns. Reviewed the KV storage interface and service to understand the data model. Identified that backend infrastructure is complete (Phases 1-6) but HTTP layer and UI updates are missing. Found that old API key endpoints were already removed, confirming clean separation is ready for new KV endpoints.

## Mermaid Diagram

sequenceDiagram
    participant UI as Settings UI<br/>(Alpine.js)
    participant Routes as routes.go<br/>(Router)
    participant Handler as kv_handler.go<br/>(HTTP Layer)
    participant Service as kv.Service<br/>(Business Logic)
    participant Storage as KeyValueStorage<br/>(SQLite)

    Note over UI,Storage: List All Keys (GET /api/kv)
    UI->>Routes: GET /api/kv
    Routes->>Handler: ListKVHandler()
    Handler->>Service: List(ctx)
    Service->>Storage: List(ctx)
    Storage-->>Service: []KeyValuePair
    Service-->>Handler: []KeyValuePair
    Handler->>Handler: maskValue() for each
    Handler-->>Routes: JSON (masked values)
    Routes-->>UI: [{key, value: "sk-1...xyz", description}]

    Note over UI,Storage: Create Key (POST /api/kv)
    UI->>Routes: POST /api/kv<br/>{key, value, description}
    Routes->>Handler: CreateKVHandler()
    Handler->>Handler: Validate key & value
    Handler->>Service: Set(ctx, key, value, desc)
    Service->>Storage: Set(ctx, key, value, desc)
    Storage-->>Service: nil (success)
    Service-->>Handler: nil
    Handler-->>Routes: 201 Created
    Routes-->>UI: {status: "success"}

    Note over UI,Storage: Update Key (PUT /api/kv/{key})
    UI->>Routes: PUT /api/kv/gemini-key<br/>{value, description}
    Routes->>Handler: UpdateKVHandler()
    Handler->>Service: Set(ctx, key, value, desc)
    Service->>Storage: Set(ctx, key, value, desc)
    Storage-->>Service: nil (upsert)
    Service-->>Handler: nil
    Handler-->>Routes: 200 OK
    Routes-->>UI: {status: "success"}

    Note over UI,Storage: Delete Key (DELETE /api/kv/{key})
    UI->>Routes: DELETE /api/kv/gemini-key
    Routes->>Handler: DeleteKVHandler()
    Handler->>Service: Delete(ctx, key)
    Service->>Storage: Delete(ctx, key)
    Storage-->>Service: nil (success)
    Service-->>Handler: nil
    Handler-->>Routes: 200 OK
    Routes-->>UI: {status: "success"}

## Proposed File Changes

### internal\handlers\kv_handler.go(NEW)

References: 

- internal\handlers\auth_handler.go
- internal\services\kv\service.go
- internal\interfaces\kv_storage.go

Create new KV handler following the pattern established in `c:/development/quaero/internal/handlers/auth_handler.go`.

**Constructor:**
- `NewKVHandler(kvService *kv.Service, logger arbor.ILogger) *KVHandler` - inject KV service and logger
- Store dependencies in struct fields for use in handler methods

**Handler Methods (5 total):**

1. **ListKVHandler** (GET /api/kv)
   - Call `kvService.List(ctx)` to retrieve all key/value pairs
   - Sanitize values in response using `maskValue()` helper (show first 4 + last 4 chars)
   - Return JSON array: `[{"key": "...", "value": "sk-12...34", "description": "...", "created_at": ..., "updated_at": ...}]`
   - Use `WriteJSON(w, http.StatusOK, sanitizedPairs)` helper

2. **GetKVHandler** (GET /api/kv/{key})
   - Extract key from URL path: `key := r.URL.Path[len("/api/kv/"):]`
   - Validate key is not empty, return 400 if missing
   - Call `kvService.Get(ctx, key)` to retrieve value
   - Return full `KeyValuePair` object with masked value
   - Handle not found error: return 404 with appropriate message

3. **CreateKVHandler** (POST /api/kv)
   - Parse JSON body into struct with fields: `key`, `value`, `description`
   - Validate required fields: `key` and `value` must not be empty
   - Call `kvService.Set(ctx, key, value, description)`
   - Return 201 Created with success message and created key
   - Handle validation errors: return 400 with error details

4. **UpdateKVHandler** (PUT /api/kv/{key})
   - Extract key from URL path
   - Parse JSON body (same structure as Create)
   - If `value` is empty in request, skip update (allow description-only updates)
   - Call `kvService.Set(ctx, key, value, description)` (Set handles upsert)
   - Return 200 OK with success message
   - Handle not found: return 404 if key doesn't exist

5. **DeleteKVHandler** (DELETE /api/kv/{key})
   - Extract key from URL path
   - Call `kvService.Delete(ctx, key)`
   - Return 200 OK with success message
   - Handle not found: return 404 with appropriate message

**Helper Methods:**

- `maskValue(value string) string` - Mask sensitive values for API responses
  - If length < 8: return "••••••••"
  - Otherwise: return first 4 chars + "..." + last 4 chars (e.g., "sk-1...xyz9")
  - This matches existing pattern in `settings-components.js` line 774-786

**Error Handling:**
- Use `RequireMethod(w, r, "GET")` helper for method validation (already exists in handlers package)
- Use `WriteError(w, statusCode, message)` for error responses
- Log all errors with structured logging: `logger.Error().Err(err).Str("key", key).Msg("...")`
- Log successful operations at Info level: `logger.Info().Str("key", key).Msg("...")`

**Security Considerations:**
- Never log actual values (only keys)
- Always mask values in responses (except when explicitly requested by client - future enhancement)
- Validate key format: no special characters that could cause injection issues
- Use context from request: `r.Context()` for all service calls

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\kv_handler.go(NEW)

Register new KV handler routes in the `setupRoutes()` method and add sub-router function.

**In `setupRoutes()` method (around line 91, near other API routes):**
- Add route registration after auth routes (line ~47): `mux.HandleFunc("/api/kv", s.handleKVRoute)` for list/create operations
- Add route registration: `mux.HandleFunc("/api/kv/", s.handleKVRoutes)` for get/update/delete operations with key parameter

**Add new sub-router function (after `handleAuthRoutes`, around line 204):**

`handleKVRoute(w http.ResponseWriter, r *http.Request)` - Routes /api/kv (no trailing slash)
- GET: Call `s.app.KVHandler.ListKVHandler(w, r)` to list all keys
- POST: Call `s.app.KVHandler.CreateKVHandler(w, r)` to create new key
- Other methods: Return 405 Method Not Allowed

`handleKVRoutes(w http.ResponseWriter, r *http.Request)` - Routes /api/kv/{key}
- Extract key from path: `path := r.URL.Path` and validate it starts with `/api/kv/`
- GET: Call `s.app.KVHandler.GetKVHandler(w, r)` to retrieve specific key
- PUT: Call `s.app.KVHandler.UpdateKVHandler(w, r)` to update specific key
- DELETE: Call `s.app.KVHandler.DeleteKVHandler(w, r)` to delete specific key
- Other methods: Return 405 Method Not Allowed
- Handle empty key case: return 400 Bad Request

**Pattern Reference:**
Follow the same pattern as `handleAuthRoutes` (lines 182-204) which routes `/api/auth/{id}` requests to appropriate handlers based on HTTP method. Use `RouteResourceItem` helper if available, or implement method switching manually.

**Placement:**
Insert route registrations in `setupRoutes()` near line 91 (after `/api/config` and before `/api/shutdown`), maintaining alphabetical ordering of API routes for consistency.

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\kv_handler.go(NEW)

Wire up the new KV handler in the application initialization.

**In the `App` struct (around line 47-114):**
- Add new field after `AuthHandler` (line 102): `KVHandler *handlers.KVHandler` to store the KV handler instance

**In `initHandlers()` method (around line 563-637):**
- Add KV handler initialization after `AuthHandler` initialization (after line 578):
  - Create handler: `a.KVHandler = handlers.NewKVHandler(a.KVService, a.Logger)`
  - Log initialization: `a.Logger.Info().Msg("KV handler initialized")`

**Dependencies:**
- `a.KVService` is already initialized in `initServices()` at line 352-356
- No additional imports needed - `handlers` package already imported
- Handler will be accessible to routes via `s.app.KVHandler` in `routes.go`

**Placement Rationale:**
Place after `AuthHandler` initialization to maintain logical grouping of data management handlers (Auth, KV, Document, etc.). This follows the existing pattern where related handlers are initialized together.

### pages\static\settings-components.js(MODIFY)

References: 

- internal\handlers\kv_handler.go(NEW)

Refactor the `authApiKeys` Alpine.js component (lines 593-788) to use new `/api/kv` endpoints and simplified data model.

**Component State Changes:**
- Rename `apiKeys` to `kvPairs` for clarity (or keep as `apiKeys` for minimal UI changes)
- Remove `service_type` from form data and display logic
- Update data structure to match `KeyValuePair`: `{key, value, description, created_at, updated_at}`

**Method Updates:**

1. **loadApiKeys()** (line 624-655)
   - Change endpoint from `/api/auth/list` to `/api/kv`
   - Remove filtering logic (lines 633-641) - no longer needed since KV store only contains keys
   - Update data mapping: `key` instead of `name`, no `service_type` field
   - Keep caching logic with `componentStateCache.authApiKeys`

2. **deleteApiKey()** (line 657-681)
   - Change endpoint from `/api/auth/api-key/${id}` to `/api/kv/${key}`
   - Update parameter from `id` to `key` (use key as identifier, not separate ID)
   - Update confirmation message to reference "key" instead of "API key name"
   - Update filter logic: `this.apiKeys.filter(item => item.key !== key)`

3. **editApiKey()** (line 683-692)
   - Update to use `key` instead of `id` for identification
   - Remove `service_type` from form data population
   - Update form data structure: `{key: apiKey.key, value: '', description: apiKey.description}`

4. **submitApiKey()** (line 694-743)
   - Change endpoints:
     - Create: `/api/kv` (POST)
     - Update: `/api/kv/${this.formData.key}` (PUT)
   - Update request body structure: `{key, value, description}` (remove `service_type`)
   - For edit mode, use `key` from form data instead of `editingId`
   - Keep validation: require `key` and `value` fields

5. **Helper Methods:**
   - **getMaskedApiKey()** (line 774-787): Update to use `item.value` instead of `item.api_key`
   - **getDescription()** (line 758-764): Simplify to return `apiKey.description || '-'` (no nested `data` object)
   - **toggleApiKeyVisibility()** (line 766-772): Update to use `key` instead of `id`

**Form Data Structure:**
Update `formData` initialization (line 604-609) and reset (line 749-755):
- Change `name` to `key`
- Change `apiKey` to `value`
- Remove `serviceType` field
- Keep `description` field

**Cache Key:**
Keep using `componentStateCache.authApiKeys` for backward compatibility with existing cache, or rename to `componentStateCache.kvPairs` if preferred.

**Error Handling:**
Maintain existing error handling patterns with `window.showNotification` and console logging. Update error messages to reference "key/value pair" instead of "API key" where appropriate.

### pages\partials\settings-auth-apikeys.html(MODIFY)

References: 

- pages\static\settings-components.js(MODIFY)

Update the API Keys settings partial to match the new KV data model and remove service_type references.

**Table Header Changes (line 40-47):**
- Change "Name" column to "Key" (line 41)
- Remove "Service Type" column (line 42) - no longer needed in generic KV store
- Keep "API Key" column but rename to "Value" for clarity
- Keep "Description", "Last Updated", and "Actions" columns

**Table Body Changes (line 50-76):**
- Update key binding: `:key="apiKey.key"` instead of `:key="apiKey.id"` (line 50)
- Change `x-text="apiKey.name"` to `x-text="apiKey.key"` (line 52)
- Remove service type badge display (lines 53-55)
- Update value display to use `apiKey.value` instead of `apiKey.api_key` (line 58)
- Update toggle visibility to use `apiKey.key` instead of `apiKey.id` (line 59)
- Update edit/delete buttons to pass `apiKey.key` instead of `apiKey.id` (lines 67, 70)

**Modal Form Changes (line 82-136):**

1. **Name Field (line 93-96):**
   - Change label from "Name" to "Key"
   - Change `id` and `x-model` from `name` to `key`
   - Update placeholder: "e.g., gemini-llm-key" or "google-places-api-key"
   - Update hint: "Unique identifier for this key/value pair"

2. **Service Type Field (line 98-108):**
   - **REMOVE ENTIRELY** - no longer needed in generic KV store
   - Users can encode service type in key name if needed (e.g., "gemini-llm-key")

3. **API Key Field (line 110-118):**
   - Change label from "API Key" to "Value"
   - Change `id` and `x-model` from `apiKey` to `value`
   - Update placeholder: "Enter value (API key, token, or secret)"
   - Keep password toggle functionality

4. **Description Field (line 120-123):**
   - Keep as-is, but update placeholder: "Optional description (e.g., 'Google Gemini API key for LLM service')"

**Empty State Message (line 28-34):**
- Update text from "No API keys stored" to "No keys stored"
- Update description: "Add keys for API keys, tokens, and other configuration values"

**Button Labels:**
- Keep "Add API Key" button text (line 10) for user familiarity, or change to "Add Key" for consistency
- Modal title (line 87): Keep as "Add API Key" / "Edit API Key" for user familiarity

**Accessibility:**
- Ensure all form labels have matching `for` attributes with input `id` values
- Update ARIA labels if any reference removed fields