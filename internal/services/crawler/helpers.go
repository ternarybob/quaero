// Package crawler provides HTML parsing utilities and helpers.
// These helpers are used by both the crawler service and specialized transformers
// (jira_transformer, confluence_transformer) for extracting structured data from HTML.
package crawler

import (
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// CreateDocument creates a goquery.Document from HTML string.
// Used by transformers to parse HTML into goquery.Document for CSS selector-based extraction.
func CreateDocument(html string) (*goquery.Document, error) {
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

// ExtractTextFromDoc tries multiple selectors in priority order and returns text from first match.
// Used by transformers to extract text using multiple fallback selectors for resilience.
func ExtractTextFromDoc(doc *goquery.Document, selectors []string) string {
	for _, selector := range selectors {
		text := strings.TrimSpace(doc.Find(selector).Text())
		if text != "" {
			return text
		}
	}
	return ""
}

// ExtractMultipleTextsFromDoc collects text from all matching elements for selectors.
// Used by transformers to extract arrays (labels, components) from HTML.
func ExtractMultipleTextsFromDoc(doc *goquery.Document, selectors []string) []string {
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

// ExtractCleanedHTML extracts and cleans HTML from multiple selectors.
// Used by transformers to extract and clean HTML content (descriptions, page content).
func ExtractCleanedHTML(doc *goquery.Document, selectors []string) string {
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

// ExtractDateFromDoc extracts date from time element, trying datetime attribute first.
// Used by transformers to extract and normalize dates from time elements.
func ExtractDateFromDoc(doc *goquery.Document, selectors []string) string {
	for _, selector := range selectors {
		element := doc.Find(selector)
		if element.Length() > 0 {
			if datetime, exists := element.Attr("datetime"); exists && datetime != "" {
				if normalized := normalizeDateToRFC3339(datetime); normalized != "" {
					return normalized
				}
				return datetime
			}
			text := strings.TrimSpace(element.Text())
			if text != "" {
				if normalized := normalizeDateToRFC3339(text); normalized != "" {
					return normalized
				}
			}
		}
	}
	return ""
}

// normalizeDateToRFC3339 attempts to parse common date formats and convert to RFC3339 (ISO 8601)
func normalizeDateToRFC3339(dateStr string) string {
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

	return ""
}

// ParseJiraIssueKey extracts issue key from text using regex.
// Used by jira_transformer to extract issue keys from text using regex.
func ParseJiraIssueKey(text string) string {
	issueKeyRegex := regexp.MustCompile(`([A-Z][A-Z0-9]+-\d+)`)
	if match := issueKeyRegex.FindString(text); match != "" {
		return match
	}
	return ""
}

// ParseConfluencePageID extracts page ID from URL path.
// Used by confluence_transformer to extract page IDs from URLs.
func ParseConfluencePageID(urlPath string) string {
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

// ParseSpaceKey extracts space key from URL path.
// Used by confluence_transformer to extract space keys from URLs.
func ParseSpaceKey(urlPath string) string {
	spaceKeyRegex := regexp.MustCompile(`(?i)/spaces/([A-Za-z0-9_-]+)/`)
	if match := spaceKeyRegex.FindStringSubmatch(urlPath); len(match) > 1 {
		return match[1]
	}
	return ""
}

// NormalizeStatus normalizes common status variations to canonical forms.
// Used by jira_transformer to normalize status values to canonical forms.
func NormalizeStatus(status string) string {
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
