package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// ConfluenceTransformer transforms Confluence crawler results into normalized documents
type ConfluenceTransformer struct {
	jobStorage                interfaces.JobStorage
	documentStorage           interfaces.DocumentStorage
	eventService              interfaces.EventService
	crawlerService            interfaces.CrawlerService
	logger                    arbor.ILogger
	enableEmptyOutputFallback bool // Controls whether to apply HTML stripping when MD conversion produces empty output
}

// NewConfluenceTransformer creates a new Confluence transformer and subscribes to collection events
// enableEmptyOutputFallback controls whether to apply HTML stripping fallback when markdown conversion produces empty output
func NewConfluenceTransformer(jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, crawlerService interfaces.CrawlerService, logger arbor.ILogger, enableEmptyOutputFallback bool) *ConfluenceTransformer {
	t := &ConfluenceTransformer{
		jobStorage:                jobStorage,
		documentStorage:           documentStorage,
		eventService:              eventService,
		crawlerService:            crawlerService,
		logger:                    logger,
		enableEmptyOutputFallback: enableEmptyOutputFallback,
	}

	// Subscribe to collection triggered events
	eventService.Subscribe(interfaces.EventCollectionTriggered, t.handleCollectionEvent)

	return t
}

// handleCollectionEvent handles collection triggered events
func (t *ConfluenceTransformer) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	t.logger.Debug().Msg("Confluence transformer received collection event")

	// Query for completed jobs
	jobsInterface, err := t.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusCompleted))
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to get completed jobs")
		return fmt.Errorf("failed to get completed jobs: %w", err)
	}

	// Filter for Confluence jobs
	var confluenceJobs []*crawler.CrawlJob
	for _, jobInterface := range jobsInterface {
		if job, ok := jobInterface.(*crawler.CrawlJob); ok {
			if job.SourceType == "confluence" {
				confluenceJobs = append(confluenceJobs, job)
			}
		}
	}

	if len(confluenceJobs) == 0 {
		t.logger.Debug().Msg("No completed Confluence jobs to transform")
		return nil
	}

	t.logger.Info().Int("job_count", len(confluenceJobs)).Msg("Processing completed Confluence jobs")

	// Transform each job
	var successCount, failCount int
	for _, job := range confluenceJobs {
		if err := t.transformJob(ctx, job); err != nil {
			t.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to transform Confluence job")
			failCount++
		} else {
			successCount++
		}
	}

	logTransformationSummary(t.logger, "confluence", successCount, failCount)
	return nil
}

// transformJob transforms a single Confluence job's results into documents
func (t *ConfluenceTransformer) transformJob(ctx context.Context, job *crawler.CrawlJob) error {
	// Parse source config snapshot to get metadata
	var sourceConfig *models.SourceConfig
	if job.SourceConfigSnapshot != "" {
		if err := json.Unmarshal([]byte(job.SourceConfigSnapshot), &sourceConfig); err != nil {
			t.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to parse source config snapshot")
		}
	}

	// Get job results (using crawler service)
	results, err := getJobResults(job, t.crawlerService, t.logger)
	if err != nil {
		t.logger.Warn().
			Err(err).
			Str("job_id", job.ID).
			Str("source_type", job.SourceType).
			Msg("Failed to get job results - transformation cannot proceed")
		return err
	}

	if len(results) == 0 {
		t.logger.Warn().
			Str("job_id", job.ID).
			Str("source_type", job.SourceType).
			Int("result_count", job.ResultCount).
			Msg("No results available for transformation despite job completion")
		return nil
	}

	// Transform each result
	var transformedCount, failedRequests, emptyContent, parseFailures, saveFailures int
	for _, result := range results {
		if result.Error != "" {
			failedRequests++
			t.logger.Debug().
				Str("url", result.URL).
				Str("error", result.Error).
				Msg("Skipping failed request result")
			continue // Skip failed requests
		}

		// Select content using shared helper
		body := selectResultBody(result)
		if len(body) == 0 {
			emptyContent++
			t.logger.Debug().
				Str("url", result.URL).
				Msg("Skipping result with no content body")
			continue // Skip if no content found
		}

		// Parse and transform Confluence page
		doc, err := t.parseConfluencePage(body, result.URL, result.Metadata, sourceConfig)
		if err != nil {
			parseFailures++
			t.logger.Warn().
				Err(err).
				Str("url", result.URL).
				Str("error_type", fmt.Sprintf("%T", err)).
				Int("body_length", len(body)).
				Msg("Failed to parse Confluence page HTML")
			continue
		}

		// Save document
		if err := t.documentStorage.SaveDocument(doc); err != nil {
			saveFailures++
			t.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Str("page_id", doc.SourceID).
				Str("title", doc.Title).
				Msg("Failed to save Confluence document to storage")
			continue
		}

		transformedCount++
	}

	t.logger.Info().
		Str("job_id", job.ID).
		Int("transformed_count", transformedCount).
		Int("total_results", len(results)).
		Int("failed_requests", failedRequests).
		Int("empty_content", emptyContent).
		Int("parse_failures", parseFailures).
		Int("save_failures", saveFailures).
		Msg("Confluence transformation complete")

	// Calculate and log success rate
	successRate := 0.0
	if totalResults := len(results); totalResults > 0 {
		successRate = float64(transformedCount) / float64(totalResults) * 100
	}
	t.logger.Info().
		Str("job_id", job.ID).
		Int("transformed_count", transformedCount).
		Int("total_results", len(results)).
		Float64("success_rate", successRate).
		Msg("Confluence transformation success rate")

	return nil
}

// parseConfluencePage parses a Confluence page HTML into a Document
func (t *ConfluenceTransformer) parseConfluencePage(body []byte, pageURL string, metadata map[string]interface{}, sourceConfig *models.SourceConfig) (*models.Document, error) {
	// Guard against JSON content (backward compatibility check)
	if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
		t.logger.Warn().Str("url", pageURL).Msg("Detected JSON content, which is no longer supported; skipping parse")
		return nil, fmt.Errorf("JSON content is no longer supported")
	}
	if contentType, ok := metadata["content_type"]; ok {
		if ct, isString := contentType.(string); isString && strings.Contains(strings.ToLower(ct), "application/json") {
			t.logger.Warn().Str("url", pageURL).Str("content_type", ct).Msg("Detected JSON content_type, which is no longer supported; skipping parse")
			return nil, fmt.Errorf("JSON content_type is no longer supported")
		}
	}

	// Convert body to HTML string
	html := string(body)

	// Parse HTML inline using shared helpers from crawler package
	doc, err := crawler.CreateDocument(html)
	if err != nil {
		return nil, fmt.Errorf("failed to create goquery document: %w", err)
	}

	// Extract PageID
	pageID := crawler.ParseConfluencePageID(pageURL)
	// Fallback: Try meta tag
	if pageID == "" {
		pageID = doc.Find(`meta[name="ajs-page-id"]`).AttrOr("content", "")
	}
	// Fallback: Try data attribute
	if pageID == "" {
		pageID = doc.Find(`#main-content`).AttrOr("data-page-id", "")
	}

	// Extract PageTitle
	pageTitle := crawler.ExtractTextFromDoc(doc, []string{
		`#title-text`,
		`[data-test-id="page-title"]`,
		`h1.page-title`,
	})
	// Fallback: Extract from title tag
	if pageTitle == "" {
		titleText := doc.Find("title").First().Text()
		pageTitle = strings.TrimSuffix(titleText, " - Confluence")
		pageTitle = strings.TrimSpace(pageTitle)
	}

	// Extract SpaceKey
	spaceKey := crawler.ParseSpaceKey(pageURL)

	// Extract SpaceName
	spaceName := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="breadcrumbs"] a[href*="/spaces/"]`,
		`.aui-nav-breadcrumbs a`,
	})
	// Fallback: Use SpaceKey as SpaceName if not found
	if spaceName == "" {
		spaceName = spaceKey
	}

	// Extract Content HTML
	contentHTML := crawler.ExtractCleanedHTML(doc, []string{
		`#main-content .wiki-content`,
		`.page-content .wiki-content`,
		`[data-test-id="page-content"]`,
	})

	// Extract Author
	author := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="page-metadata-author"] a`,
		`.author a`,
	})
	// Fallback: Try meta tag
	if author == "" {
		author = doc.Find(`meta[name="confluence-author"]`).AttrOr("content", "")
	}

	// Extract Version
	versionText := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="page-metadata-version"]`,
		`.page-metadata .version`,
	})
	version := 1
	if versionText != "" {
		// Parse version number from text using regex
		re := regexp.MustCompile(`\d+`)
		if match := re.FindString(versionText); match != "" {
			if v, err := strconv.Atoi(match); err == nil {
				version = v
			}
		}
	}

	// Determine ContentType
	contentType := "page"
	if strings.Contains(strings.ToLower(pageURL), "/blogposts/") {
		contentType = "blogpost"
	}

	// Extract LastModified
	lastModifiedStr := crawler.ExtractDateFromDoc(doc, []string{
		`[data-test-id="page-metadata-modified"] time`,
		`.last-modified time`,
	})

	// Extract CreatedDate
	createdDateStr := crawler.ExtractDateFromDoc(doc, []string{
		`[data-test-id="page-metadata-created"] time`,
		`.created time`,
	})

	// Debug logging sampled to reduce noise during large-scale conversions
	if shouldLogDebug() {
		t.logger.Debug().
			Str("page_id", pageID).
			Str("url", pageURL).
			Msg("Parsed Confluence page from HTML")
	}

	// Validate critical fields
	if pageID == "" {
		return nil, fmt.Errorf("missing required field: PageID")
	}
	if pageTitle == "" {
		return nil, fmt.Errorf("missing required field: PageTitle")
	}

	// Resolve document URL early for better link resolution in HTML→MD conversion
	docURL := resolveDocumentURL(pageURL, pageURL, sourceConfig, t.logger)

	// Convert content HTML to markdown using resolved URL as base for link resolution
	contentMarkdown := convertHTMLToMarkdown(contentHTML, docURL, t.enableEmptyOutputFallback, t.logger)

	// Log markdown quality metrics (empty-output check now handled centrally in helper)
	// Debug logging sampled to reduce noise
	if shouldLogDebug() {
		if strings.TrimSpace(contentMarkdown) == "" && contentHTML != "" {
			t.logger.Debug().
				Str("page_id", pageID).
				Msg("Markdown conversion produced empty output despite non-empty HTML content")
		}

		t.logger.Debug().
			Str("page_id", pageID).
			Int("markdown_length", len(contentMarkdown)).
			Int("html_length", len(contentHTML)).
			Msg("Confluence page markdown conversion completed")
	}

	// Parse dates from RFC3339 strings to *time.Time
	var lastModified, createdDate *time.Time
	if lastModifiedStr != "" {
		if parsed, err := time.Parse(time.RFC3339, lastModifiedStr); err == nil {
			lastModified = &parsed
		} else {
			t.logger.Warn().Err(err).Str("date", lastModifiedStr).Msg("Failed to parse last modified date")
		}
	}
	if createdDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, createdDateStr); err == nil {
			createdDate = &parsed
		} else {
			t.logger.Warn().Err(err).Str("date", createdDateStr).Msg("Failed to parse created date")
		}
	}

	// Build metadata using central models.ConfluenceMetadata
	confluenceMetadata := &models.ConfluenceMetadata{
		PageID:       pageID,
		PageTitle:    pageTitle,
		SpaceKey:     spaceKey,
		SpaceName:    spaceName,
		Author:       author,
		Version:      version,
		ContentType:  contentType,
		LastModified: lastModified,
		CreatedDate:  createdDate,
	}

	// Log metadata completeness (sampled)
	if shouldLogDebug() {
		t.logger.Debug().
			Str("page_id", pageID).
			Str("has_author", fmt.Sprintf("%v", confluenceMetadata.Author != "")).
			Str("has_space_name", fmt.Sprintf("%v", confluenceMetadata.SpaceName != "")).
			Int("version", confluenceMetadata.Version).
			Msg("Confluence metadata populated")
	}

	// Convert to map for document storage
	metadataMap, err := confluenceMetadata.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata to map: %w", err)
	}

	// Note: docURL already resolved early for HTML→MD conversion

	// Create document
	now := time.Now()
	document := &models.Document{
		ID:              generateDocumentID(),
		SourceType:      "confluence",
		SourceID:        pageID,
		Title:           pageTitle,
		ContentMarkdown: contentMarkdown,
		Metadata:        metadataMap,
		URL:             docURL,
		DetailLevel:     "full",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return document, nil
}
