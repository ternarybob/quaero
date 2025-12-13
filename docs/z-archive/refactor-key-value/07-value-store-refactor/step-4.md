# Step 4: Update HTML Template

**Skill:** @go-coder
**Files:** `pages/partials/settings-auth-apikeys.html`

---

## Iteration 1

### Agent 2 - Implementation
Updated the HTML template to match the new KV data model by removing service_type references and changing field names from name/apiKey to key/value.

**Changes made:**
- `pages/partials/settings-auth-apikeys.html`:
  - **Empty state message** (lines 32-33):
    - Changed "No API keys stored" to "No keys stored"
    - Updated description: "Add keys for API keys, tokens, and other configuration values"

  - **Table structure** (lines 39-73):
    - **Headers** (lines 40-46): Changed columns from [Name, Service Type, API Key, Description, Last Updated, Actions] to [Key, Value, Description, Last Updated, Actions]
    - **Template key binding** (line 49): Changed `:key="apiKey.id"` to `:key="apiKey.key"` (use key field as unique identifier)
    - **Key column** (line 51): Changed `x-text="apiKey.name"` to `x-text="apiKey.key"`
    - **Removed service_type** (lines 52-55): Completely removed service type badge column
    - **Value column** (lines 52-59): Kept masked value display with visibility toggle, but updated references:
      - Line 55: Changed `@click="toggleApiKeyVisibility(apiKey.id)"` to `@click="toggleApiKeyVisibility(apiKey.key)"`
      - Line 55: Changed `:title="getShowFull(apiKey.id)` to `:title="getShowFull(apiKey.key)"`
      - Line 56: Changed `:class="getShowFull(apiKey.id)` to `:class="getShowFull(apiKey.key)"`
    - **Action buttons** (lines 63-68): Updated to use key instead of id:
      - Line 63: Updated title from "Edit API Key" to "Edit Key"
      - Line 66: Changed `@click="deleteApiKey(apiKey.id, apiKey.name)"` to `@click="deleteApiKey(apiKey.key, apiKey.key)"`
      - Line 66: Changed `:disabled="deleting === apiKey.id"` to `:disabled="deleting === apiKey.key"`
      - Line 66: Updated title from "Delete API Key" to "Delete Key"
      - Line 67: Changed `:class="deleting === apiKey.id"` to `:class="deleting === apiKey.key"`

  - **Modal form** (lines 77-120):
    - **Key field** (lines 88-92):
      - Changed label from "Name *" to "Key *"
      - Changed `id`, `x-model` from "name" to "key"
      - Updated placeholder: "e.g., gemini-llm-key or google-places-api-key"
      - Added `:disabled="showEditModal"` attribute to prevent editing key on update (line 90)
      - Updated hint: "Unique identifier for this key/value pair (used in job definitions)"
    - **Removed service_type field entirely** (lines 98-108 removed)
    - **Value field** (lines 94-102):
      - Changed label from "API Key *" to "Value *"
      - Changed `id`, `x-model` from "apiKey" to "value"
      - Updated placeholder: "Enter value (API key, token, or secret)"
      - Kept password visibility toggle functionality
    - **Description field** (lines 104-107):
      - Updated placeholder: "Optional description (e.g., 'Google Gemini API key for LLM service')"

**Key UX improvements:**
- Key field disabled during edit mode (prevents accidental key changes)
- Clearer placeholder examples showing real-world usage
- Simplified form structure (removed unnecessary service_type dropdown)
- All button titles updated from "API Key" to "Key" for consistency

### Agent 3 - Validation
**Skill:** @go-coder

**Code Quality:**
✅ Complete removal of service_type references
✅ All field name changes applied consistently (name→key, apiKey→value)
✅ Table columns updated to match new data model
✅ Form fields properly updated with correct x-model bindings
✅ Accessibility: All label `for` attributes match input `id` values
✅ UX improvement: Key field disabled during edit (line 90)
✅ Clear user guidance with updated placeholders and hints
✅ Consistent terminology throughout ("Key" instead of "API Key")

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
HTML template successfully updated to match new KV data model. All references to old fields (name, service_type, id) removed and replaced with new structure (key, value). Form properly configured with disabled key field during edits.

**→ Continuing to Step 5**
