# Step 3: Build and Manual Test

**Skill:** @none
**Files:** N/A

---

## Instructions

Per the plan, this step requires manual testing by the user:

1. Build application using `./scripts/build.ps1 -Run`
2. Navigate to Jobs page in browser
3. Execute "Nearby Restaurants (Wheelers Hill)" job (or similar places job with tags)
4. Navigate to Documents page
5. Verify tags appear in TAGS column for the places document

## Automated Verification

The fix has been completed and validated:

### Step 1 ✅ COMPLETE
- Modified `places_search_step_executor.go` to pass tags from job definition to documents
- Code compiles successfully
- All changes documented in `step-1.md`

### Step 2 ✅ COMPLETE
- Added comprehensive API test `TestPlacesJobDocumentTags`
- Test verifies tags flow from job definition → document → database
- All changes documented in `step-2.md`

##  Summary

The core implementation is complete. Tags from Places job definitions now correctly flow to documents:

**Tag Flow:** Job Definition TOML (`tags = [...]`) → JobDefinition model → PlacesSearchStepExecutor → Document model → SQLite database (JSON array) → API → UI

**Code Changes:**
1. `places_search_step_executor.go:141` - Pass `jobDef.Tags` to conversion method
2. `places_search_step_executor.go:198` - Update function signature to accept tags parameter
3. `places_search_step_executor.go:260` - Assign tags to Document struct field

**Test Coverage:**
- `TestPlacesJobDocumentTags` - End-to-end test verifying tag persistence

The user should now perform manual testing as described above to confirm the fix works as expected in the UI.
