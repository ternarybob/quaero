# Implementation Tasks: Google Places List Action

## Task Ordering Principles

- ✅ **User-visible progress:** Each task delivers testable functionality
- ✅ **Bottom-up dependencies:** Foundation → Integration → UI
- ✅ **Parallel opportunities:** Tasks marked with 🔄 can run concurrently
- ✅ **Validation checkpoints:** Each task includes test/verification step

---

## Phase 1: Foundation (Database & Configuration)

### Task 1.1: Add Places API Configuration to Config File
**Estimate:** 30 minutes
**Dependencies:** None
**Parallelizable:** 🔄

**Implementation:**
1. Edit `internal/common/config.go`:
   - Add `PlacesAPIConfig` struct with fields:
     - `APIKey string`
     - `RateLimit time.Duration`
     - `RequestTimeout time.Duration`
     - `MaxResultsPerSearch int`
   - Add `PlacesAPI PlacesAPIConfig` field to `Config` struct
   - Update `NewDefaultConfig()` with default values:
     - `RateLimit: 1 * time.Second`
     - `RequestTimeout: 30 * time.Second`
     - `MaxResultsPerSearch: 20`
   - Add environment variable override in `applyEnvOverrides()`:
     - `QUAERO_PLACES_API_KEY` → `config.PlacesAPI.APIKey`

2. Update `quaero.toml.example` with Places API section:
   ```toml
   [places_api]
   api_key = "YOUR_GOOGLE_PLACES_API_KEY"
   rate_limit = "1s"
   request_timeout = "30s"
   max_results_per_search = 20
   ```

**Validation:**
- ✅ Config loads successfully with Places API section
- ✅ Environment variable override works (`QUAERO_PLACES_API_KEY`)
- ✅ Default values applied when section missing

**Files Modified:**
- `internal/common/config.go` (~40 lines added)
- `quaero.toml.example` (~5 lines added)

---

### Task 1.2: Create Database Schema for Place Lists
**Estimate:** 45 minutes
**Dependencies:** None
**Parallelizable:** 🔄

**Implementation:**
1. Edit `internal/storage/sqlite/schema.go`:
   - Add `places_lists` table creation SQL:
     ```sql
     CREATE TABLE places_lists (
         id TEXT PRIMARY KEY,
         job_id TEXT NOT NULL,
         name TEXT NOT NULL,
         search_query TEXT NOT NULL,
         search_type TEXT NOT NULL,
         total_results INTEGER DEFAULT 0,
         status TEXT NOT NULL,
         error TEXT,
         created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
         updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
         FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
     );
     CREATE INDEX idx_places_lists_job_id ON places_lists(job_id);
     CREATE INDEX idx_places_lists_status ON places_lists(status);
     CREATE INDEX idx_places_lists_created_at ON places_lists(created_at DESC);
     ```

   - Add `places_items` table creation SQL:
     ```sql
     CREATE TABLE places_items (
         id TEXT PRIMARY KEY,
         list_id TEXT NOT NULL,
         place_id TEXT NOT NULL,
         name TEXT NOT NULL,
         address TEXT,
         formatted_address TEXT,
         phone_number TEXT,
         international_phone_number TEXT,
         website TEXT,
         rating REAL,
         user_ratings_total INTEGER,
         price_level INTEGER,
         latitude REAL,
         longitude REAL,
         types TEXT,
         opening_hours TEXT,
         photos TEXT,
         metadata TEXT,
         created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
         FOREIGN KEY (list_id) REFERENCES places_lists(id) ON DELETE CASCADE
     );
     CREATE INDEX idx_places_items_list_id ON places_items(list_id);
     CREATE INDEX idx_places_items_place_id ON places_items(place_id);
     CREATE INDEX idx_places_items_rating ON places_items(rating DESC);
     CREATE INDEX idx_places_items_name ON places_items(name);
     ```

   - Add FTS5 index for place search:
     ```sql
     CREATE VIRTUAL TABLE places_items_fts USING fts5(
         name,
         address,
         formatted_address,
         content=places_items,
         content_rowid=rowid
     );
     ```

2. Add schema upgrade migration (if needed)

**Validation:**
- ✅ Tables created successfully on fresh database
- ✅ Indexes created correctly
- ✅ FTS5 table created for text search
- ✅ Foreign key constraints work (CASCADE DELETE)
- ✅ Test with manual INSERT/SELECT queries

**Files Modified:**
- `internal/storage/sqlite/schema.go` (~60 lines added)

---

## Phase 2: Models & Interfaces

### Task 2.1: Create Places Data Models
**Estimate:** 30 minutes
**Dependencies:** Task 1.2 (schema)
**Parallelizable:** 🔄

**Implementation:**
1. Create `internal/models/places.go`:
   - Define `PlacesList` struct matching `places_lists` table
   - Define `PlaceItem` struct matching `places_items` table
   - Define `SearchRequest` struct for API requests:
     ```go
     type SearchRequest struct {
         SearchQuery  string
         SearchType   string // "text_search", "nearby_search"
         MaxResults   int
         ListName     string // Optional: override auto-generated name
         Location     *Location // For nearby_search
         Filters      *SearchFilters // Optional filters
     }
     ```
   - Define `Location` struct:
     ```go
     type Location struct {
         Latitude  float64
         Longitude float64
         Radius    int // meters
     }
     ```
   - Define `SearchFilters` struct:
     ```go
     type SearchFilters struct {
         MinRating   float64
         OpenNow     bool
         PriceLevel  []int // 1-4
     }
     ```
   - Add JSON marshal/unmarshal methods for database storage

**Validation:**
- ✅ Struct fields match database columns
- ✅ JSON serialization works for complex fields (types, opening_hours, photos)
- ✅ Compile check passes

**Files Created:**
- `internal/models/places.go` (~150 lines)

---

### Task 2.2: Define Places Storage Interface
**Estimate:** 20 minutes
**Dependencies:** Task 2.1 (models)
**Parallelizable:** 🔄

**Implementation:**
1. Create `internal/interfaces/places_storage.go`:
   - Define `PlacesStorage` interface:
     ```go
     type PlacesStorage interface {
         // List operations
         CreateList(ctx context.Context, list *models.PlacesList) error
         GetList(ctx context.Context, listID string) (*models.PlacesList, error)
         UpdateList(ctx context.Context, list *models.PlacesList) error
         DeleteList(ctx context.Context, listID string) error
         ListPlacesLists(ctx context.Context, opts *ListOptions) ([]*models.PlacesList, error)

         // Item operations
         AddPlaceItem(ctx context.Context, item *models.PlaceItem) error
         GetPlaceItem(ctx context.Context, itemID string) (*models.PlaceItem, error)
         ListPlaceItems(ctx context.Context, listID string) ([]*models.PlaceItem, error)
         DeletePlaceItem(ctx context.Context, itemID string) error

         // Bulk operations
         AddPlaceItemsBulk(ctx context.Context, items []*models.PlaceItem) error
     }
     ```

2. Update `internal/interfaces/storage.go` to include `PlacesStorage` if needed

**Validation:**
- ✅ Interface compiles successfully
- ✅ Methods cover all CRUD operations
- ✅ Bulk operations included for performance

**Files Created:**
- `internal/interfaces/places_storage.go` (~30 lines)

---

### Task 2.3: Define Places Service Interface
**Estimate:** 15 minutes
**Dependencies:** Task 2.1 (models)
**Parallelizable:** 🔄

**Implementation:**
1. Create `internal/interfaces/places_service.go`:
   - Define `PlacesService` interface:
     ```go
     type PlacesService interface {
         // Search operations
         SearchPlaces(ctx context.Context, req *models.SearchRequest) (string, error) // Returns list_id

         // Retrieval operations
         GetList(ctx context.Context, listID string) (*models.PlacesList, error)
         ListPlacesLists(ctx context.Context, opts *ListOptions) ([]*models.PlacesList, error)
         GetPlaceItem(ctx context.Context, itemID string) (*models.PlaceItem, error)
         ListPlaceItems(ctx context.Context, listID string) ([]*models.PlaceItem, error)
     }
     ```

**Validation:**
- ✅ Interface compiles successfully
- ✅ Methods align with use cases from spec

**Files Created:**
- `internal/interfaces/places_service.go` (~20 lines)

---

## Phase 3: Storage Implementation

### Task 3.1: Implement Places Storage (SQLite)
**Estimate:** 90 minutes
**Dependencies:** Task 1.2 (schema), Task 2.2 (interface)

**Implementation:**
1. Create `internal/storage/sqlite/places_storage.go`:
   - Implement `PlacesStorageImpl` struct with SQLite connection
   - Implement `CreateList()`:
     - INSERT into `places_lists`
     - Return error on constraint violation
   - Implement `GetList()`:
     - SELECT with JOIN to count items
     - Return nil if not found
   - Implement `UpdateList()`:
     - UPDATE `places_lists` SET ...
     - Update `updated_at` timestamp
   - Implement `DeleteList()`:
     - DELETE CASCADE handles items automatically
   - Implement `ListPlacesLists()`:
     - Support filtering by status, date range
     - ORDER BY created_at DESC
   - Implement `AddPlaceItem()`:
     - INSERT into `places_items`
     - Handle JSON serialization for complex fields
   - Implement `AddPlaceItemsBulk()`:
     - Use transaction for batch insert (20 items per tx)
     - Rollback on error
   - Implement `GetPlaceItem()`:
     - SELECT with JSON deserialization
   - Implement `ListPlaceItems()`:
     - SELECT WHERE list_id = ? ORDER BY name
   - Implement `DeletePlaceItem()`:
     - DELETE WHERE id = ?

2. Add constructor `NewPlacesStorage(db *sql.DB, logger arbor.ILogger)`

**Validation:**
- ✅ All interface methods implemented
- ✅ Unit tests pass (use in-memory SQLite)
- ✅ Transaction rollback works on error
- ✅ Bulk insert performance acceptable (< 1s for 20 items)

**Files Created:**
- `internal/storage/sqlite/places_storage.go` (~300 lines)
- `internal/storage/sqlite/places_storage_test.go` (~200 lines)

---

## Phase 4: Service Implementation

### Task 4.1: Implement Places Service (Core Logic)
**Estimate:** 120 minutes
**Dependencies:** Task 1.1 (config), Task 2.3 (interface), Task 3.1 (storage)

**Implementation:**
1. Create `internal/services/places/service.go`:
   - Implement `PlacesServiceImpl` struct:
     ```go
     type PlacesServiceImpl struct {
         config       *common.PlacesAPIConfig
         storage      interfaces.PlacesStorage
         eventService interfaces.EventService
         logger       arbor.ILogger
         httpClient   *http.Client
         rateLimiter  *time.Ticker // For rate limiting
     }
     ```

   - Implement `NewService()` constructor:
     - Initialize HTTP client with timeout
     - Create rate limiter ticker from config
     - Validate API key is not empty

   - Implement `SearchPlaces()`:
     - Generate UUID for list_id
     - Create PlacesList record with status "running"
     - Call appropriate API method based on search_type
     - Store results via `storage.AddPlaceItemsBulk()`
     - Update list status to "completed"
     - Publish events (started, progress, completed)
     - Return list_id

   - Implement `GetList()`:
     - Delegate to storage layer

   - Implement `ListPlacesLists()`:
     - Delegate to storage layer

   - Implement `GetPlaceItem()`:
     - Delegate to storage layer

   - Implement `ListPlaceItems()`:
     - Delegate to storage layer

**Validation:**
- ✅ Service initializes with valid config
- ✅ Mock API responses can be processed
- ✅ Events published correctly
- ✅ Error handling works (API errors, storage errors)

**Files Created:**
- `internal/services/places/service.go` (~250 lines)
- `internal/services/places/service_test.go` (~150 lines)

---

### Task 4.2: Implement Google Places API Client
**Estimate:** 90 minutes
**Dependencies:** Task 4.1 (service structure)

**Implementation:**
1. Create `internal/services/places/google_api.go`:
   - Implement `callTextSearchAPI()`:
     ```go
     func (s *PlacesServiceImpl) callTextSearchAPI(ctx context.Context, query string, maxResults int) ([]PlaceResult, error)
     ```
     - Build API URL: `https://maps.googleapis.com/maps/api/place/textsearch/json`
     - Add query parameters: `?query={query}&key={api_key}`
     - Make HTTP GET request
     - Parse JSON response
     - Extract results array
     - Return slice of PlaceResult structs

   - Implement `callNearbySearchAPI()`:
     ```go
     func (s *PlacesServiceImpl) callNearbySearchAPI(ctx context.Context, req *SearchRequest) ([]PlaceResult, error)
     ```
     - Build API URL with location, radius parameters
     - Parse response similar to text search

   - Implement `parseAPIResponse()`:
     - Map Google API fields to PlaceItem model
     - Handle missing fields gracefully (NULL in database)
     - Extract geometry.location for lat/lng

   - Implement rate limiting:
     - Wait for ticker before each API call
     - Ensure minimum interval between requests

   - Implement retry logic:
     - Exponential backoff on 429 (rate limit)
     - Max 3 retries
     - Fail on 403 (invalid API key)

**Validation:**
- ✅ Mock API responses parse correctly
- ✅ Rate limiter enforces minimum interval
- ✅ Retry logic works for 429 responses
- ✅ Error handling for invalid API keys

**Files Created:**
- `internal/services/places/google_api.go` (~200 lines)
- `internal/services/places/google_api_test.go` (~100 lines)

---

### Task 4.3: Add Event Definitions for Places
**Estimate:** 15 minutes
**Dependencies:** None
**Parallelizable:** 🔄

**Implementation:**
1. Edit `internal/interfaces/event_service.go`:
   - Add event type constants:
     ```go
     const (
         EventPlacesSearchStarted   = "places_search_started"
         EventPlacesSearchProgress  = "places_search_progress"
         EventPlacesSearchCompleted = "places_search_completed"
         EventPlacesSearchFailed    = "places_search_failed"
     )
     ```

**Validation:**
- ✅ Constants added successfully
- ✅ Compile check passes

**Files Modified:**
- `internal/interfaces/event_service.go` (~5 lines added)

---

## Phase 5: Job Executor Integration

### Task 5.1: Implement Places Search Step Executor
**Estimate:** 60 minutes
**Dependencies:** Task 4.1 (service), Task 2.1 (models)

**Implementation:**
1. Create `internal/jobs/executor/places_search_step_executor.go`:
   - Implement `PlacesSearchStepExecutor` struct:
     ```go
     type PlacesSearchStepExecutor struct {
         placesService interfaces.PlacesService
         logger        arbor.ILogger
     }
     ```

   - Implement `NewPlacesSearchStepExecutor()` constructor

   - Implement `ExecuteStep()`:
     - Extract step.Config map
     - Validate required fields (search_query, search_type)
     - Build SearchRequest struct from config
     - Call placesService.SearchPlaces()
     - Log execution details
     - Return list_id as result

   - Implement `GetStepType()`:
     - Return `"places_search"`

   - Implement config parsing helpers:
     - `parseSearchConfig(configMap map[string]interface{}) (*models.SearchRequest, error)`
     - Handle type assertions for string, int, float64 fields
     - Parse nested location config if present

**Validation:**
- ✅ Executor implements StepExecutor interface
- ✅ Config parsing handles all field types
- ✅ Validation errors returned for missing fields
- ✅ Service call succeeds with valid config

**Files Created:**
- `internal/jobs/executor/places_search_step_executor.go` (~150 lines)
- `internal/jobs/executor/places_search_step_executor_test.go` (~100 lines)

---

### Task 5.2: Register Places Search Executor in App
**Estimate:** 15 minutes
**Dependencies:** Task 5.1 (executor), Task 4.1 (service)

**Implementation:**
1. Edit `internal/app/app.go`:
   - Initialize PlacesStorage:
     ```go
     placesStorage := sqlite_storage.NewPlacesStorage(db, logger)
     ```

   - Initialize PlacesService:
     ```go
     placesService := places.NewService(
         &config.PlacesAPI,
         placesStorage,
         eventService,
         logger,
     )
     ```

   - Create PlacesSearchStepExecutor:
     ```go
     placesSearchExecutor := executor.NewPlacesSearchStepExecutor(
         placesService,
         logger,
     )
     ```

   - Register executor with JobExecutor:
     ```go
     jobExecutor.RegisterStepExecutor(placesSearchExecutor)
     ```

**Validation:**
- ✅ Application starts successfully
- ✅ Executor registered in job executor
- ✅ No initialization errors in logs

**Files Modified:**
- `internal/app/app.go` (~15 lines added)

---

## Phase 6: Job Definitions & Examples

### Task 6.1: Create Example Job Definition Directory
**Estimate:** 10 minutes
**Dependencies:** None
**Parallelizable:** 🔄

**Implementation:**
1. Create directory structure:
   ```
   deployments/local/job-definitions/places/
   ```

2. Add `.gitkeep` or README to ensure directory is tracked

**Validation:**
- ✅ Directory created successfully
- ✅ Visible in version control

**Files Created:**
- `deployments/local/job-definitions/places/.gitkeep`

---

### Task 6.2: Create Text Search Example Job
**Estimate:** 20 minutes
**Dependencies:** Task 6.1 (directory)

**Implementation:**
1. Create `deployments/local/job-definitions/places/text-search-example.toml`:
   ```toml
   # Example: Google Places Text Search
   # This job searches for coffee shops in Seattle using text-based query

   id = "places-text-search-example"
   name = "Example: Text Search for Coffee Shops"
   type = "custom"
   job_type = "user"
   description = "Demonstrates Google Places Text Search API integration"
   enabled = false  # Disabled by default - enable to run
   auto_start = false
   schedule = ""    # Manual execution only

   [[steps]]
   name = "search_coffee_shops"
   action = "places_search"
   on_error = "fail"

   [steps.config]
   search_query = "coffee shops in Seattle, WA"
   search_type = "text_search"
   max_results = 20
   list_name = "Seattle Coffee Shops"

   # Optional: Add filters (commented out by default)
   # [steps.config.filters]
   # min_rating = 4.0
   # open_now = false
   # price_level = [1, 2, 3]  # 1=$ to 4=$$$$
   ```

**Validation:**
- ✅ File is valid TOML
- ✅ Job definition passes validation
- ✅ Comments are clear and helpful

**Files Created:**
- `deployments/local/job-definitions/places/text-search-example.toml` (~30 lines)

---

### Task 6.3: Create Nearby Search Example Job
**Estimate:** 20 minutes
**Dependencies:** Task 6.1 (directory)
**Parallelizable:** 🔄 (with Task 6.2)

**Implementation:**
1. Create `deployments/local/job-definitions/places/nearby-search-example.toml`:
   ```toml
   # Example: Google Places Nearby Search
   # This job searches for restaurants near Seattle downtown using location coordinates

   id = "places-nearby-search-example"
   name = "Example: Nearby Search for Restaurants"
   type = "custom"
   job_type = "user"
   description = "Demonstrates Google Places Nearby Search API with location bounds"
   enabled = false  # Disabled by default - enable to run
   auto_start = false
   schedule = ""    # Manual execution only

   [[steps]]
   name = "search_nearby_restaurants"
   action = "places_search"
   on_error = "fail"

   [steps.config]
   search_query = "restaurants"
   search_type = "nearby_search"
   max_results = 20
   list_name = "Seattle Downtown Restaurants"

   # Location configuration (required for nearby_search)
   [steps.config.location]
   latitude = 47.6062   # Seattle downtown
   longitude = -122.3321
   radius = 5000        # 5km radius in meters

   # Optional: Add filters
   # [steps.config.filters]
   # min_rating = 4.0
   # open_now = true
   # price_level = [2, 3]  # $$ to $$$
   ```

**Validation:**
- ✅ File is valid TOML
- ✅ Location configuration present
- ✅ Job definition passes validation

**Files Created:**
- `deployments/local/job-definitions/places/nearby-search-example.toml` (~35 lines)

---

## Phase 7: Testing & Validation

### Task 7.1: Write API Integration Tests
**Estimate:** 60 minutes
**Dependencies:** Task 4.2 (API client), Task 3.1 (storage)

**Implementation:**
1. Create `test/api/places_service_test.go`:
   - Test `SearchPlaces()` with mock API responses
   - Test `GetList()` retrieval
   - Test `ListPlaceItems()` for a completed list
   - Test error handling (invalid API key, network errors)
   - Test rate limiting (ensure minimum interval enforced)
   - Use `SetupTestEnvironment()` for automatic service lifecycle

2. Test scenarios:
   - ✅ Successful text search stores results correctly
   - ✅ Successful nearby search with location
   - ✅ API error returns appropriate error message
   - ✅ Rate limiter enforces delay between requests
   - ✅ Bulk insert stores all items in transaction

**Validation:**
- ✅ All tests pass with `go test -v ./test/api/`
- ✅ Test coverage > 70% for service layer
- ✅ Mock API responses realistic

**Files Created:**
- `test/api/places_service_test.go` (~250 lines)

---

### Task 7.2: Write Job Execution Integration Test
**Estimate:** 45 minutes
**Dependencies:** Task 5.1 (executor), Task 6.2 (example jobs)

**Implementation:**
1. Create `test/api/places_job_execution_test.go`:
   - Load example job definition from TOML
   - Trigger job execution
   - Wait for completion (use polling or event subscription)
   - Verify list created in database
   - Verify items stored correctly
   - Check job status transitions (pending → running → completed)

2. Test scenarios:
   - ✅ Text search job executes successfully
   - ✅ Nearby search job with location executes successfully
   - ✅ Job with missing search_query fails validation
   - ✅ Job with invalid API key fails with error message

**Validation:**
- ✅ All tests pass with `go test -v ./test/api/`
- ✅ Example jobs loadable and executable

**Files Created:**
- `test/api/places_job_execution_test.go` (~200 lines)

---

### Task 7.3: Manual UI Testing (Jobs Page)
**Estimate:** 30 minutes
**Dependencies:** Task 5.2 (app integration), Task 6.2/6.3 (examples)

**Implementation:**
1. Start Quaero with `.\scripts\build.ps1 -Run`
2. Navigate to Jobs page in browser
3. Verify example place search jobs visible
4. Enable and trigger a place search job
5. Verify:
   - Job appears with status "pending"
   - Status transitions to "running"
   - WebSocket updates show progress
   - Status transitions to "completed"
   - Job details show search query and results count
   - Logs visible and contain API calls

**Validation:**
- ✅ Jobs page displays place search jobs
- ✅ Job execution visible in real-time
- ✅ Status transitions work correctly
- ✅ Error messages displayed for failures
- ✅ Job logs accessible and readable

**Manual Test Checklist:**
- [ ] Place search job visible in Jobs UI
- [ ] Job status updates in real-time
- [ ] Job completion shows total_results count
- [ ] Job error displayed if API key invalid
- [ ] Job logs contain structured information

---

## Phase 8: Documentation & Deployment

### Task 8.1: Update User Documentation
**Estimate:** 30 minutes
**Dependencies:** Task 6.2/6.3 (examples)

**Implementation:**
1. Create `docs/google-places-integration.md`:
   - Overview of feature
   - Prerequisites (Google Places API key)
   - Configuration instructions
   - Example job definitions
   - Troubleshooting section
   - API quota management tips

2. Update main README.md:
   - Add section on Google Places integration
   - Link to detailed documentation

**Validation:**
- ✅ Documentation clear and complete
- ✅ Code examples tested and work
- ✅ Screenshots included (optional)

**Files Created:**
- `docs/google-places-integration.md` (~150 lines)

**Files Modified:**
- `README.md` (~10 lines added)

---

### Task 8.2: Update CLAUDE.md with Architecture Notes
**Estimate:** 20 minutes
**Dependencies:** All implementation tasks

**Implementation:**
1. Edit `CLAUDE.md`:
   - Add section on Google Places integration
   - Document new service (PlacesService)
   - Document new step executor (PlacesSearchStepExecutor)
   - Document database schema (places_lists, places_items)
   - Add to "Adding a New Data Source" section (mention Places as example)

**Validation:**
- ✅ CLAUDE.md updated with Places architecture
- ✅ AI agents have context for future changes

**Files Modified:**
- `CLAUDE.md` (~50 lines added)

---

### Task 8.3: Final Build & Deployment Test
**Estimate:** 20 minutes
**Dependencies:** All tasks

**Implementation:**
1. Clean build: `.\scripts\build.ps1 -Clean`
2. Full build: `.\scripts\build.ps1 -Deploy`
3. Run all tests: `go test ./...`
4. Start service: `.\scripts\build.ps1 -Run`
5. Verify:
   - Service starts without errors
   - Example jobs loadable
   - API integration works (if API key configured)
   - Database migrations applied

**Validation:**
- ✅ Build succeeds without errors
- ✅ All tests pass
- ✅ Service starts and runs stable
- ✅ Example jobs executable

**Manual Test Checklist:**
- [ ] Build completes successfully
- [ ] No compilation errors or warnings
- [ ] All unit tests pass (`go test ./...`)
- [ ] All integration tests pass (`cd test/api && go test -v`)
- [ ] Service starts without errors
- [ ] Example jobs visible in UI
- [ ] Database schema created correctly

---

## Summary

**Total Estimated Time:** ~17 hours (2-3 days for single developer)

**Dependency Graph:**
```
Task 1.1 (Config) ──┬─→ Task 4.1 (Service) ──→ Task 5.1 (Executor) ──→ Task 5.2 (App Integration)
Task 1.2 (Schema) ──┼─→ Task 3.1 (Storage) ──┘                                 ↓
Task 2.1 (Models) ──┤                                                   Task 7.2 (Job Tests)
Task 2.2 (Interface) ┘                                                         ↓
                                                                        Task 8.3 (Final Tests)
Task 6.1 (Examples) ──→ Task 6.2 (Text) ──┬─→ Task 7.3 (UI Testing)
                    └─→ Task 6.3 (Nearby) ┘

Task 4.3 (Events) ──→ (No dependencies, can run anytime)
Task 8.1 (Docs) ───→ (After all implementation)
Task 8.2 (CLAUDE.md) → (After all implementation)
```

**Parallel Opportunities:**
- Phase 1 tasks (1.1, 1.2) can run in parallel
- Phase 2 tasks (2.1, 2.2, 2.3) can run in parallel after Phase 1
- Task 4.3 (Events) can run anytime
- Task 6.2 and 6.3 (Examples) can run in parallel

**Key Milestones:**
1. ✅ **Milestone 1 (Foundation):** Database schema and config ready → Tasks 1.1, 1.2 complete
2. ✅ **Milestone 2 (Service):** Places service working with mock data → Task 4.1, 4.2 complete
3. ✅ **Milestone 3 (Integration):** Jobs can execute place searches → Task 5.1, 5.2 complete
4. ✅ **Milestone 4 (User-Ready):** Example jobs available and tested → Task 6.2, 6.3, 7.3 complete
5. ✅ **Milestone 5 (Production):** All tests pass, docs complete → Task 8.1, 8.2, 8.3 complete
