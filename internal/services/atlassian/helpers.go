package atlassian

import (
	"crypto/sha256"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// Debug logging sampling configuration
// To enable detailed conversion logging for troubleshooting:
// - Set debugLogSampleRate to 1 to log every conversion
// - Or set your log level to DEBUG and review sampled output
var (
	debugLogCounter    uint64 = 0
	debugLogSampleRate uint64 = 10 // Log every 10th conversion (adjust as needed)
)

// shouldLogDebug determines whether to log debug messages based on sampling rate
func shouldLogDebug() bool {
	count := atomic.AddUint64(&debugLogCounter, 1)
	return count%debugLogSampleRate == 0
}

// resolveDocumentURL resolves a document URL to an absolute URL using sourceConfig.BaseURL and pageURL as fallbacks
// This ensures that relative links in HTML→MD conversion can be properly resolved
func resolveDocumentURL(documentURL string, pageURL string, sourceConfig *models.SourceConfig, logger arbor.ILogger) string {
	// Already absolute - return as-is
	if strings.HasPrefix(documentURL, "http://") || strings.HasPrefix(documentURL, "https://") {
		return documentURL
	}

	resolvedURL := documentURL

	// Try to resolve using sourceConfig.BaseURL first
	if sourceConfig != nil && sourceConfig.BaseURL != "" {
		baseURL, err := url.Parse(sourceConfig.BaseURL)
		if err != nil {
			if shouldLogDebug() {
				logger.Debug().Err(err).Str("base_url", sourceConfig.BaseURL).Msg("Failed to parse base URL")
			}
		} else {
			relativeURL, err := url.Parse(documentURL)
			if err != nil {
				if shouldLogDebug() {
					logger.Debug().Err(err).Str("relative_url", documentURL).Msg("Failed to parse relative URL")
				}
			} else {
				resolved := baseURL.ResolveReference(relativeURL)
				resolvedURL = resolved.String()
				if shouldLogDebug() {
					logger.Debug().
						Str("original_url", documentURL).
						Str("resolved_url", resolvedURL).
						Msg("Resolved relative URL to absolute using base URL")
				}
				return resolvedURL
			}
		}
	}

	// Fallback: resolve against pageURL if still relative
	if !strings.HasPrefix(resolvedURL, "http://") && !strings.HasPrefix(resolvedURL, "https://") {
		pageBase, err := url.Parse(pageURL)
		if err != nil {
			if shouldLogDebug() {
				logger.Debug().Err(err).Str("page_url", pageURL).Msg("Failed to parse pageURL for fallback resolution")
			}
		} else {
			rel, err := url.Parse(resolvedURL)
			if err != nil {
				if shouldLogDebug() {
					logger.Debug().Err(err).Str("relative_url", resolvedURL).Msg("Failed to parse relative URL for fallback resolution")
				}
			} else {
				resolved := pageBase.ResolveReference(rel)
				resolvedURL = resolved.String()
				if shouldLogDebug() {
					logger.Debug().
						Str("original_url", documentURL).
						Str("resolved_url", resolvedURL).
						Str("via", "pageURL").
						Msg("Resolved relative URL to absolute via pageURL fallback")
				}
			}
		}
	}

	return resolvedURL
}

// selectResultBody selects content from a CrawlResult in priority order:
// Priority 1: metadata["html"] - preferred for HTML parsing
// Priority 2: metadata["response_body"] - backward compatibility
// Priority 3: result.Body if it looks like HTML (starts with "<")
// Note: Does not fall back to markdown to avoid breaking HTML parsers
func selectResultBody(result *crawler.CrawlResult) []byte {
	// Priority 1: metadata["html"]
	if htmlRaw, ok := result.Metadata["html"]; ok {
		if html, isString := htmlRaw.(string); isString && html != "" {
			return []byte(html)
		}
	}

	// Priority 2: metadata["response_body"] (backward compatibility)
	if bodyRaw, ok := result.Metadata["response_body"]; ok {
		switch v := bodyRaw.(type) {
		case []byte:
			return v
		case string:
			return []byte(v)
		}
	}

	// Priority 3: result.Body if it looks like HTML
	if result.Body != nil && len(result.Body) > 0 {
		trimmed := strings.TrimSpace(string(result.Body))
		if strings.HasPrefix(trimmed, "<") {
			return result.Body
		}
	}

	// Do not fall back to markdown for HTML parsers
	return nil
}

// getJobResults retrieves job results from the crawler service
func getJobResults(job *crawler.CrawlJob, crawlerService interfaces.CrawlerService, logger arbor.ILogger) ([]*crawler.CrawlResult, error) {
	// Use crawler service's GetJobResults to retrieve results
	resultsInterface, err := crawlerService.GetJobResults(job.ID)
	if err != nil {
		// If job is completed but results are unavailable, return empty slice with warning
		// This allows transformers to proceed gracefully instead of failing
		if job.Status == crawler.JobStatusCompleted {
			logger.Warn().
				Err(err).
				Str("job_id", job.ID).
				Str("status", string(job.Status)).
				Msg("Job completed but results unavailable - returning empty results for graceful handling")
			return []*crawler.CrawlResult{}, nil
		}

		// For non-completed jobs, return error as before
		logger.Warn().
			Err(err).
			Str("job_id", job.ID).
			Str("status", string(job.Status)).
			Msg("Failed to get job results from crawler service")
		return []*crawler.CrawlResult{}, err
	}

	// Type assert to []*crawler.CrawlResult
	results, ok := resultsInterface.([]*crawler.CrawlResult)
	if !ok {
		logger.Warn().
			Str("job_id", job.ID).
			Msg("Unexpected result type from GetJobResults")
		return []*crawler.CrawlResult{}, nil
	}

	return results, nil
}

// stripHTMLTags removes basic HTML tags for fallback cases
func stripHTMLTags(htmlStr string) string {
	// Remove HTML tags using regex
	re := regexp.MustCompile(`<[^>]*>`)
	stripped := re.ReplaceAllString(htmlStr, "")

	// Clean up multiple whitespaces
	spaceRe := regexp.MustCompile(`\s+`)
	cleaned := spaceRe.ReplaceAllString(stripped, " ")

	// Decode HTML entities
	unescaped := html.UnescapeString(cleaned)

	return strings.TrimSpace(unescaped)
}

// convertHTMLToMarkdown converts HTML content to markdown with fallback to stripHTMLTags on error
// enableEmptyOutputFallback: when true, applies HTML stripping fallback if markdown conversion produces empty output
// This option is configurable via CrawlerConfig.EnableEmptyOutputFallback (default: true)
func convertHTMLToMarkdown(html string, baseURL string, enableEmptyOutputFallback bool, logger arbor.ILogger) string {
	if html == "" {
		return ""
	}

	// Log input HTML length (sampled to reduce noise during large-scale conversions)
	if shouldLogDebug() {
		logger.Debug().Int("html_length", len(html)).Msg("Converting HTML to markdown")
	}

	// Try HTML-to-markdown conversion
	mdConverter := md.NewConverter(baseURL, true, nil)
	converted, err := mdConverter.ConvertString(html)
	if err != nil {
		// WARN logs are always unconditional for error tracking
		logger.Warn().Err(err).Str("fallback", "stripHTMLTags").Msg("HTML to markdown conversion failed, using fallback")
		// Fallback: strip HTML tags
		stripped := stripHTMLTags(html)
		if shouldLogDebug() {
			logger.Debug().Int("stripped_length", len(stripped)).Msg("Fallback HTML stripping completed")
		}
		return stripped
	}

	// Log output markdown length and compression ratio (sampled)
	if shouldLogDebug() {
		logger.Debug().
			Int("markdown_length", len(converted)).
			Int("html_length", len(html)).
			Msg("HTML to markdown conversion successful")
	}

	// Validate markdown output quality
	trimmedMarkdown := strings.TrimSpace(converted)
	if trimmedMarkdown == "" && html != "" {
		// Empty output despite non-empty HTML - check if fallback is enabled
		if enableEmptyOutputFallback {
			// Apply fallback strip (WARN is unconditional)
			logger.Warn().
				Int("html_length", len(html)).
				Msg("HTML to markdown conversion produced empty output despite non-empty HTML, applying fallback strip")
			stripped := stripHTMLTags(html)
			if shouldLogDebug() {
				logger.Debug().Int("stripped_length", len(stripped)).Msg("Fallback HTML stripping completed after empty conversion")
			}
			return stripped
		} else {
			// Fallback disabled - return empty output and log (sampled)
			if shouldLogDebug() {
				logger.Debug().
					Int("html_length", len(html)).
					Msg("HTML to markdown conversion produced empty output but fallback is disabled, returning empty")
			}
			return converted
		}
	} else if len(trimmedMarkdown) < 10 {
		// Calculate HTML hash for correlation without logging content (sampled)
		if shouldLogDebug() {
			htmlHash := sha256.Sum256([]byte(html))
			logger.Debug().
				Int("length", len(converted)).
				Str("base_url", baseURL).
				Str("html_hash", fmt.Sprintf("%x", htmlHash[:8])).
				Msg("HTML to markdown conversion produced very short output")
		}
	}

	return converted
}

// truncateString truncates long strings with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// aggregateErrors combines multiple errors into single error with count
func aggregateErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	var msgs []string
	for _, err := range errs {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}

	return fmt.Errorf("%d errors occurred: %s", len(errs), strings.Join(msgs, "; "))
}

// logTransformationSummary logs transformation results
func logTransformationSummary(logger arbor.ILogger, sourceType string, successCount, failCount int) {
	if successCount > 0 {
		logger.Info().
			Str("source_type", sourceType).
			Int("success_count", successCount).
			Int("fail_count", failCount).
			Msgf("Transformed %d %s items into documents", successCount, sourceType)
	} else if failCount > 0 {
		logger.Warn().
			Str("source_type", sourceType).
			Int("fail_count", failCount).
			Msg("No items successfully transformed")
	} else {
		logger.Debug().
			Str("source_type", sourceType).
			Msg("No items to transform")
	}
}
