# Step 4: Update UI tests to validate settings accordion

**Skill:** @test-writer
**Files:** test/ui/auth_test.go

---

## Iteration 1

### Agent 2 - Implementation

Updated the UI tests to validate authentication functionality through the settings page accordion instead of the legacy `/auth` page. The changes ensure tests verify the accordion structure and that authentication sections are properly expanded via URL parameters.

**Changes made:**
- `test/ui/auth_test.go:27-93`: Updated `TestAuthPageLoad` function:
  - Changed navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
  - Updated expected title from "Authentication Management - Quaero" to "Settings - Quaero"
  - Changed page heading check from "Authentication Management" to "Settings"
  - Added verification for accordion sections being expanded
  - Updated log message to reference settings page with auth accordions

- `test/ui/auth_test.go:95-142`: Updated `TestAuthPageElements` function:
  - Changed navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
  - Updated test cases to verify accordion structure (accordion container, items, headers)
  - Kept existing checks for authentication table elements
  - Updated comments to reference settings.html instead of auth.html

- `test/ui/auth_test.go:144-209`: Updated `TestAuthNavbar` function:
  - Changed navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
  - Removed "AUTH" from expected menu items list
  - Changed active menu item check from AUTH to SETTINGS
  - Updated test to verify SETTINGS is active on settings page with auth accordions

- `test/ui/auth_test.go:211-305`: Updated `TestAuthCookieInjection` function:
  - Changed navigation URL from `/auth` to `/settings?a=auth-apikeys,auth-cookies`
  - Updated screenshot filename to reflect settings page context
  - Kept all authentication verification logic unchanged

**Commands run:**
```bash
cd test/ui && go test -v -run TestAuth
```

### Agent 3 - Validation

**Skill:** @test-writer

**Compilation:**
✅ Compiles cleanly

**Tests:**
✅ All tests pass
- TestAuthPageLoad: PASS (4.73s)
- TestAuthPageElements: PASS (5.06s)
- TestAuthNavbar: PASS (5.62s)
- TestAuthCookieInjection: PASS (5.32s)

**Code Quality:**
✅ Follows existing test patterns
✅ Correctly updated all URL references
✅ Updated selectors to match accordion structure (.accordion-item)
✅ Removed AUTH from expected navbar items
✅ Changed active menu item check to SETTINGS
✅ All authentication functionality tests still pass

**Quality Score:** 10/10

**Issues Found:**
None - Initial test run revealed incorrect selector (.accordion vs .accordion-item), which was fixed in iteration.

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
UI tests successfully updated to validate settings accordion approach. All four test functions pass:
- TestAuthPageLoad verifies settings page loads with correct title and accordion structure
- TestAuthPageElements confirms page elements are present
- TestAuthNavbar validates navbar no longer has AUTH link and SETTINGS is active
- TestAuthCookieInjection ensures authentication functionality works through settings page

**→ Continuing to Step 5**
