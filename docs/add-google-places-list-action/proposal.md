# Add Google Places List Action

**Change ID:** `add-google-places-list-action`
**Status:** Proposed
**Created:** 2025-11-10
**Author:** AI Assistant (via user request)

## Why

Quaero's crawler currently requires manually-provided seed URLs or pre-defined source patterns. To enable dynamic discovery of web content based on real-world business locations, we need Google Places API integration to automatically generate place lists that can drive future crawler operations.

## What Changes

- New job action type `places_search` alongside existing `crawl`, `transform`, `embed`
- Google Places API client for Text Search, Nearby Search, Place Details
- Database tables: `places_lists` and `places_items` for storing search results
- Configuration in `quaero.toml` for API key and settings
- Job executor integration via `PlacesSearchStepExecutor`
- Example TOML job definitions in `deployments/local/job-definitions/places/`
- Jobs UI displays place search jobs with status tracking

## Impact

**Affected Specs:**
- NEW: `places-list` - Place list management and Google Places API integration

**Affected Code:**
- `internal/common/config.go` - Add Places API configuration
- `internal/storage/sqlite/schema.go` - Add places_lists and places_items tables
- `internal/services/places/` - NEW service for Google Places API
- `internal/jobs/executor/` - Add PlacesSearchStepExecutor
- `internal/app/app.go` - Register new service and executor
- `deployments/local/job-definitions/places/` - Example job definitions

**Breaking Changes:** None
