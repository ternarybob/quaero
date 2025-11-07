# Web Crawler Enhancement Requirements

## Introduction

This specification defines the requirements for enhancing the Quaero web crawler system to provide comprehensive website crawling capabilities with real-time monitoring, parent-child job relationships, and content processing using ChromeDP for JavaScript-rendered sites.

## Glossary

- **Crawler_System**: The web crawling subsystem within Quaero that processes websites and extracts content
- **Parent_Job**: A top-level crawler job that orchestrates the crawling process and spawns child jobs
- **Child_Job**: Individual URL crawling tasks spawned by a parent job
- **Job_Queue**: The standard goqite-based message queue system that manages job execution without modification
- **WebSocket_Logger**: Real-time logging system that streams job progress to the UI via WebSocket
- **ChromeDP_Engine**: Headless Chrome automation tool used for rendering JavaScript-heavy websites
- **Content_Processor**: Component that converts HTML content to markdown format
- **Job_Definition**: TOML configuration that defines crawler behavior and parameters
- **Queue_Page**: UI page that displays active jobs with real-time status updates
- **Job_Details_Page**: UI page that shows individual job information with live updates for running jobs

## Requirements

### Requirement 1: Single Parent Job with All Children Architecture

**User Story:** As a system administrator, I want crawler jobs to show as a single parent job with all spawned URL tasks as children, so that I can see one unified job instead of multiple separate jobs.

#### Acceptance Criteria

1. WHEN a crawler job definition is executed, THE Crawler_System SHALL create exactly one parent job visible in the UI
2. WHEN the parent job processes start URLs, THE Crawler_System SHALL spawn child jobs that are NOT visible as separate jobs in the queue
3. WHEN child jobs discover additional URLs, THE Crawler_System SHALL spawn more child jobs that ALL reference the same parent_id
4. WHEN viewing the queue page, THE Queue_Page SHALL display only the parent job, never showing child jobs as separate entries
5. WHEN viewing job details, THE Queue_Page SHALL show all child job activity aggregated under the single parent job

### Requirement 2: Inline Status Display and Real-Time Monitoring

**User Story:** As a user monitoring crawler progress, I want to see status information displayed inline on a single line matching the existing UI style, so that the interface remains consistent and clean.

#### Acceptance Criteria

1. WHEN displaying crawler job status, THE Queue_Page SHALL show status information inline on a single line, not in separate boxes
2. WHEN showing crawling progress, THE Queue_Page SHALL display "0 of 2 URLs processed" format inline with other job metadata
3. WHEN job logs are generated, THE WebSocket_Logger SHALL stream the latest 3-5 log entries to connected clients
4. WHEN job status changes, THE Queue_Page SHALL update the status indicator within 2 seconds using the existing status badge style
5. WHEN viewing job details, THE Queue_Page SHALL display scrolling logs with automatic refresh

### Requirement 3: Functional ChromeDP-Based Content Extraction

**User Story:** As a content collector, I want the crawler to actually work and process URLs successfully, so that I can extract content and see crawling progress.

#### Acceptance Criteria

1. WHEN a crawler job starts, THE Crawler_System SHALL immediately begin processing the configured start URLs
2. WHEN processing a URL, THE ChromeDP_Engine SHALL successfully navigate to the page and extract content
3. WHEN content is extracted, THE Crawler_System SHALL store the document and update the progress counter
4. WHEN URLs are discovered, THE Crawler_System SHALL follow them according to the configured depth and patterns
5. WHEN crawling progresses, THE Queue_Page SHALL show updated counts like "1 of 2 URLs processed" in real-time

### Requirement 4: Generic Content Processing

**User Story:** As a developer, I want the crawler to remain agnostic to specific website types, so that it can process any website without hardcoded logic for particular platforms.

#### Acceptance Criteria

1. WHEN processing website content, THE Content_Processor SHALL use generic HTML-to-markdown conversion
2. WHEN extracting metadata, THE Crawler_System SHALL capture standard fields (URL, title, content size, processing time)
3. WHEN storing documents, THE Crawler_System SHALL save content in a standardized format regardless of source website
4. WHEN encountering different content types, THE Content_Processor SHALL handle them using generic parsing rules
5. WHERE site-specific processing is needed, THE Crawler_System SHALL provide extension points for future customization

### Requirement 5: Comprehensive Job Configuration

**User Story:** As a system configurator, I want to define crawler behavior through TOML configuration files, so that I can customize crawling parameters without code changes.

#### Acceptance Criteria

1. WHEN defining a crawler job, THE Job_Definition SHALL support start_urls configuration
2. WHEN configuring crawl behavior, THE Job_Definition SHALL support max_depth, max_pages, and concurrency settings
3. WHEN setting URL filters, THE Job_Definition SHALL support include_patterns and exclude_patterns
4. WHEN scheduling jobs, THE Job_Definition SHALL support cron schedule expressions
5. WHEN setting timeouts, THE Job_Definition SHALL support configurable execution time limits

### Requirement 6: Document Storage and Metadata

**User Story:** As a content analyst, I want crawled documents to include comprehensive metadata, so that I can analyze and search content effectively.

#### Acceptance Criteria

1. WHEN storing a crawled document, THE Crawler_System SHALL record the source URL
2. WHEN processing content, THE Crawler_System SHALL measure and store content size in bytes
3. WHEN completing URL processing, THE Crawler_System SHALL record processing time in milliseconds
4. WHEN extracting page content, THE Crawler_System SHALL capture page title and description
5. WHEN saving documents, THE Crawler_System SHALL store both original HTML and converted markdown

### Requirement 7: Error Handling and Resilience

**User Story:** As a system operator, I want the crawler to handle errors gracefully, so that individual failures don't stop the entire crawling process.

#### Acceptance Criteria

1. WHEN a URL fails to load, THE Crawler_System SHALL log the error and continue with remaining URLs
2. WHEN ChromeDP encounters an error, THE Crawler_System SHALL retry up to 3 times before marking as failed
3. WHEN content conversion fails, THE Crawler_System SHALL store the raw HTML as fallback
4. WHEN job execution exceeds timeout, THE Crawler_System SHALL cancel the job and update status
5. WHEN database operations fail, THE Crawler_System SHALL retry with exponential backoff

### Requirement 8: Queue Integration and Logging

**User Story:** As a system administrator, I want all crawler activity to be logged through the arbor logging system, so that I can monitor and debug crawling operations.

#### Acceptance Criteria

1. WHEN crawler jobs execute, THE WebSocket_Logger SHALL capture all log messages via arbor channel logger
2. WHEN job status changes, THE Crawler_System SHALL send status updates via WebSocket using standard job management
3. WHEN errors occur, THE Crawler_System SHALL log detailed error information with context using standard job logging
4. WHEN jobs complete, THE Crawler_System SHALL log summary statistics (URLs processed, success rate, duration)
5. WHEN viewing job logs, THE Queue_Page SHALL display logs using parent_id metadata to group related jobs

### Requirement 9: UI Integration and User Experience

**User Story:** As a user managing crawler jobs, I want a clean interface to monitor job progress and view results, so that I can effectively manage crawling operations.

#### Acceptance Criteria

1. WHEN viewing the queue page, THE Queue_Page SHALL show parent jobs with expandable child job lists
2. WHEN a job is running, THE Queue_Page SHALL display progress indicators and live log updates
3. WHEN clicking on a job, THE Queue_Page SHALL navigate to detailed job view with full logs and configuration
4. WHEN viewing job details for a running job, THE Job_Details_Page SHALL poll for status and log updates every 2 seconds
5. WHEN jobs complete, THE Queue_Page SHALL show completion status and link to extracted documents
6. WHEN errors occur, THE Queue_Page SHALL highlight failed jobs with error details

### Requirement 10: Working Crawler Job Execution

**User Story:** As a user running crawler jobs, I want the jobs to actually execute and make progress, so that I can successfully crawl websites and extract content.

#### Acceptance Criteria

1. WHEN a crawler job is started, THE Crawler_System SHALL immediately transition from "Pending" to "Running" status
2. WHEN processing URLs, THE Crawler_System SHALL show visible progress updates with incrementing URL counts
3. WHEN a URL is successfully processed, THE Crawler_System SHALL store the extracted document and increment the completed count
4. WHEN all URLs are processed, THE Crawler_System SHALL transition the job status to "Completed"
5. WHEN errors occur during processing, THE Crawler_System SHALL log the errors but continue processing remaining URLs

### Requirement 11: Performance and Scalability

**User Story:** As a system administrator, I want the crawler to handle multiple concurrent jobs efficiently, so that I can process large websites without performance degradation.

#### Acceptance Criteria

1. WHEN multiple crawler jobs run simultaneously, THE Crawler_System SHALL limit concurrent ChromeDP instances to prevent resource exhaustion
2. WHEN processing large websites, THE Crawler_System SHALL respect rate limiting to avoid overwhelming target servers
3. WHEN storing documents, THE Crawler_System SHALL batch database operations for improved performance
4. WHEN memory usage exceeds thresholds, THE Crawler_System SHALL implement cleanup procedures
5. WHEN queue depth grows large, THE Job_Queue SHALL maintain processing efficiency using standard goqite operations