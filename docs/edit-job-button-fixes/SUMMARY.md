# Edit Job Button Fixes - Implementation Plan Summary

## Overview

This plan addresses two related issues with the job management UI:

1. **Edit Button Behavior**: Change the edit button to navigate to the job_add page with `?id=` parameter instead of opening a modal
2. **Save Button Simplification**: Replace "Create Job" and "Save as Invalid" buttons with a single "Save" button

## Current State Analysis

### Jobs Page (pages/jobs.html)
- **Edit Button**: Line 183 calls `editJobDefinition(jobDef, $event)`
- **Behavior**: Opens a modal dialog for editing (legacy approach)
- **Restriction**: System jobs (job_type='system') have the edit button disabled

### Job Add Page (pages/job_add.html)
- **Current Purpose**: Create new jobs via TOML upload
- **Two Save Buttons**:
  - "Create Job" (line 112-115): Validates first, only saves if valid
  - "Save as Invalid" (line 116-119): Saves without validation for draft/invalid jobs
- **No Edit Support**: Cannot load existing jobs for editing

### Alpine.js Component (pages/static/common.js)
- **jobDefinitionsManagement**: Lines 281-411
- **editJobDefinition()**: Lines 436-454 - opens modal, loads job data
- **Current Modal Flow**: `showEditModal = true`, manages focus, loads job definitions

## Proposed Solution Architecture

### Navigation Flow Change
```
BEFORE: Jobs Page -> Edit Button -> Modal Dialog -> Save
AFTER:  Jobs Page -> Edit Button -> /job_add?id={id} -> Save
```

### Save Button Logic
```
Single "Save" Button:
1. Validate TOML content
2. If valid: Save normally (create or update)
3. If invalid: Prompt user to save as draft
4. Detect create vs update based on URL parameter
```

## Key Implementation Steps

### 1. Frontend Changes (Pages)

**jobs.html**: No changes needed in HTML - only JavaScript function

**job_add.html**:
- Add URL parameter detection in `init()`
- Fetch job definition via `/api/job-definitions/{id}/export`
- Load TOML into CodeMirror editor
- Update page title dynamically (Add vs Edit)
- Replace two save buttons with one
- Merge save logic to handle both create and update

**common.js**:
- Simplify `editJobDefinition()` to just navigate: `window.location.href = '/job_add?id=' + jobDef.id`
- Keep system job check (disabled button)

### 2. Backend Changes (Handlers)

**job_definition_handler.go** - `UploadJobDefinitionTOMLHandler`:
- Check if job with ID already exists
- If exists: call `UpdateJobDefinition` (validate not system job)
- If new: call `SaveJobDefinition` (current behavior)
- Return 200 for update, 201 for create

This makes the upload endpoint smart enough to handle both scenarios.

### 3. API Endpoints Used

| Endpoint | Method | Purpose | Used When |
|----------|--------|---------|-----------|
| `/api/job-definitions/{id}/export` | GET | Get TOML content | Loading for edit |
| `/api/job-definitions/validate` | POST | Validate TOML | Before saving |
| `/api/job-definitions/upload` | POST | Create or update | Saving job |
| `/api/job-definitions/save-invalid` | POST | Save invalid draft | Saving invalid TOML |

## Technical Considerations

### URL Parameter Handling
```javascript
// In job_add.html init()
const urlParams = new URLSearchParams(window.location.search);
const jobDefId = urlParams.get('id');
if (jobDefId) {
  // Load existing job
  this.loadJobDefinition(jobDefId);
}
```

### CodeMirror Integration
```javascript
// Populate editor with fetched content
this.editor.setValue(tomlContent);
```

### Save Logic Decision Tree
```
User clicks "Save"
├─> Has jobDefId? (edit mode)
│   ├─> Yes: POST /api/job-definitions/upload (backend detects existing ID)
│   └─> No: POST /api/job-definitions/upload (creates new)
└─> Validation fails?
    └─> Prompt: "Save as draft?" -> POST /api/job-definitions/save-invalid
```

## System Job Protection

System jobs (`job_type='system'`) are protected at multiple layers:
1. **UI**: Edit button disabled (`jobDef.job_type === 'system'`)
2. **Backend**: Update/Delete handlers check `IsSystemJob()` and return 403
3. **Export**: System jobs can be viewed but not edited

## Testing Strategy

### UI Tests (test/ui/jobs_test.go)
- [ ] Click edit button navigates to `/job_add?id={id}`
- [ ] Job add page loads job definition from URL
- [ ] Save updates existing job (not create duplicate)
- [ ] System jobs cannot be edited (button disabled)
- [ ] Invalid ID shows error message
- [ ] Create new job still works (no URL parameter)

### Manual Tests
- [ ] Create new job via job_add page
- [ ] Edit existing job via edit button
- [ ] Save valid TOML in both modes
- [ ] Save invalid TOML (should prompt)
- [ ] Test system job edit restriction
- [ ] Verify redirect to /jobs after save
- [ ] Test error cases (invalid ID, network error)

## Dependencies

**No new dependencies required** - uses existing infrastructure:
- Alpine.js (already in use)
- CodeMirror (already in job_add.html)
- URLSearchParams (browser API)
- Fetch API (browser API)
- Existing backend endpoints

## Risk Assessment

### Low Risk
- Navigation change (simple window.location.href)
- Button text change (cosmetic)
- URL parameter reading (standard pattern)

### Medium Risk
- Merging save logic (requires careful testing)
- Backend handler modification (need to maintain backward compatibility)

### Mitigation
- Comprehensive testing of both create and edit flows
- Keep existing API contracts intact
- Add error handling for edge cases
- Manual testing before deployment

## Implementation Order

The 12 steps in the plan should be implemented in sequence:

1. Update `editJobDefinition()` to navigate (quick win)
2. Add URL parameter detection to `job_add.html`
3. Dynamic page title
4. Simplify buttons (remove extra save button)
5. Merge save logic
6. Add TOML to JSON conversion for updates
7. **Backend**: Update upload handler to support update
8. Add loading states
9. Error handling
10. UI tests
11. Documentation
12. Manual testing and cleanup

**Estimated effort**: Steps 1-6 (Frontend) = 2-3 hours, Step 7 (Backend) = 1 hour, Steps 8-12 (Testing/Polish) = 2 hours

## Success Criteria

✅ Edit button navigates to job_add page with ID parameter
✅ Job add page loads and displays existing job TOML
✅ Single "Save" button handles both create and update
✅ System jobs remain protected from editing
✅ All existing functionality continues to work
✅ Tests pass
✅ No console errors
✅ Documentation updated

## Notes for Implementation

- The key insight is that the backend upload endpoint can be made smart enough to detect existing IDs and update instead of always creating new jobs
- This simplifies the frontend logic - no need for separate create vs update code paths
- The export endpoint already returns TOML, perfect for loading into the editor
- URLSearchParams is a standard browser API, no polyfill needed for modern browsers
- CodeMirror's `setValue()` method makes it easy to populate the editor programmatically
