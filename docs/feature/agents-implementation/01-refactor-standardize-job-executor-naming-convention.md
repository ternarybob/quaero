I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

The codebase has **two crawler executor implementations** in `internal/jobs/processor/`:

1. **`crawler_executor.go`** - A stub implementation with `CrawlerURLExecutor` type containing only TODO comments and placeholder logic (110 lines)
2. **`enhanced_crawler_executor.go`** - The full production implementation with `EnhancedCrawlerExecutor` type containing complete ChromeDP rendering, content processing, authentication, and child job spawning (1034 lines)
3. **`enhanced_crawler_executor_auth.go`** - Authentication cookie injection logic for `EnhancedCrawlerExecutor` (495 lines)

The "Enhanced" prefix violates the project's naming convention. Other executors follow the `{Type}Executor` pattern:
- `ParentJobExecutor` in `parent_job_executor.go`
- `DatabaseMaintenanceExecutor` in `database_maintenance_executor.go`

The stub `crawler_executor.go` was likely created as a placeholder but never completed, while the "enhanced" version became the actual implementation.

## References Found

- **`internal/app/app.go`** (lines 292-303): Creates `enhancedCrawlerExecutor` using `processor.NewEnhancedCrawlerExecutor()` and registers it with `jobProcessor.RegisterExecutor()`
- No test files reference the enhanced crawler executor
- The executor handles job type `"crawler_url"` and is registered with the job processor

### Approach

## Standardization Strategy

**Delete the stub, rename the production implementation, update references.**

This refactoring enforces the `{Type}Executor` naming convention across all job executors. The stub file serves no purpose and creates confusion. The "Enhanced" prefix suggests a temporary or experimental implementation, but this is the production code.

**Why this approach:**
- Maintains consistency with `ParentJobExecutor` and `DatabaseMaintenanceExecutor`
- Eliminates confusion from having two files with similar names
- Simplifies the codebase by removing dead code
- Follows the principle that there should be ONE executor per job type

### Reasoning

I explored the repository structure, read the three relevant files mentioned by the user (`enhanced_crawler_executor.go`, `app.go`, `processor.go`), discovered the stub `crawler_executor.go` file, examined other executors to confirm the naming pattern (`ParentJobExecutor`, `DatabaseMaintenanceExecutor`), and searched for all references to the enhanced crawler types to ensure complete coverage of the refactoring.

## Proposed File Changes

### internal\jobs\processor\crawler_executor.go(DELETE)

Remove the stub `CrawlerURLExecutor` implementation. This file contains only placeholder code with TODO comments and was never completed. The actual implementation exists in `enhanced_crawler_executor.go` which will be renamed to take its place.

### internal\jobs\processor\enhanced_crawler_executor.go → internal\jobs\processor\crawler_executor.go

References: 

- internal\interfaces\job_executor.go
- internal\services\crawler\service.go
- internal\jobs\manager.go
- internal\queue\manager.go

Rename the file from `enhanced_crawler_executor.go` to `crawler_executor.go` to follow the standard `{type}_executor.go` naming convention.

**Type and Constructor Renaming:**
- Rename struct type `EnhancedCrawlerExecutor` to `CrawlerExecutor` (line 27)
- Update constructor function name from `NewEnhancedCrawlerExecutor` to `NewCrawlerExecutor` (line 43)
- Update constructor return type from `*EnhancedCrawlerExecutor` to `*CrawlerExecutor` (line 52)
- Update struct initialization from `&EnhancedCrawlerExecutor{` to `&CrawlerExecutor{` (line 53)

**Method Receiver Updates:**
Update all method receivers from `(e *EnhancedCrawlerExecutor)` to `(e *CrawlerExecutor)` for:
- `GetJobType()` method (line 67)
- `Validate()` method (line 72)
- `Execute()` method (line 104)
- `extractCrawlConfig()` method (line 481)
- `renderPageWithChromeDp()` method (line 547)
- `spawnChildJob()` method (line 814)
- `publishCrawlerJobLog()` method (line 903)
- `publishCrawlerProgressUpdate()` method (line 933)
- `publishLinkDiscoveryEvent()` method (line 978)
- `publishJobSpawnEvent()` method (line 1002)

**Comment Updates:**
- Update file header comment from "Enhanced Crawler Executor" to "Crawler Executor" (line 1)
- Update struct comment from "EnhancedCrawlerExecutor executes enhanced crawler jobs" to "CrawlerExecutor executes crawler jobs" (line 25)
- Update constructor comment from "NewEnhancedCrawlerExecutor creates a new enhanced crawler executor" to "NewCrawlerExecutor creates a new crawler executor" (line 42)
- Update Execute method comment from "Execute executes an enhanced crawler job" to "Execute executes a crawler job" (line 98)

The file implements the `interfaces.JobExecutor` interface with methods `GetJobType()`, `Validate()`, and `Execute()`. It handles ChromeDP browser automation, content extraction, markdown conversion, authentication cookie injection, link discovery, and child job spawning for discovered URLs.

### internal\jobs\processor\enhanced_crawler_executor_auth.go → internal\jobs\processor\crawler_executor_auth.go

References: 

- internal\jobs\processor\crawler_executor.go(DELETE)
- internal\interfaces\auth.go

Rename the file from `enhanced_crawler_executor_auth.go` to `crawler_executor_auth.go` to match the renamed main executor file.

**Comment Updates:**
- Update file header comment from "Enhanced Crawler Executor - Authentication Cookie Injection" to "Crawler Executor - Authentication Cookie Injection" (line 2)

**Method Receiver Update:**
- Update method receiver from `(e *EnhancedCrawlerExecutor)` to `(e *CrawlerExecutor)` for the `injectAuthCookies()` method (line 24)

This file contains the authentication cookie injection logic that loads credentials from storage and injects them into the ChromeDP browser context. The method signature remains unchanged: `injectAuthCookies(ctx context.Context, browserCtx context.Context, parentJobID, targetURL string, logger arbor.ILogger) error`.

The method is called from the main `Execute()` method in `crawler_executor.go` during page rendering to enable authenticated crawling of protected resources.

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\processor\crawler_executor.go(DELETE)
- internal\jobs\processor\processor.go(MODIFY)

Update all references to the renamed crawler executor in the service initialization code.

**Variable Naming (lines 292-303):**
- Rename variable from `enhancedCrawlerExecutor` to `crawlerExecutor` (line 292)
- Update constructor call from `processor.NewEnhancedCrawlerExecutor()` to `processor.NewCrawlerExecutor()` (line 292)
- Update registration call from `jobProcessor.RegisterExecutor(enhancedCrawlerExecutor)` to `jobProcessor.RegisterExecutor(crawlerExecutor)` (line 302)

**Comment Updates:**
- Update comment from "Register enhanced crawler_url executor (new interface with ChromeDP and content processing)" to "Register crawler_url executor (ChromeDP rendering and content processing)" (line 291)
- Update log message from "Enhanced crawler URL executor registered for job type: crawler_url" to "Crawler URL executor registered for job type: crawler_url" (line 303)

The executor is initialized with dependencies: `CrawlerService`, `jobMgr`, `queueMgr`, `DocumentStorage`, `AuthStorage`, `JobDefinitionStorage`, `Logger`, and `EventService`. These parameters remain unchanged.

The executor is registered with `jobProcessor` which routes jobs of type `"crawler_url"` to this executor for processing. The registration happens in the `initServices()` method after the job processor is created (line 265) and before the parent job executor is created (line 308).

### internal\jobs\processor\processor.go(MODIFY)

References: 

- internal\interfaces\job_executor.go
- internal\jobs\processor\crawler_executor.go(DELETE)

**Verification Only - No Changes Required**

Confirm that the `JobProcessor` correctly handles the renamed executor. The processor uses a type-agnostic approach:

- Executors are registered via `RegisterExecutor(executor interfaces.JobExecutor)` method (line 47)
- The processor stores executors in a map keyed by job type: `executors map[string]interfaces.JobExecutor` (line 21)
- Job type is retrieved dynamically via `executor.GetJobType()` (line 48)
- The `CrawlerExecutor.GetJobType()` method returns `"crawler_url"` which remains unchanged

Since the processor interacts with executors through the `interfaces.JobExecutor` interface and uses dynamic type resolution, the rename from `EnhancedCrawlerExecutor` to `CrawlerExecutor` requires no changes to this file. The registration in `app.go` will automatically use the correct type name after the refactoring.

This file is included for completeness to document that the processor's design correctly supports the refactoring without modification.