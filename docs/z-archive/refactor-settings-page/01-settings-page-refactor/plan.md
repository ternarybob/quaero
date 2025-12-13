# Plan: Settings Page Refactor

## Overview
Extract 5 sections from existing HTML pages into self-contained partial files following the `service-logs.html` pattern. Move Alpine.js component definitions from inline `<script>` tags to `common.js` for centralization. Create a shared date formatting utility to eliminate duplication.

## Steps

### Step 1: Create settings-status.html partial
- **Skill:** @go-coder
- **Files:** `pages\partials\settings-status.html(NEW)`, `pages\settings.html(MODIFY)`
- **Description:** Extract Service Status section from settings.html (lines 24-46) into new self-contained partial with Alpine.js component reference
- **User decision:** No

### Step 2: Create settings-config.html partial
- **Skill:** @go-coder
- **Files:** `pages\partials\settings-config.html(NEW)`, `pages\settings.html(MODIFY)`
- **Description:** Extract Configuration Details section from settings.html (lines 48-56) into new self-contained partial with Alpine.js component reference
- **User decision:** No

### Step 3: Create settings-auth-cookies.html partial
- **Skill:** @go-coder
- **Files:** `pages\partials\settings-auth-cookies.html(NEW)`, `pages\auth.html(MODIFY)`
- **Description:** Extract Cookie-Based Authentication section from auth.html (lines 22-91) into new self-contained partial with Alpine.js component reference
- **User decision:** No

### Step 4: Create settings-auth-apikeys.html partial
- **Skill:** @go-coder
- **Files:** `pages\partials\settings-auth-apikeys.html(NEW)`, `pages\auth.html(MODIFY)`
- **Description:** Extract API Key Management section from auth.html (lines 94-232) into new self-contained partial with Alpine.js component reference
- **User decision:** No

### Step 5: Create settings-danger.html partial
- **Skill:** @go-coder
- **Files:** `pages\partials\settings-danger.html(NEW)`, `pages\settings.html(MODIFY)`
- **Description:** Extract Danger Zone section from settings.html (lines 60-83) into new self-contained partial with Alpine.js component reference
- **User decision:** No

### Step 6: Centralize Alpine.js components in common.js
- **Skill:** @go-coder
- **Files:** `pages\static\common.js(MODIFY)`
- **Description:** Move all Alpine.js component registrations from inline scripts to common.js and create shared date formatting utility
- **User decision:** No

## Success Criteria
- ✅ 5 new self-contained partial files following service-logs.html pattern
- ✅ Original pages updated to use partials instead of inline sections
- ✅ All Alpine.js components centralized in common.js
- ✅ Shared date formatting utility eliminates duplication
- ✅ Code follows existing patterns and conventions
- ✅ All pages render correctly after refactor
