I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Issue Analysis:**

The settings page displays console errors: `'activeSection is not defined'`, `'loading is not defined'`, `'content is not defined'`. Investigation reveals:

1. **Component Definition** (`pages/static/settings-components.js`, lines 288-403): The `settingsNavigation` component is properly defined with all required properties (`content`, `loading`, `loadedSections`, `activeSection`)

2. **Template Bindings** (`pages/settings.html`, lines 70-76): The HTML uses Alpine.js directives that access `loading[activeSection]` and `content[activeSection]`

3. **Timing Issue**: Alpine.js evaluates template bindings during component initialization, but `activeSection` is initialized to `null` (line 293), causing undefined property access errors when Alpine tries to evaluate `loading[null]` and `content[null]`

4. **Script Loading Order** (`pages/partials/head.html`): Scripts load correctly - `settings-components.js` loads before Alpine.js (line 8 in settings.html uses non-deferred loading, while Alpine.js uses `defer` in head.html line 40)

**Why the Error Occurs:**

Alpine.js's reactivity system evaluates all template expressions immediately when the component is created. The sequence is:
1. Component data object created with `activeSection: null`
2. Alpine.js evaluates `loading[activeSection]` → `loading[null]` → undefined
3. Console error logged
4. `init()` method runs and sets `activeSection` to valid value
5. Subsequent evaluations work correctly

**Solution Approach:**

Initialize `activeSection` to the `defaultSection` value (`'auth-apikeys'`) immediately in the data object, rather than waiting for `init()` to set it. This provides a valid string key for property access during initial template evaluation, while the `init()` method can still override it based on URL parameters.

### Approach

Fix Alpine.js console errors in the settings page by initializing `activeSection` to `defaultSection` value immediately in the component data object, preventing undefined property access during template evaluation. This ensures Alpine.js bindings have valid values before `init()` executes.

### Reasoning

Explored the repository structure, read the two JavaScript files mentioned by the user (`settings-components.js` and `common.js`), examined the settings page HTML to understand component usage and template bindings, and reviewed the head partial to verify script loading order. Identified that the console errors stem from Alpine.js evaluating template expressions before the `init()` method sets `activeSection` to a valid value.

## Mermaid Diagram

sequenceDiagram
    participant Browser
    participant Alpine as Alpine.js
    participant Component as settingsNavigation
    participant Template as settings.html

    Note over Browser,Template: BEFORE FIX (causes errors)
    Browser->>Alpine: Load Alpine.js
    Alpine->>Component: Create component data
    Component-->>Alpine: activeSection = null
    Alpine->>Template: Evaluate x-show="loading[activeSection]"
    Template-->>Alpine: Access loading[null]
    Alpine-->>Browser: ❌ Console Error: undefined
    Alpine->>Component: Call init()
    Component->>Component: Set activeSection = 'auth-apikeys'
    Note over Component: Now works, but error already logged

    Note over Browser,Template: AFTER FIX (no errors)
    Browser->>Alpine: Load Alpine.js
    Alpine->>Component: Create component data
    Component-->>Alpine: activeSection = 'auth-apikeys'
    Alpine->>Template: Evaluate x-show="loading[activeSection]"
    Template-->>Alpine: Access loading['auth-apikeys']
    Alpine-->>Browser: ✅ No error (valid property access)
    Alpine->>Component: Call init()
    Component->>Component: Parse URL, validate, update if needed
    Note over Component: Works correctly from start

## Proposed File Changes

### pages\static\settings-components.js(MODIFY)

References: 

- pages\settings.html

**Root Cause**: Alpine.js evaluates template bindings (`x-show="loading[activeSection]"` and `x-html="content[activeSection]"` in `pages/settings.html`) during component initialization, but `activeSection` is initially `null` (line 293), causing "undefined" errors when accessing `loading[null]` and `content[null]`.

**Fix on line 293**: Change `activeSection: null,` to `activeSection: 'auth-apikeys',` to match the `defaultSection` value (line 294). This ensures the property has a valid string value immediately when Alpine.js evaluates template bindings.

**Why this works**: The `init()` method (lines 297-308) already handles URL parameter parsing and validation, and will override this initial value if a valid URL parameter exists. By setting the initial value to the default section ID, we provide a safe fallback that prevents undefined property access during the brief moment between component creation and `init()` execution.

**Alternative considered but rejected**: Using `x-show="activeSection && loading[activeSection]"` in the HTML would add null checks to every binding, increasing template complexity and violating the principle of keeping logic in the component rather than the template. The current fix is cleaner and more maintainable.

**No other changes needed**: The `init()` method's logic (lines 297-308) remains unchanged - it still parses URL parameters, validates section IDs, and calls `selectSection()` appropriately. The only difference is that `activeSection` now starts with a valid value instead of `null`.