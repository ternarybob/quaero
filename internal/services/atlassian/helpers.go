package atlassian

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// generateDocumentID generates a UUID with "doc_" prefix
func generateDocumentID() string {
	return "doc_" + uuid.New().String()
}

// getJobResults attempts to extract results from job metadata or reconstruct from stored data
func getJobResults(job *crawler.CrawlJob, jobStorage interfaces.JobStorage, logger arbor.ILogger) ([]*crawler.CrawlResult, error) {
	// Note: This is a limitation - after service restart, raw API responses are lost
	// unless we re-crawl. The CrawlResult objects are only stored in-memory during
	// the job execution and are not persisted to the database.

	// For now, return empty slice with warning
	logger.Warn().
		Str("job_id", job.ID).
		Msg("Job results unavailable after service restart - raw API responses not persisted")

	return []*crawler.CrawlResult{}, nil
}

// stripHTMLTags removes basic HTML tags for fallback cases
func stripHTMLTags(html string) string {
	// Remove HTML tags using regex
	re := regexp.MustCompile(`<[^>]*>`)
	stripped := re.ReplaceAllString(html, "")

	// Clean up multiple whitespaces
	spaceRe := regexp.MustCompile(`\s+`)
	cleaned := spaceRe.ReplaceAllString(stripped, " ")

	return strings.TrimSpace(cleaned)
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
