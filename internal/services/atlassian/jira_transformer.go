package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// JiraTransformer transforms Jira crawler results into normalized documents
type JiraTransformer struct {
	jobStorage                interfaces.JobStorage
	documentStorage           interfaces.DocumentStorage
	eventService              interfaces.EventService
	crawlerService            interfaces.CrawlerService
	logger                    arbor.ILogger
	enableEmptyOutputFallback bool // Controls whether to apply HTML stripping when MD conversion produces empty output
}

// NewJiraTransformer creates a new Jira transformer and subscribes to collection events
// enableEmptyOutputFallback controls whether to apply HTML stripping fallback when markdown conversion produces empty output
func NewJiraTransformer(jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, crawlerService interfaces.CrawlerService, logger arbor.ILogger, enableEmptyOutputFallback bool) *JiraTransformer {
	t := &JiraTransformer{
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
func (t *JiraTransformer) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	t.logger.Debug().Msg("Jira transformer received collection event")

	// Query for completed jobs
	jobsInterface, err := t.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusCompleted))
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to get completed jobs")
		return fmt.Errorf("failed to get completed jobs: %w", err)
	}

	// Filter for Jira jobs
	var jiraJobs []*models.JobModel
	for _, job := range jobsInterface {
		// Check if source_type in metadata is "jira"
		if sourceType, ok := job.Metadata["source_type"].(string); ok && sourceType == "jira" {
			jiraJobs = append(jiraJobs, job)
		}
	}

	if len(jiraJobs) == 0 {
		t.logger.Debug().Msg("No completed Jira jobs to transform")
		return nil
	}

	t.logger.Info().Int("job_count", len(jiraJobs)).Msg("Processing completed Jira jobs")

	// Transform each job
	var successCount, failCount int
	for _, job := range jiraJobs {
		if err := t.transformJob(ctx, job); err != nil {
			t.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to transform Jira job")
			failCount++
		} else {
			successCount++
		}
	}

	logTransformationSummary(t.logger, "jira", successCount, failCount)
	return nil
}

// transformJob transforms a single Jira job's results into documents
func (t *JiraTransformer) transformJob(ctx context.Context, job *models.JobModel) error {
	// Parse source config snapshot from metadata
	var sourceConfig *models.SourceConfig
	if sourceConfigSnapshot, ok := job.Metadata["source_config_snapshot"].(string); ok && sourceConfigSnapshot != "" {
		if err := json.Unmarshal([]byte(sourceConfigSnapshot), &sourceConfig); err != nil {
			t.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to parse source config snapshot")
		}
	}

	// Get source_type from metadata
	sourceType, _ := job.Metadata["source_type"].(string)

	// Get job results (using crawler service)
	// Note: getJobResults needs to be updated to work with JobModel
	// For now, we'll skip this transformation as it requires crawler service refactoring
	t.logger.Warn().
		Str("job_id", job.ID).
		Str("source_type", sourceType).
		Msg("Jira transformation temporarily disabled - requires crawler service refactoring")
	return nil

	/* TODO: Re-enable after crawler service refactoring
	results, err := getJobResults(job, t.crawlerService, t.logger)
	if err != nil {
		t.logger.Warn().
			Err(err).
			Str("job_id", job.ID).
			Str("source_type", sourceType).
			Msg("Failed to get job results - transformation cannot proceed")
		return err
	}

	if len(results) == 0 {
		t.logger.Warn().
			Str("job_id", job.ID).
			Str("source_type", sourceType).
			Msg("No results available for transformation despite job completion")
		return nil
	}
	*/

	/* TODO: Re-enable after crawler service refactoring
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

		// Parse and transform Jira issue
		doc, err := t.parseJiraIssue(body, result.URL, result.Metadata, sourceConfig)
		if err != nil {
			parseFailures++
			t.logger.Warn().
				Err(err).
				Str("url", result.URL).
				Str("error_type", fmt.Sprintf("%T", err)).
				Int("body_length", len(body)).
				Msg("Failed to parse Jira issue HTML")
			continue
		}

		// Check if document already exists by URL (to avoid duplicates from crawler immediate save)
		existingDoc, err := t.documentStorage.GetDocumentBySource("jira", result.URL)
		if err != nil && err.Error() != "sql: no rows in result set" {
			t.logger.Warn().
				Err(err).
				Str("url", result.URL).
				Msg("Failed to check for existing document by URL")
		}

		// If document exists, update it instead of creating new
		if existingDoc != nil {
			// Preserve existing ID, update content and metadata
			doc.ID = existingDoc.ID
			doc.DetailLevel = models.DetailLevelFull
			doc.CreatedAt = existingDoc.CreatedAt // Preserve original creation time

			t.logger.Debug().
				Str("doc_id", doc.ID).
				Str("issue_key", doc.SourceID).
				Str("url", result.URL).
				Msg("Updating existing document created by crawler")
		}

		// Save document (upsert logic in SaveDocument handles conflict resolution)
		if err := t.documentStorage.SaveDocument(doc); err != nil {
			saveFailures++
			t.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Str("issue_key", doc.SourceID).
				Str("title", doc.Title).
				Msg("Failed to save Jira document to storage")
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
		Msg("Jira transformation complete")

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
		Msg("Jira transformation success rate")

	return nil
	*/
}

// parseJiraIssue parses a Jira issue HTML page into a Document
func (t *JiraTransformer) parseJiraIssue(body []byte, pageURL string, metadata map[string]interface{}, sourceConfig *models.SourceConfig) (*models.Document, error) {
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

	// Extract IssueKey
	issueKey := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.issue-base.foundation.breadcrumbs.current-issue.item"]`,
		`#key-val`,
		`#issuekey-val`,
	})
	// Fallback: Parse from page title
	if issueKey == "" {
		titleText := doc.Find("title").First().Text()
		issueKey = crawler.ParseJiraIssueKey(titleText)
	}

	// Extract ProjectKey from IssueKey
	var projectKey string
	if issueKey != "" {
		parts := strings.Split(issueKey, "-")
		if len(parts) > 0 {
			projectKey = parts[0]
		}
	}

	// Extract Summary
	summary := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.issue-base.foundation.summary.heading"]`,
		`#summary-val`,
		`h1[data-test-id="issue-view-heading"]`,
	})
	// Fallback: Extract from title tag
	if summary == "" {
		titleText := doc.Find("title").First().Text()
		summary = strings.TrimSuffix(titleText, " - Jira")
		summary = strings.TrimSpace(summary)
	}

	// Extract Description HTML
	descriptionHTML := crawler.ExtractCleanedHTML(doc, []string{
		`[data-test-id="issue.views.field.rich-text.description"]`,
		`#description-val`,
		`.user-content-block`,
	})

	// Extract IssueType
	issueType := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.field.issue-type"] span`,
		`#type-val`,
		`.issue-type-icon`,
	})

	// Extract Status
	status := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.field.status"] span`,
		`#status-val`,
		`.status`,
		`.aui-lozenge`,
	})
	status = crawler.NormalizeStatus(status)

	// Extract Priority
	priority := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.field.priority"] span`,
		`#priority-val`,
		`.priority-icon`,
	})

	// Extract Assignee
	assignee := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.field.assignee"] span`,
		`#assignee-val`,
		`.user-hover`,
	})
	if strings.Contains(strings.ToLower(assignee), "unassigned") {
		assignee = ""
	}

	// Extract Reporter
	reporter := crawler.ExtractTextFromDoc(doc, []string{
		`[data-test-id="issue.views.field.reporter"] span`,
		`#reporter-val`,
	})

	// Extract Labels
	labels := crawler.ExtractMultipleTextsFromDoc(doc, []string{
		`[data-test-id="issue.views.field.labels"] a`,
		`#labels-val .labels a`,
		`.labels .lozenge`,
	})

	// Extract Components
	components := crawler.ExtractMultipleTextsFromDoc(doc, []string{
		`[data-test-id="issue.views.field.components"] a`,
		`#components-val a`,
	})

	// Extract CreatedDate
	createdDateStr := crawler.ExtractDateFromDoc(doc, []string{
		`[data-test-id="issue.views.field.created"] time`,
		`#created-val time`,
	})

	// Extract UpdatedDate
	updatedDateStr := crawler.ExtractDateFromDoc(doc, []string{
		`[data-test-id="issue.views.field.updated"] time`,
		`#updated-val time`,
	})

	// Extract ResolutionDate
	resolutionDateStr := crawler.ExtractDateFromDoc(doc, []string{
		`[data-test-id="issue.views.field.resolved"] time`,
	})

	// Debug logging sampled to reduce noise during large-scale conversions
	if shouldLogDebug() {
		t.logger.Debug().
			Str("issue_key", issueKey).
			Str("url", pageURL).
			Msg("Parsed Jira issue from HTML")
	}

	// Validate critical fields
	if issueKey == "" {
		return nil, fmt.Errorf("missing required field: IssueKey")
	}
	if summary == "" {
		return nil, fmt.Errorf("missing required field: Summary")
	}

	// Resolve document URL early for better link resolution in HTML→MD conversion
	docURL := resolveDocumentURL(pageURL, pageURL, sourceConfig, t.logger)

	// Convert description HTML to markdown using resolved URL as base for link resolution
	contentMarkdown := convertHTMLToMarkdown(descriptionHTML, docURL, t.enableEmptyOutputFallback, t.logger)

	// Log markdown quality metrics (empty-output check now handled centrally in helper)
	// Debug logging sampled to reduce noise
	if shouldLogDebug() {
		if strings.TrimSpace(contentMarkdown) == "" && descriptionHTML != "" {
			t.logger.Debug().
				Str("issue_key", issueKey).
				Msg("Markdown conversion produced empty output despite non-empty HTML description")
		}

		t.logger.Debug().
			Str("issue_key", issueKey).
			Int("markdown_length", len(contentMarkdown)).
			Int("html_length", len(descriptionHTML)).
			Msg("Jira issue markdown conversion completed")
	}

	// Parse dates from RFC3339 strings to *time.Time
	var createdDate, updatedDate, resolutionDate *time.Time
	if createdDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, createdDateStr); err == nil {
			createdDate = &parsed
		} else {
			t.logger.Warn().Err(err).Str("date", createdDateStr).Msg("Failed to parse created date")
		}
	}
	if updatedDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, updatedDateStr); err == nil {
			updatedDate = &parsed
		} else {
			t.logger.Warn().Err(err).Str("date", updatedDateStr).Msg("Failed to parse updated date")
		}
	}
	if resolutionDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, resolutionDateStr); err == nil {
			resolutionDate = &parsed
		} else {
			t.logger.Warn().Err(err).Str("date", resolutionDateStr).Msg("Failed to parse resolution date")
		}
	}

	// Build metadata using central models.JiraMetadata
	jiraMetadata := &models.JiraMetadata{
		IssueKey:       issueKey,
		ProjectKey:     projectKey,
		IssueType:      issueType,
		Status:         status,
		Priority:       priority,
		Assignee:       assignee,
		Reporter:       reporter,
		Labels:         labels,
		Components:     components,
		Summary:        summary,
		CreatedDate:    createdDate,
		UpdatedDate:    updatedDate,
		ResolutionDate: resolutionDate,
	}

	// Log metadata completeness (sampled)
	if shouldLogDebug() {
		t.logger.Debug().
			Str("issue_key", issueKey).
			Str("has_assignee", fmt.Sprintf("%v", jiraMetadata.Assignee != "")).
			Str("has_labels", fmt.Sprintf("%v", len(jiraMetadata.Labels) > 0)).
			Str("has_components", fmt.Sprintf("%v", len(jiraMetadata.Components) > 0)).
			Msg("Jira metadata populated")
	}

	// Convert to map for document storage
	metadataMap, err := jiraMetadata.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata to map: %w", err)
	}

	// Note: docURL already resolved early for HTML→MD conversion

	// Create document
	now := time.Now()
	document := &models.Document{
		ID:              common.NewDocumentID(),
		SourceType:      "jira",
		SourceID:        issueKey,
		Title:           summary,
		ContentMarkdown: contentMarkdown,
		Metadata:        metadataMap,
		URL:             docURL,
		DetailLevel:     "full",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return document, nil
}
