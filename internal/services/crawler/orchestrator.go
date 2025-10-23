package crawler

// orchestrator.go contains link filtering functions for URL processing.
// Worker management has been migrated to queue.WorkerPool.
// Job execution is handled by queue-based job types (internal/jobs/types/crawler.go).
// VERIFICATION COMMENT 9: Progress tracking functions removed (now handled by queue-based jobs).
// VERIFICATION COMMENT 3: Regex imports removed (filtering now handled by shared LinkFilter helper).

// filterLinks applies include/exclude patterns using shared LinkFilter helper
// VERIFICATION COMMENT 3: Replaced duplicate filtering logic with shared LinkFilter (DRY principle)
// Returns: (filteredLinks, excludedSamples, notIncludedSamples)
func (s *Service) filterLinks(jobID string, links []string, config CrawlConfig) ([]string, []string, []string) {
	// VERIFICATION COMMENT 3: Use shared LinkFilter helper (DRY principle)
	// Source type is "web" for generic crawler links (no Jira/Confluence validation needed)
	linkFilter := NewLinkFilter(config.IncludePatterns, config.ExcludePatterns, "web", s.logger)

	// Apply filtering using shared helper
	filtered, excludedLinks, notIncludedLinks := linkFilter.FilterLinks(links)

	// Summary log for filtered links
	if len(excludedLinks) > 0 || len(notIncludedLinks) > 0 {
		s.logger.Debug().
			Int("excluded_count", len(excludedLinks)).
			Int("not_included_count", len(notIncludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Pattern filtering summary")

		// Comment 2: Removed DEBUG database logging for pattern filtering to prevent log bloat
		// Pattern filtering details are visible in console logs above
	}

	// Collect samples for database logging (Comment 9: limit to 2 each to avoid log spam)
	excludedSamples := []string{}
	if len(excludedLinks) > 0 {
		sampleCount := 2
		if len(excludedLinks) < 2 {
			sampleCount = len(excludedLinks)
		}
		excludedSamples = excludedLinks[:sampleCount]
	}

	notIncludedSamples := []string{}
	if len(notIncludedLinks) > 0 {
		sampleCount := 2
		if len(notIncludedLinks) < 2 {
			sampleCount = len(notIncludedLinks)
		}
		notIncludedSamples = notIncludedLinks[:sampleCount]
	}

	return filtered, excludedSamples, notIncludedSamples
}

// VERIFICATION COMMENT 9: Removed unused progress tracking functions:
// - updateProgress() - not called anywhere, progress now tracked in CrawlJob via JobStorage
// - updateCurrentURL() - not called anywhere, current URL tracked in queue messages
// - updatePendingCount() - not called anywhere, pending count managed by queue system
// - emitProgress() - not called anywhere, progress events emitted from job handlers
