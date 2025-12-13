# Plan: Create HTTP API Layer for Key/Value Store

## Overview
Complete the KV store implementation by adding the HTTP API layer (handler, routes) and updating the frontend to use the new endpoints. Backend infrastructure (service, storage) is already complete from Phases 1-6.

## Steps

1. **Create KV Handler**
   - Skill: @go-coder
   - Files: `internal/handlers/kv_handler.go` (NEW)
   - User decision: no
   - Implement 5 HTTP handlers (List, Get, Create, Update, Delete) following auth_handler.go pattern
   - Add value masking helper for security (show first 4 + last 4 chars)

2. **Register KV Routes**
   - Skill: @go-coder
   - Files: `internal/server/routes.go`, `internal/app/app.go`
   - User decision: no
   - Register /api/kv and /api/kv/{key} routes in routes.go
   - Wire handler in app.go initialization

3. **Update Frontend Component**
   - Skill: @go-coder
   - Files: `pages/static/settings-components.js`
   - User decision: no
   - Refactor authApiKeys component to use /api/kv endpoints
   - Remove service_type field, use key instead of name

4. **Update HTML Template**
   - Skill: @go-coder
   - Files: `pages/partials/settings-auth-apikeys.html`
   - User decision: no
   - Update table columns (remove service_type, change name to key)
   - Update modal form fields to match new data model

5. **Test API Endpoints**
   - Skill: @go-coder
   - Files: test endpoints with curl/manual testing
   - User decision: no
   - Verify all CRUD operations work correctly
   - Test value masking in responses

## Success Criteria
- KV handler implements all 5 CRUD operations (List, Get, Create, Update, Delete)
- Routes registered and accessible at /api/kv and /api/kv/{key}
- Frontend component uses new endpoints successfully
- Values are masked in API responses (first 4 + last 4 chars)
- Old API key UI works seamlessly with new backend
- Full codebase compiles without errors
- Manual testing confirms CRUD operations work end-to-end
