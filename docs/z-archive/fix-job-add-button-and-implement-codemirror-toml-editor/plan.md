# Fix Job Add Button and Implement CodeMirror TOML Editor

## Overview

**Goal:** Fix the non-functional "Add Job" button on the Job Management page and replace the current TOML textarea with CodeMirror editor for better editing experience.

**Complexity:** HIGH

**Reason for High Complexity:**
- Frontend library integration (CodeMirror) requires careful CDN/npm package selection
- Backend service implementation for TOML validation
- Database schema changes (add validation_status field)
- Multiple file modifications across frontend and backend layers
- Testing required for validation logic and UI integration

## Problem Analysis

### Issue 1: Add Job Button Not Working

**Location:** `C:\development\quaero\pages\jobs.html` (Line 109-112)

```html
<button class="btn btn-sm btn-primary">
    <i class="fas fa-plus"></i>
    <span>Add Job</span>
</button>
```

**Problem:** Button has no click handler or navigation logic. The Alpine.js component `jobDefinitionsManagement` is defined but the button doesn't call any method.

**Root Cause:** Missing `@click` directive or `href` attribute to navigate to `/job_add` page.

### Issue 2: Current TOML Editor Implementation

**Location:** `C:\development\quaero\pages\job_add.html`

**Current State:**
- CodeMirror is ALREADY included via CDN (lines 8-10, 169-170)
- CodeMirror editor is ALREADY initialized in Alpine.js component (lines 186-196)
- TOML syntax highlighting is ALREADY enabled (mode: 'toml', theme: 'monokai')
- Validation endpoint ALREADY exists: `POST /api/job-definitions/validate` (handler in `internal/handlers/job_definition_handler.go:687`)

**Findings:** The CodeMirror editor is ALREADY implemented! The screenshots from the user show this working. The task description may be based on outdated information or the user wants to UPGRADE the CodeMirror version or add additional features.

### Issue 3: Validation Status Storage

**Current State:**
- JobDefinition model exists at `C:\development\quaero\internal\models\job_definition.go`
- Validation happens via `ValidateJobDefinitionTOMLHandler` (line 687 in job_definition_handler.go)
- Validation returns JSON response but does NOT store validation status
- No `validation_status` field exists in JobDefinition model

**Requirement:** Add `validation_status` field to track whether TOML has been validated and store result.

## Decision Points for User

### 1. CodeMirror Version Choice

**Current:** CodeMirror v5.65.2 (from CDN)

**Options:**
- **Keep v5.65.2** - Already working, stable, good documentation
- **Upgrade to v6.x** - Modern modular architecture, better performance, but requires different setup
- **Use npm package** - Better for version control and offline development

**Recommendation:** Keep v5.65.2 unless user specifically wants v6 features (collaborative editing, better mobile support, etc.)

### 2. Validation Status Field Design

**Options:**

**Option A: Simple Boolean Status**
```go
ValidationStatus bool   `json:"validation_status"` // true = valid, false = invalid
ValidatedAt      time.Time `json:"validated_at"`
```

**Option B: Detailed Status with Error Messages**
```go
ValidationStatus  string    `json:"validation_status"` // "valid", "invalid", "not_validated"
ValidationMessage string    `json:"validation_message"` // Error message if invalid
ValidatedAt       time.Time `json:"validated_at"`
```

**Recommendation:** Option B - provides more context for debugging and UI display.

### 3. Validation Status Display on Jobs List

**Options:**
- **Badge indicator** - Show colored badge (green=valid, red=invalid, gray=not validated)
- **Icon only** - Show checkmark/warning icon
- **Tooltip** - Show validation message on hover

**Recommendation:** Badge + tooltip for best UX.

## Implementation Steps

### Step 1: Fix Add Job Button
**Files:** `C:\development\quaero\pages\jobs.html`

**Changes:**
```html
<!-- Line 109-112: Add @click handler to navigate to job_add page -->
<button class="btn btn-sm btn-primary" @click="window.location.href='/job_add'">
    <i class="fas fa-plus"></i>
    <span>Add Job</span>
</button>
```

**Alternative (better for Alpine.js consistency):**
```javascript
// In Alpine.js component jobDefinitionsManagement
addJob() {
    window.location.href = '/job_add';
}
```

```html
<button class="btn btn-sm btn-primary" @click="addJob()">
    <i class="fas fa-plus"></i>
    <span>Add Job</span>
</button>
```

**Testing:**
- Click button and verify navigation to `/job_add` page
- Verify page loads correctly with empty TOML editor

---

### Step 2: Add Validation Status Fields to JobDefinition Model
**Files:** `C:\development\quaero\internal\models\job_definition.go`

**Changes:**
```go
// Around line 118, add new fields to JobDefinition struct:
type JobDefinition struct {
    // ... existing fields ...

    // Validation fields
    ValidationStatus  string    `json:"validation_status" db:"validation_status"`   // "valid", "invalid", "not_validated"
    ValidationMessage string    `json:"validation_message" db:"validation_message"` // Error message if invalid
    ValidatedAt       time.Time `json:"validated_at" db:"validated_at"`            // Timestamp of last validation

    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}
```

**Database Migration:**
Create migration file: `C:\development\quaero\internal\storage\sqlite\migrations\add_validation_status.sql`

```sql
-- Add validation status fields to job_definitions table
ALTER TABLE job_definitions ADD COLUMN validation_status TEXT NOT NULL DEFAULT 'not_validated';
ALTER TABLE job_definitions ADD COLUMN validation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE job_definitions ADD COLUMN validated_at INTEGER NOT NULL DEFAULT 0;

-- Create index for querying by validation status
CREATE INDEX IF NOT EXISTS idx_job_definitions_validation_status ON job_definitions(validation_status);
```

**Testing:**
- Verify model compiles without errors
- Test database migration applies successfully
- Verify existing job definitions have default 'not_validated' status

---

### Step 3: Create Validation Service
**Files:** `C:\development\quaero\internal\services\validation\toml_validation_service.go` (NEW)

**Purpose:** Centralized TOML validation logic for reusability.

**Implementation:**
```go
package validation

import (
    "fmt"
    "github.com/pelletier/go-toml/v2"
    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/models"
    "github.com/ternarybob/quaero/internal/storage/sqlite"
)

type ValidationResult struct {
    IsValid bool
    Message string
    Errors  []string
}

type TOMLValidationService struct {
    logger arbor.ILogger
}

func NewTOMLValidationService(logger arbor.ILogger) *TOMLValidationService {
    return &TOMLValidationService{
        logger: logger,
    }
}

// ValidateTOML validates TOML content and returns detailed result
func (s *TOMLValidationService) ValidateTOML(tomlContent string) *ValidationResult {
    result := &ValidationResult{
        IsValid: true,
        Message: "TOML is valid",
        Errors:  []string{},
    }

    // Parse TOML
    var crawlerJob sqlite.CrawlerJobDefinitionFile
    if err := toml.Unmarshal([]byte(tomlContent), &crawlerJob); err != nil {
        result.IsValid = false
        result.Message = fmt.Sprintf("TOML syntax error: %v", err)
        result.Errors = append(result.Errors, err.Error())
        return result
    }

    // Validate crawler job file structure
    if err := crawlerJob.Validate(); err != nil {
        result.IsValid = false
        result.Message = fmt.Sprintf("Validation failed: %v", err)
        result.Errors = append(result.Errors, err.Error())
        return result
    }

    // Convert to JobDefinition and validate
    jobDef := crawlerJob.ToJobDefinition()
    if err := jobDef.Validate(); err != nil {
        result.IsValid = false
        result.Message = fmt.Sprintf("Job definition validation failed: %v", err)
        result.Errors = append(result.Errors, err.Error())
        return result
    }

    return result
}
```

**Testing:**
- Unit tests for valid TOML
- Unit tests for invalid TOML (syntax errors, missing fields, etc.)
- Unit tests for edge cases (empty TOML, malformed URLs, etc.)

---

### Step 4: Update JobDefinitionHandler to Use Validation Service
**Files:**
- `C:\development\quaero\internal\handlers\job_definition_handler.go`
- `C:\development\quaero\internal/app/app.go` (dependency injection)

**Changes to job_definition_handler.go:**

```go
// Around line 27, add validation service to handler struct:
type JobDefinitionHandler struct {
    jobDefStorage      interfaces.JobDefinitionStorage
    jobStorage         interfaces.JobStorage
    jobExecutor        *executor.JobExecutor
    authStorage        interfaces.AuthStorage
    validationService  *validation.TOMLValidationService  // NEW
    logger             arbor.ILogger
}

// Update constructor (around line 36):
func NewJobDefinitionHandler(
    jobDefStorage interfaces.JobDefinitionStorage,
    jobStorage interfaces.JobStorage,
    jobExecutor *executor.JobExecutor,
    authStorage interfaces.AuthStorage,
    validationService *validation.TOMLValidationService,  // NEW
    logger arbor.ILogger,
) *JobDefinitionHandler {
    // ... null checks ...

    return &JobDefinitionHandler{
        jobDefStorage:     jobDefStorage,
        jobStorage:        jobStorage,
        jobExecutor:       jobExecutor,
        authStorage:       authStorage,
        validationService: validationService,  // NEW
        logger:            logger,
    }
}

// Update ValidateJobDefinitionTOMLHandler (line 687):
func (h *JobDefinitionHandler) ValidateJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
    tomlContent, err := io.ReadAll(r.Body)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to read request body")
        WriteError(w, http.StatusBadRequest, "Failed to read request body")
        return
    }
    defer r.Body.Close()

    // Use validation service
    result := h.validationService.ValidateTOML(string(tomlContent))

    if result.IsValid {
        h.logger.Info().Msg("TOML validation successful")
        WriteJSON(w, http.StatusOK, map[string]interface{}{
            "status":  "valid",
            "message": result.Message,
        })
    } else {
        h.logger.Error().Strs("errors", result.Errors).Msg("TOML validation failed")
        WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
            "status":  "invalid",
            "message": result.Message,
            "errors":  result.Errors,
        })
    }
}
```

**Changes to app.go (dependency injection):**

```go
// Initialize validation service (before JobDefinitionHandler)
validationService := validation.NewTOMLValidationService(app.logger)

// Pass to handler constructor
app.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
    app.Storage,
    app.Storage,
    jobExecutor,
    app.Storage,
    validationService,  // NEW
    app.logger,
)
```

**Testing:**
- API test: POST `/api/job-definitions/validate` with valid TOML
- API test: POST `/api/job-definitions/validate` with invalid TOML
- Verify validation service is called correctly
- Verify error messages are descriptive

---

### Step 5: Update Save/Upload Handlers to Store Validation Status
**Files:** `C:\development\quaero\internal\handlers/job_definition_handler.go`

**Changes to UploadJobDefinitionTOMLHandler (line 738):**

```go
func (h *JobDefinitionHandler) UploadJobDefinitionTOMLHandler(w http.ResponseWriter, r *http.Request) {
    tomlContent, err := io.ReadAll(r.Body)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to read request body")
        WriteError(w, http.StatusBadRequest, "Failed to read request body")
        return
    }
    defer r.Body.Close()

    // Validate TOML using service
    result := h.validationService.ValidateTOML(string(tomlContent))

    // Parse and convert (even if validation fails, for save-invalid flow)
    var crawlerJob sqlite.CrawlerJobDefinitionFile
    if err := toml.Unmarshal(tomlContent, &crawlerJob); err != nil {
        h.logger.Error().Err(err).Msg("Invalid TOML syntax")
        WriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid TOML syntax: %v", err))
        return
    }

    jobDef := crawlerJob.ToJobDefinition()

    // Set validation status
    if result.IsValid {
        jobDef.ValidationStatus = "valid"
        jobDef.ValidationMessage = result.Message
    } else {
        jobDef.ValidationStatus = "invalid"
        jobDef.ValidationMessage = result.Message
    }
    jobDef.ValidatedAt = time.Now()

    // Store raw TOML
    jobDef.TOML = string(tomlContent)

    // Check if job exists (update vs create)
    ctx := r.Context()
    existingJobDef, err := h.jobDefStorage.GetJobDefinition(ctx, jobDef.ID)
    isUpdate := false

    if err == nil && existingJobDef != nil {
        if existingJobDef.IsSystemJob() {
            h.logger.Warn().Str("job_def_id", jobDef.ID).Msg("Cannot update system job via upload")
            WriteError(w, http.StatusForbidden, "Cannot update system-managed jobs")
            return
        }
        isUpdate = true
    }

    // Save or update
    if isUpdate {
        if err := h.jobDefStorage.UpdateJobDefinition(ctx, jobDef); err != nil {
            h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to update job definition")
            WriteError(w, http.StatusInternalServerError, "Failed to update job definition")
            return
        }
        h.logger.Info().
            Str("job_def_id", jobDef.ID).
            Str("validation_status", jobDef.ValidationStatus).
            Msg("Job definition updated with validation status")
        WriteJSON(w, http.StatusOK, jobDef)
    } else {
        if err := h.jobDefStorage.SaveJobDefinition(ctx, jobDef); err != nil {
            h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to save job definition")
            WriteError(w, http.StatusInternalServerError, "Failed to save job definition")
            return
        }
        h.logger.Info().
            Str("job_def_id", jobDef.ID).
            Str("validation_status", jobDef.ValidationStatus).
            Msg("Job definition created with validation status")
        WriteJSON(w, http.StatusCreated, jobDef)
    }
}
```

**Testing:**
- Upload valid TOML and verify validation_status="valid"
- Upload invalid TOML and verify validation_status="invalid"
- Verify validation_message contains error details
- Verify validated_at timestamp is set

---

### Step 6: Update Frontend to Show Validation Status on Jobs List
**Files:** `C:\development\quaero\pages\jobs.html`

**Changes around line 132-219 (job definition cards):**

```html
<template x-for="jobDef in jobDefinitions" :key="jobDef.id">
    <div class="card" style="margin-bottom: 0.8rem;">
        <div class="card-body">
            <div class="columns">
                <!-- Left side: Content -->
                <div class="column col-10">
                    <div style="display: flex; align-items: center; gap: 0.5rem;">
                        <div class="card-title h5" x-text="jobDef.name"></div>

                        <!-- System Job Badge -->
                        <span x-show="jobDef.job_type === 'system'" class="label label-warning"
                            title="System-managed job (readonly)">
                            <i class="fas fa-lock"></i> System
                        </span>

                        <!-- NEW: Validation Status Badge -->
                        <span x-show="jobDef.validation_status === 'valid'"
                            class="label label-success"
                            :title="'Validated: ' + (jobDef.validation_message || 'TOML is valid')">
                            <i class="fas fa-check-circle"></i> Valid
                        </span>
                        <span x-show="jobDef.validation_status === 'invalid'"
                            class="label label-error"
                            :title="'Invalid: ' + (jobDef.validation_message || 'Validation failed')">
                            <i class="fas fa-exclamation-circle"></i> Invalid
                        </span>
                        <span x-show="!jobDef.validation_status || jobDef.validation_status === 'not_validated'"
                            class="label label-secondary"
                            title="TOML has not been validated">
                            <i class="fas fa-question-circle"></i> Not Validated
                        </span>
                    </div>

                    <!-- Rest of card content unchanged -->
                    <div class="card-subtitle text-gray"
                        x-text="jobDef.description || 'No description provided'"></div>

                    <!-- ... existing metadata ... -->
                </div>

                <!-- Right side: Actions unchanged -->
                <div class="column col-2 text-right">
                    <!-- ... existing buttons ... -->
                </div>
            </div>
        </div>
    </div>
</template>
```

**CSS for validation badges (add to `C:\development\quaero\pages\static\quaero.css` if needed):**
```css
.label-success {
    background-color: #32b643;
    color: white;
}

.label-error {
    background-color: #e85600;
    color: white;
}

.label-secondary {
    background-color: #bcc3ce;
    color: #303742;
}
```

**Testing:**
- Verify valid jobs show green "Valid" badge
- Verify invalid jobs show red "Invalid" badge
- Verify unvalidated jobs show gray "Not Validated" badge
- Hover over badges and verify tooltips show validation messages

---

### Step 7: Update job_add.html to Show Persistent Validation Status
**Files:** `C:\development\quaero\pages\job_add.html`

**Changes to validation message display (around line 104):**

```html
<!-- Enhanced validation message with timestamp -->
<div x-show="validationMessage"
    class="validation-message"
    :class="validationStatus === 'success' ? 'success' : 'error'">
    <div style="display: flex; justify-content: space-between; align-items: center;">
        <span x-text="validationMessage"></span>
        <span x-show="validatedAt"
            style="font-size: 0.85rem; color: #666;"
            x-text="'Validated: ' + formatValidationTimestamp(validatedAt)"></span>
    </div>
</div>
```

**Changes to Alpine.js component (around line 173):**

```javascript
function jobAddPage() {
    return {
        editor: null,
        loading: false,
        isLoadingJob: false,
        jobDefId: null,
        validationMessage: '',
        validationStatus: '', // 'success' or 'error'
        validatedAt: null,    // NEW: Track validation timestamp

        // ... existing methods ...

        async loadJobDefinition(jobId) {
            this.isLoadingJob = true;
            this.validationMessage = '';
            this.validationStatus = '';
            this.validatedAt = null;  // Reset

            try {
                const response = await fetch(`/api/job-definitions/${jobId}/export`, {
                    method: 'GET',
                    headers: {
                        'Accept': 'text/plain'
                    }
                });

                if (response.ok) {
                    const tomlContent = await response.text();
                    this.editor.setValue(tomlContent);

                    // Fetch job definition to get validation status
                    const jobDefResponse = await fetch(`/api/job-definitions/${jobId}`);
                    if (jobDefResponse.ok) {
                        const jobDef = await jobDefResponse.json();
                        if (jobDef.validation_status === 'valid') {
                            this.validationStatus = 'success';
                            this.validationMessage = '✓ ' + (jobDef.validation_message || 'TOML is valid');
                        } else if (jobDef.validation_status === 'invalid') {
                            this.validationStatus = 'error';
                            this.validationMessage = '✗ ' + (jobDef.validation_message || 'Validation failed');
                        }
                        this.validatedAt = jobDef.validated_at;
                    }

                    this.editor.focus();
                } else {
                    const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
                    const errorMsg = `Failed to load job definition: ${errorData.error || response.statusText}`;
                    this.validationMessage = errorMsg;
                    this.validationStatus = 'error';
                    window.showNotification(errorMsg, 'error');
                    this.loadExample();
                }
            } catch (error) {
                const errorMsg = `Error loading job definition: ${error.message}`;
                this.validationMessage = errorMsg;
                this.validationStatus = 'error';
                window.showNotification(errorMsg, 'error');
                this.loadExample();
            } finally {
                this.isLoadingJob = false;
            }
        },

        formatValidationTimestamp(timestamp) {
            if (!timestamp) return '';
            const date = new Date(timestamp);
            return date.toLocaleString();
        }

        // ... rest of methods unchanged ...
    }
}
```

**Testing:**
- Load existing job definition and verify validation status shows
- Validate TOML and verify status updates immediately
- Verify timestamp displays correctly
- Save job and verify validation status persists after reload

---

### Step 8: Update Database Schema and Storage Layer
**Files:**
- `C:\development\quaero\internal\storage\sqlite\job_definition_storage.go`
- Database migration (create migration file)

**Database Migration (create new file):**
`C:\development\quaero\internal\storage\sqlite\migrations\009_add_validation_status.sql`

```sql
-- Add validation status fields to job_definitions table
ALTER TABLE job_definitions ADD COLUMN validation_status TEXT NOT NULL DEFAULT 'not_validated';
ALTER TABLE job_definitions ADD COLUMN validation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE job_definitions ADD COLUMN validated_at INTEGER NOT NULL DEFAULT 0;

-- Create index for querying by validation status
CREATE INDEX IF NOT EXISTS idx_job_definitions_validation_status ON job_definitions(validation_status);
```

**Changes to job_definition_storage.go (SaveJobDefinition method):**

```go
// Around line 80 (SaveJobDefinition method), ensure new fields are included:
func (s *JobDefinitionStorage) SaveJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
    // ... existing marshaling code ...

    query := `
        INSERT INTO job_definitions (
            id, name, type, job_type, description, toml, source_type, base_url, auth_id,
            steps, schedule, timeout, enabled, auto_start, config, pre_jobs, post_jobs,
            error_tolerance,
            validation_status, validation_message, validated_at,  -- NEW FIELDS
            created_at, updated_at
        ) VALUES (
            ?, ?, ?, ?, ?, ?, ?, ?, ?,
            ?, ?, ?, ?, ?, ?, ?, ?, ?,
            ?, ?, ?,  -- NEW PLACEHOLDERS
            ?, ?
        )
        ON CONFLICT(id) DO UPDATE SET
            name = excluded.name,
            type = excluded.type,
            job_type = excluded.job_type,
            description = excluded.description,
            toml = excluded.toml,
            source_type = excluded.source_type,
            base_url = excluded.base_url,
            auth_id = excluded.auth_id,
            steps = excluded.steps,
            schedule = excluded.schedule,
            timeout = excluded.timeout,
            enabled = excluded.enabled,
            auto_start = excluded.auto_start,
            config = excluded.config,
            pre_jobs = excluded.pre_jobs,
            post_jobs = excluded.post_jobs,
            error_tolerance = excluded.error_tolerance,
            validation_status = excluded.validation_status,      -- NEW
            validation_message = excluded.validation_message,    -- NEW
            validated_at = excluded.validated_at,                -- NEW
            updated_at = excluded.updated_at
    `

    validatedAtUnix := int64(0)
    if !jobDef.ValidatedAt.IsZero() {
        validatedAtUnix = jobDef.ValidatedAt.Unix()
    }

    _, err = s.db.ExecContext(ctx, query,
        jobDef.ID,
        jobDef.Name,
        jobDef.Type,
        jobDef.JobType,
        jobDef.Description,
        jobDef.TOML,
        jobDef.SourceType,
        jobDef.BaseURL,
        jobDef.AuthID,
        stepsJSON,
        jobDef.Schedule,
        jobDef.Timeout,
        jobDef.Enabled,
        jobDef.AutoStart,
        configJSON,
        preJobsJSON,
        postJobsJSON,
        errorToleranceJSON,
        jobDef.ValidationStatus,   // NEW
        jobDef.ValidationMessage,  // NEW
        validatedAtUnix,           // NEW
        jobDef.CreatedAt.Unix(),
        jobDef.UpdatedAt.Unix(),
    )

    return err
}
```

**Changes to GetJobDefinition and ListJobDefinitions methods:**
- Update SELECT queries to include new fields
- Update row scanning to populate new fields

```go
// Around line 150 (GetJobDefinition method):
query := `
    SELECT
        id, name, type, job_type, description, toml, source_type, base_url, auth_id,
        steps, schedule, timeout, enabled, auto_start, config, pre_jobs, post_jobs,
        error_tolerance,
        validation_status, validation_message, validated_at,  -- NEW
        created_at, updated_at
    FROM job_definitions
    WHERE id = ?
`

// Update row scanning:
var validatedAtUnix int64
err := row.Scan(
    &jobDef.ID,
    &jobDef.Name,
    &jobDef.Type,
    &jobDef.JobType,
    &jobDef.Description,
    &jobDef.TOML,
    &jobDef.SourceType,
    &jobDef.BaseURL,
    &jobDef.AuthID,
    &stepsJSON,
    &jobDef.Schedule,
    &jobDef.Timeout,
    &jobDef.Enabled,
    &jobDef.AutoStart,
    &configJSON,
    &preJobsJSON,
    &postJobsJSON,
    &errorToleranceJSON,
    &jobDef.ValidationStatus,   // NEW
    &jobDef.ValidationMessage,  // NEW
    &validatedAtUnix,           // NEW
    &createdAtUnix,
    &updatedAtUnix,
)

// Convert timestamps:
if validatedAtUnix > 0 {
    jobDef.ValidatedAt = time.Unix(validatedAtUnix, 0)
}
```

**Testing:**
- Run database migration and verify schema changes
- Create job definition and verify validation fields are saved
- Update job definition and verify validation fields are updated
- Query job definitions and verify validation fields are loaded correctly

---

### Step 9: End-to-End Testing

**Test Plan:**

1. **Add Job Button Test:**
   - Navigate to `/jobs` page
   - Click "Add Job" button
   - Verify navigation to `/job_add` page
   - Verify CodeMirror editor loads with empty/example content

2. **TOML Validation Test:**
   - Paste invalid TOML (syntax error) into editor
   - Click "Validate" button
   - Verify error message displays with details
   - Fix TOML and click "Validate" again
   - Verify success message displays

3. **Save with Validation Status Test:**
   - Create valid TOML job definition
   - Click "Validate" then "Save"
   - Navigate back to `/jobs` page
   - Verify job definition shows green "Valid" badge
   - Click job definition to edit
   - Verify validation status persists

4. **Invalid TOML Save Test:**
   - Create invalid TOML job definition
   - Click "Validate" (should fail)
   - Attempt to save (should prompt confirmation)
   - Confirm save of invalid TOML
   - Navigate back to `/jobs` page
   - Verify job definition shows red "Invalid" badge with error message

5. **Validation Status Display Test:**
   - Load `/jobs` page
   - Verify all job definitions show correct validation badges:
     - Valid jobs: green badge with checkmark
     - Invalid jobs: red badge with warning icon
     - Unvalidated jobs: gray badge with question mark
   - Hover over badges and verify tooltips show validation messages

6. **Edit Job Definition Test:**
   - Click "Edit" on existing job definition
   - Verify TOML content loads correctly
   - Verify validation status displays (if previously validated)
   - Modify TOML and click "Validate"
   - Verify validation status updates
   - Save and verify changes persist

7. **Migration Test:**
   - Backup database
   - Run migration script
   - Verify existing job definitions have `validation_status='not_validated'`
   - Create new job definition
   - Verify validation fields work correctly
   - Rollback migration and verify database integrity

**Automated Testing:**
- Unit tests for ValidationService
- Integration tests for validation endpoints
- UI tests for button click and navigation
- API tests for save/update with validation status

---

## Files Modified Summary

### Frontend Files:
1. `C:\development\quaero\pages\jobs.html` - Add click handler to Add Job button, show validation badges
2. `C:\development\quaero\pages\job_add.html` - Show persistent validation status with timestamp
3. `C:\development\quaero\pages\static\common.js` - Add validation timestamp formatting (optional enhancement)
4. `C:\development\quaero\pages\static\quaero.css` - Add validation badge styles (if not using Bulma defaults)

### Backend Files:
1. `C:\development\quaero\internal\models\job_definition.go` - Add validation status fields
2. `C:\development\quaero\internal\services\validation\toml_validation_service.go` - NEW service for validation logic
3. `C:\development\quaero\internal\handlers\job_definition_handler.go` - Update handlers to use validation service and store status
4. `C:\development\quaero\internal\storage\sqlite\job_definition_storage.go` - Update queries to include validation fields
5. `C:\development\quaero\internal\storage\sqlite\migrations\009_add_validation_status.sql` - NEW migration for schema changes
6. `C:\development\quaero\internal\app\app.go` - Initialize validation service and inject into handler

### Test Files (to be created):
1. `C:\development\quaero\test\api\job_definition_validation_test.go` - API tests for validation endpoints
2. `C:\development\quaero\test\unit\validation_service_test.go` - Unit tests for validation service
3. `C:\development\quaero\test\ui\job_add_button_test.go` - UI test for Add Job button navigation

---

## Rollback Plan

If implementation fails or causes issues:

1. **Database Rollback:**
   ```sql
   -- Remove validation columns
   ALTER TABLE job_definitions DROP COLUMN validation_status;
   ALTER TABLE job_definitions DROP COLUMN validation_message;
   ALTER TABLE job_definitions DROP COLUMN validated_at;
   DROP INDEX IF EXISTS idx_job_definitions_validation_status;
   ```

2. **Code Rollback:**
   - Revert commits for validation service
   - Remove validation fields from JobDefinition model
   - Restore original handler implementations
   - Remove validation badges from UI

3. **Verify System Functionality:**
   - Test job creation still works
   - Test job execution still works
   - Test existing job definitions load correctly

---

## Estimated Time to Complete

- **Step 1:** Fix Add Job Button - 15 minutes
- **Step 2:** Add Validation Fields to Model - 30 minutes
- **Step 3:** Create Validation Service - 1 hour
- **Step 4:** Update Handler to Use Service - 45 minutes
- **Step 5:** Update Save/Upload Handlers - 1 hour
- **Step 6:** Update Jobs List UI - 45 minutes
- **Step 7:** Update job_add.html UI - 45 minutes
- **Step 8:** Database Migration & Storage Updates - 1.5 hours
- **Step 9:** End-to-End Testing - 2 hours

**Total Estimated Time:** 8.5 hours

---

## Success Criteria

1. ✅ Add Job button navigates to `/job_add` page
2. ✅ CodeMirror editor is functional (already implemented)
3. ✅ TOML validation endpoint returns detailed error messages
4. ✅ Validation status is stored in database
5. ✅ Jobs list page shows validation badges with correct colors and tooltips
6. ✅ Validation status persists after page reload
7. ✅ Database migration applies successfully without data loss
8. ✅ All existing tests pass
9. ✅ New tests cover validation functionality

---

## Notes

- **CodeMirror is ALREADY implemented** - Screenshots show it working. User may want:
  - Different version (v6 instead of v5)
  - Different theme/configuration
  - Additional features (autocomplete, linting, etc.)

  **Action:** Clarify with user if they want to keep current CodeMirror implementation or make changes.

- **Validation Service Benefits:**
  - Centralized validation logic (DRY principle)
  - Easier to test in isolation
  - Reusable across different endpoints
  - Can be extended with additional validation rules

- **Database Migration Strategy:**
  - Use numbered migration files (009_add_validation_status.sql)
  - Apply migration on application startup via migration runner
  - Backward compatible with existing job definitions (default values provided)

- **Future Enhancements:**
  - Add "Re-validate" button on jobs list page
  - Batch validation for all unvalidated jobs
  - Validation history tracking (audit log)
  - TOML syntax linting in CodeMirror editor (real-time validation)
  - Autocomplete for TOML field names in editor

---

## Open Questions for User

1. **CodeMirror Version:** Keep v5.65.2 or upgrade to v6.x?
2. **Validation Badge Placement:** Current plan shows badges next to job name. Is this acceptable or prefer different location?
3. **Invalid TOML Save:** Should invalid TOML be saveable at all, or block save until fixed?
4. **Validation Auto-Trigger:** Should validation run automatically on TOML content change (debounced), or only on button click?
5. **Validation History:** Should we track validation history (multiple validation attempts), or only store latest validation result?

Please answer these questions before implementation begins to ensure alignment with requirements.
