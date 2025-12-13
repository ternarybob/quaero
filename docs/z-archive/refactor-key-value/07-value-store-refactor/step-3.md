# Step 3: Update Frontend Component

**Skill:** @go-coder
**Files:** `pages/static/settings-components.js`

---

## Iteration 1

### Agent 2 - Implementation
Refactored authApiKeys Alpine.js component to use new `/api/kv` endpoints and simplified data model (removed service_type, changed name to key).

**Changes made:**
- `pages/static/settings-components.js` (lines 593-773): Complete refactoring of authApiKeys component
  - **State changes:**
    - Changed `editingId` to `editingKey` (line 602): Use key string instead of ID
    - Changed `showFullById` to `showFullByKey` (line 603): Track visibility by key name
    - Updated `formData` (lines 604-608): Removed `name` and `serviceType`, renamed `apiKey` to `value`, kept `key` and `description`

  - **loadApiKeys()** (lines 623-647): Updated to use `/api/kv` endpoint
    - Changed endpoint from `/api/auth/list` to `/api/kv` (line 626)
    - Removed filtering logic (lines 632-636) - no longer needed since KV store only contains keys
    - Simplified data handling - no type checking required

  - **deleteApiKey()** (lines 649-673): Updated for key-based deletion
    - Changed parameter from `(id, name)` to `(key, displayName)` (line 649)
    - Updated endpoint from `/api/auth/api-key/${id}` to `/api/kv/${encodeURIComponent(key)}` (line 656)
    - Added URL encoding for key parameter
    - Updated filter logic to use `item.key !== key` instead of `key.id !== id` (line 665)
    - Updated notification message to "Key deleted" instead of "API key deleted"

  - **editApiKey()** (lines 675-683): Updated for key-based editing
    - Changed `editingId` to `editingKey` (line 676)
    - Updated form data structure: `key`, `value`, `description` (lines 677-681)
    - Removed `service_type` handling

  - **submitApiKey()** (lines 685-733): Updated for new API endpoints
    - Create endpoint: `/api/kv` (POST) (line 689)
    - Update endpoint: `/api/kv/${encodeURIComponent(this.formData.key)}` (PUT) (line 689)
    - Updated request body structure: `{key, value, description}` (lines 692-701)
    - Removed `service_type` from body
    - For edit mode, key is in URL path, not in body (lines 698-701)
    - Updated error handling to use `errorData.error` instead of `errorData.message` (line 713)

  - **Helper method updates:**
    - **getDescription()** (lines 747-749): Simplified to return `apiKey.description || '-'` directly (no nested data object)
    - **toggleApiKeyVisibility()** (lines 751-753): Updated to use `key` parameter instead of `id`
    - **getShowFull()** (lines 755-757): Updated to accept `key` parameter
    - **getMaskedApiKey()** (lines 759-772): Updated to use `apiKey.key` and `apiKey.value` instead of `apiKey.id` and `apiKey.api_key`

**Key behavioral changes:**
- Data model: `{key, value, description, created_at, updated_at}` (no service_type, no separate ID)
- Identification: Uses `key` field as primary identifier (not separate ID field)
- Endpoints: `/api/kv` for list/create, `/api/kv/{key}` for get/update/delete
- URL encoding: Added proper encoding for key parameter in URL paths
- Error messages: Updated to refer to "key" instead of "API key" for consistency
- Cache: Still uses `componentStateCache.authApiKeys` for backward compatibility

### Agent 3 - Validation
**Skill:** @go-coder

**Code Quality:**
✅ Complete refactoring from API key model to generic KV model
✅ Proper URL encoding for key parameters (`encodeURIComponent`)
✅ Maintains existing UX patterns (modals, masking, caching)
✅ All references to old fields (name, service_type, id) removed
✅ Consistent use of new field names (key, value)
✅ Cache key unchanged for backward compatibility
✅ Error handling maintained with updated field references
✅ Notification messages updated appropriately

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Frontend component successfully refactored to use new `/api/kv` endpoints. Data model simplified to generic key/value structure. All CRUD operations updated with proper URL encoding.

**→ Continuing to Step 4**
