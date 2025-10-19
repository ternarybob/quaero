package crawler

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ternarybob/arbor"
)

// JiraIssueData contains all extracted data from a Jira issue page
type JiraIssueData struct {
	IssueKey       string   // Issue key (e.g., "PROJ-123")
	ProjectKey     string   // Project key extracted from issue key
	Summary        string   // Issue title/summary
	Description    string   // Issue description content (HTML or markdown)
	IssueType      string   // Bug, Story, Task, Epic
	Status         string   // Open, In Progress, Done
	Priority       string   // Priority level
	Assignee       string   // Assigned user
	Reporter       string   // Reporter user
	Labels         []string // Labels array
	Components     []string // Components array
	CreatedDate    string   // Created date
	UpdatedDate    string   // Updated date
	ResolutionDate string   // Resolution date if issue is resolved
	URL            string   // Full URL to the issue page
	RawHTML        string   // Raw HTML for debugging/fallback
}

// ParseJiraIssuePage extracts structured data from a Jira issue page
func ParseJiraIssuePage(html string, pageURL string, logger arbor.ILogger) (*JiraIssueData, error) {
	doc, err := createDocument(html)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	data := &JiraIssueData{
		URL:     pageURL,
		RawHTML: html,
	}

	// Extract issue key and project key
	data.IssueKey = extractJiraIssueKey(doc, logger)
	if data.IssueKey != "" {
		parts := strings.Split(data.IssueKey, "-")
		if len(parts) >= 2 {
			data.ProjectKey = parts[0]
		}
	}

	// Extract summary
	data.Summary = extractJiraSummary(doc, logger)

	// Extract description
	data.Description = extractJiraDescription(doc, logger)

	// Extract metadata fields
	data.IssueType = extractJiraIssueType(doc, logger)
	data.Status = extractJiraStatus(doc, logger)
	data.Priority = extractJiraPriority(doc, logger)
	data.Assignee = extractJiraAssignee(doc, logger)
	data.Reporter = extractJiraReporter(doc, logger)

	// Extract arrays
	data.Labels = extractJiraLabels(doc, logger)
	data.Components = extractJiraComponents(doc, logger)

	// Extract dates
	data.CreatedDate, data.UpdatedDate, data.ResolutionDate = extractJiraDates(doc, logger)

	// Validate critical fields
	if err := validateJiraCriticalFields(data, logger); err != nil {
		return nil, err
	}

	return data, nil
}

// extractJiraIssueKey extracts the issue key from the document
func extractJiraIssueKey(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.issue-base.foundation.breadcrumbs.current-issue.item"]`,
		`#key-val`,
		`#issuekey-val`,
	}
	issueKey := extractTextFromDoc(doc, selectors, logger)
	if issueKey == "" {
		// Try to parse from page title using regex
		title := doc.Find("title").Text()
		issueKey = parseJiraIssueKey(title)
	}
	if issueKey == "" {
		logger.Warn().Msg("Issue key not found")
	}
	return issueKey
}

// extractJiraSummary extracts the issue summary
func extractJiraSummary(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.issue-base.foundation.summary.heading"]`,
		`#summary-val`,
		`h1[data-test-id="issue-view-heading"]`,
	}
	summary := extractTextFromDoc(doc, selectors, logger)
	if summary == "" {
		// Fallback to title tag and strip suffix
		title := doc.Find("title").Text()
		summary = strings.TrimSuffix(title, " - Jira")
		summary = strings.TrimSpace(summary)
	}
	return summary
}

// extractJiraDescription extracts the issue description HTML
func extractJiraDescription(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.rich-text.description"]`,
		`#description-val`,
		`.user-content-block`,
	}
	return extractCleanedHTML(doc, selectors, logger)
}

// extractJiraIssueType extracts the issue type
func extractJiraIssueType(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.issue-type"] span`,
		`#type-val`,
		`.issue-type-icon`,
	}
	return extractTextFromDoc(doc, selectors, logger)
}

// extractJiraStatus extracts and normalizes the issue status
func extractJiraStatus(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.status"] span`,
		`#status-val`,
		`.status`,
		`.aui-lozenge`,
	}
	status := extractTextFromDoc(doc, selectors, logger)
	return normalizeStatus(status)
}

// extractJiraPriority extracts the priority
func extractJiraPriority(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.priority"] span`,
		`#priority-val`,
		`.priority-icon`,
	}
	return extractTextFromDoc(doc, selectors, logger)
}

// extractJiraAssignee extracts the assignee
func extractJiraAssignee(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.assignee"] span`,
		`#assignee-val`,
		`.user-hover`,
	}
	assignee := extractTextFromDoc(doc, selectors, logger)
	if assignee == "Unassigned" {
		return ""
	}
	return assignee
}

// extractJiraReporter extracts the reporter
func extractJiraReporter(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="issue.views.field.reporter"] span`,
		`#reporter-val`,
	}
	return extractTextFromDoc(doc, selectors, logger)
}

// extractJiraLabels extracts all labels
func extractJiraLabels(doc *goquery.Document, logger arbor.ILogger) []string {
	selectors := []string{
		`[data-test-id="issue.views.field.labels"] a`,
		`#labels-val .labels a`,
		`.labels .lozenge`,
	}
	return extractMultipleTextsFromDoc(doc, selectors, logger)
}

// extractJiraComponents extracts all components
func extractJiraComponents(doc *goquery.Document, logger arbor.ILogger) []string {
	selectors := []string{
		`[data-test-id="issue.views.field.components"] a`,
		`#components-val a`,
	}
	return extractMultipleTextsFromDoc(doc, selectors, logger)
}

// extractJiraDates extracts created, updated, and resolution dates
func extractJiraDates(doc *goquery.Document, logger arbor.ILogger) (created, updated, resolved string) {
	createdSelectors := []string{
		`[data-test-id="issue.views.field.created"] time`,
		`#created-val time`,
	}
	created = extractDateFromDoc(doc, createdSelectors, logger)

	updatedSelectors := []string{
		`[data-test-id="issue.views.field.updated"] time`,
		`#updated-val time`,
	}
	updated = extractDateFromDoc(doc, updatedSelectors, logger)

	resolvedSelectors := []string{
		`[data-test-id="issue.views.field.resolved"] time`,
	}
	resolved = extractDateFromDoc(doc, resolvedSelectors, logger)

	return created, updated, resolved
}

// validateJiraCriticalFields validates that required fields are present
func validateJiraCriticalFields(data *JiraIssueData, logger arbor.ILogger) error {
	var missingFields []string
	if data.IssueKey == "" {
		missingFields = append(missingFields, "IssueKey")
	}
	if data.Summary == "" {
		missingFields = append(missingFields, "Summary")
	}
	if len(missingFields) > 0 {
		logger.Error().Str("url", data.URL).Strs("missing_fields", missingFields).
			Msg("Critical fields missing from Jira issue page")
		return fmt.Errorf("critical fields missing from Jira issue page at %s: %s",
			data.URL, strings.Join(missingFields, ", "))
	}
	return nil
}
