# Design: Google Places List Action

## Architecture Overview

The Google Places integration follows Quaero's existing job execution architecture:

```
┌─────────────────────────────────────────┐
│  Job Definition (TOML)                  │
│  action: "places_search"                │
│  config: { search_query, max_results }  │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  JobExecutor                            │
│  → Dispatches to PlacesSearchExecutor   │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  PlacesSearchStepExecutor               │
│  → Creates PlacesService job            │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  PlacesService                          │
│  → Calls Google Places API              │
│  → Stores results in places_lists table │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│  Database: places_lists, places_items   │
│  → List metadata + individual places    │
└─────────────────────────────────────────┘
```

## Component Design

### 1. Configuration (Config File)

Add Google Places API configuration to `quaero.toml`:

```toml
[places_api]
api_key = "YOUR_GOOGLE_PLACES_API_KEY"
rate_limit = "1s"              # Minimum time between API requests
request_timeout = "30s"         # HTTP request timeout
max_results_per_search = 20    # Google Places API limit per request
enable_pagination = false      # Phase 2 feature
```

**Location:** `internal/common/config.go`
**Struct:** New `PlacesAPIConfig` struct added to `Config`

### 2. Job Definition (TOML)

Example job definition for place search:

```toml
# deployments/local/job-definitions/places/seattle-coffee-shops.toml

id = "places-seattle-coffee"
name = "Seattle Coffee Shops Search"
type = "custom"                # Uses custom action type
job_type = "user"
description = "Search for coffee shops in Seattle using Google Places API"
enabled = true
auto_start = false
schedule = ""                  # Manual execution only

[[steps]]
name = "search_coffee_shops"
action = "places_search"
on_error = "fail"

[steps.config]
search_query = "coffee shops in Seattle, WA"
search_type = "text_search"    # text_search, nearby_search, place_details
max_results = 20
list_name = "Seattle Coffee Shops"  # Optional: override auto-generated name

# Optional filters
[steps.config.filters]
min_rating = 4.0
open_now = false
price_level = [1, 2, 3]        # 1=$ to 4=$$$$

# Optional location bounds (for nearby_search)
[steps.config.location]
latitude = 47.6062
longitude = -122.3321
radius = 5000                  # meters
```

### 3. Database Schema

#### Table: `places_lists`
Stores metadata about each place search job/list.

```sql
CREATE TABLE places_lists (
    id TEXT PRIMARY KEY,                    -- UUID for the list
    job_id TEXT NOT NULL,                   -- Reference to jobs table
    name TEXT NOT NULL,                     -- User-friendly list name
    search_query TEXT NOT NULL,             -- Original search query
    search_type TEXT NOT NULL,              -- text_search, nearby_search, etc.
    total_results INTEGER DEFAULT 0,        -- Count of places in this list
    status TEXT NOT NULL,                   -- pending, running, completed, failed
    error TEXT,                             -- Error message if failed
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_places_lists_job_id ON places_lists(job_id);
CREATE INDEX idx_places_lists_status ON places_lists(status);
CREATE INDEX idx_places_lists_created_at ON places_lists(created_at DESC);
```

#### Table: `places_items`
Stores individual place details from Google Places API.

```sql
CREATE TABLE places_items (
    id TEXT PRIMARY KEY,                    -- UUID for the item record
    list_id TEXT NOT NULL,                  -- Reference to places_lists
    place_id TEXT NOT NULL,                 -- Google Places place_id (unique identifier)
    name TEXT NOT NULL,
    address TEXT,
    formatted_address TEXT,
    phone_number TEXT,
    international_phone_number TEXT,
    website TEXT,
    rating REAL,                            -- Google rating (0.0 - 5.0)
    user_ratings_total INTEGER,
    price_level INTEGER,                    -- 0-4 (free to very expensive)
    latitude REAL,
    longitude REAL,
    types TEXT,                             -- JSON array of place types
    opening_hours TEXT,                     -- JSON object with hours
    photos TEXT,                            -- JSON array of photo references
    metadata TEXT,                          -- JSON object for additional fields
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (list_id) REFERENCES places_lists(id) ON DELETE CASCADE
);

CREATE INDEX idx_places_items_list_id ON places_items(list_id);
CREATE INDEX idx_places_items_place_id ON places_items(place_id);
CREATE INDEX idx_places_items_rating ON places_items(rating DESC);
CREATE INDEX idx_places_items_name ON places_items(name);

-- FTS5 index for full-text search on place names and addresses
CREATE VIRTUAL TABLE places_items_fts USING fts5(
    name,
    address,
    formatted_address,
    content=places_items,
    content_rowid=rowid
);
```

### 4. Service Layer

#### New Service: `PlacesService`

**Location:** `internal/services/places/service.go`

**Responsibilities:**
- Call Google Places API (Text Search, Nearby Search, Place Details)
- Parse API responses into structured data
- Store results in `places_lists` and `places_items` tables
- Handle rate limiting and retries
- Emit events for job progress (via EventService)

**Key Methods:**
```go
type PlacesService struct {
    config       *common.PlacesAPIConfig
    storage      interfaces.PlacesStorage
    eventService interfaces.EventService
    logger       arbor.ILogger
    httpClient   *http.Client
}

// SearchPlaces performs a Google Places API search and stores results
func (s *PlacesService) SearchPlaces(ctx context.Context, req SearchRequest) (string, error)

// GetPlaceDetails fetches detailed information for a specific place_id
func (s *PlacesService) GetPlaceDetails(ctx context.Context, placeID string) (*PlaceDetails, error)

// GetList retrieves a places list by ID with all items
func (s *PlacesService) GetList(ctx context.Context, listID string) (*PlacesList, error)

// ListPlacesLists returns all place lists with optional filtering
func (s *PlacesService) ListPlacesLists(ctx context.Context, opts ListOptions) ([]*PlacesList, error)
```

### 5. Job Executor Integration

#### New Executor: `PlacesSearchStepExecutor`

**Location:** `internal/jobs/executor/places_search_step_executor.go`

**Responsibilities:**
- Extract `places_search` step config from job definition
- Validate search parameters
- Call PlacesService.SearchPlaces()
- Update job status and progress
- Return list_id as step result

**Key Methods:**
```go
type PlacesSearchStepExecutor struct {
    placesService interfaces.PlacesService
    logger        arbor.ILogger
}

// ExecuteStep implements StepExecutor interface
func (e *PlacesSearchStepExecutor) ExecuteStep(
    ctx context.Context,
    step models.JobStep,
    jobDef *models.JobDefinition,
    parentJobID string,
) (string, error)

// GetStepType returns "places_search"
func (e *PlacesSearchStepExecutor) GetStepType() string
```

**Registration:**
Executor registered in `internal/app/app.go` during service initialization:
```go
placesSearchExecutor := executor.NewPlacesSearchStepExecutor(placesService, logger)
jobExecutor.RegisterStepExecutor(placesSearchExecutor)
```

### 6. API Integration

#### Google Places API Endpoints

**Text Search:**
```
GET https://maps.googleapis.com/maps/api/place/textsearch/json
  ?query={search_query}
  &key={api_key}
```

**Nearby Search:**
```
GET https://maps.googleapis.com/maps/api/place/nearbysearch/json
  ?location={lat},{lng}
  &radius={meters}
  &type={place_type}
  &key={api_key}
```

**Place Details:**
```
GET https://maps.googleapis.com/maps/api/place/details/json
  ?place_id={place_id}
  &fields=name,formatted_address,website,rating,opening_hours
  &key={api_key}
```

**Response Parsing:**
- Handle pagination via `next_page_token` (Phase 2)
- Extract and normalize place data
- Store in `places_items` table with proper type conversion

### 7. Storage Interface

#### New Interface: `PlacesStorage`

**Location:** `internal/interfaces/places_storage.go`

```go
type PlacesStorage interface {
    // List operations
    CreateList(ctx context.Context, list *PlacesList) error
    GetList(ctx context.Context, listID string) (*PlacesList, error)
    UpdateList(ctx context.Context, list *PlacesList) error
    DeleteList(ctx context.Context, listID string) error
    ListPlacesLists(ctx context.Context, opts *ListOptions) ([]*PlacesList, error)

    // Item operations
    AddPlaceItem(ctx context.Context, item *PlaceItem) error
    GetPlaceItem(ctx context.Context, itemID string) (*PlaceItem, error)
    ListPlaceItems(ctx context.Context, listID string) ([]*PlaceItem, error)
    DeletePlaceItem(ctx context.Context, itemID string) error

    // Bulk operations
    AddPlaceItemsBulk(ctx context.Context, items []*PlaceItem) error
}
```

**Implementation:** `internal/storage/sqlite/places_storage.go`

### 8. Event System Integration

New event types for place search progress:

```go
const (
    EventPlacesSearchStarted   = "places_search_started"
    EventPlacesSearchProgress  = "places_search_progress"
    EventPlacesSearchCompleted = "places_search_completed"
    EventPlacesSearchFailed    = "places_search_failed"
)
```

**Event Payloads:**
```go
// EventPlacesSearchStarted
{
    "list_id": "uuid",
    "job_id": "uuid",
    "search_query": "coffee shops in Seattle",
    "search_type": "text_search",
    "timestamp": "2025-11-10T12:00:00Z"
}

// EventPlacesSearchProgress
{
    "list_id": "uuid",
    "job_id": "uuid",
    "results_found": 15,
    "timestamp": "2025-11-10T12:00:05Z"
}

// EventPlacesSearchCompleted
{
    "list_id": "uuid",
    "job_id": "uuid",
    "total_results": 20,
    "search_query": "coffee shops in Seattle",
    "timestamp": "2025-11-10T12:00:10Z"
}
```

## Data Flow

### Sequence: Place Search Execution

```
1. User triggers job (via UI or scheduler)
   → JobDefinition with action="places_search" loaded from database

2. JobExecutor processes job definition
   → Dispatches to PlacesSearchStepExecutor

3. PlacesSearchStepExecutor validates config
   → Extracts search_query, search_type, max_results
   → Calls PlacesService.SearchPlaces()

4. PlacesService creates PlacesList record
   → Status: "running"
   → Publishes EventPlacesSearchStarted

5. PlacesService calls Google Places API
   → HTTP GET to Text Search or Nearby Search endpoint
   → Parses JSON response

6. PlacesService stores results
   → Inserts PlaceItem records into places_items table
   → Updates PlacesList.total_results
   → Publishes EventPlacesSearchProgress

7. PlacesService completes
   → Updates PlacesList.status = "completed"
   → Publishes EventPlacesSearchCompleted
   → Returns list_id to executor

8. JobExecutor marks job as completed
   → Stores list_id in job metadata
   → Job visible in UI with "completed" status
```

## Error Handling

### API Errors
- **Rate limit exceeded (429):** Retry with exponential backoff
- **Invalid API key (403):** Fail immediately, log error
- **Network timeout:** Retry up to 3 times
- **Invalid search query (400):** Fail immediately, log error

### Storage Errors
- **Database locked:** Retry with backoff (BusyTimeoutMS from config)
- **Constraint violation:** Log warning, continue with next item
- **Transaction failure:** Rollback, fail job

### Configuration Errors
- **Missing API key:** Fail job startup, display error in logs
- **Invalid config:** Fail validation, prevent job execution

## Security Considerations

### API Key Storage
- Store API key in `quaero.toml` config file
- Never log API key in plain text
- Redact API key in error messages and logs
- Consider environment variable override: `QUAERO_PLACES_API_KEY`

### Rate Limiting
- Respect Google Places API rate limits (default: 1 request/second)
- Implement exponential backoff for 429 responses
- Track API usage per job to prevent quota exhaustion

### Data Privacy
- Place search results contain public information only
- No personal data stored (reviews, user info excluded)
- Comply with Google Places API Terms of Service

## Performance Considerations

### API Call Optimization
- Batch place details requests when possible (Phase 2)
- Cache place results for 24 hours to avoid redundant API calls (Phase 2)
- Use `fields` parameter to request only needed data

### Database Performance
- Bulk insert place items in transactions (20 items per transaction)
- Index on frequently queried fields (list_id, place_id, rating)
- FTS5 index for fast text search on place names

### Memory Management
- Stream API responses, don't load entire JSON into memory
- Process place items in batches (20 at a time)
- Limit max results per search to prevent memory exhaustion

## Testing Strategy

### Unit Tests
- PlacesService: Mock Google Places API responses
- PlacesSearchStepExecutor: Mock PlacesService calls
- PlacesStorage: In-memory SQLite for fast tests

### Integration Tests
- End-to-end job execution with real database
- API integration tests (optional: use Google Places API sandbox)
- Storage tests with transaction rollback

### UI Tests
- Verify place search jobs visible in Jobs page (test/ui/)
- Test job status updates via WebSocket
- Verify error messages displayed correctly

## Migration Path

### Phase 1 → Phase 2 Migration
- Add `crawled` flag to `places_items` table
- Add `crawler_job_id` foreign key to `places_items`
- Extend job definition to support `post_jobs` for crawler integration
- Add UI for manual list editing and refinement

## Alternative Approaches Considered

### Approach 1: Direct Crawler Integration (Rejected)
- **Why rejected:** Violates single responsibility principle. Crawler should focus on crawling, not API integration.

### Approach 2: Generic API Action (Considered)
- **Why not chosen:** Too generic, difficult to provide good UX for place-specific features. Better to specialize for Places API first, generalize later if needed.

### Approach 3: Store as Documents (Rejected)
- **Why rejected:** Places are structured entities, not unstructured documents. Separate tables provide better query performance and data integrity.

## Open Design Questions

1. **Should we auto-fetch Place Details for each result?**
   - **Pro:** Complete data immediately available
   - **Con:** Additional API calls (cost), slower execution
   - **Decision:** Phase 1 uses Text/Nearby Search only. Phase 2 adds optional Place Details fetch.

2. **How to handle duplicate places across multiple searches?**
   - **Pro (Separate):** Simple, no deduplication logic
   - **Con (Separate):** Redundant data storage
   - **Decision:** Phase 1 allows duplicates. Phase 2 adds optional deduplication by `place_id`.

3. **Should lists be mutable or immutable?**
   - **Decision:** Immutable in Phase 1 (search creates new list). Mutable in Phase 2 (manual editing).
