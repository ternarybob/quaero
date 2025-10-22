package crawler

// orchestrator.go contains orchestration functions for managing workers, progress tracking,
// and job monitoring. The orchestrator coordinates multiple workers and tracks their progress.

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// startWorkers launches worker goroutines for a job
func (s *Service) startWorkers(jobID string, config CrawlConfig) {
	for i := 0; i < config.Concurrency; i++ {
		s.wg.Add(1)
		go s.workerLoop(jobID, i, config)
	}

	// Monitor completion
	go s.monitorCompletion(jobID)
}

// filterLinks applies include/exclude patterns
// Returns: (filteredLinks, excludedSamples, notIncludedSamples)
func (s *Service) filterLinks(jobID string, links []string, config CrawlConfig) ([]string, []string, []string) {
	// Precompile exclude patterns
	excludeRegexes := make([]*regexp.Regexp, 0, len(config.ExcludePatterns))
	for _, pattern := range config.ExcludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			excludeRegexes = append(excludeRegexes, re)
		}
	}

	// Precompile include patterns
	includeRegexes := make([]*regexp.Regexp, 0, len(config.IncludePatterns))
	for _, pattern := range config.IncludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			includeRegexes = append(includeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var excludedLinks, notIncludedLinks []string

	for _, link := range links {
		// Apply exclude patterns
		excluded := false
		var matchedExcludePattern string
		for _, re := range excludeRegexes {
			if re.MatchString(link) {
				excluded = true
				matchedExcludePattern = re.String()
				break
			}
		}
		if excluded {
			excludedLinks = append(excludedLinks, link)
			s.logger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Link excluded by pattern")
			continue
		}

		// Apply include patterns (if any)
		if len(includeRegexes) > 0 {
			included := false
			for _, re := range includeRegexes {
				if re.MatchString(link) {
					included = true
					break
				}
			}
			if !included {
				notIncludedLinks = append(notIncludedLinks, link)
				s.logger.Debug().
					Str("link", link).
					Msg("Link did not match any include pattern")
				continue
			}
		}

		filtered = append(filtered, link)
	}

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

// enqueueLinks adds discovered links to queue with depth tracking
// Propagates source_type and entity_type from parent for link discovery
func (s *Service) enqueueLinks(jobID string, links []string, parent *URLQueueItem) {
	var enqueuedCount int
	for i, link := range links {
		// Propagate metadata from parent
		metadata := map[string]interface{}{
			"job_id": jobID,
		}
		if sourceType, ok := parent.Metadata["source_type"]; ok {
			metadata["source_type"] = sourceType
		}
		if entityType, ok := parent.Metadata["entity_type"]; ok {
			metadata["entity_type"] = entityType
		}

		item := &URLQueueItem{
			URL:       link,
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Priority:  parent.Priority + i + 1,
			AddedAt:   time.Now(),
			Metadata:  metadata,
		}

		if s.queue.Push(item) {
			s.updatePendingCount(jobID, 1)
			enqueuedCount++

			// Log individual link enqueue decision
			s.logger.Debug().
				Str("job_id", jobID).
				Str("link", link).
				Int("depth", item.Depth).
				Str("parent_url", parent.URL).
				Int("priority", item.Priority).
				Msg("Link enqueued for processing")
		}
	}

	// Summary log for enqueued links
	if enqueuedCount > 0 {
		// Collect sample URLs (first 3)
		sampleURLs := []string{}
		if len(links) > 0 {
			sampleCount := 3
			if len(links) < 3 {
				sampleCount = len(links)
			}
			sampleURLs = links[:sampleCount]
		}

		s.logger.Debug().
			Str("job_id", jobID).
			Str("parent_url", parent.URL).
			Int("enqueued_count", enqueuedCount).
			Int("total_links", len(links)).
			Strs("sample_urls", sampleURLs).
			Msg("Link enqueueing complete")

		// Comment 2: Removed DEBUG database logging for link enqueueing to prevent log bloat
		// Link enqueue details are visible in console logs above
	}
}

// updateProgress updates job progress stats
func (s *Service) updateProgress(jobID string, success bool, failed bool) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	if success {
		job.Progress.CompletedURLs++
		job.Progress.PendingURLs-- // Decrement pending when URL is completed
	}
	if failed {
		job.Progress.FailedURLs++
		job.Progress.PendingURLs-- // Decrement pending when URL fails
	}

	job.Progress.Percentage = float64(job.Progress.CompletedURLs+job.Progress.FailedURLs) / float64(job.Progress.TotalURLs) * 100

	// Estimate completion
	elapsed := time.Since(job.Progress.StartTime)
	if job.Progress.CompletedURLs > 0 {
		avgTime := elapsed / time.Duration(job.Progress.CompletedURLs)
		remaining := job.Progress.TotalURLs - job.Progress.CompletedURLs - job.Progress.FailedURLs
		job.Progress.EstimatedCompletion = time.Now().Add(avgTime * time.Duration(remaining))
	}
}

// updateCurrentURL updates the current URL being processed
func (s *Service) updateCurrentURL(jobID string, url string) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	job.Progress.CurrentURL = url
}

// updatePendingCount updates pending URL count
func (s *Service) updatePendingCount(jobID string, delta int) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	job.Progress.TotalURLs += delta
	job.Progress.PendingURLs += delta
}

// emitProgress publishes progress event
func (s *Service) emitProgress(job *CrawlJob) {
	payload := map[string]interface{}{
		"job_id":               job.ID,
		"source_type":          job.SourceType,
		"entity_type":          job.EntityType,
		"status":               string(job.Status),
		"total_urls":           job.Progress.TotalURLs,
		"completed_urls":       job.Progress.CompletedURLs,
		"failed_urls":          job.Progress.FailedURLs,
		"pending_urls":         job.Progress.PendingURLs,
		"current_url":          job.Progress.CurrentURL,
		"percentage":           job.Progress.Percentage,
		"estimated_completion": job.Progress.EstimatedCompletion,
	}

	event := interfaces.Event{
		Type:    interfaces.EventCrawlProgress,
		Payload: payload,
	}

	if err := s.eventService.Publish(s.ctx, event); err != nil {
		s.logger.Debug().Err(err).Msg("Failed to publish crawl progress event")
	}
}

// monitorCompletion monitors job completion
func (s *Service) monitorCompletion(jobID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	heartbeatCounter := 0 // Track ticks for heartbeat updates (every 15 ticks = 30 seconds)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			heartbeatCounter++

			s.jobsMu.RLock()
			job, exists := s.activeJobs[jobID]
			if !exists {
				s.jobsMu.RUnlock()
				return
			}

			// Check if job is in a terminal state (cancelled or failed) and exit
			// Comment 7: Terminal state logs removed - already logged by CancelJob/FailJob with progress details
			if job.Status == JobStatusCancelled || job.Status == JobStatusFailed {
				s.jobsMu.RUnlock()

				// Only remove from activeJobs if jobStorage is nil or job was already persisted
				// When jobStorage is nil, keep job in memory for lookup
				if s.jobStorage == nil {
					s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Keeping terminal state job in memory (no job storage)")
				} else {
					// Job should have been persisted by CancelJob/FailJob, safe to remove
					s.jobsMu.Lock()
					delete(s.activeJobs, jobID)
					s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Removed terminal state job from active jobs")
					s.jobsMu.Unlock()
				}

				s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Monitor goroutine exiting for terminal job status")
				return
			}

			// Update heartbeat every 30 seconds (15 ticks)
			if heartbeatCounter >= 15 {
				if s.jobStorage != nil {
					if err := s.jobStorage.UpdateJobHeartbeat(s.ctx, jobID); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job heartbeat")
					}
				}
				heartbeatCounter = 0
			}

			// Check if job is complete
			if job.Status == JobStatusRunning && job.Progress.PendingURLs == 0 && job.Progress.CompletedURLs+job.Progress.FailedURLs >= job.Progress.TotalURLs {
				s.jobsMu.RUnlock()

				// Mark job as completed
				s.jobsMu.Lock()
				job.Status = JobStatusCompleted
				job.CompletedAt = time.Now()
				job.ResultCount = job.Progress.CompletedURLs
				job.FailedCount = job.Progress.FailedURLs
				s.jobsMu.Unlock()

				// Persist job completion to database
				persistSucceeded := false
				if s.jobStorage != nil {
					if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to persist job completion - keeping job in memory")
					} else {
						persistSucceeded = true
					}

					// Comment 8: Append enhanced job completion log with duration and success rate
					duration := job.CompletedAt.Sub(job.StartedAt)
					totalProcessed := job.Progress.CompletedURLs + job.Progress.FailedURLs
					successRate := 0.0
					if totalProcessed > 0 {
						successRate = float64(job.Progress.CompletedURLs) / float64(totalProcessed) * 100
					}

					completionMsg := fmt.Sprintf("Job completed: %d successful, %d failed, duration=%s, success_rate=%.1f%%",
						job.Progress.CompletedURLs, job.Progress.FailedURLs, duration.Round(time.Second), successRate)

					logEntry := models.JobLogEntry{
						Timestamp: time.Now().Format("15:04:05"),
						Level:     "info",
						Message:   completionMsg,
					}
					if err := s.jobStorage.AppendJobLog(s.ctx, jobID, logEntry); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append completion log")
					}
				}

				s.emitProgress(job)

				// Only remove from activeJobs if persistence succeeded or jobStorage is nil
				if s.jobStorage == nil {
					s.logger.Debug().Str("job_id", jobID).Msg("Keeping completed job in memory (no job storage)")
				} else if persistSucceeded {
					// Persistence succeeded, safe to clean up and remove from memory
					s.jobsMu.Lock()
					if _, exists := s.jobClients[jobID]; exists {
						delete(s.jobClients, jobID)
						s.logger.Debug().Str("job_id", jobID).Msg("Cleaned up per-job HTTP client")
					}
					delete(s.activeJobs, jobID)
					s.logger.Debug().Str("job_id", jobID).Msg("Removed completed job from active jobs")
					s.jobsMu.Unlock()
				} else {
					// Persistence failed, keep job in memory for subsequent lookups
					s.logger.Debug().Str("job_id", jobID).Msg("Keeping completed job in memory due to persistence failure")
				}

				s.logger.Info().
					Str("job_id", jobID).
					Int("completed", job.Progress.CompletedURLs).
					Int("failed", job.Progress.FailedURLs).
					Msg("Crawl job completed")

				// Continue monitoring if persistence failed (job still in activeJobs)
				if !persistSucceeded && s.jobStorage != nil {
					continue
				}

				return
			}
			s.jobsMu.RUnlock()
		}
	}
}

// logQueueDiagnostics logs current queue state and job progress for debugging
// This method helps diagnose queue health issues, stalled jobs, and worker activity
func (s *Service) logQueueDiagnostics(jobID string) {
	// Get current queue length
	queueLen := s.queue.Len()

	// Get job progress with lock
	s.jobsMu.RLock()
	job, exists := s.activeJobs[jobID]
	if !exists {
		s.jobsMu.RUnlock()
		return
	}

	pendingURLs := job.Progress.PendingURLs
	completedURLs := job.Progress.CompletedURLs
	failedURLs := job.Progress.FailedURLs
	totalURLs := job.Progress.TotalURLs
	s.jobsMu.RUnlock()

	// Calculate queue health indicators
	queueHealthy := true
	var healthIssues []string

	// Issue 1: Queue has items but pending count is zero
	if queueLen > 0 && pendingURLs == 0 {
		queueHealthy = false
		healthIssues = append(healthIssues, "queue_has_items_but_pending_zero")
	}

	// Issue 2: Pending count is non-zero but queue is empty
	if queueLen == 0 && pendingURLs > 0 {
		queueHealthy = false
		healthIssues = append(healthIssues, "pending_nonzero_but_queue_empty")
	}

	// Issue 3: Total processed (completed + failed) doesn't match expected
	totalProcessed := completedURLs + failedURLs
	expectedProcessed := totalURLs - pendingURLs
	if totalProcessed != expectedProcessed {
		queueHealthy = false
		healthIssues = append(healthIssues, fmt.Sprintf("count_mismatch_processed=%d_expected=%d", totalProcessed, expectedProcessed))
	}

	// Log queue diagnostics with health status
	logEvent := s.logger.Debug().
		Str("job_id", jobID).
		Int("queue_len", queueLen).
		Int("pending_urls", pendingURLs).
		Int("completed_urls", completedURLs).
		Int("failed_urls", failedURLs).
		Int("total_urls", totalURLs).
		Str("queue_healthy", fmt.Sprintf("%v", queueHealthy))

	if !queueHealthy {
		logEvent = logEvent.Strs("health_issues", healthIssues)
	}

	logEvent.Msg("Queue diagnostics")

	// Persist diagnostics if health issues detected
	if !queueHealthy && s.jobStorage != nil {
		diagMsg := fmt.Sprintf("Queue health issues detected: queue_len=%d, pending=%d, completed=%d, failed=%d, issues=%v",
			queueLen, pendingURLs, completedURLs, failedURLs, healthIssues)
		contextLogger := s.logger.WithContextWriter(jobID)
		contextLogger.Warn().Msg(diagMsg)
	}
}
