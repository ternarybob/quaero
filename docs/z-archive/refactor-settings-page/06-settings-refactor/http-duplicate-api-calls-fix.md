# HTTP Duplicate API Calls Fix

**Issue**: Authentication accordion was making multiple duplicate HTTP requests to `/api/auth/list`

**Date Fixed**: 2025-11-13

## Problem

When the Authentication accordion (auth-cookies) was clicked, the browser made 20+ duplicate HTTP requests to `/api/auth/list` instead of just one. This was discovered by reviewing application logs:

```
bin\logs\quaero.2025-11-13T18-19-42.log
18:19:51 path=/api/auth/list status=200 (called multiple times)
```

The same issue affected the API Keys accordion (auth-apikeys).

## Root Cause

The issue occurred due to how Alpine.js components are loaded when using `x-html` to dynamically insert HTML content:

1. **Accordion loads HTML** via `/settings/auth-cookies.html`
2. **Alpine re-initializes component** when HTML is inserted into DOM
3. **Component's `init()` method runs** and calls `loadAuthentications()`
4. **Multiple rapid re-renders** caused multiple initializations

### Why Instance-Level Flags Don't Work

Initially attempted fix using instance-level `hasLoaded` flag:

```javascript
Alpine.data('authCookies', () => ({
    hasLoaded: false,  // ❌ This doesn't persist across re-initializations

    init() {
        if (!this.hasLoaded) {  // ❌ Always false on new instance
            this.loadAuthentications();
        }
    }
}));
```

**Problem**: Each time Alpine re-renders the component (e.g., when `x-html` injects content), it creates a NEW component instance. The `hasLoaded` flag is reset to `false` every time.

## Solution

Implemented a **global component state cache** that persists across component re-initializations:

### Step 1: Create Global Cache

```javascript
document.addEventListener('alpine:init', () => {
    // === GLOBAL COMPONENT STATE CACHE ===
    // Prevents duplicate API calls when components are re-initialized by Alpine
    // when accordion content is re-rendered via x-html
    const componentStateCache = {
        authCookies: { hasLoaded: false, data: [] },
        authApiKeys: { hasLoaded: false, data: [] }
    };

    // ... rest of components
});
```

### Step 2: Check Cache in Component Init

```javascript
Alpine.data('authCookies', () => ({
    authentications: [],
    isLoading: false,
    // ... other state

    init() {
        // Check global cache to prevent duplicate API calls when component re-initializes
        if (componentStateCache.authCookies.hasLoaded) {
            logger.debug('AuthCookies', 'Loading from cache, skipping API call');
            this.authentications = componentStateCache.authCookies.data;
            return;  // ✅ Skip API call
        }

        // First load - fetch from API
        this.loadAuthentications();
    },
}));
```

### Step 3: Store Data in Cache After Loading

```javascript
async loadAuthentications() {
    // ... fetch logic ...

    this.authentications = data
        .filter(auth => this.isValidCookieAuth(auth))
        .map(auth => this.sanitizeAuthData(auth));

    // Store in global cache to prevent duplicate API calls on re-initialization
    componentStateCache.authCookies.hasLoaded = true;
    componentStateCache.authCookies.data = this.authentications;  // ✅ Cache the data

    // ... rest of method
}
```

## Testing

### Before Fix

Log analysis showed 20+ duplicate calls:
```bash
$ grep -c "path=/api/auth/list" bin/logs/quaero.2025-11-13T18-19-42.log
20+
```

### After Fix

Test verified only 1 API call is made:
```bash
$ grep -c "path=/api/auth/list" test/results/ui/settings-20251113-182633/SettingsAuthenticationAccordion/service.log
1
```

### Test Command

```bash
cd test/ui && go test -v -run TestSettingsAuthenticationAccordion
```

**Result**: ✅ PASS - Authentication accordion loads with only 1 HTTP request

## Files Modified

- `pages/static/settings-components.js`
  - Added `componentStateCache` global object (lines 35-41)
  - Updated `authCookies` component to use cache (lines 481-503, 535-537)
  - Updated `authApiKeys` component to use cache (lines 648-677, 701-703)

## Benefits

1. **Performance**: Eliminates 19+ unnecessary HTTP requests
2. **Server Load**: Reduces server load from duplicate requests
3. **User Experience**: Faster accordion interactions
4. **Maintainability**: Clear separation between component instances and shared state

## Related Issues

This pattern should be applied to other dynamically-loaded accordion components if they exhibit similar behavior:
- ✅ `authCookies` (Authentication)
- ✅ `authApiKeys` (API Keys)
- Status: Check if needed for other accordions

## Implementation Notes

**When to Use Global Cache**:
- Component is loaded via `x-html` or dynamic HTML insertion
- Component needs to persist data across re-initializations
- Component makes expensive API calls that shouldn't be repeated

**When NOT to Use**:
- Component needs fresh data on every render
- Component is only initialized once per page load
- Data is specific to a single component instance
