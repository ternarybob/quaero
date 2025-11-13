I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The authentication functionality has been successfully migrated to the settings page accordion with two sections: `auth-cookies` and `auth-apikeys`. The legacy `/auth` route and AUTH navigation link are now redundant. The codebase has:

1. **Active `/auth` route** in `routes.go` (line 19) serving `auth.html`
2. **AUTH navigation link** in `navbar.html` (line 11) pointing to `/auth`
3. **Legacy auth.html page** that includes the new partials (`settings-auth-cookies.html` and `settings-auth-apikeys.html`)
4. **UI tests** in `test/ui/auth_test.go` that validate the `/auth` page

All authentication functionality is now accessible via `/settings?a=auth-apikeys,auth-cookies`. To maintain backward compatibility for bookmarks and external links, the `/auth` route should redirect to the settings page with the appropriate accordion sections expanded, rather than returning a 404 error.

### Approach

Remove the legacy `/auth` route and AUTH navigation link while maintaining backward compatibility through a redirect handler. Update UI tests to validate the new settings accordion approach. Delete the obsolete `auth.html` page. The redirect ensures existing bookmarks and external links continue to work seamlessly.

### Reasoning

Listed the repository structure, read the three key files (`routes.go`, `navbar.html`, `auth.html`), searched for all references to `/auth` and `auth.html` across the codebase using grep, identified the UI test file that needs updating, and confirmed that all authentication functionality is now available through the settings page accordion.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Browser
    participant Router
    participant SettingsPage
    participant Accordion

    Note over User,Accordion: Legacy /auth Bookmark Redirect Flow
    
    User->>Browser: Click bookmark: /auth
    Browser->>Router: GET /auth
    Router->>Browser: 301 Redirect to /settings?a=auth-apikeys,auth-cookies
    Browser->>Router: GET /settings?a=auth-apikeys,auth-cookies
    Router->>SettingsPage: Serve settings.html
    SettingsPage->>Browser: Render page with accordion
    Browser->>Accordion: settingsAccordion.init()
    Accordion->>Accordion: Parse URL params: ["auth-apikeys", "auth-cookies"]
    Accordion->>Browser: Check checkboxes for auth-apikeys, auth-cookies
    Accordion->>Router: GET /settings/auth-apikeys.html
    Router->>Accordion: Return partial HTML
    Accordion->>Router: GET /settings/auth-cookies.html
    Router->>Accordion: Return partial HTML
    Accordion->>Browser: Render expanded accordions
    Browser->>User: Display settings page with auth sections expanded

    Note over User,Accordion: Direct Navigation via Settings Link
    
    User->>Browser: Click SETTINGS in navbar
    Browser->>Router: GET /settings
    Router->>SettingsPage: Serve settings.html
    SettingsPage->>Browser: Render page with accordion (all collapsed)
    User->>Browser: Click "Authentication" accordion
    Browser->>Accordion: @change event (auth-cookies, true)
    Accordion->>Router: GET /settings/auth-cookies.html
    Router->>Accordion: Return partial HTML
    Accordion->>Browser: Update URL to /settings?a=auth-cookies
    Accordion->>Browser: Render expanded accordion
    Browser->>User: Display authentication section

## Proposed File Changes

### internal\server\routes.go(MODIFY)

References: 

- pages\settings.html

Replace the `/auth` route handler on line 19 with a redirect handler that sends users to `/settings?a=auth-apikeys,auth-cookies`. This maintains backward compatibility for bookmarks and external links.

**Implementation approach:**
- Remove line 19: `mux.HandleFunc("/auth", s.app.PageHandler.ServePage("auth.html", "auth"))`
- Add a new redirect handler: `mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/settings?a=auth-apikeys,auth-cookies", http.StatusMovedPermanently) })`
- Use `http.StatusMovedPermanently` (301) to indicate the resource has permanently moved, which helps search engines and browsers update their cached links
- The query parameter `a=auth-apikeys,auth-cookies` will automatically expand both authentication accordion sections when the settings page loads

**Alternative considered:** Returning `http.StatusNotFound` (404) was rejected because it breaks existing bookmarks and external links. A redirect provides a better user experience and maintains backward compatibility.

**Placement:** Keep the redirect handler in the same location (line 19) to maintain the logical grouping of UI page routes.

### pages\partials\navbar.html(MODIFY)

References: 

- pages\settings.html

Remove the AUTH navigation link on line 11 since authentication management is now accessible through the SETTINGS page.

**Implementation approach:**
- Delete line 11: `<a href="/auth" @click="mobileMenuOpen = false" {{if eq .Page "auth" }}class="active" {{end}}>AUTH</a>`
- No replacement needed - the SETTINGS link (line 16) provides access to all authentication features via the accordion

**Navigation flow after change:**
- Users access authentication via: SETTINGS → Expand "API Keys" or "Authentication" accordion sections
- The settings page URL will be `/settings?a=auth-apikeys,auth-cookies` when both sections are expanded
- Mobile menu behavior remains unchanged (Alpine.js `@click` handler closes menu on navigation)

**Active state handling:** The removed line includes template logic `{{if eq .Page "auth" }}class="active" {{end}}` which highlights the AUTH link when on the auth page. After removal, users on `/auth` will be redirected to `/settings`, and the SETTINGS link will be highlighted instead (line 16 already handles this with `{{if eq .Page "settings" }}class="active" {{end}}`).

**Visual impact:** The navigation bar will have one fewer link, improving visual clarity and reducing cognitive load. All functionality remains accessible through the SETTINGS page.

### pages\auth.html(DELETE)

References: 

- pages\settings.html
- internal\server\routes.go(MODIFY)

Delete the legacy authentication page since all authentication functionality is now integrated into the settings page accordion. The page is no longer served by any route (the `/auth` route now redirects to `/settings`).

**Rationale for deletion:**
- The page includes `settings-auth-cookies.html` and `settings-auth-apikeys.html` partials (lines 23, 28) which are now loaded dynamically by the settings accordion
- The `/auth` route redirects to `/settings?a=auth-apikeys,auth-cookies`, so this template is never rendered
- Keeping unused templates increases maintenance burden and can cause confusion
- The page structure (head, navbar, footer, snackbar) is duplicated from other pages - no unique functionality exists here

**Impact analysis:**
- No code references `auth.html` except the route handler (which is being changed to redirect)
- UI tests in `test/ui/auth_test.go` will be updated to test the settings page instead
- Documentation references to `/auth` in archived docs are historical and don't affect functionality

**Alternative considered:** Keeping the file as a legacy fallback was rejected because:
- It's never rendered (redirect happens before template execution)
- It creates confusion about which page is the "source of truth" for authentication UI
- It violates the DRY principle (duplicates settings page structure)

### test\ui\auth_test.go(MODIFY)

References: 

- pages\settings.html
- pages\partials\settings-auth-cookies.html
- pages\partials\settings-auth-apikeys.html

Update the UI tests to validate authentication functionality through the settings page accordion instead of the legacy `/auth` page.

**Test function updates:**

1. **TestAuthPageLoads** (lines 54-93):
   - Change navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
   - Update expected title from "Authentication Management - Quaero" to "Settings - Quaero"
   - Update page heading check from "Authentication Management" to "Settings"
   - Verify both accordion sections are expanded by checking for accordion items with `auth-apikeys` and `auth-cookies` IDs
   - Update log message from "Auth page (auth.html) loads correctly" to "Settings page with auth accordions loads correctly"

2. **TestAuthPageElements** (lines 95-164):
   - Change navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
   - Update test cases to verify accordion structure:
     - Check for accordion container with class `accordion`
     - Check for accordion items with IDs `auth-apikeys` and `auth-cookies`
     - Check for accordion headers with labels "API Keys" and "Authentication"
     - Check for accordion bodies containing the authentication tables
   - Keep existing checks for authentication table elements (they should still be present in the loaded accordion content)
   - Update comments referencing `auth.html` to reference `settings.html` with accordion parameters

**Test validation approach:**
- Use ChromeDP selectors to verify accordion items are present and expanded
- Check that accordion bodies contain the expected authentication UI elements
- Verify URL parameter `a=auth-apikeys,auth-cookies` correctly expands both sections on page load
- Ensure all existing authentication functionality tests still pass (table display, delete buttons, etc.)

**Screenshot updates:**
- Screenshots saved by `takeScreenshot()` helper will now show the settings page with expanded accordions instead of the standalone auth page
- Screenshot filenames remain the same (e.g., `auth-page-load.png`) but content will reflect the new accordion-based UI

**Alternative considered:** Creating new test functions for accordion-specific behavior was rejected because:
- The existing tests already validate the authentication UI elements
- The accordion is just a different layout mechanism - the underlying functionality is unchanged
- Updating existing tests maintains test coverage continuity and reduces duplication