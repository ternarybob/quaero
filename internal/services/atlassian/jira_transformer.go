package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// JiraTransformer transforms Jira crawler results into normalized documents
type JiraTransformer struct {
	jobStorage      interfaces.JobStorage
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// JiraMetadata represents Jira-specific metadata
type JiraMetadata struct {
	IssueKey   string   `json:"issue_key"`
	ProjectKey string   `json:"project_key"`
	IssueType  string   `json:"issue_type"`
	Status     string   `json:"status"`
	Priority   string   `json:"priority"`
	Assignee   string   `json:"assignee,omitempty"`
	Reporter   string   `json:"reporter,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	Components []string `json:"components,omitempty"`
}

// ToMap converts JiraMetadata to a map for storage
func (m *JiraMetadata) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	result["issue_key"] = m.IssueKey
	result["project_key"] = m.ProjectKey
	result["issue_type"] = m.IssueType
	result["status"] = m.Status
	result["priority"] = m.Priority

	if m.Assignee != "" {
		result["assignee"] = m.Assignee
	}
	if m.Reporter != "" {
		result["reporter"] = m.Reporter
	}
	if len(m.Labels) > 0 {
		result["labels"] = m.Labels
	}
	if len(m.Components) > 0 {
		result["components"] = m.Components
	}

	return result
}

// NewJiraTransformer creates a new Jira transformer and subscribes to collection events
func NewJiraTransformer(jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, logger arbor.ILogger) *JiraTransformer {
	t := &JiraTransformer{
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
func (t *JiraTransformer) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	t.logger.Debug().Msg("Jira transformer received collection event")

	// Query for completed jobs
	jobsInterface, err := t.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusCompleted))
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to get completed jobs")
		return fmt.Errorf("failed to get completed jobs: %w", err)
	}

	// Filter for Jira jobs
	var jiraJobs []*crawler.CrawlJob
	for _, jobInterface := range jobsInterface {
		if job, ok := jobInterface.(*crawler.CrawlJob); ok {
			if job.SourceType == "jira" {
				jiraJobs = append(jiraJobs, job)
			}
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
func (t *JiraTransformer) transformJob(ctx context.Context, job *crawler.CrawlJob) error {
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

		// Parse and transform Jira issue
		doc, err := t.parseJiraIssue(body, sourceConfig)
		if err != nil {
			t.logger.Warn().Err(err).Str("url", result.URL).Msg("Failed to parse Jira issue")
			continue
		}

		// Save document
		if err := t.documentStorage.SaveDocument(doc); err != nil {
			t.logger.Error().Err(err).Str("issue_key", doc.SourceID).Msg("Failed to save Jira document")
			continue
		}

		transformedCount++
	}

	t.logger.Info().
		Str("job_id", job.ID).
		Int("transformed_count", transformedCount).
		Int("total_results", len(results)).
		Msg("Transformed Jira job results")

	return nil
}

// parseJiraIssue parses a Jira issue JSON response into a Document
func (t *JiraTransformer) parseJiraIssue(body []byte, sourceConfig *models.SourceConfig) (*models.Document, error) {
	var issue struct {
		ID     string `json:"id"`
		Key    string `json:"key"`
		Self   string `json:"self"`
		Fields struct {
			Summary     string                 `json:"summary"`
			Description map[string]interface{} `json:"description"` // ADF format
			IssueType   struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
			Assignee struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
			Reporter struct {
				DisplayName string `json:"displayName"`
			} `json:"reporter"`
			Labels     []string `json:"labels"`
			Components []struct {
				Name string `json:"name"`
			} `json:"components"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
		} `json:"fields"`
	}

	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Jira issue: %w", err)
	}

	// Convert ADF description to markdown
	contentMarkdown := t.adfToMarkdown(issue.Fields.Description)

	// Build metadata
	var componentNames []string
	for _, comp := range issue.Fields.Components {
		componentNames = append(componentNames, comp.Name)
	}

	metadata := &JiraMetadata{
		IssueKey:   issue.Key,
		ProjectKey: issue.Fields.Project.Key,
		IssueType:  issue.Fields.IssueType.Name,
		Status:     issue.Fields.Status.Name,
		Priority:   issue.Fields.Priority.Name,
		Assignee:   issue.Fields.Assignee.DisplayName,
		Reporter:   issue.Fields.Reporter.DisplayName,
		Labels:     issue.Fields.Labels,
		Components: componentNames,
	}

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              generateDocumentID(),
		SourceType:      "jira",
		SourceID:        issue.Key,
		Title:           issue.Fields.Summary,
		ContentMarkdown: contentMarkdown,
		Metadata:        metadata.ToMap(),
		URL:             issue.Self,
		DetailLevel:     "full",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return doc, nil
}

// adfToMarkdown converts Atlassian Document Format to markdown
func (t *JiraTransformer) adfToMarkdown(adf map[string]interface{}) string {
	if adf == nil {
		return ""
	}

	var markdown strings.Builder

	// Extract content array
	contentRaw, ok := adf["content"]
	if !ok {
		return ""
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return ""
	}

	// Process each content node
	for _, nodeRaw := range content {
		node, ok := nodeRaw.(map[string]interface{})
		if !ok {
			continue
		}

		nodeType, _ := node["type"].(string)

		switch nodeType {
		case "paragraph":
			t.processParagraph(node, &markdown)
			markdown.WriteString("\n\n")

		case "heading":
			t.processHeading(node, &markdown)
			markdown.WriteString("\n\n")

		case "bulletList":
			t.processBulletList(node, &markdown)
			markdown.WriteString("\n")

		case "orderedList":
			t.processOrderedList(node, &markdown)
			markdown.WriteString("\n")

		case "codeBlock":
			t.processCodeBlock(node, &markdown)
			markdown.WriteString("\n\n")
		}
	}

	return strings.TrimSpace(markdown.String())
}

// processParagraph processes a paragraph node
func (t *JiraTransformer) processParagraph(node map[string]interface{}, markdown *strings.Builder) {
	contentRaw, ok := node["content"]
	if !ok {
		return
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return
	}

	for _, itemRaw := range content {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := item["type"].(string)
		if itemType == "text" {
			text, _ := item["text"].(string)
			markdown.WriteString(text)
		}
	}
}

// processHeading processes a heading node
func (t *JiraTransformer) processHeading(node map[string]interface{}, markdown *strings.Builder) {
	level, _ := node["attrs"].(map[string]interface{})["level"].(float64)
	headingPrefix := strings.Repeat("#", int(level))

	markdown.WriteString(headingPrefix + " ")

	contentRaw, ok := node["content"]
	if !ok {
		return
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return
	}

	for _, itemRaw := range content {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := item["type"].(string)
		if itemType == "text" {
			text, _ := item["text"].(string)
			markdown.WriteString(text)
		}
	}
}

// processBulletList processes a bullet list node
func (t *JiraTransformer) processBulletList(node map[string]interface{}, markdown *strings.Builder) {
	contentRaw, ok := node["content"]
	if !ok {
		return
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return
	}

	for _, itemRaw := range content {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		markdown.WriteString("- ")
		t.processListItem(item, markdown)
		markdown.WriteString("\n")
	}
}

// processOrderedList processes an ordered list node
func (t *JiraTransformer) processOrderedList(node map[string]interface{}, markdown *strings.Builder) {
	contentRaw, ok := node["content"]
	if !ok {
		return
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return
	}

	for i, itemRaw := range content {
		item, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		markdown.WriteString(fmt.Sprintf("%d. ", i+1))
		t.processListItem(item, markdown)
		markdown.WriteString("\n")
	}
}

// processListItem processes a list item node
func (t *JiraTransformer) processListItem(node map[string]interface{}, markdown *strings.Builder) {
	contentRaw, ok := node["content"]
	if !ok {
		return
	}

	content, ok := contentRaw.([]interface{})
	if !ok {
		return
	}

	for _, paraRaw := range content {
		para, ok := paraRaw.(map[string]interface{})
		if !ok {
			continue
		}

		t.processParagraph(para, markdown)
	}
}

// processCodeBlock processes a code block node
func (t *JiraTransformer) processCodeBlock(node map[string]interface{}, markdown *strings.Builder) {
	language := ""
	if attrs, ok := node["attrs"].(map[string]interface{}); ok {
		if lang, ok := attrs["language"].(string); ok {
			language = lang
		}
	}

	markdown.WriteString("```" + language + "\n")

	contentRaw, ok := node["content"]
	if ok {
		content, ok := contentRaw.([]interface{})
		if ok {
			for _, itemRaw := range content {
				item, ok := itemRaw.(map[string]interface{})
				if !ok {
					continue
				}

				itemType, _ := item["type"].(string)
				if itemType == "text" {
					text, _ := item["text"].(string)
					markdown.WriteString(text)
				}
			}
		}
	}

	markdown.WriteString("\n```")
}
