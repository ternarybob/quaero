I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The accordion implementation in `settings.html` makes AJAX requests to URLs like `/settings/auth-cookies.html`, but the actual files are named `settings-auth-cookies.html` in the `pages/partials/` directory. The URL-to-file mapping needs to prepend `settings-` to the requested filename. The existing `StaticFileHandler` provides a good security pattern with `filepath.HasPrefix()` checks, but an allowlist approach is more secure for this use case since we have a fixed set of known partial files. The partial files are already self-contained HTML fragments that don't require template execution - they can be served as raw files.

### Approach

Add a new route handler in `routes.go` for the `/settings/{partial}` pattern and create a `ServePartial()` method in `page_handler.go` to serve raw HTML fragments from the `pages/partials/` directory. Implement security validation using an allowlist approach to prevent directory traversal attacks. Set proper `Content-Type: text/html` headers for HTML fragments.

### Reasoning

Listed the repository structure, read `routes.go` and `page_handler.go` to understand existing routing and handler patterns, examined the `pages/partials/` directory to identify the partial files that need to be served, and reviewed a sample partial file (`settings-status.html`) to confirm they are self-contained HTML fragments with Alpine.js components.

## Mermaid Diagram

sequenceDiagram
    participant Browser
    participant Router
    participant PageHandler
    participant FileSystem

    Note over Browser,FileSystem: User clicks accordion item in /settings page
    
    Browser->>Router: GET /settings/auth-cookies.html
    Router->>PageHandler: ServePartial(w, r)
    
    PageHandler->>PageHandler: Extract filename: "auth-cookies.html"
    PageHandler->>PageHandler: Check allowlist: ["auth-apikeys.html", "auth-cookies.html", ...]
    
    alt Filename NOT in allowlist
        PageHandler->>PageHandler: Log warning (security attempt)
        PageHandler->>Browser: 404 Not Found
    else Filename in allowlist
        PageHandler->>PageHandler: Map filename: "settings-auth-cookies.html"
        PageHandler->>PageHandler: Construct path: pages/partials/settings-auth-cookies.html
        PageHandler->>FileSystem: os.Stat(fullPath)
        
        alt File does NOT exist
            FileSystem-->>PageHandler: Error
            PageHandler->>PageHandler: Log error
            PageHandler->>Browser: 404 Not Found
        else File exists
            FileSystem-->>PageHandler: File info
            PageHandler->>PageHandler: Set Content-Type: text/html
            PageHandler->>FileSystem: http.ServeFile(fullPath)
            FileSystem-->>Browser: HTML fragment content
            Browser->>Browser: Alpine.js initializes component
        end
    end

## Proposed File Changes

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\page_handler.go(MODIFY)

Add a new route handler for the `/settings/` pattern immediately after the main settings page route (after line 28). Register the route as `mux.HandleFunc("/settings/", s.app.PageHandler.ServePartial)` to handle requests like `/settings/auth-cookies.html`, `/settings/status.html`, etc.

**Important:** The route must be registered AFTER the `/settings` route (line 28) to avoid shadowing the main settings page. Go's ServeMux matches the longest pattern first, so `/settings` will match exact requests and `/settings/` will match subpaths.

**Placement:** Insert between line 28 (settings page route) and line 30 (static files comment).

**Rationale:** This follows the existing pattern where specific routes are registered before wildcard routes (e.g., `/api/jobs` before `/api/jobs/`).

### internal\handlers\page_handler.go(MODIFY)

References: 

- internal\server\routes.go(MODIFY)

Add a new method `ServePartial` to the `PageHandler` struct (after the `StaticFileHandler` method, around line 89) to serve partial HTML fragments from the `pages/partials/` directory.

**Method Signature:** `func (h *PageHandler) ServePartial(w http.ResponseWriter, r *http.Request)`

**Implementation Steps:**

1. **Extract Partial Name:** Remove the `/settings/` prefix from `r.URL.Path` to get the requested filename (e.g., `auth-cookies.html`).

2. **Security Validation (Allowlist Approach):** Create a map or slice of allowed partial filenames to prevent directory traversal attacks. The allowed partials are:
   - `auth-apikeys.html`
   - `auth-cookies.html`
   - `config.html`
   - `danger.html`
   - `status.html`
   
   Check if the requested filename exists in the allowlist. If not, return `http.NotFound(w, r)` and log a warning with the attempted filename.

3. **File Path Mapping:** Prepend `settings-` to the requested filename to match the actual file naming convention in `pages/partials/`. For example:
   - Request: `/settings/auth-cookies.html` → File: `pages/partials/settings-auth-cookies.html`
   - Request: `/settings/status.html` → File: `pages/partials/settings-status.html`

4. **Locate Partials Directory:** Use the existing `findPagesDir()` function to locate the pages directory, then construct the full path to the partials subdirectory using `filepath.Join(pagesDir, "partials", filename)`.

5. **File Existence Check:** Use `os.Stat()` to verify the file exists. If not, return `http.NotFound(w, r)` and log an error.

6. **Set Content-Type Header:** Set `w.Header().Set("Content-Type", "text/html; charset=utf-8")` to ensure the browser interprets the response as HTML.

7. **Serve File:** Use `http.ServeFile(w, r, fullPath)` to serve the partial HTML file.

8. **Error Logging:** Log any errors (file not found, invalid partial name) using the `h.logger` with appropriate context (requested path, resolved filename).

**Security Rationale:** The allowlist approach is more secure than path validation alone because:
- It explicitly defines which partials can be served
- It prevents any directory traversal attempts (e.g., `../../../etc/passwd`)
- It's easier to audit and maintain than regex or path sanitization
- It fails closed (denies by default) rather than trying to detect all attack patterns

**Alternative Considered:** Using `filepath.HasPrefix()` checks like `StaticFileHandler` (line 82), but rejected because:
- Allowlist is more explicit and secure for a fixed set of known files
- The filename mapping (`auth-cookies.html` → `settings-auth-cookies.html`) makes path validation more complex
- The allowlist serves as documentation of available partials

**Performance Note:** The allowlist check is O(1) with a map or O(n) with a slice. Since there are only 5 partials, either approach is acceptable. A map is slightly more efficient but a slice is more readable.