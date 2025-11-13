I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires extracting 5 sections from existing HTML pages into self-contained partial files. The current architecture uses Alpine.js components defined inline within pages. The `service-logs.html` partial demonstrates the pattern: self-contained HTML with `x-data` attribute referencing a component registered in `common.js`. The auth page contains two separate Alpine.js components (`authPage()` and `apiKeysPage()`) that need to be split into separate partials. Date formatting logic is duplicated across components and should be consolidated.


### Approach

Extract sections from `settings.html` and `auth.html` into 5 new partial files following the `service-logs.html` pattern. Each partial will be self-contained HTML with Alpine.js component references. Move Alpine.js component definitions from inline `<script>` tags to `common.js` for centralization. Create a shared date formatting utility to eliminate duplication. Follow alphabetical naming convention: `settings-auth-apikeys.html`, `settings-auth-cookies.html`, `settings-config.html`, `settings-danger.html`, `settings-status.html`.


### Reasoning

Listed repository structure, read the three key files (`settings.html`, `auth.html`, `service-logs.html`), examined the partials directory structure, and reviewed `common.js` to understand Alpine.js component registration patterns and identify code duplication opportunities.


## Proposed File Changes

### pages\partials\settings-status.html(NEW)

References: 

- pages\settings.html
- pages\partials\service-logs.html

Create a new partial file for the Service Status section extracted from `c:/development/quaero/pages/settings.html` (lines 24-46). Structure as a self-contained card component with `x-data="settingsStatus"` attribute. Include the card header with refresh button and online/offline status label, and card body displaying version, build, port, and host information. Use Alpine.js directives (`@click`, `x-text`, `:class`) for interactivity. Follow the pattern established in `c:/development/quaero/pages/partials/service-logs.html` where the Alpine.js component logic is registered in `c:/development/quaero/pages/static/common.js` rather than inline.

### pages\partials\settings-config.html(NEW)

References: 

- pages\settings.html
- pages\partials\service-logs.html

Create a new partial file for the Configuration Details section extracted from `c:/development/quaero/pages/settings.html` (lines 48-56). Structure as a self-contained card component with `x-data="settingsConfig"` attribute. Include the card header with title and card body displaying formatted configuration JSON in a scrollable pre/code block. Use Alpine.js `x-text` directive to display formatted config. The Alpine.js component logic will be registered in `c:/development/quaero/pages/static/common.js`.

### pages\partials\settings-auth-cookies.html(NEW)

References: 

- pages\auth.html
- pages\partials\service-logs.html

Create a new partial file for the Cookie-Based Authentication section extracted from `c:/development/quaero/pages/auth.html` (lines 22-91). Structure as a self-contained card component with `x-data="authCookies"` attribute. Include the card header with refresh button, card body with loading state, empty state (no credentials message), and authentication table displaying site domain, name, service type, last updated, and delete actions. Use Alpine.js directives (`@click`, `x-show`, `x-for`, `x-text`) for interactivity. The Alpine.js component logic (currently `authPage()` function in lines 247-346) will be moved to `c:/development/quaero/pages/static/common.js` and renamed to `authCookies`. Follow the pattern in `c:/development/quaero/pages/partials/service-logs.html`.

### pages\partials\settings-auth-apikeys.html(NEW)

References: 

- pages\auth.html
- pages\partials\service-logs.html

Create a new partial file for the API Key Management section extracted from `c:/development/quaero/pages/auth.html` (lines 94-232). Structure as a self-contained card component with `x-data="authApiKeys"` attribute. Include the card header with refresh and add buttons, card body with loading state, empty state (no API keys message), API keys table displaying name, service type, masked API key with show/hide toggle, description, last updated, and edit/delete actions. Include the create/edit modal for API key management with form fields for name, service type, API key (with password visibility toggle), and description. Use Alpine.js directives (`@click`, `x-show`, `x-for`, `x-text`, `@submit.prevent`) for interactivity. The Alpine.js component logic (currently `apiKeysPage()` function in lines 349-566) will be moved to `c:/development/quaero/pages/static/common.js` and renamed to `authApiKeys`. Follow the pattern in `c:/development/quaero/pages/partials/service-logs.html`.

### pages\partials\settings-danger.html(NEW)

References: 

- pages\settings.html
- pages\partials\service-logs.html

Create a new partial file for the Danger Zone section extracted from `c:/development/quaero/pages/settings.html` (lines 60-83). Structure as a self-contained card component with `x-data="settingsDanger"` attribute. Include the card header with red-colored title "Danger Zone" and card body with description of the clear all documents action and a delete button. The button should call a function to confirm and execute the delete operation. The Alpine.js component logic (currently `confirmDeleteAllDocuments()` function in lines 140-166) will be moved to `c:/development/quaero/pages/static/common.js` as part of the `settingsDanger` component. Follow the pattern in `c:/development/quaero/pages/partials/service-logs.html`.

### pages\static\common.js(MODIFY)

References: 

- pages\settings.html
- pages\auth.html

Add five new Alpine.js component registrations within the `document.addEventListener('alpine:init', ...)` block (after line 24):

1. **settingsStatus** - Extract and adapt the `settingsPage()` component logic from `c:/development/quaero/pages/settings.html` (lines 98-137), focusing only on the service status functionality (isOnline, version, build, port, host, loadConfig method). Remove the config formatting logic as that belongs to settingsConfig component.

2. **settingsConfig** - Create a new component for configuration display with config data property and formatConfig method (extracted from settingsPage() lines 132-135 in `c:/development/quaero/pages/settings.html`). Include loadConfig method to fetch configuration from `/api/config`.

3. **authCookies** - Move the `authPage()` component from `c:/development/quaero/pages/auth.html` (lines 247-346) and rename to `authCookies`. Include properties: authentications array, loading state, deleting state. Include methods: init, loadAuthentications (fetch from `/api/auth/list` and filter for non-API key credentials), deleteAuthentication, formatDate.

4. **authApiKeys** - Move the `apiKeysPage()` component from `c:/development/quaero/pages/auth.html` (lines 349-566) and rename to `authApiKeys`. Include properties: apiKeys array, loading state, deleting state, saving state, showPassword, showCreateModal, showEditModal, editingId, showFullById, formData. Include methods: init, loadApiKeys (fetch from `/api/auth/list` and filter for API key credentials), deleteApiKey, editApiKey, submitApiKey, closeModals, getDescription, toggleApiKeyVisibility, getShowFull, getMaskedApiKey, formatDate.

5. **settingsDanger** - Create a new component with a confirmDeleteAllDocuments method extracted from `c:/development/quaero/pages/settings.html` (lines 140-166). This method should prompt for confirmation and call `/api/documents/clear-all` DELETE endpoint, then show notification with results.

Additionally, create a shared utility function `formatDate(timestamp)` that can be reused across components to eliminate the duplicated date formatting logic found in authPage() and apiKeysPage() (lines 301-344 and 521-564 in `c:/development/quaero/pages/auth.html`). Export this as `window.formatDate` similar to how job type utilities are exported (lines 272-274). Update the new component methods to use this shared utility instead of duplicating the logic.