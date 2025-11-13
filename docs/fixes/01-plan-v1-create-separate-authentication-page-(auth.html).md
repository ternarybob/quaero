I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The current `/auth` route (line 19 in `routes.go`) already exists but serves `jobs.html` with page identifier "auth". The authentication UI section (lines 22-91) and Alpine.js component `authPage()` (lines 239-309) in `jobs.html` are self-contained and can be cleanly extracted. The page structure follows a consistent pattern across all pages (head, navbar, main container, footer, snackbar partials) as seen in `settings.html`.

### Approach

Create a new dedicated Authentication Management page (`auth.html`) by extracting the authentication section from `jobs.html`. Update the route handler to serve the new template. This separates concerns and provides a cleaner user experience with dedicated pages for authentication and job management.

### Reasoning

Examined the repository structure, read `jobs.html` to identify the authentication section and Alpine.js component, reviewed `settings.html` to understand the standard page template structure, and checked `routes.go` to confirm the existing route configuration. Also reviewed `navbar.html` to understand navigation patterns.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Browser
    participant Server
    participant PageHandler
    participant AuthAPI
    
    User->>Browser: Navigate to /auth
    Browser->>Server: GET /auth
    Server->>PageHandler: ServePage("auth.html", "auth")
    PageHandler->>Browser: Render auth.html template
    Browser->>Browser: Alpine.js init authPage()
    Browser->>AuthAPI: GET /api/auth/list
    AuthAPI->>Browser: Return authentication list
    Browser->>User: Display Authentication Management page
    
    Note over Browser,AuthAPI: User can delete auth credentials
    User->>Browser: Click delete button
    Browser->>Browser: Show confirmation dialog
    User->>Browser: Confirm deletion
    Browser->>AuthAPI: DELETE /api/auth/{id}
    AuthAPI->>Browser: Success response
    Browser->>User: Update UI & show notification

## Proposed File Changes

### pages\auth.html(NEW)

References: 

- pages\settings.html
- pages\jobs.html

Create a new HTML template file for the Authentication Management page following the standard page structure pattern from `c:/development/quaero/pages/settings.html`.

**Page Structure:**
- Include `{{template "head.html" .}}` partial in the `<head>` section
- Set page title to "Authentication Management - Quaero"
- Include `{{template "navbar.html" .}}` partial after opening `<body>` tag
- Create `<main class="page-container">` container
- Include `{{template "footer.html" .}}` partial before closing `</body>` tag
- Include `{{template "snackbar.html" .}}` partial for notifications

**Page Title Section:**
- Add a `<div class="page-title">` with:
  - `<h1>Authentication Management</h1>`
  - `<p>Manage authentication credentials captured from websites</p>`

**Authentication Section:**
Extract the entire authentication section from `c:/development/quaero/pages/jobs.html` (lines 22-91):
- Wrap in `<section style="margin-top: 1.5rem;" x-data="authPage()">`
- Include the card with header containing "Authentication" title and refresh button
- Include loading state with spinner icon
- Include empty state with lock icon and instructions about Chrome extension
- Include authentication table with columns: Site, Name, Service Type, Last Updated, Actions
- Include delete button functionality with confirmation

**Service Logs Section:**
- Add `<section style="margin-top: 1.5rem;">` containing `{{template "service-logs.html" .}}`
- This provides real-time log streaming for authentication operations

**Alpine.js Component:**
Extract the `authPage()` Alpine.js component from `c:/development/quaero/pages/jobs.html` (lines 239-309) and place in a `<script>` tag before closing `</body>`:
- Include the complete `authPage()` function with properties: `authentications`, `loading`, `deleting`
- Include `init()` method that calls `loadAuthentications()`
- Include `loadAuthentications()` async method that fetches from `/api/auth/list`
- Include `deleteAuthentication(id, siteDomain)` async method with confirmation dialog
- Include `formatDate(timestamp)` helper method for relative time display

**Page Initialization:**
- Add DOMContentLoaded event listener for logging page load events
- Add alpine:init event listener for Alpine.js initialization confirmation
- Follow the same logging pattern as `c:/development/quaero/pages/jobs.html` (lines 314-327)

**Important Notes:**
- Maintain exact Alpine.js syntax and API endpoints from the original implementation
- Keep all CSS classes and styling consistent with existing pages
- Ensure WebSocket integration works via the service-logs partial
- The page should be fully self-contained with no dependencies on jobs.html

### internal\server\routes.go(MODIFY)

References: 

- pages\auth.html(NEW)

Update the `/auth` route handler to serve the new `auth.html` template instead of `jobs.html`.

**Change Required:**
On line 19, modify the route registration:
- **Current:** `mux.HandleFunc("/auth", s.app.PageHandler.ServePage("jobs.html", "auth"))`
- **New:** `mux.HandleFunc("/auth", s.app.PageHandler.ServePage("auth.html", "auth"))`

**Explanation:**
- The first parameter to `ServePage()` is the template filename (change from "jobs.html" to "auth.html")
- The second parameter "auth" is the page identifier used for navbar active state highlighting (keep unchanged)
- This change routes the `/auth` URL to render the new dedicated authentication page
- The `PageHandler.ServePage()` method is defined in the page handler and returns an `http.HandlerFunc`

**No Other Changes Required:**
- All API endpoints (`/api/auth/list`, `/api/auth/{id}`, etc.) remain unchanged
- The page identifier "auth" is still passed to the template for navbar highlighting
- No changes to route registration order or other routes needed