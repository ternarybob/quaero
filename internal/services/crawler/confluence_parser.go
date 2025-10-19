package crawler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ternarybob/arbor"
)

// ConfluencePageData contains all extracted data from a Confluence page
type ConfluencePageData struct {
	PageID       string   // Page ID extracted from URL or meta tags
	PageTitle    string   // Page title
	SpaceKey     string   // Space key extracted from URL path or breadcrumb
	SpaceName    string   // Space name from breadcrumb or space link
	Content      string   // Page content (HTML or markdown)
	Author       string   // Page author
	Version      int      // Page version number
	ContentType  string   // "page" or "blogpost"
	LastModified string   // Last modified date
	CreatedDate  string   // Created date
	Labels       []string // Page labels
	URL          string   // Full URL to the page
	RawHTML      string   // Raw HTML for debugging/fallback
}

// ParseConfluencePage extracts structured data from a Confluence page
func ParseConfluencePage(html string, pageURL string, logger arbor.ILogger) (*ConfluencePageData, error) {
	doc, err := createDocument(html)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	data := &ConfluencePageData{
		URL:     pageURL,
		RawHTML: html,
	}

	// Extract page ID
	data.PageID = extractConfluencePageID(doc, pageURL, logger)

	// Extract page title
	data.PageTitle = extractConfluenceTitle(doc, logger)

	// Extract space key and name
	data.SpaceKey, data.SpaceName = extractConfluenceSpace(doc, pageURL, logger)

	// Extract content
	data.Content = extractConfluenceContent(doc, logger)

	// Extract metadata
	data.Author = extractConfluenceAuthor(doc, logger)
	data.Version = extractConfluenceVersion(doc, logger)
	data.ContentType = extractConfluenceContentType(pageURL)

	// Extract dates
	data.LastModified, data.CreatedDate = extractConfluenceDates(doc, logger)

	// Extract labels
	data.Labels = extractConfluenceLabels(doc, logger)

	// Validate critical fields
	if err := validateConfluenceCriticalFields(data, logger); err != nil {
		return nil, err
	}

	return data, nil
}

// extractConfluencePageID extracts the page ID from URL or meta tags
func extractConfluencePageID(doc *goquery.Document, pageURL string, logger arbor.ILogger) string {
	pageID := parseConfluencePageID(pageURL)
	if pageID == "" {
		// Try meta tag
		if metaContent, exists := doc.Find(`meta[name="ajs-page-id"]`).Attr("content"); exists {
			pageID = metaContent
		}
	}
	if pageID == "" {
		// Try data attribute
		if dataPageID, exists := doc.Find(`#main-content`).Attr("data-page-id"); exists {
			pageID = dataPageID
		}
	}
	if pageID == "" {
		logger.Warn().Str("url", pageURL).Msg("Page ID not found")
	}
	return pageID
}

// extractConfluenceTitle extracts the page title
func extractConfluenceTitle(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`#title-text`,
		`[data-test-id="page-title"]`,
		`h1.page-title`,
	}
	title := extractTextFromDoc(doc, selectors, logger)
	if title == "" {
		// Fallback to title tag
		title = doc.Find("title").Text()
		title = strings.TrimSuffix(title, " - Confluence")
		title = strings.TrimSpace(title)
	}
	return title
}

// extractConfluenceSpace extracts space key and name
func extractConfluenceSpace(doc *goquery.Document, pageURL string, logger arbor.ILogger) (key, name string) {
	key = parseSpaceKey(pageURL)

	selectors := []string{
		`[data-test-id="breadcrumbs"] a[href*="/spaces/"]`,
		`.aui-nav-breadcrumbs a`,
	}
	name = extractTextFromDoc(doc, selectors, logger)
	if name == "" {
		name = key // Fallback to space key
	}
	return key, name
}

// extractConfluenceContent extracts the page content HTML
func extractConfluenceContent(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`#main-content .wiki-content`,
		`.page-content .wiki-content`,
		`[data-test-id="page-content"]`,
	}
	return extractCleanedHTML(doc, selectors, logger)
}

// extractConfluenceAuthor extracts the page author
func extractConfluenceAuthor(doc *goquery.Document, logger arbor.ILogger) string {
	selectors := []string{
		`[data-test-id="page-metadata-author"] a`,
		`.author a`,
	}
	author := extractTextFromDoc(doc, selectors, logger)
	if author == "" {
		// Try meta tag
		if metaAuthor, exists := doc.Find(`meta[name="confluence-author"]`).Attr("content"); exists {
			author = metaAuthor
		}
	}
	return author
}

// extractConfluenceVersion extracts the page version number
func extractConfluenceVersion(doc *goquery.Document, logger arbor.ILogger) int {
	selectors := []string{
		`[data-test-id="page-metadata-version"]`,
		`.page-metadata .version`,
	}
	versionText := extractTextFromDoc(doc, selectors, logger)
	version := 1
	if versionText != "" {
		// Parse from "Version {number}" text
		versionRegex := regexp.MustCompile(`\d+`)
		if match := versionRegex.FindString(versionText); match != "" {
			if v, err := strconv.Atoi(match); err == nil {
				version = v
			}
		}
	}
	return version
}

// extractConfluenceContentType determines if page or blogpost
func extractConfluenceContentType(pageURL string) string {
	if strings.Contains(pageURL, "/blogposts/") {
		return "blogpost"
	}
	return "page"
}

// extractConfluenceDates extracts last modified and created dates
func extractConfluenceDates(doc *goquery.Document, logger arbor.ILogger) (lastModified, created string) {
	lastModifiedSelectors := []string{
		`[data-test-id="page-metadata-modified"] time`,
		`.last-modified time`,
	}
	lastModified = extractDateFromDoc(doc, lastModifiedSelectors, logger)

	createdSelectors := []string{
		`[data-test-id="page-metadata-created"] time`,
		`.created time`,
	}
	created = extractDateFromDoc(doc, createdSelectors, logger)

	return lastModified, created
}

// extractConfluenceLabels extracts all page labels
func extractConfluenceLabels(doc *goquery.Document, logger arbor.ILogger) []string {
	selectors := []string{
		`[data-test-id="page-metadata-labels"] a`,
		`.labels a.label`,
		`.aui-label`,
	}
	return extractMultipleTextsFromDoc(doc, selectors, logger)
}

// validateConfluenceCriticalFields validates that required fields are present
func validateConfluenceCriticalFields(data *ConfluencePageData, logger arbor.ILogger) error {
	var missingFields []string
	if data.PageID == "" {
		missingFields = append(missingFields, "PageID")
	}
	if data.PageTitle == "" {
		missingFields = append(missingFields, "PageTitle")
	}
	if len(missingFields) > 0 {
		logger.Error().Str("url", data.URL).Strs("missing_fields", missingFields).
			Msg("Critical fields missing from Confluence page")
		return fmt.Errorf("critical fields missing from Confluence page at %s: %s",
			data.URL, strings.Join(missingFields, ", "))
	}
	return nil
}
