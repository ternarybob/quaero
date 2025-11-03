package transform

import (
	"fmt"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/ternarybob/arbor"
)

// Service provides HTML to markdown conversion functionality
// This is a generic service that can transform any HTML content
// Future: Can be extended to use LLM for intelligent transformation
type Service struct {
	logger arbor.ILogger
}

// NewService creates a new transform service
func NewService(logger arbor.ILogger) *Service {
	return &Service{
		logger: logger,
	}
}

// HTMLToMarkdown converts HTML content to markdown
// baseURL is used for resolving relative links
// Returns markdown string or error if conversion fails
func (s *Service) HTMLToMarkdown(html string, baseURL string) (string, error) {
	if html == "" {
		return "", nil
	}

	s.logger.Debug().
		Int("html_length", len(html)).
		Str("base_url", baseURL).
		Msg("Converting HTML to markdown")

	// Try HTML-to-markdown conversion
	mdConverter := md.NewConverter(baseURL, true, nil)
	converted, err := mdConverter.ConvertString(html)
	if err != nil {
		s.logger.Warn().Err(err).Msg("HTML to markdown conversion failed, using fallback")
		// Fallback: strip HTML tags
		stripped := stripHTMLTags(html)
		s.logger.Debug().Int("stripped_length", len(stripped)).Msg("Fallback HTML stripping completed")
		return stripped, nil // Return stripped version but no error
	}

	s.logger.Debug().
		Int("markdown_length", len(converted)).
		Int("html_length", len(html)).
		Msg("HTML to markdown conversion successful")

	// Check for empty output
	trimmedMarkdown := strings.TrimSpace(converted)
	if trimmedMarkdown == "" && html != "" {
		s.logger.Warn().
			Int("html_length", len(html)).
			Msg("HTML to markdown conversion produced empty output, applying fallback")
		stripped := stripHTMLTags(html)
		return stripped, nil
	}

	return converted, nil
}

// stripHTMLTags removes basic HTML tags for fallback cases
func stripHTMLTags(htmlStr string) string {
	// Remove HTML tags using regex
	re := regexp.MustCompile(`<[^>]*>`)
	stripped := re.ReplaceAllString(htmlStr, "")

	// Clean up multiple whitespaces
	spaceRe := regexp.MustCompile(`\s+`)
	cleaned := spaceRe.ReplaceAllString(stripped, " ")

	// Decode HTML entities (basic set)
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")

	return strings.TrimSpace(cleaned)
}

// ValidateHTML checks if the input looks like valid HTML
func (s *Service) ValidateHTML(content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return fmt.Errorf("empty content")
	}

	// Basic HTML validation - check for HTML tags
	if !strings.Contains(trimmed, "<") {
		return fmt.Errorf("content does not appear to be HTML")
	}

	return nil
}
