// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:12:11 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package atlassian

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// JiraScraperService scrapes Jira projects and issues
type JiraScraperService struct {
	authService     interfaces.AtlassianAuthService
	jiraStorage     interfaces.JiraStorage
	documentService interfaces.DocumentService
	eventService    interfaces.EventService
	logger          arbor.ILogger
	uiLogger        interface{}
}

// NewJiraScraperService creates a new Jira scraper service
func NewJiraScraperService(
	jiraStorage interfaces.JiraStorage,
	documentService interfaces.DocumentService,
	authService interfaces.AtlassianAuthService,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *JiraScraperService {
	service := &JiraScraperService{
		jiraStorage:     jiraStorage,
		documentService: documentService,
		authService:     authService,
		eventService:    eventService,
		logger:          logger,
	}

	// Subscribe to collection events
	if eventService != nil {
		handler := func(ctx context.Context, event interfaces.Event) error {
			return service.handleCollectionEvent(ctx, event)
		}
		if err := eventService.Subscribe(interfaces.EventCollectionTriggered, handler); err != nil {
			logger.Error().Err(err).Msg("Failed to subscribe Jira service to collection events")
		}
	}

	return service
}

// Close closes the scraper and releases resources
func (s *JiraScraperService) Close() error {
	return nil
}

// SetUILogger sets a UI logger for real-time updates
func (s *JiraScraperService) SetUILogger(logger interface{}) {
	s.uiLogger = logger
}

func (s *JiraScraperService) makeRequest(method, path string) ([]byte, error) {
	if !s.authService.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated: please authenticate using Chrome extension")
	}

	reqURL := s.authService.GetBaseURL() + path

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.authService.GetUserAgent())
	req.Header.Set("Accept", "application/json, text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	httpClient := s.authService.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized: authentication required")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		s.logger.Error().
			Str("url", reqURL).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("HTTP request failed")

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("auth expired (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, readErr
}

// transformToDocument converts Jira issue to normalized document
func (s *JiraScraperService) transformToDocument(issue *models.JiraIssue) (*models.Document, error) {
	docID := fmt.Sprintf("doc_%s", uuid.New().String())

	// Extract fields from the Fields map
	summary := s.getStringField(issue.Fields, "summary")
	description := s.getStringField(issue.Fields, "description")

	// Extract project info
	projectKey := ""
	if project, ok := issue.Fields["project"].(map[string]interface{}); ok {
		projectKey = s.getStringField(project, "key")
	}

	// Extract issue type
	issueType := ""
	if issueTypeObj, ok := issue.Fields["issuetype"].(map[string]interface{}); ok {
		issueType = s.getStringField(issueTypeObj, "name")
	}

	// Extract status
	status := ""
	if statusObj, ok := issue.Fields["status"].(map[string]interface{}); ok {
		status = s.getStringField(statusObj, "name")
	}

	// Extract priority
	priority := ""
	if priorityObj, ok := issue.Fields["priority"].(map[string]interface{}); ok {
		priority = s.getStringField(priorityObj, "name")
	}

	// Extract assignee and reporter
	assignee := ""
	if assigneeObj, ok := issue.Fields["assignee"].(map[string]interface{}); ok {
		assignee = s.getStringField(assigneeObj, "displayName")
	}

	reporter := ""
	if reporterObj, ok := issue.Fields["reporter"].(map[string]interface{}); ok {
		reporter = s.getStringField(reporterObj, "displayName")
	}

	// Extract labels
	labels := []string{}
	if labelsArray, ok := issue.Fields["labels"].([]interface{}); ok {
		for _, label := range labelsArray {
			if labelStr, ok := label.(string); ok {
				labels = append(labels, labelStr)
			}
		}
	}

	// Extract components
	components := []string{}
	if componentsArray, ok := issue.Fields["components"].([]interface{}); ok {
		for _, comp := range componentsArray {
			if compMap, ok := comp.(map[string]interface{}); ok {
				components = append(components, s.getStringField(compMap, "name"))
			}
		}
	}

	// Build plain text content
	content := fmt.Sprintf("Issue: %s\n\nSummary: %s\n\nDescription:\n%s\n\nProject: %s\nType: %s\nStatus: %s\nPriority: %s\nAssignee: %s\nReporter: %s\nLabels: %v\nComponents: %v",
		issue.Key, summary, description, projectKey, issueType, status, priority, assignee, reporter, labels, components)

	// Build markdown content
	contentMD := fmt.Sprintf("# %s\n\n**Summary:** %s\n\n## Description\n\n%s\n\n## Details\n\n- **Project:** %s\n- **Type:** %s\n- **Status:** %s\n- **Priority:** %s\n- **Assignee:** %s\n- **Reporter:** %s\n- **Labels:** %v\n- **Components:** %v",
		issue.Key, summary, description, projectKey, issueType, status, priority, assignee, reporter, labels, components)

	// Extract resolution date if available
	var resolutionDate *time.Time
	if resolutionDateStr, ok := issue.Fields["resolutiondate"].(string); ok && resolutionDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, resolutionDateStr); err == nil {
			resolutionDate = &parsed
		}
	}

	// Extract created/updated dates (we already parse these below, so reference them)
	var createdDate, updatedDate *time.Time
	if createdStr, ok := issue.Fields["created"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			createdDate = &parsed
		}
	}
	if updatedStr, ok := issue.Fields["updated"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			updatedDate = &parsed
		}
	}

	// Build metadata
	metadata := models.JiraMetadata{
		IssueKey:       issue.Key,
		ProjectKey:     projectKey,
		IssueType:      issueType,
		Status:         status,
		Priority:       priority,
		Assignee:       assignee,
		Reporter:       reporter,
		Labels:         labels,
		Components:     components,
		Summary:        summary,
		ResolutionDate: resolutionDate,
		CreatedDate:    createdDate,
		UpdatedDate:    updatedDate,
	}

	metadataMap, err := metadata.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata: %w", err)
	}

	// Extract timestamps
	now := time.Now()
	createdAt := now
	updatedAt := now

	if createdStr, ok := issue.Fields["created"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			createdAt = parsed
		}
	}

	if updatedStr, ok := issue.Fields["updated"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			updatedAt = parsed
		}
	}

	return &models.Document{
		ID:              docID,
		SourceType:      "jira",
		SourceID:        issue.Key,
		Title:           fmt.Sprintf("[%s] %s", issue.Key, summary),
		Content:         content,
		ContentMarkdown: contentMD,
		Metadata:        metadataMap,
		URL:             fmt.Sprintf("%s/browse/%s", s.authService.GetBaseURL(), issue.Key),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

// getStringField safely extracts a string field from a map
func (s *JiraScraperService) getStringField(m map[string]interface{}, field string) string {
	if val, ok := m[field]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// handleCollectionEvent processes collection triggered events
// NOTE: This does NOT scrape/download data - scraping is user-driven
// This event triggers processing of already-scraped data (issues → documents)
func (s *JiraScraperService) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	s.logger.Info().Msg(">>> JIRA SERVICE: Collection push event received")

	// Run processing synchronously (not in goroutine) to prevent overlap with embedding
	s.logger.Debug().Msg(">>> JIRA SERVICE: Starting collection push (issues → documents)")

	// Get all projects from storage
	projects, err := s.jiraStorage.GetAllProjects(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg(">>> JIRA SERVICE: Failed to get projects")
		return err
	}

	if len(projects) == 0 {
		s.logger.Info().Msg(">>> JIRA SERVICE: No projects found - nothing to process")
		return nil
	}

	// Process issues for each project
	totalIssues := 0
	totalDocuments := 0
	for _, project := range projects {
		s.logger.Debug().
			Str("project", project.Key).
			Msg(">>> JIRA SERVICE: Processing project issues")

		err := s.ProcessIssuesForProject(ctx, project.Key)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("project", project.Key).
				Msg(">>> JIRA SERVICE: Failed to process project")
			continue
		}

		// Get count for logging
		count, _ := s.jiraStorage.CountIssuesByProject(ctx, project.Key)
		totalIssues += count
		totalDocuments += count
	}

	s.logger.Info().
		Int("projects", len(projects)).
		Int("issues", totalIssues).
		Int("documents", totalDocuments).
		Msg(">>> JIRA SERVICE: Collection push completed successfully")

	return nil
}

// ProcessIssuesForProject transforms and saves Jira issues as documents
func (s *JiraScraperService) ProcessIssuesForProject(ctx context.Context, projectKey string) error {
	// Get issues from storage
	issues, err := s.jiraStorage.GetIssuesByProject(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("failed to get issues: %w", err)
	}

	if len(issues) == 0 {
		s.logger.Info().Str("project", projectKey).Msg("No issues to process")
		return nil
	}

	// Transform to documents
	documents := make([]*models.Document, 0, len(issues))
	for _, issue := range issues {
		doc, err := s.transformToDocument(issue)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("issue", issue.Key).
				Msg("Failed to transform issue")
			continue
		}
		documents = append(documents, doc)
	}

	// Save documents (embedding handled independently by coordinator)
	if err := s.documentService.SaveDocuments(ctx, documents); err != nil {
		return fmt.Errorf("failed to save documents: %w", err)
	}

	s.logger.Info().
		Str("project", projectKey).
		Int("issues", len(issues)).
		Int("documents", len(documents)).
		Msg("Processed Jira issues to documents")

	return nil
}

// GetProjectStatus returns the last updated time and details for Jira projects
func (s *JiraScraperService) GetProjectStatus() (lastUpdated int64, details string, err error) {
	ctx := context.Background()
	project, timestamp, err := s.jiraStorage.GetMostRecentProject(ctx)
	if err != nil {
		// No projects found or error
		return 0, "No projects found", nil
	}

	details = fmt.Sprintf("Project %s (%s) was scanned and added to the database", project.Key, project.Name)
	return timestamp, details, nil
}

// GetIssueStatus returns the last updated time and details for Jira issues
func (s *JiraScraperService) GetIssueStatus() (lastUpdated int64, details string, err error) {
	ctx := context.Background()
	issue, timestamp, err := s.jiraStorage.GetMostRecentIssue(ctx)
	if err != nil {
		// No issues found or error
		return 0, "No issues found", nil
	}

	// Extract summary from fields
	summary := s.getStringField(issue.Fields, "summary")
	if summary == "" {
		summary = issue.Key
	}

	details = fmt.Sprintf("Issue %s (%s) was scanned and added to the database", issue.Key, summary)
	return timestamp, details, nil
}
