# Plan: Unified Job Definitions Architecture

## Overview
Consolidate all job types (crawler, places, future types) into a single unified job definitions system where:
- All jobs use `job_definitions` table (no type-specific tables like `places_lists`)
- Job type is identified by `type` field in TOML (e.g., `type = "places"`)
- Type-specific data stored as JSON in `config` and `steps[].config` fields
- Job execution is agnostic - executors registered by action name (e.g., `places_search`)

## Current State Analysis
- **Job Definitions**: Already generic with `JobDefinition` model supporting steps and JSON config
- **Problem 1**: `places_lists` and `places_items` tables exist (type-specific schema)
- **Problem 2**: Places job definitions not loaded at startup alongside crawler jobs
- **Problem 3**: Job loading code in `load_job_definitions.go` may need to support all types

## Architecture Decision

**Option A: Job Definitions = Runtime Configuration Only**
- Job definitions store TOML config and steps
- Execution creates transient job records in `jobs` table
- Places search results stored in job progress/result JSON fields
- No `places_lists` or `places_items` tables

**Option B: Job Definitions + Execution Results Tables**
- Job definitions unified in `job_definitions` table
- Keep type-specific result tables (`places_lists`, `places_items`) for structured queries
- Tradeoff: Some duplication but better query performance for large result sets

**User Decision: No** - Requirements clearly state "specific context stored as variable fields (json), specific tables not created"
**Selected: Option A**

## Steps

### 1. **Remove Places-Specific Tables**
   - Skill: @code-architect
   - Files:
     - `internal/storage/sqlite/schema.go` (remove places tables DDL)
     - `internal/storage/sqlite/places_storage.go` (DELETE file)
     - `internal/storage/sqlite/manager.go` (remove PlacesStorage field/method)
     - `internal/interfaces/storage.go` (remove PlacesStorage interface)
     - `internal/interfaces/places_storage.go` (DELETE file)
   - User decision: no
   - Action: Remove all places-specific storage code

### 2. **Update Places Models to Store in JobDefinition**
   - Skill: @code-architect
   - Files:
     - `internal/models/places.go` (simplify to request/response models only)
     - `internal/services/places/service.go` (remove storage dependency, return results as JSON)
   - User decision: no
   - Action: Places service returns structured data for job to store in progress JSON

### 3. **Update Job Definitions Schema for Generic Type Support**
   - Skill: @go-coder
   - Files:
     - `internal/models/job_definition.go` (add `JobDefinitionTypePlaces` constant)
     - `internal/storage/sqlite/schema.go` (verify job_definitions supports all types)
   - User decision: no
   - Action: Add "places" as valid job type constant

### 4. **Update Job Definition Loader for All Types**
   - Skill: @go-coder
   - Files:
     - `internal/storage/sqlite/load_job_definitions.go` (ensure loads all *.toml regardless of type)
   - User decision: no
   - Action: Verify loader is type-agnostic (loads by file pattern, not type filtering)

### 5. **Refactor Places Step Executor**
   - Skill: @go-coder
   - Files:
     - `internal/jobs/executor/places_search_step_executor.go` (update to store results in job progress)
     - `internal/services/places/service.go` (return results as structured data)
   - User decision: no
   - Action: Executor stores places results in job's progress JSON field

### 6. **Update App Initialization**
   - Skill: @go-coder
   - Files:
     - `internal/app/app.go` (simplify PlacesService init, no storage dependency)
   - User decision: no
   - Action: Remove PlacesStorage from service initialization

### 7. **Build and Validate**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Action: Compile and verify no broken references

### 8. **Update Example Job Definitions**
   - Skill: @none
   - Files:
     - `deployments/local/job-definitions/seattle-coffee-shops-places.toml`
     - `deployments/local/job-definitions/nearby-restaurants-places.toml`
   - User decision: no
   - Action: Ensure TOMLs use unified structure, verify they load at startup

## Success Criteria

- ✅ No `places_lists` or `places_items` tables in schema
- ✅ All job types (crawler, places, future) use `job_definitions` table
- ✅ Places results stored in `jobs.progress` JSON field (via job execution)
- ✅ Job definition loader scans directory and loads all TOML files regardless of type
- ✅ Places step executor registered by action name (`places_search`)
- ✅ Application compiles successfully
- ✅ Example places jobs load at startup alongside crawler jobs

## Migration Notes

- **Breaking Change**: Existing places_lists/places_items data will be lost
- **No Migration Required**: User confirmed breaking changes acceptable
- **Result Storage**: Places search results now in `jobs` table progress/result fields
