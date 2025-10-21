## Codebase Investigation: Application Crash during Crawler Job

### Summary of Findings

The application crashes during crawler job execution due to excessive resource consumption in the `HTMLScraper` service. The root cause is the creation of a new headless Chrome browser instance (`chromedp`) for every single URL being crawled. This leads to high memory and CPU usage, causing the operating system to terminate the application process, resulting in a crash without a specific error log. The fix requires refactoring the `HTMLScraper` to use a pool of long-lived, reusable browser instances instead of creating a new one for each request. This will stabilize resource consumption and make the crawler scalable.

### Exploration Trace

1.  Used `list_directory` to explore `internal/services/crawler` and `internal/services/jobs`.
2.  Read `internal/services/jobs/executor.go` and `internal/services/jobs/crawl_collect_job.go` to understand the two different job execution flows.
3.  Read `internal/services/jobs/job_helper.go` to understand how a crawl job is configured and started.
4.  Read `internal/services/crawler/service.go` to understand the core crawling logic, including worker loops, job management, and state handling.
5.  Identified a potential deadlock in the `WaitForJob` function which was later found to be resolved by a polling mechanism.
6.  Read `internal/services/crawler/html_scraper.go` and identified the root cause of the crash: creating a new browser instance for every scraped URL.

### Relevant Locations

*   **`internal/services/crawler/html_scraper.go`**: This file contains the `HTMLScraper` which is responsible for scraping URLs using `chromedp`. The `ScrapeURL` function incorrectly creates a new browser instance for every URL, causing resource exhaustion and application crashes. This is the primary location for the fix.
    *   **Key Symbols**: `HTMLScraper`, `ScrapeURL`, `chromedp.NewExecAllocator`, `chromedp.NewContext`

*   **`internal/services/crawler/service.go`**: This file contains the main `Service` that orchestrates the entire crawling process. It manages the job lifecycle, worker goroutines, and calls the `HTMLScraper`. While the crash does not originate here, this service will need to be modified to manage the lifecycle of the proposed `chromedp` browser pool.
    *   **Key Symbols**: `Service`, `NewService`, `startWorkers`, `makeRequest`, `Shutdown`

*   **`internal/services/jobs/crawl_collect_job.go`**: This file defines a specific type of crawl job and calls the crawler service. It represents one of the execution paths that can trigger the crash.
    *   **Key Symbols**: `CrawlCollectJob`, `processSource`

*   **`internal/services/jobs/executor.go`**: This file contains the generic job executor which can also trigger a crawl job as part of a series of steps. It represents the second execution path that can trigger the crash.
    *   **Key Symbols**: `JobExecutor`, `Execute`, `pollCrawlJobs`
