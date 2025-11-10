# Spec: Places List Management

**Capability:** `places-list`
**Status:** Proposed

## ADDED Requirements

### Requirement: Google Places API Configuration
The system MUST support configuration of Google Places API credentials and settings in `quaero.toml`.

#### Scenario: Configure API key and settings
**WHEN** administrator adds Places API section to config file with api_key, rate_limit, request_timeout, and max_results_per_search
**THEN** the Places service MUST initialize successfully
**AND** the rate limit MUST be enforced between API requests

---

### Requirement: Place Search Job Definition
The system MUST support job definitions with `places_search` action type.

#### Scenario: Define text search job
**WHEN** a TOML job definition contains action type `places_search` with search_query and search_type
**THEN** the job definition MUST be loaded and validated successfully
**AND** the job MUST appear in the Jobs UI

#### Scenario: Define nearby search job
**WHEN** a job definition specifies search_type as `nearby_search` with location coordinates
**THEN** the Places service MUST call the Nearby Search API endpoint
**AND** results MUST be filtered by the specified location and radius

---

### Requirement: Place List Storage
The system MUST persist place search results in dedicated database tables `places_lists` and `places_items`.

#### Scenario: Store list metadata
**WHEN** a place search completes
**THEN** a record MUST be created in `places_lists` with id, job_id, name, search_query, search_type, total_results, status, and timestamps

#### Scenario: Store individual place items
**WHEN** Google Places API returns results
**THEN** each place MUST be stored in `places_items` with place_id, name, address, phone, website, rating, latitude, longitude, and metadata

---

### Requirement: Place Search Execution
The system MUST execute place search jobs asynchronously via the job executor framework.

#### Scenario: Execute text search
**WHEN** a job with `places_search` action is triggered
**THEN** PlacesSearchStepExecutor MUST call Google Places Text Search API
**AND** results MUST be parsed and stored in the database
**AND** job status MUST transition from pending → running → completed

#### Scenario: Handle API errors
**WHEN** Google Places API returns an error response
**THEN** the job MUST fail with status "failed"
**AND** the error message MUST be stored and logged

---

### Requirement: Google Places API Integration
The system MUST integrate with Google Places API endpoints for Text Search and Nearby Search.

#### Scenario: Call Text Search API
**WHEN** PlacesService executes a text search with query "coffee shops in Seattle"
**THEN** the service MUST make HTTP GET request to https://maps.googleapis.com/maps/api/place/textsearch/json with query parameter

#### Scenario: Parse API response
**WHEN** Google Places API returns results with name, address, rating, website, and place_id
**THEN** each field MUST be extracted and stored in PlaceItem model

---

### Requirement: Event System Integration
The system MUST emit events for place search lifecycle milestones.

#### Scenario: Emit search started event
**WHEN** a place search job begins execution
**THEN** an EventPlacesSearchStarted MUST be published with list_id, job_id, search_query, and timestamp

#### Scenario: Emit search completed event
**WHEN** all results are stored and list status is updated
**THEN** an EventPlacesSearchCompleted MUST be published with total_results count

---

### Requirement: Job UI Integration
The system MUST display place search jobs in the existing Jobs UI.

#### Scenario: Display place search job
**WHEN** user navigates to Jobs page
**THEN** place search jobs MUST appear in the jobs list with name, status, and search query

#### Scenario: Show execution progress
**WHEN** place search job is running
**THEN** UI MUST update in real-time via WebSocket
**AND** progress MUST show results_found count

---

### Requirement: Place List Retrieval
The system MUST provide interfaces to retrieve and query place lists.

#### Scenario: Get list by ID
**WHEN** PlacesService.GetList() is called with a valid list_id
**THEN** the service MUST return the PlacesList record with metadata

#### Scenario: List all place lists
**WHEN** PlacesService.ListPlacesLists() is called
**THEN** the service MUST return all lists ordered by creation date descending

---

### Requirement: Configuration Validation
The system MUST validate place search job configurations before execution.

#### Scenario: Validate required fields
**WHEN** job configuration is missing search_query
**THEN** validation MUST fail with error "search_query is required"

#### Scenario: Validate search type
**WHEN** job configuration has invalid search_type
**THEN** validation MUST fail with error "search_type must be one of: text_search, nearby_search"

---

### Requirement: Rate Limiting
The system MUST enforce rate limiting to comply with Google Places API quotas.

#### Scenario: Enforce minimum request interval
**WHEN** multiple API requests are made
**THEN** the service MUST enforce at least the configured rate_limit duration between requests

---

### Requirement: API Key Security
The system MUST securely handle Google Places API keys.

#### Scenario: Redact API key in logs
**WHEN** API requests are logged
**THEN** the API key MUST be redacted and appear as api_key=***REDACTED***

#### Scenario: Support environment variable override
**WHEN** environment variable QUAERO_PLACES_API_KEY is set
**THEN** the service MUST use the environment variable value instead of config file

---

### Requirement: Example Job Definitions
The system MUST provide example job definitions for common use cases.

#### Scenario: Provide text search example
**WHEN** user navigates to deployments/local/job-definitions/places/ directory
**THEN** a file text-search-example.toml MUST exist with working configuration and explanatory comments

#### Scenario: Provide nearby search example
**WHEN** user navigates to deployments/local/job-definitions/places/ directory
**THEN** a file nearby-search-example.toml MUST exist with location and radius configuration
