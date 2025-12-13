# Step 1: Update settings.html to use Spectre accordion patterns

**Skill:** @go-coder
**Files:** pages/settings.html

---

## Iteration 1

### Agent 2 - Implementation

Updated the settings accordion HTML structure to match Spectre CSS patterns exactly. Replaced Font Awesome icons with Spectre's `icon icon-arrow-right`, simplified checkbox inputs to use `hidden` attribute only, replaced Font Awesome loading spinners with Spectre's `loading` class, and changed wrapper element to use `accordion` class.

**Changes made:**
- `pages/settings.html:22`: Changed `<section x-data="settingsAccordion">` to `<div x-data="settingsAccordion" class="accordion">`
- `pages/settings.html:25-40`: Updated API Keys accordion item:
  - Removed `class="accordion-checkbox"` from input (line 26)
  - Added `name="accordion-checkbox"` and `hidden` attributes to input
  - Changed icon from `<i class="fas fa-key"></i>` to `<i class="icon icon-arrow-right mr-1"></i>` (line 28)
  - Removed `<span>` wrapper around "API Keys" text (line 29)
  - Replaced Font Awesome spinner with Spectre loading (lines 32-36)
- `pages/settings.html:42-58`: Updated Authentication accordion item with same pattern
- `pages/settings.html:60-76`: Updated Configuration accordion item with same pattern
- `pages/settings.html:78-94`: Updated Danger Zone accordion item with same pattern
- `pages/settings.html:96-112`: Updated Service Status accordion item with same pattern
- `pages/settings.html:114`: Changed closing `</section>` to `</div>`

**Commands run:**
No compilation needed for HTML changes.

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ No compilation required for HTML template

**Tests:**
⚙️ No tests applicable (HTML template change)

**Code Quality:**
✅ Follows Spectre CSS patterns exactly
✅ All Font Awesome icons replaced with Spectre icons
✅ Checkbox inputs simplified to use `hidden` attribute
✅ Loading spinners use Spectre's native `loading` class
✅ Wrapper element uses proper `accordion` class
✅ Consistent pattern applied to all 5 accordion sections

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
HTML structure successfully updated to match Spectre CSS accordion patterns. All Font Awesome dependencies removed from accordion structure. Loading spinners now use Spectre's native loading class. Structure is clean and ready for minimal CSS in next step.

**→ Continuing to Step 2**
