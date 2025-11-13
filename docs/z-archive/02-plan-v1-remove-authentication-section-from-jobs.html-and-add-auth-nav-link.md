I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The authentication section in `jobs.html` is cleanly isolated between lines 22-91, and the `authPage()` component is self-contained in lines 239-309. The navbar currently highlights the JOBS link for both `/jobs` and `/auth` routes (line 10: `{{if or (eq .Page "jobs" ) (eq .Page "auth" )}}class="active" {{end}}`). After removal, the jobs page will focus solely on job definitions management, and a new AUTH link will provide direct access to the authentication page.

### Approach

Remove the authentication management section from the jobs page and add a dedicated AUTH navigation link in the navbar. This completes the separation of concerns started with the creation of `auth.html`, ensuring each page has a single, focused purpose.

### Reasoning

Read `pages/jobs.html` to identify the authentication section (lines 22-91) and the `authPage()` Alpine.js component (lines 239-309) that need to be removed. Examined `pages/partials/navbar.html` to understand the current navigation structure and active state logic (line 10 shows JOBS is active for both "jobs" and "auth" pages).

## Proposed File Changes

### pages\jobs.html(MODIFY)

References: 

- pages\auth.html

Remove the authentication management section and Alpine.js component to focus the page solely on job definitions.

**Section 1: Remove Authentication List Section (lines 22-91)**

Delete the entire `<section>` block that contains:
- The `x-data="authPage()"` Alpine.js component binding
- The authentication card with header ("Authentication" title and refresh button)
- Loading state with spinner
- Empty state with lock icon and Chrome extension instructions
- Authentication table with columns: Site, Name, Service Type, Last Updated, Actions
- Delete button functionality

**Important:** Keep the comment on line 93 (`<!-- ============================= JOB DEFINITIONS PANEL ============================= -->`) as it marks the start of the job definitions section.

**Section 2: Remove authPage() Alpine.js Component (lines 239-309)**

Delete the entire `authPage()` function definition including:
- Function declaration: `function authPage() { return { ... } }`
- Properties: `authentications`, `loading`, `deleting`
- Methods: `init()`, `loadAuthentications()`, `deleteAuthentication()`, `formatDate()`
- All API calls to `/api/auth/list` and `/api/auth/{id}`

**Important:** Keep the comment on line 238 (`// ============================= AUTH PAGE COMPONENT =============================`) can be removed as well since there's no auth component anymore.

**Section 3: Update Page Title and Description (lines 16-18)**

Update the page description to reflect the focused purpose:
- Line 17: Keep `<h1>Job Management</h1>` unchanged
- Line 18: Change from `<p>Manage authentication and job definitions for data collection</p>` to `<p>Manage job definitions for data collection</p>`

**What Remains:**
- Job Definitions Management section (lines 96-224) - unchanged
- Service Logs section (lines 226-229) - unchanged
- Page initialization scripts (lines 314-327) - unchanged
- All template includes (head, navbar, footer, snackbar) - unchanged

**Result:** The jobs page will be dedicated to job definitions management only, with authentication management now handled by the separate `auth.html` page created in the previous implementation.

### pages\partials\navbar.html(MODIFY)

References: 

- pages\auth.html
- pages\jobs.html(MODIFY)

Add a dedicated AUTH navigation link and update the active state logic to separate AUTH from JOBS.

**Change 1: Add AUTH Navigation Link (after line 10)**

Insert a new navigation link between the JOBS link (line 10) and the QUEUE link (line 11):
```
<a href="/auth" @click="mobileMenuOpen = false" {{if eq .Page "auth" }}class="active" {{end}}>AUTH</a>
```

**Attributes:**
- `href="/auth"` - Routes to the authentication management page
- `@click="mobileMenuOpen = false"` - Closes mobile menu on click (consistent with other links)
- `{{if eq .Page "auth" }}class="active" {{end}}` - Highlights link when on auth page
- Link text: `AUTH` (uppercase, consistent with other navigation items)

**Change 2: Update JOBS Link Active State (line 10)**

Modify the JOBS link to remove the `(eq .Page "auth")` condition:

**Current (line 10):**
```
<a href="/jobs" @click="mobileMenuOpen = false" {{if or (eq .Page "jobs" ) (eq .Page "auth" )}}class="active" {{end}}>JOBS</a>
```

**Updated:**
```
<a href="/jobs" @click="mobileMenuOpen = false" {{if eq .Page "jobs" }}class="active" {{end}}>JOBS</a>
```

**Explanation:** Remove the `or (eq .Page "auth")` condition so the JOBS link is only highlighted when on the `/jobs` page, not when on the `/auth` page.

**Navigation Order After Changes:**
1. HOME
2. JOBS (active only on /jobs)
3. AUTH (active only on /auth) ← NEW
4. QUEUE
5. DOCUMENTS
6. SEARCH
7. CHAT
8. SETTINGS

**No Other Changes Required:**
- Mobile menu toggle functionality remains unchanged
- WebSocket status indicator remains unchanged
- All other navigation links remain unchanged
- Alpine.js mobile menu logic remains unchanged