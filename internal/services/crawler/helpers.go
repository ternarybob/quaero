package crawler

import (
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ternarybob/arbor"
)

// createDocument creates a goquery.Document from HTML string
func createDocument(html string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

// filterUIElements removes UI-only elements like toolbars, buttons, edit controls from a selection
func filterUIElements(selection *goquery.Selection) *goquery.Selection {
	cloned := selection.Clone()

	uiSelectors := []string{
		"button", ".toolbar", ".edit-button",
		"[data-test-id*='button']", "[data-test-id*='edit']", "[data-test-id*='toolbar']",
		".comment-block", ".comments-section",
		".page-metadata", ".issue-metadata", ".actions",
		"[role='toolbar']", "[role='button']",
		".aui-button", ".aui-toolbar",
	}

	for _, selector := range uiSelectors {
		cloned.Find(selector).Remove()
	}

	return cloned
}

// extractTextFromDoc tries multiple selectors in priority order and returns text from first match
func extractTextFromDoc(doc *goquery.Document, selectors []string, logger arbor.ILogger) string {
	for _, selector := range selectors {
		text := strings.TrimSpace(doc.Find(selector).Text())
		if text != "" {
			return text
		}
	}
	logger.Debug().Strs("selectors", selectors).Msg("No matching selector found")
	return ""
}

// extractMultipleTextsFromDoc collects text from all matching elements for selectors
func extractMultipleTextsFromDoc(doc *goquery.Document, selectors []string, logger arbor.ILogger) []string {
	textMap := make(map[string]bool)
	var results []string

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && !textMap[text] {
				textMap[text] = true
				results = append(results, text)
			}
		})
		if len(results) > 0 {
			break
		}
	}

	return results
}

// extractCleanedHTML extracts and cleans HTML from multiple selectors
func extractCleanedHTML(doc *goquery.Document, selectors []string, logger arbor.ILogger) string {
	for _, selector := range selectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			cleanedSelection := filterUIElements(selection)
			html, err := cleanedSelection.Html()
			if err == nil && html != "" {
				return html
			}
		}
	}
	return ""
}

// extractDateFromDoc extracts date from time element, trying datetime attribute first
func extractDateFromDoc(doc *goquery.Document, selectors []string, logger arbor.ILogger) string {
	for _, selector := range selectors {
		element := doc.Find(selector)
		if element.Length() > 0 {
			if datetime, exists := element.Attr("datetime"); exists && datetime != "" {
				if normalized := normalizeDateToRFC3339(datetime, logger); normalized != "" {
					return normalized
				}
				return datetime
			}
			text := strings.TrimSpace(element.Text())
			if text != "" {
				if normalized := normalizeDateToRFC3339(text, logger); normalized != "" {
					return normalized
				}
			}
		}
	}
	return ""
}

// normalizeDateToRFC3339 attempts to parse common date formats and convert to RFC3339 (ISO 8601)
func normalizeDateToRFC3339(dateStr string, logger arbor.ILogger) string {
	formats := []string{
		time.RFC3339, "2006-01-02T15:04:05.999Z", "2006-01-02T15:04:05",
		"2006-01-02 15:04:05", "2006-01-02",
		"02 Jan 2006", "02 Jan 2006 15:04",
		"Jan 2, 2006", "Jan 2, 2006 3:04 PM", "January 2, 2006",
		"2006/01/02", "01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format(time.RFC3339)
		}
	}

	logger.Warn().Str("date_string", dateStr).Msg("Failed to parse date format")
	return ""
}

// parseJiraIssueKey extracts issue key from text using regex
func parseJiraIssueKey(text string) string {
	issueKeyRegex := regexp.MustCompile(`([A-Z][A-Z0-9]+-\d+)`)
	if match := issueKeyRegex.FindString(text); match != "" {
		return match
	}
	return ""
}

// parseConfluencePageID extracts page ID from URL path
func parseConfluencePageID(urlPath string) string {
	pageIDRegex := regexp.MustCompile(`/pages/(\d+)/`)
	if match := pageIDRegex.FindStringSubmatch(urlPath); len(match) > 1 {
		return match[1]
	}
	altRegex := regexp.MustCompile(`pageId=(\d+)`)
	if match := altRegex.FindStringSubmatch(urlPath); len(match) > 1 {
		return match[1]
	}
	return ""
}

// parseSpaceKey extracts space key from URL path
func parseSpaceKey(urlPath string) string {
	spaceKeyRegex := regexp.MustCompile(`(?i)/spaces/([A-Za-z0-9_-]+)/`)
	if match := spaceKeyRegex.FindStringSubmatch(urlPath); len(match) > 1 {
		return match[1]
	}
	return ""
}

// normalizeStatus normalizes common status variations to canonical forms
func normalizeStatus(status string) string {
	status = strings.TrimSpace(status)
	statusMap := map[string]string{
		"TODO": "To Do", "IN PROGRESS": "In Progress",
		"DONE": "Done", "CLOSED": "Closed", "OPEN": "Open",
	}
	upperStatus := strings.ToUpper(status)
	if canonical, exists := statusMap[upperStatus]; exists {
		return canonical
	}
	return status
}
