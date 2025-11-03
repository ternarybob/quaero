package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerExecutor executes crawler jobs
type CrawlerExecutor struct {
	crawlerService *crawler.Service
	jobMgr         *jobs.Manager
	jobStorage     interfaces.JobStorage
	config         *common.Config
	logger         arbor.ILogger
}

// CrawlerPayload represents the payload for a crawler job
type CrawlerPayload struct {
	URL         string                 `json:"url"`
	Depth       int                    `json:"depth"`
	ParentID    string                 `json:"parent_id"`
	Phase       string                 `json:"phase,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	MaxDepth    int                    `json:"max_depth,omitempty"`
	FollowLinks bool                   `json:"follow_links,omitempty"`
}

func NewCrawlerExecutor(crawlerService *crawler.Service, jobMgr *jobs.Manager, jobStorage interfaces.JobStorage, config *common.Config, logger arbor.ILogger) *CrawlerExecutor {
	return &CrawlerExecutor{
		crawlerService: crawlerService,
		jobMgr:         jobMgr,
		jobStorage:     jobStorage,
		config:         config,
		logger:         logger,
	}
}

func (e *CrawlerExecutor) Execute(ctx context.Context, jobID string, payload []byte) error {
	// 1. Parse payload
	var crawlPayload CrawlerPayload
	if err := json.Unmarshal(payload, &crawlPayload); err != nil {
		return fmt.Errorf("failed to unmarshal crawler payload: %w", err)
	}

	e.logger.Info().
		Str("job_id", jobID).
		Str("url", crawlPayload.URL).
		Int("depth", crawlPayload.Depth).
		Msg("Executing crawler job")

	e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Crawling URL: %s (depth: %d)", crawlPayload.URL, crawlPayload.Depth))

	// 2. Build crawler configuration from payload
	crawlerConfig := e.buildCrawlerConfig(crawlPayload)

	// 3. Initialize HTML scraper for this job
	httpClient, err := e.crawlerService.BuildHTTPClientFromAuth(ctx)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to build HTTP client from auth, using default")
		httpClient = nil // Will use default in NewHTMLScraper
	}

	scraper := crawler.NewHTMLScraper(crawlerConfig, e.logger, httpClient, nil)
	defer scraper.Close()

	// 4. Fetch URL using HTMLScraper
	e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Fetching URL: %s", crawlPayload.URL))

	result, err := scraper.ScrapeURL(ctx, crawlPayload.URL)
	if err != nil {
		e.jobMgr.AddJobLog(ctx, jobID, "error", fmt.Sprintf("Failed to scrape URL: %v", err))
		return fmt.Errorf("failed to scrape URL %s: %w", crawlPayload.URL, err)
	}

	if !result.Success {
		e.jobMgr.AddJobLog(ctx, jobID, "warn", fmt.Sprintf("URL returned status %d: %s", result.StatusCode, crawlPayload.URL))
	} else {
		e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Successfully scraped URL (status: %d, links found: %d)", result.StatusCode, len(result.Links)))
	}

	// 5. Extract links and create child jobs (if following links and not at max depth)
	linksEnqueued := 0
	maxDepth := crawlPayload.MaxDepth
	if maxDepth == 0 {
		maxDepth = 3 // Default max depth
	}

	shouldFollowLinks := crawlPayload.FollowLinks && crawlPayload.Depth < maxDepth

	if shouldFollowLinks && len(result.Links) > 0 {
		e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Processing %d discovered links (depth: %d/%d)", len(result.Links), crawlPayload.Depth, maxDepth))

		for _, link := range result.Links {
			// 6. Check URL deduplication via job_seen_urls table
			parentID := crawlPayload.ParentID
			if parentID == "" {
				parentID = jobID // If no parent, this is the parent
			}

			alreadySeen, err := e.jobStorage.MarkURLSeen(ctx, parentID, link)
			if err != nil {
				e.logger.Warn().Err(err).Str("url", link).Msg("Failed to check URL deduplication")
				continue
			}

			if alreadySeen {
				e.logger.Debug().Str("url", link).Msg("URL already seen, skipping")
				continue
			}

			// 7. Create child job for discovered link
			childPayload := CrawlerPayload{
				URL:         link,
				Depth:       crawlPayload.Depth + 1,
				ParentID:    parentID,
				Phase:       "core", // Child jobs are always in core phase
				Config:      crawlPayload.Config,
				MaxDepth:    maxDepth,
				FollowLinks: crawlPayload.FollowLinks,
			}

			childJobID, err := e.jobMgr.CreateChildJob(ctx, parentID, "crawler_url", "core", childPayload)
			if err != nil {
				e.logger.Warn().Err(err).Str("url", link).Msg("Failed to create child job")
				e.jobMgr.AddJobLog(ctx, jobID, "warn", fmt.Sprintf("Failed to enqueue link: %s", link))
				continue
			}

			linksEnqueued++
			e.logger.Debug().
				Str("child_job_id", childJobID).
				Str("url", link).
				Int("depth", crawlPayload.Depth+1).
				Msg("Created child job for discovered link")
		}

		if linksEnqueued > 0 {
			e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Enqueued %d new links for crawling", linksEnqueued))
		}
	}

	// 8. Store crawl result (optional - could save to documents or results table)
	// For now, we'll just log success

	// 9. Update job progress
	e.jobMgr.UpdateJobProgress(ctx, jobID, 1, 1)
	e.jobMgr.AddJobLog(ctx, jobID, "info", "Crawl completed successfully")

	e.logger.Info().
		Str("job_id", jobID).
		Str("url", crawlPayload.URL).
		Int("status_code", result.StatusCode).
		Int("links_discovered", len(result.Links)).
		Int("links_enqueued", linksEnqueued).
		Msg("Crawler job completed")

	return nil
}

// buildCrawlerConfig constructs a CrawlerConfig from the payload config map
func (e *CrawlerExecutor) buildCrawlerConfig(payload CrawlerPayload) common.CrawlerConfig {
	// Start with default config from service
	config := e.config.Crawler

	// Override with payload-specific config if provided
	if payload.Config != nil {
		if val, ok := payload.Config["request_timeout"]; ok {
			if timeout, ok := val.(float64); ok {
				config.RequestTimeout = time.Duration(timeout) * time.Millisecond
			}
		}
		if val, ok := payload.Config["request_delay"]; ok {
			if delay, ok := val.(float64); ok {
				config.RequestDelay = time.Duration(delay) * time.Millisecond
			}
		}
		if val, ok := payload.Config["include_metadata"]; ok {
			if includeMeta, ok := val.(bool); ok {
				config.IncludeMetadata = includeMeta
			}
		}
		if val, ok := payload.Config["include_links"]; ok {
			if includeLinks, ok := val.(bool); ok {
				config.IncludeLinks = includeLinks
			}
		}
		if val, ok := payload.Config["output_format"]; ok {
			if format, ok := val.(string); ok {
				config.OutputFormat = format
			}
		}
	}

	// Ensure links are always included for crawling
	config.IncludeLinks = true

	return config
}
