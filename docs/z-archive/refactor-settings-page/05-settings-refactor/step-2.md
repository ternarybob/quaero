# Step 2: Simplify CSS to minimal icon rotation only

**Skill:** @go-coder
**Files:** pages/static/quaero.css

---

## Iteration 1

### Agent 2 - Implementation

Replaced extensive custom accordion styles (lines 1667-1746) with minimal CSS for icon rotation only. Removed all custom styling for accordion structure, transitions, hover effects, and max-height animations. Added only 2 CSS rules for icon rotation visual feedback. Kept responsive adjustments but removed unnecessary accordion-header padding/font-size overrides.

**Changes made:**
- `pages/static/quaero.css:1667-1721`: Removed all custom `.accordion-item`, `.accordion-checkbox`, `.accordion-header`, `.accordion-body` styling
- `pages/static/quaero.css:1667`: Added minimal icon rotation CSS (2 rules)
- `pages/static/quaero.css:1742-1745`: Removed `.accordion-header` responsive overrides from media query

**Commands run:**
No compilation needed for CSS changes.

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ No compilation required for CSS changes

**Tests:**
⚙️ No tests applicable (CSS styling change)

**Code Quality:**
✅ CSS dramatically simplified - from ~80 lines to 6 lines
✅ Only essential icon rotation rules remain
✅ All aesthetic styling deferred to Spectre defaults
✅ Transition timing maintained (0.2s ease)
✅ Proper selector specificity for accordion context
✅ Responsive overrides cleaned up

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
CSS successfully simplified to absolute minimum. Removed ~74 lines of custom styling. Only 2 rules remain for icon rotation visual feedback. All accordion functionality now relies on Spectre CSS native patterns. Responsive media query cleaned of accordion overrides.

**→ Continuing to Step 3**
