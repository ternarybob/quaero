// -----------------------------------------------------------------------
// Content Processor - HTML to Markdown conversion and metadata extraction
// -----------------------------------------------------------------------

package crawler

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ternarybob/arbor"
)

// ContentProcessor handles HTML content processing and markdown conversion
type ContentProcessor struct {
	logger arbor.ILogger
}

// NewContentProcessor creates a new content processor
func NewContentProcessor(logger arbor.ILogger) *ContentProcessor {
	return &ContentProcessor{
		logger: logger,
	}
}

// ProcessedContent represents the result of processing HTML content
type ProcessedContent struct {
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`      // Original HTML content
	Markdown    string                 `json:"markdown"`     // Converted markdown content
	Links       []string               `json:"links"`        // All discovered links
	Metadata    map[string]interface{} `json:"metadata"`     // Extracted metadata
	ProcessTime time.Duration          `json:"process_time"` // Time taken to process
	ContentSize int                    `json:"content_size"` // Size of content in bytes
}

// ProcessHTML processes HTML content and converts it to markdown with metadata extraction
func (p *ContentProcessor) ProcessHTML(html string, sourceURL string) (*ProcessedContent, error) {
	startTime := time.Now()

	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract title
	title := p.extractTitle(doc)

	// Extract main content and convert to markdown
	markdown := p.convertToMarkdown(doc)

	// Extract links
	links := p.extractLinks(doc, sourceURL)

	// Extract metadata
	metadata := p.extractMetadata(doc, sourceURL)

	// Calculate processing time and content size
	processTime := time.Since(startTime)
	contentSize := len(html)

	result := &ProcessedContent{
		Title:       title,
		Content:     html,
		Markdown:    markdown,
		Links:       links,
		Metadata:    metadata,
		ProcessTime: processTime,
		ContentSize: contentSize,
	}

	p.logger.Debug().
		Str("source_url", sourceURL).
		Str("title", title).
		Int("content_size", contentSize).
		Int("links_found", len(links)).
		Dur("process_time", processTime).
		Msg("HTML content processed successfully")

	return result, nil
}

// extractTitle extracts the page title from various sources
func (p *ContentProcessor) extractTitle(doc *goquery.Document) string {
	// Try <title> tag first
	if title := doc.Find("title").First().Text(); title != "" {
		return strings.TrimSpace(title)
	}

	// Try Open Graph title
	if ogTitle, exists := doc.Find("meta[property='og:title']").Attr("content"); exists && ogTitle != "" {
		return strings.TrimSpace(ogTitle)
	}

	// Try h1 tag
	if h1 := doc.Find("h1").First().Text(); h1 != "" {
		return strings.TrimSpace(h1)
	}

	// Try Twitter title
	if twitterTitle, exists := doc.Find("meta[name='twitter:title']").Attr("content"); exists && twitterTitle != "" {
		return strings.TrimSpace(twitterTitle)
	}

	return "Untitled"
}

// convertToMarkdown converts HTML content to markdown format
func (p *ContentProcessor) convertToMarkdown(doc *goquery.Document) string {
	var markdown strings.Builder

	// Remove script and style elements
	doc.Find("script, style, nav, footer, aside").Remove()

	// Process the main content area
	contentSelector := "main, article, .content, .main-content, #content, #main, body"
	content := doc.Find(contentSelector).First()
	if content.Length() == 0 {
		content = doc.Find("body")
	}

	p.processElement(content, &markdown, 0)

	// Clean up the markdown
	result := markdown.String()
	result = p.cleanMarkdown(result)

	return result
}

// processElement recursively processes HTML elements and converts them to markdown
func (p *ContentProcessor) processElement(selection *goquery.Selection, markdown *strings.Builder, depth int) {
	selection.Contents().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) == "#text" {
			// Handle text nodes
			text := strings.TrimSpace(s.Text())
			if text != "" {
				markdown.WriteString(text)
			}
		} else {
			// Handle element nodes
			tagName := goquery.NodeName(s)
			switch tagName {
			case "h1":
				markdown.WriteString("\n\n# ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "h2":
				markdown.WriteString("\n\n## ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "h3":
				markdown.WriteString("\n\n### ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "h4":
				markdown.WriteString("\n\n#### ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "h5":
				markdown.WriteString("\n\n##### ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "h6":
				markdown.WriteString("\n\n###### ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "p":
				markdown.WriteString("\n\n")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "br":
				markdown.WriteString("\n")
			case "strong", "b":
				markdown.WriteString("**")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("**")
			case "em", "i":
				markdown.WriteString("*")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("*")
			case "code":
				markdown.WriteString("`")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("`")
			case "pre":
				markdown.WriteString("\n\n```\n")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n```\n\n")
			case "blockquote":
				markdown.WriteString("\n\n> ")
				p.processElement(s, markdown, depth+1)
				markdown.WriteString("\n\n")
			case "ul":
				markdown.WriteString("\n\n")
				p.processListItems(s, markdown, "-", depth+1)
				markdown.WriteString("\n\n")
			case "ol":
				markdown.WriteString("\n\n")
				p.processListItems(s, markdown, "1.", depth+1)
				markdown.WriteString("\n\n")
			case "li":
				// Handled by processListItems
				p.processElement(s, markdown, depth+1)
			case "a":
				if href, exists := s.Attr("href"); exists {
					markdown.WriteString("[")
					p.processElement(s, markdown, depth+1)
					markdown.WriteString("](")
					markdown.WriteString(href)
					markdown.WriteString(")")
				} else {
					p.processElement(s, markdown, depth+1)
				}
			case "img":
				if src, exists := s.Attr("src"); exists {
					alt, _ := s.Attr("alt")
					markdown.WriteString("![")
					markdown.WriteString(alt)
					markdown.WriteString("](")
					markdown.WriteString(src)
					markdown.WriteString(")")
				}
			case "table":
				p.processTable(s, markdown, depth+1)
			case "hr":
				markdown.WriteString("\n\n---\n\n")
			default:
				// For other elements, just process their content
				p.processElement(s, markdown, depth+1)
			}
		}
	})
}

// processListItems handles list item processing
func (p *ContentProcessor) processListItems(selection *goquery.Selection, markdown *strings.Builder, marker string, depth int) {
	selection.Find("li").Each(func(i int, s *goquery.Selection) {
		markdown.WriteString(marker)
		markdown.WriteString(" ")
		p.processElement(s, markdown, depth+1)
		markdown.WriteString("\n")
	})
}

// processTable handles table conversion to markdown
func (p *ContentProcessor) processTable(selection *goquery.Selection, markdown *strings.Builder, depth int) {
	markdown.WriteString("\n\n")

	// Process table rows
	selection.Find("tr").Each(func(i int, row *goquery.Selection) {
		markdown.WriteString("|")
		row.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			markdown.WriteString(" ")
			p.processElement(cell, markdown, depth+1)
			markdown.WriteString(" |")
		})
		markdown.WriteString("\n")

		// Add header separator for first row
		if i == 0 {
			cellCount := row.Find("td, th").Length()
			markdown.WriteString("|")
			for k := 0; k < cellCount; k++ {
				markdown.WriteString("---|")
			}
			markdown.WriteString("\n")
		}
	})

	markdown.WriteString("\n\n")
}

// cleanMarkdown cleans up the generated markdown
func (p *ContentProcessor) cleanMarkdown(markdown string) string {
	// Remove excessive whitespace
	re := regexp.MustCompile(`\n{3,}`)
	markdown = re.ReplaceAllString(markdown, "\n\n")

	// Remove leading/trailing whitespace
	markdown = strings.TrimSpace(markdown)

	// Remove empty lines at the beginning and end
	lines := strings.Split(markdown, "\n")
	start := 0
	end := len(lines)

	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if start < end {
		return strings.Join(lines[start:end], "\n")
	}

	return ""
}

// extractLinks extracts all links from the HTML document
func (p *ContentProcessor) extractLinks(doc *goquery.Document, sourceURL string) []string {
	var links []string
	linkSet := make(map[string]bool) // For deduplication

	// Parse source URL for resolving relative links
	baseURL, err := url.Parse(sourceURL)
	if err != nil {
		p.logger.Warn().Err(err).Str("source_url", sourceURL).Msg("Failed to parse source URL for link resolution")
		baseURL = nil
	}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		// Skip javascript: and mailto: links
		if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
			return
		}

		// Resolve relative URLs
		if baseURL != nil {
			if resolvedURL, err := baseURL.Parse(href); err == nil {
				href = resolvedURL.String()
			}
		}

		// Deduplicate links
		if !linkSet[href] {
			linkSet[href] = true
			links = append(links, href)
		}
	})

	return links
}

// extractMetadata extracts metadata from the HTML document
func (p *ContentProcessor) extractMetadata(doc *goquery.Document, sourceURL string) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Basic metadata
	metadata["source_url"] = sourceURL
	metadata["extracted_at"] = time.Now().UTC()

	// Meta description
	if description, exists := doc.Find("meta[name='description']").Attr("content"); exists {
		metadata["description"] = strings.TrimSpace(description)
	}

	// Meta keywords
	if keywords, exists := doc.Find("meta[name='keywords']").Attr("content"); exists {
		keywordList := strings.Split(keywords, ",")
		for i, keyword := range keywordList {
			keywordList[i] = strings.TrimSpace(keyword)
		}
		metadata["keywords"] = keywordList
	}

	// Language
	if lang, exists := doc.Find("html").Attr("lang"); exists {
		metadata["language"] = lang
	}

	// Author
	if author, exists := doc.Find("meta[name='author']").Attr("content"); exists {
		metadata["author"] = strings.TrimSpace(author)
	}

	// Canonical URL
	if canonical, exists := doc.Find("link[rel='canonical']").Attr("href"); exists {
		metadata["canonical_url"] = canonical
	}

	// Open Graph metadata
	ogMetadata := make(map[string]string)
	doc.Find("meta[property^='og:']").Each(func(i int, s *goquery.Selection) {
		if property, exists := s.Attr("property"); exists {
			if content, exists := s.Attr("content"); exists {
				ogMetadata[property] = content
			}
		}
	})
	if len(ogMetadata) > 0 {
		metadata["open_graph"] = ogMetadata
	}

	// Twitter Card metadata
	twitterMetadata := make(map[string]string)
	doc.Find("meta[name^='twitter:']").Each(func(i int, s *goquery.Selection) {
		if name, exists := s.Attr("name"); exists {
			if content, exists := s.Attr("content"); exists {
				twitterMetadata[name] = content
			}
		}
	})
	if len(twitterMetadata) > 0 {
		metadata["twitter_card"] = twitterMetadata
	}

	// Content statistics
	textContent := doc.Find("body").Text()
	metadata["text_length"] = len(strings.TrimSpace(textContent))
	metadata["word_count"] = len(strings.Fields(textContent))

	// Count different element types
	metadata["heading_count"] = doc.Find("h1, h2, h3, h4, h5, h6").Length()
	metadata["paragraph_count"] = doc.Find("p").Length()
	metadata["link_count"] = doc.Find("a[href]").Length()
	metadata["image_count"] = doc.Find("img").Length()

	return metadata
}
