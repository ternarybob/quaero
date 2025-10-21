I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

User wants a simple recursive crawl: Start at base_url → save page → extract links → filter → follow → repeat. The source already has BaseURL field. Just need to: (1) Add Filters field back to SourceConfig for include/exclude patterns (comma-delimited), (2) Fix generateSeedURLs() to return just source.BaseURL, (3) Ensure documents are saved during crawl, (4) Use source filters for link filtering.

### Approach

**SIMPLIFIED SOLUTION**: The user wants a simple recursive crawl process starting from a SINGLE base URL. No multiple seed URLs, no complex generation logic. Just: (1) Start at source.BaseURL, (2) Save page as document, (3) Extract links, (4) Filter using source filters (comma-delimited include/exclude patterns), (5) Follow matching links recursively until max depth. The fix is to use source.BaseURL directly as the single starting point and add back the Filters field to SourceConfig for link filtering.

### Reasoning

Read the user's latest clarification that there should be ONE initial URL from source config, not multiple seed URLs. Re-examined source.go and confirmed it has a BaseURL field. The bug is that generateSeedURLs() is creating multiple URLs when it should just return the single base URL.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI
    participant JH as job_helper.go
    participant CS as Crawler Service
    participant W as Worker
    participant DS as Document Storage
    
    Note over User,DS: Simple Recursive Crawl Process
    
    User->>UI: Configure Source
    User->>UI: Set base_url (single URL)
    User->>UI: Set filters (include/exclude patterns)
    UI->>DS: Save source config
    
    User->>UI: Start Crawl Job
    UI->>JH: StartCrawlJob(source)
    JH->>JH: generateSeedURLs(source)
    JH->>JH: Return [source.BaseURL]
    JH->>JH: Extract filters from source.Filters
    JH->>JH: Parse comma-delimited patterns
    JH->>CS: StartCrawl([baseURL], config with filters)
    
    CS->>W: Enqueue base URL (depth=0)
    
    loop Recursive Crawl (until max depth)
        W->>W: Pop URL from queue
        W->>W: Fetch page with auth
        W->>DS: Save page as document
        DS->>DS: Store in documents table
        W->>W: Extract links from HTML
        W->>W: Apply source filters (include/exclude)
        
        alt Link matches include pattern
            alt Link does NOT match exclude pattern
                W->>W: Enqueue link (depth+1)
            end
        end
    end
    
    W->>CS: Update job progress
    CS->>User: Job completed

## Proposed File Changes

### internal\models\source.go(MODIFY)

Add back the Filters field to SourceConfig struct: (1) Add `Filters map[string]interface{}` with JSON tag `json:"filters"` after the CrawlConfig field, (2) Add documentation comment explaining Filters contains include_patterns and exclude_patterns as comma-delimited strings, (3) Add validateFilters() method to validate the Filters map structure, check that include_patterns and exclude_patterns are strings if present, (4) Update Validate() method to call validateFilters().

### internal\storage\sqlite\schema.go(MODIFY)

Add back the filters column to sources table: (1) In schemaSQL constant, add `filters TEXT,` column after crawl_config in sources table definition, (2) Create migration function migrateAddBackSourcesFiltersColumn() that checks if filters column exists using PRAGMA table_info, if missing adds it using ALTER TABLE sources ADD COLUMN filters TEXT, (3) Add migration call to runMigrations() method.

### internal\storage\sqlite\source_storage.go(MODIFY)

References: 

- internal\models\source.go(MODIFY)

Add back filters serialization: (1) In SaveSource(), add json.Marshal(source.Filters) and add filters column to INSERT query and Exec() parameters, (2) In GetSource(), add filters to SELECT column list, (3) In scanSource(), add filtersJSON variable, add to Scan() call, add json.Unmarshal logic for filters field, (4) Apply same changes to ListSources(), GetSourcesByType(), GetEnabledSources() and their scan helpers.

### internal\services\jobs\job_helper.go(MODIFY)

References: 

- internal\models\source.go(MODIFY)
- internal\services\crawler\types.go

Simplify generateSeedURLs() to return ONLY the base URL: (1) Replace the entire function body with a simple check: if source.BaseURL is empty return error, otherwise return []string{source.BaseURL}, (2) Remove all the type-specific URL generation logic (Jira /browse, Confluence /wiki, etc.), (3) Add logging to show using base URL as single starting point. Extract filters from source.Filters: (1) After building crawlerConfig, extract include_patterns from source.Filters map as comma-delimited string, (2) Extract exclude_patterns from source.Filters map as comma-delimited string, (3) Parse comma-delimited strings using strings.Split() and trim whitespace from each pattern, (4) Set crawlerConfig.IncludePatterns and crawlerConfig.ExcludePatterns from parsed arrays, (5) Add logging showing filter configuration.

### internal\handlers\sources_handler.go(MODIFY)

References: 

- internal\models\source.go(MODIFY)

Add back filter validation: (1) In CreateSourceHandler() and UpdateSourceHandler(), add call to validateSourceFilters(&source) before saving, (2) Add validateSourceFilters() function that validates source.Filters map, checks include_patterns and exclude_patterns are strings (comma-delimited) if present, parses and validates patterns are not empty, returns error if validation fails, (3) Add logging to show parsed filter patterns.

### pages\sources.html(MODIFY)

Add back UI fields for filters: (1) In source configuration form, add Include Patterns input field with label, type text, placeholder showing comma-separated examples like "browse,projects,issues", hint text explaining only links matching these patterns will be followed, (2) Add Exclude Patterns input field with label, type text, placeholder showing comma-separated examples like "admin,logout,settings", hint text explaining links matching these patterns will be skipped, (3) In sources table, add Filters column to display configured filters.

### pages\static\common.js(MODIFY)

Add back JavaScript logic for filters: (1) In getDefaultSource(), add filters initialization: filters: { include_patterns: '', exclude_patterns: '' }, (2) In editSource(), add filter conversion from source.filters object to comma-separated strings for input fields, (3) In saveSource(), add filter processing to convert comma-separated strings to filter object structure with include_patterns and exclude_patterns, (4) Add formatFilterDisplay() method to show filters in sources table.