package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// ConfluenceTransformer transforms Confluence crawler results into normalized documents
type ConfluenceTransformer struct {
	jobStorage      interfaces.JobStorage
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// ConfluenceMetadata represents Confluence-specific metadata
type ConfluenceMetadata struct {
	PageID      string `json:"page_id"`
	PageTitle   string `json:"page_title"`
	SpaceKey    string `json:"space_key"`
	SpaceName   string `json:"space_name"`
	Author      string `json:"author,omitempty"`
	Version     int    `json:"version"`
	ContentType string `json:"content_type"`
}

// ToMap converts ConfluenceMetadata to a map for storage
func (m *ConfluenceMetadata) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["page_id"] = m.PageID
	result["page_title"] = m.PageTitle
	result["space_key"] = m.SpaceKey
	result["space_name"] = m.SpaceName
	result["version"] = m.Version
	result["content_type"] = m.ContentType

	if m.Author != "" {
		result["author"] = m.Author
	}

	return result
}

// NewConfluenceTransformer creates a new Confluence transformer and subscribes to collection events
func NewConfluenceTransformer(jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, logger arbor.ILogger) *ConfluenceTransformer {
	t := &ConfluenceTransformer{
		jobStorage:      jobStorage,
		documentStorage: documentStorage,
		eventService:    eventService,
		logger:          logger,
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

	// Get job results (note: may be unavailable after restart)
	results, err := getJobResults(job, t.jobStorage, t.logger)
	if err != nil {
		t.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to get job results")
		return err
	}

	if len(results) == 0 {
		t.logger.Debug().Str("job_id", job.ID).Msg("No results available for transformation")
		return nil
	}

	// Transform each result
	var transformedCount int
	for _, result := range results {
		if result.Error != "" {
			continue // Skip failed requests
		}

		// Extract response body
		bodyRaw, ok := result.Metadata["response_body"]
		if !ok {
			continue
		}

		body, ok := bodyRaw.([]byte)
		if !ok {
			continue
		}

		// Parse and transform Confluence page
		doc, err := t.parseConfluencePage(body, sourceConfig)
		if err != nil {
			t.logger.Warn().Err(err).Str("url", result.URL).Msg("Failed to parse Confluence page")
			continue
		}

		// Save document
		if err := t.documentStorage.SaveDocument(doc); err != nil {
			t.logger.Error().Err(err).Str("page_id", doc.SourceID).Msg("Failed to save Confluence document")
			continue
		}

		transformedCount++
	}

	t.logger.Info().
		Str("job_id", job.ID).
		Int("transformed_count", transformedCount).
		Int("total_results", len(results)).
		Msg("Transformed Confluence job results")

	return nil
}

// parseConfluencePage parses a Confluence page JSON response into a Document
func (t *ConfluenceTransformer) parseConfluencePage(body []byte, sourceConfig *models.SourceConfig) (*models.Document, error) {
	var page struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
		Body  struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		Space struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"space"`
		Version struct {
			Number int `json:"number"`
			By     struct {
				DisplayName string `json:"displayName"`
			} `json:"by"`
		} `json:"version"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}

	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Confluence page: %w", err)
	}

	// Convert storage format HTML to markdown
	contentMarkdown := t.storageToMarkdown(page.Body.Storage.Value)

	// Build metadata
	metadata := &ConfluenceMetadata{
		PageID:      page.ID,
		PageTitle:   page.Title,
		SpaceKey:    page.Space.Key,
		SpaceName:   page.Space.Name,
		Author:      page.Version.By.DisplayName,
		Version:     page.Version.Number,
		ContentType: page.Type,
	}

	// Build full URL (combine base URL with webui path if needed)
	pageURL := page.Links.WebUI
	if sourceConfig != nil && !strings.HasPrefix(pageURL, "http") {
		// Relative URL, construct full URL
		pageURL = sourceConfig.BaseURL + pageURL
	}

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              generateDocumentID(),
		SourceType:      "confluence",
		SourceID:        page.ID,
		Title:           page.Title,
		ContentMarkdown: contentMarkdown,
		Metadata:        metadata.ToMap(),
		URL:             pageURL,
		DetailLevel:     "full",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return doc, nil
}

// storageToMarkdown converts Confluence storage format HTML to markdown
func (t *ConfluenceTransformer) storageToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	markdown := html

	// Convert headings
	for i := 6; i >= 1; i-- {
		openTag := fmt.Sprintf("<h%d>", i)
		closeTag := fmt.Sprintf("</h%d>", i)
		headingPrefix := strings.Repeat("#", i)
		markdown = strings.ReplaceAll(markdown, openTag, headingPrefix+" ")
		markdown = strings.ReplaceAll(markdown, closeTag, "\n\n")
	}

	// Convert paragraphs
	markdown = strings.ReplaceAll(markdown, "<p>", "")
	markdown = strings.ReplaceAll(markdown, "</p>", "\n\n")

	// Convert line breaks
	markdown = strings.ReplaceAll(markdown, "<br>", "\n")
	markdown = strings.ReplaceAll(markdown, "<br/>", "\n")
	markdown = strings.ReplaceAll(markdown, "<br />", "\n")

	// Convert bold
	markdown = strings.ReplaceAll(markdown, "<strong>", "**")
	markdown = strings.ReplaceAll(markdown, "</strong>", "**")
	markdown = strings.ReplaceAll(markdown, "<b>", "**")
	markdown = strings.ReplaceAll(markdown, "</b>", "**")

	// Convert italic
	markdown = strings.ReplaceAll(markdown, "<em>", "*")
	markdown = strings.ReplaceAll(markdown, "</em>", "*")
	markdown = strings.ReplaceAll(markdown, "<i>", "*")
	markdown = strings.ReplaceAll(markdown, "</i>", "*")

	// Convert code
	markdown = strings.ReplaceAll(markdown, "<code>", "`")
	markdown = strings.ReplaceAll(markdown, "</code>", "`")

	// Convert preformatted text to code blocks
	re := regexp.MustCompile(`<pre>(.*?)</pre>`)
	markdown = re.ReplaceAllString(markdown, "```\n$1\n```\n")

	// Convert unordered lists
	markdown = strings.ReplaceAll(markdown, "<ul>", "")
	markdown = strings.ReplaceAll(markdown, "</ul>", "\n")
	markdown = strings.ReplaceAll(markdown, "<li>", "- ")
	markdown = strings.ReplaceAll(markdown, "</li>", "\n")

	// Convert ordered lists (simple approach - assumes sequential numbering)
	markdown = strings.ReplaceAll(markdown, "<ol>", "")
	markdown = strings.ReplaceAll(markdown, "</ol>", "\n")
	// Note: This is a simplification - proper ordered list conversion would require parsing

	// Convert links
	linkRe := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	markdown = linkRe.ReplaceAllString(markdown, "[$2]($1)")

	// Strip remaining HTML tags (Confluence macros, etc.)
	markdown = stripHTMLTags(markdown)

	// Clean up excessive whitespace
	markdown = regexp.MustCompile(`\n{3,}`).ReplaceAllString(markdown, "\n\n")
	markdown = strings.TrimSpace(markdown)

	return markdown
}
