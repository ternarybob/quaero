# Edit Job Flow - Architecture Diagram

## Current Flow (Before Changes)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Jobs Page                               │
│                      (pages/jobs.html)                          │
│                                                                 │
│  ┌───────────────────────────────────────────────────────┐    │
│  │ Job Definition Card                                   │    │
│  │                                                       │    │
│  │  Name: My Crawler                                    │    │
│  │  [Run] [Edit] [Download] [Delete]                    │    │
│  │          │                                            │    │
│  │          └─> @click="editJobDefinition(jobDef)"      │    │
│  └───────────────────────────────────────────────────────┘    │
│                          │                                      │
└──────────────────────────┼──────────────────────────────────────┘
                           │
                           ▼
              ┌─────────────────────────┐
              │ Opens Modal Dialog      │
              │ (in same page)          │
              │                         │
              │ [Form Fields]           │
              │ [Save] [Cancel]         │
              └─────────────────────────┘
```

## New Flow (After Changes)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Jobs Page                               │
│                      (pages/jobs.html)                          │
│                                                                 │
│  ┌───────────────────────────────────────────────────────┐    │
│  │ Job Definition Card                                   │    │
│  │                                                       │    │
│  │  Name: My Crawler                                    │    │
│  │  [Run] [Edit] [Download] [Delete]                    │    │
│  │          │                                            │    │
│  │          └─> window.location.href =                  │    │
│  │              "/job_add?id=my-crawler"                │    │
│  └───────────────────────────────────────────────────────┘    │
│                          │                                      │
└──────────────────────────┼──────────────────────────────────────┘
                           │
                           ▼ (Navigation)
┌──────────────────────────────────────────────────────────────────┐
│                      Job Add/Edit Page                           │
│                     (pages/job_add.html)                         │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ Page Title: "Edit Job Definition"                     │    │
│  │ (dynamic based on ?id= parameter)                      │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ CodeMirror TOML Editor                                 │    │
│  │                                                         │    │
│  │  # Job TOML content loaded from API                    │    │
│  │  id = "my-crawler"                                     │    │
│  │  name = "My Crawler"                                   │    │
│  │  ...                                                    │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
│  [Validate] [Save] [Load Example] [Clear]                      │
│              │                                                   │
│              └─> saveJob() - detects create vs update          │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Data Flow Diagram

### Edit Existing Job

```
User                 Frontend (job_add.html)           Backend
  │                           │                          │
  │  Click Edit              │                          │
  │  on Jobs Page            │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │  Navigate to             │                          │
  │  /job_add?id=xyz         │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │                           │  GET /api/job-defs/{id}/export
  │                           │ ─────────────────────>   │
  │                           │                          │
  │                           │  Returns TOML content    │
  │                           │ <─────────────────────   │
  │                           │                          │
  │  Editor populated        │                          │
  │  with TOML               │                          │
  │ <─────────────────────   │                          │
  │                           │                          │
  │  Edit TOML               │                          │
  │  content                 │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │  Click Save              │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │                           │  POST /api/job-defs/validate
  │                           │ ─────────────────────>   │
  │                           │                          │
  │                           │  Validation result       │
  │                           │ <─────────────────────   │
  │                           │                          │
  │                           │  POST /api/job-defs/upload
  │                           │  (backend detects        │
  │                           │   existing ID and        │
  │                           │   calls Update)          │
  │                           │ ─────────────────────>   │
  │                           │                          │
  │                           │  Success (200)           │
  │                           │ <─────────────────────   │
  │                           │                          │
  │  Redirect to /jobs       │                          │
  │ <─────────────────────   │                          │
  │                           │                          │
```

### Create New Job

```
User                 Frontend (job_add.html)           Backend
  │                           │                          │
  │  Navigate to             │                          │
  │  /job_add                │                          │
  │  (no ?id parameter)      │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │  Empty editor            │                          │
  │  OR example loaded       │                          │
  │ <─────────────────────   │                          │
  │                           │                          │
  │  Write TOML              │                          │
  │  content                 │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │  Click Save              │                          │
  │ ──────────────────────>  │                          │
  │                           │                          │
  │                           │  POST /api/job-defs/validate
  │                           │ ─────────────────────>   │
  │                           │                          │
  │                           │  Validation result       │
  │                           │ <─────────────────────   │
  │                           │                          │
  │                           │  POST /api/job-defs/upload
  │                           │  (backend detects new    │
  │                           │   ID and creates)        │
  │                           │ ─────────────────────>   │
  │                           │                          │
  │                           │  Success (201)           │
  │                           │ <─────────────────────   │
  │                           │                          │
  │  Redirect to /jobs       │                          │
  │ <─────────────────────   │                          │
  │                           │                          │
```

## Component Interaction Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Alpine.js Components                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  jobDefinitionsManagement (in jobs.html)                       │
│  ┌─────────────────────────────────────────────┐              │
│  │ • jobDefinitions: []                        │              │
│  │ • loadJobDefinitions()                      │              │
│  │ • editJobDefinition(jobDef) ───────────┐   │              │
│  │   └─> window.location.href =           │   │              │
│  │       "/job_add?id=" + jobDef.id       │   │              │
│  │ • executeJobDefinition()               │   │              │
│  │ • deleteJobDefinition()                │   │              │
│  └─────────────────────────────────────────────┘              │
│                                        │                        │
│                                        │ Navigation             │
│                                        ▼                        │
│  jobAddPage (in job_add.html)                                  │
│  ┌─────────────────────────────────────────────┐              │
│  │ • jobDefId: null                           │              │
│  │ • editor: CodeMirror                       │              │
│  │ • loading: false                           │              │
│  │ • validationStatus: ''                     │              │
│  │                                             │              │
│  │ • init()                                    │              │
│  │   ├─> Check URLSearchParams for ?id=      │              │
│  │   ├─> If ID exists: loadJobDefinition()   │              │
│  │   └─> Else: load example                   │              │
│  │                                             │              │
│  │ • loadJobDefinition(id)                    │              │
│  │   ├─> GET /api/job-defs/{id}/export       │              │
│  │   └─> editor.setValue(tomlContent)         │              │
│  │                                             │              │
│  │ • validateJob()                            │              │
│  │   └─> POST /api/job-defs/validate         │              │
│  │                                             │              │
│  │ • saveJob() ────────────────────┐         │              │
│  │   ├─> Validate first            │         │              │
│  │   ├─> If valid or user confirms │         │              │
│  │   └─> POST /api/job-defs/upload │         │              │
│  │       (backend handles           │         │              │
│  │        create vs update)         │         │              │
│  └─────────────────────────────────────────────┘              │
│                                        │                        │
└────────────────────────────────────────┼────────────────────────┘
                                         ▼
                           ┌──────────────────────────┐
                           │  Backend Handlers        │
                           ├──────────────────────────┤
                           │                          │
                           │  ExportJobDefinition     │
                           │  ValidateJobDefinition   │
                           │  UploadJobDefinition     │
                           │    ├─> Check if exists   │
                           │    ├─> Update if exists  │
                           │    └─> Create if new     │
                           │  SaveInvalidJobDef       │
                           │                          │
                           └──────────────────────────┘
```

## State Diagram - Save Button Logic

```
                          ┌─────────────┐
                          │ User Clicks │
                          │    Save     │
                          └──────┬──────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │ Validate TOML Content  │
                    │ POST /api/.../validate │
                    └────────┬───────────────┘
                             │
                ┌────────────┴────────────┐
                │                         │
                ▼                         ▼
        ┌──────────────┐          ┌──────────────┐
        │  Validation  │          │  Validation  │
        │    PASS      │          │    FAIL      │
        └──────┬───────┘          └──────┬───────┘
               │                         │
               ▼                         ▼
    ┌──────────────────┐       ┌─────────────────────┐
    │ Check jobDefId   │       │ Show Error Message  │
    │ (from URL param) │       │ Prompt: "Save as    │
    └────┬─────────────┘       │  draft anyway?"     │
         │                     └──────┬──────────────┘
    ┌────┴─────┐                      │
    │          │                ┌─────┴──────┐
    ▼          ▼                │            │
┌─────┐    ┌─────┐              ▼            ▼
│ Has │    │ No  │           ┌────┐      ┌─────┐
│ ID  │    │ ID  │           │Yes │      │ No  │
└──┬──┘    └──┬──┘           └─┬──┘      └──┬──┘
   │          │                 │            │
   │          │                 │            │
   ▼          ▼                 │            ▼
┌──────┐  ┌──────┐              │      ┌─────────┐
│Update│  │Create│              │      │ Cancel  │
└──┬───┘  └──┬───┘              │      └─────────┘
   │         │                  │
   │         │                  ▼
   │         │         POST /api/.../save-invalid
   │         │                  │
   │         │                  │
   └────┬────┘                  │
        │                       │
        ▼                       │
  POST /api/.../upload          │
  (Backend detects ID)          │
        │                       │
        └───────┬───────────────┘
                │
                ▼
        ┌───────────────┐
        │ Show Success  │
        │ Redirect to   │
        │    /jobs      │
        └───────────────┘
```

## File Structure Changes

```
pages/
├── jobs.html                    (✓ No changes needed in HTML)
├── job_add.html                 (✏️  Modified - add URL param support, merge buttons)
└── static/
    └── common.js                (✏️  Modified - simplify editJobDefinition)

internal/
└── handlers/
    └── job_definition_handler.go  (✏️  Modified - upload endpoint handles update)

test/
└── ui/
    └── jobs_test.go             (✏️  Modified - add edit navigation tests)

docs/
└── edit-job-button-fixes/
    ├── plan.json                (✅ Created)
    ├── SUMMARY.md               (✅ Created)
    └── ARCHITECTURE.md          (✅ This file)
```

## API Contract

### Existing Endpoints (No Changes)

| Endpoint | Method | Request | Response | Status Codes |
|----------|--------|---------|----------|--------------|
| `/api/job-definitions/{id}/export` | GET | - | TOML content (text/plain) | 200, 404, 400 (non-crawler) |
| `/api/job-definitions/validate` | POST | TOML content | `{"status": "valid", "message": "..."}` | 200 (valid), 400 (invalid) |
| `/api/job-definitions/save-invalid` | POST | TOML content | JobDefinition JSON | 201 |

### Modified Endpoint

| Endpoint | Method | Request | Response | Behavior Change |
|----------|--------|---------|----------|-----------------|
| `/api/job-definitions/upload` | POST | TOML content | JobDefinition JSON | **NEW**: Check if ID exists → Update if found → Create if new |

**Before**: Always creates new job
**After**: Smart endpoint that updates if ID exists, creates if new

### Status Code Changes

- Returns `201 Created` for new jobs
- Returns `200 OK` for updated jobs
- Returns `403 Forbidden` if trying to update system job
- Returns `404 Not Found` if ID doesn't exist and can't be created

## Security Considerations

### System Job Protection

System jobs are protected at multiple layers:

1. **UI Layer** (jobs.html):
   ```html
   <button ... :disabled="jobDef.job_type === 'system'" ...>
   ```

2. **Frontend Logic** (common.js):
   ```javascript
   if (jobDef.job_type === 'system') {
     // Don't navigate or show error
     return;
   }
   ```

3. **Backend Validation** (job_definition_handler.go):
   ```go
   if existingJobDef.IsSystemJob() {
     WriteError(w, http.StatusForbidden, "Cannot edit system-managed jobs")
     return
   }
   ```

### Input Validation

- TOML content is validated before saving
- Invalid TOML can only be saved explicitly (save-invalid endpoint)
- Job IDs are validated format
- All user input is sanitized

## Performance Considerations

- **Navigation**: Page load instead of modal = slightly slower but cleaner UX
- **Export Endpoint**: Returns TOML directly from storage (fast)
- **CodeMirror**: Already loaded on page, just needs content
- **Validation**: Happens before save (same as before)
- **No additional API calls** compared to current implementation

## Browser Compatibility

All features use standard web APIs:
- URLSearchParams (IE 11+, all modern browsers)
- Fetch API (all modern browsers)
- CodeMirror (already in use)
- Alpine.js (already in use)

No polyfills needed for target browsers.
