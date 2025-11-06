# Implementation Checklist

Use this checklist to track progress during implementation of the edit job button fixes.

## Phase 1: Frontend - Navigation Change
- [ ] **Step 1**: Update `editJobDefinition()` in `pages/static/common.js`
  - [ ] Replace modal logic with navigation: `window.location.href = '/job_add?id=' + jobDef.id`
  - [ ] Keep system job validation check
  - [ ] Test: Click edit button navigates to correct URL

## Phase 2: Frontend - Job Add Page Enhancement
- [ ] **Step 2**: Add URL parameter detection to `pages/job_add.html`
  - [ ] Add `jobDefId` property to Alpine component state
  - [ ] In `init()`, check for `?id=` parameter using URLSearchParams
  - [ ] Create `loadJobDefinition(id)` method
  - [ ] Fetch from `/api/job-definitions/{id}/export`
  - [ ] Load TOML into editor using `editor.setValue()`
  - [ ] Handle errors (job not found, network issues)
  - [ ] Test: Navigate to /job_add?id=xyz loads job

- [ ] **Step 3**: Dynamic page title
  - [ ] Add computed property for page mode (edit vs create)
  - [ ] Update h1 with `x-text="pageTitle"`
  - [ ] Test: Title shows "Edit" when ID present, "Add" otherwise

- [ ] **Step 4**: Simplify save buttons
  - [ ] Remove "Create Job" button (lines 112-115)
  - [ ] Remove "Save as Invalid" button (lines 116-119)
  - [ ] Add single "Save" button
  - [ ] Update button text and click handler
  - [ ] Test: Only one save button visible

- [ ] **Step 5**: Merge save logic
  - [ ] Rename/merge `createJob()` and `saveAsInvalid()` into `saveJob()`
  - [ ] Check if `jobDefId` exists
  - [ ] Call validate endpoint first
  - [ ] If valid: call upload endpoint
  - [ ] If invalid: prompt user, then call save-invalid endpoint
  - [ ] Handle success/error responses
  - [ ] Redirect to /jobs on success
  - [ ] Test: Create new job works
  - [ ] Test: Update existing job works
  - [ ] Test: Invalid TOML prompts user

- [ ] **Step 6**: TOML/JSON conversion (if needed)
  - [ ] Verify upload endpoint handles TOML for updates
  - [ ] If needed, add conversion logic
  - [ ] Test: Both create and update work with same endpoint

- [ ] **Step 8**: Loading states
  - [ ] Add `isLoading` state property
  - [ ] Show spinner while fetching job definition
  - [ ] Disable editor and buttons while loading
  - [ ] Clear loading state after fetch
  - [ ] Test: Loading indicator appears and disappears

- [ ] **Step 9**: Error handling
  - [ ] Handle job not found (404)
  - [ ] Handle system job export restriction
  - [ ] Handle network errors
  - [ ] Show user-friendly error messages
  - [ ] Provide recovery options (back button, clear editor)
  - [ ] Test: Invalid ID shows error
  - [ ] Test: Network error handled gracefully

## Phase 3: Backend - Handler Enhancement
- [ ] **Step 7**: Update `UploadJobDefinitionTOMLHandler` in `internal/handlers/job_definition_handler.go`
  - [ ] After parsing TOML, extract job ID
  - [ ] Call `GetJobDefinition(ctx, id)` to check if exists
  - [ ] If exists:
    - [ ] Verify not a system job (return 403 if system)
    - [ ] Call `UpdateJobDefinition(ctx, jobDef)`
    - [ ] Return 200 status code
  - [ ] If not exists:
    - [ ] Call `SaveJobDefinition(ctx, jobDef)` (current behavior)
    - [ ] Return 201 status code
  - [ ] Test: POST with existing ID updates job
  - [ ] Test: POST with new ID creates job
  - [ ] Test: POST with system job ID returns 403

## Phase 4: Testing
- [ ] **Step 10**: UI Tests (`test/ui/jobs_test.go`)
  - [ ] Add `TestEditJobNavigation` - verify edit button navigates
  - [ ] Add `TestJobAddLoadExisting` - verify job loads from URL param
  - [ ] Add `TestJobAddSaveUpdate` - verify save updates existing job
  - [ ] Add `TestJobAddSaveCreate` - verify save creates new job
  - [ ] Add `TestSystemJobEditDisabled` - verify system jobs protected
  - [ ] Add `TestInvalidJobId` - verify error handling
  - [ ] Run tests: `cd test/ui && go test -v`
  - [ ] All tests pass

## Phase 5: Documentation & Cleanup
- [ ] **Step 11**: Update documentation
  - [ ] Update CLAUDE.md with new edit flow
  - [ ] Document URL parameter pattern
  - [ ] Update any architecture diagrams if needed
  - [ ] Add notes about single Save button behavior

- [ ] **Step 12**: Manual testing and cleanup
  - [ ] Manual test: Create new job
    - [ ] Navigate to /job_add
    - [ ] Enter valid TOML
    - [ ] Click Save
    - [ ] Verify redirect to /jobs
    - [ ] Verify job appears in list
  - [ ] Manual test: Edit existing job
    - [ ] Navigate to /jobs
    - [ ] Click edit on a job
    - [ ] Verify URL has ?id=
    - [ ] Verify TOML loads in editor
    - [ ] Modify TOML
    - [ ] Click Save
    - [ ] Verify redirect to /jobs
    - [ ] Verify changes saved
  - [ ] Manual test: Edit system job
    - [ ] Navigate to /jobs
    - [ ] Verify edit button disabled for system jobs
  - [ ] Manual test: Save invalid TOML
    - [ ] Enter invalid TOML in editor
    - [ ] Click Save
    - [ ] Verify validation error shows
    - [ ] Accept prompt to save as draft
    - [ ] Verify saved with invalid- prefix
  - [ ] Manual test: Error cases
    - [ ] Navigate to /job_add?id=invalid-id
    - [ ] Verify error message
    - [ ] Verify recovery options work
  - [ ] Code cleanup
    - [ ] Remove unused modal-related code from common.js
    - [ ] Remove unused functions (old createJob, saveAsInvalid)
    - [ ] Update code comments
    - [ ] Remove console.log debugging statements
  - [ ] Browser testing
    - [ ] Test in Chrome
    - [ ] Test in Firefox
    - [ ] Test in Edge
    - [ ] Check for console errors
    - [ ] Verify responsive design works

## Final Verification
- [ ] All steps completed
- [ ] All tests passing
- [ ] No console errors
- [ ] No broken functionality
- [ ] Documentation updated
- [ ] Ready for code review

## Rollback Plan (If Needed)
If issues are discovered:
1. Revert `common.js` changes (restore modal logic)
2. Revert `job_add.html` changes (restore two buttons)
3. Revert backend handler changes
4. All functionality returns to original state

## Notes
Add any implementation notes or issues encountered below:

---
**Implementation Start Date**: _____________
**Completion Date**: _____________
**Implemented By**: _____________
