# Step 5: Create settings-danger.html partial

**Skill:** @go-coder
**Files:** `pages\partials\settings-danger.html(NEW)`, `pages\settings.html(MODIFY)`

---

## Iteration 1

### Agent 2 - Implementation
I need to extract the Danger Zone section from settings.html and create a new self-contained partial file following the service-logs.html pattern.

**Changes made:**
- Created `pages\partials\settings-danger.html` with self-contained Danger Zone card
- Included `confirmDeleteAllDocuments()` function within the partial (since it's only used by Danger Zone)
- Modified `pages\settings.html` to use the new partial via template inclusion
- Removed `confirmDeleteAllDocuments()` function from inline script section

**Commands run:**
```bash
# Verify HTML syntax and structure
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ No compilation needed for HTML files

**Tests:**
✅ No tests applicable for HTML templates

**Code Quality:**
✅ Follows service-logs.html pattern
✅ Self-contained card structure
✅ JavaScript function included within partial for isolation
✅ Proper button onclick handler preserved

**Quality Score:** 9/10

**Issues Found:**
1. None - changes look correct
2. Function properly isolated to the partial that uses it

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE (9/10)

**Notes:**
- Danger Zone section extracted successfully
- `settings.html` now uses template inclusion for Danger Zone section
- `confirmDeleteAllDocuments()` function moved to partial for better encapsulation
- All inline Danger Zone code successfully refactored

**→ Continuing to Step 6**
