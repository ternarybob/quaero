package transformer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// Service is a generic document transformer that processes crawler results
type Service struct {
	jobStorage      interfaces.JobStorage
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	crawlerService  interfaces.CrawlerService
	logger          arbor.ILogger
}

// NewService creates a new transformer service and subscribes to collection events
func NewService(jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, crawlerService interfaces.CrawlerService, logger arbor.ILogger) *Service {
	s := &Service{
		jobStorage:      jobStorage,
		documentStorage: documentStorage,
		eventService:    eventService,
		crawlerService:  crawlerService,
		logger:          logger,
	}

	// Subscribe to collection triggered events
	if err := eventService.Subscribe(interfaces.EventCollectionTriggered, s.handleCollectionEvent); err != nil {
		logger.Error().Err(err).Msg("Failed to subscribe to collection events")
	}

	return s
}

// handleCollectionEvent handles collection triggered events
func (s *Service) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	s.logger.Debug().Msg("Transformer received collection event")

	// Parse event payload to extract job_id, source_id, and source_type for logging
	var targetJobID, sourceID, sourceType string
	if event.Payload != nil {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			if jobID, ok := payload["job_id"].(string); ok && jobID != "" {
				targetJobID = jobID
			}
			if srcID, ok := payload["source_id"].(string); ok && srcID != "" {
				sourceID = srcID
			}
			if srcType, ok := payload["source_type"].(string); ok && srcType != "" {
				sourceType = srcType
			}

			// Log with all available context
			logEvent := s.logger.Debug()
			if targetJobID != "" {
				logEvent = logEvent.Str("job_id", targetJobID)
			}
			if sourceID != "" {
				logEvent = logEvent.Str("source_id", sourceID)
			}
			if sourceType != "" {
				logEvent = logEvent.Str("source_type", sourceType)
			}
			logEvent.Msg("Processing specific job from event payload")
		}
	}

	// Fast path: Process only the specific job if job_id is provided
	if targetJobID != "" {
		// Get the specific job
		jobInterface, err := s.jobStorage.GetJob(ctx, targetJobID)
		if err != nil {
			logErr := s.logger.Warn().Err(err).Str("job_id", targetJobID)
			if sourceID != "" {
				logErr = logErr.Str("source_id", sourceID)
			}
			if sourceType != "" {
				logErr = logErr.Str("source_type", sourceType)
			}
			logErr.Msg("Failed to get specific job")
			// Fall through to process all completed jobs as fallback
		} else {
			job, ok := jobInterface.(*crawler.CrawlJob)
			if !ok {
				s.logger.Warn().Str("job_id", targetJobID).Msg("Job is not a CrawlJob")
				return fmt.Errorf("job %s is not a CrawlJob", targetJobID)
			}

			// Only process if job is completed
			if job.Status == crawler.JobStatusCompleted {
				if err := s.transformJob(ctx, job); err != nil {
					logErr := s.logger.Error().Err(err).Str("job_id", job.ID)
					if sourceID != "" {
						logErr = logErr.Str("source_id", sourceID)
					}
					if sourceType != "" {
						logErr = logErr.Str("source_type", sourceType)
					}
					logErr.Msg("Failed to transform job")
					return err
				}
				logInfo := s.logger.Info().Str("job_id", job.ID)
				if sourceID != "" {
					logInfo = logInfo.Str("source_id", sourceID)
				}
				if sourceType != "" {
					logInfo = logInfo.Str("source_type", sourceType)
				}
				logInfo.Msg("Successfully transformed specific job")
				return nil
			} else {
				s.logger.Debug().Str("job_id", targetJobID).Str("status", string(job.Status)).Msg("Job not in completed status, skipping")
				return nil
			}
		}
	}

	// Fallback: Process all completed jobs (if no specific job_id or fast path failed)
	s.logger.Debug().Msg("Processing all completed jobs (no specific job_id in payload or fast path failed)")

	jobsInterface, err := s.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusCompleted))
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get completed jobs")
		return fmt.Errorf("failed to get completed jobs: %w", err)
	}

	if len(jobsInterface) == 0 {
		s.logger.Debug().Msg("No completed jobs to transform")
		return nil
	}

	s.logger.Info().Int("job_count", len(jobsInterface)).Msg("Processing completed crawler jobs")

	// Transform each job
	var successCount, failCount int
	for _, jobInterface := range jobsInterface {
		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			s.logger.Warn().Msg("Job is not a CrawlJob, skipping")
			continue
		}

		if err := s.transformJob(ctx, job); err != nil {
			s.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to transform job")
			failCount++
		} else {
			successCount++
		}
	}

	s.logger.Info().
		Int("success_count", successCount).
		Int("fail_count", failCount).
		Msg("Transformation summary")

	return nil
}

// transformJob transforms a single job's results into documents
func (s *Service) transformJob(ctx context.Context, job *crawler.CrawlJob) error {
	// Get source config snapshot using helper method
	sourceConfig, err := job.GetSourceConfigSnapshot()
	if err != nil {
		s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to parse source config snapshot")
		// Continue with nil sourceConfig - not critical
	}

	// Get job results from crawler service
	results, err := getJobResults(job, s.crawlerService, s.logger)
	if err != nil {
		s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to get job results")
		return err
	}

	if len(results) == 0 {
		s.logger.Debug().Str("job_id", job.ID).Msg("No results available for transformation")
		return nil
	}

	// Transform each result
	var transformedCount int
	for _, result := range results {
		if result.Error != "" {
			continue // Skip failed requests
		}

		// Get body from result.Body or fallback to result.Metadata["response_body"]
		body := result.Body
		if len(body) == 0 {
			// Fallback to metadata when Body is empty
			if metadataBody, ok := extractBodyFromMetadata(result.Metadata); ok {
				body = metadataBody
				s.logger.Debug().Str("url", result.URL).Msg("Using body from metadata (result.Body was empty)")
			}
		}

		// Guard against empty body after fallback
		if len(body) == 0 {
			s.logger.Warn().Str("url", result.URL).Msg("Skipping result with empty body (checked both Body and Metadata)")
			continue
		}

		// Extract document from response body
		doc, err := s.extractDocument(body, result.URL, job.SourceType, sourceConfig)
		if err != nil {
			s.logger.Warn().Err(err).Str("url", result.URL).Msg("Failed to extract document")
			continue
		}

		// Save document (upsert based on source_type + source_id)
		if err := s.documentStorage.SaveDocument(doc); err != nil {
			s.logger.Error().Err(err).Str("source_id", doc.SourceID).Msg("Failed to save document")
			continue
		}

		transformedCount++
	}

	s.logger.Info().
		Str("job_id", job.ID).
		Int("transformed_count", transformedCount).
		Int("total_results", len(results)).
		Msg("Transformed job results")

	return nil
}

// extractDocument performs generic content extraction from JSON or HTML/text responses
func (s *Service) extractDocument(body []byte, url string, sourceType string, sourceConfig *models.SourceConfig) (*models.Document, error) {
	var title, content, sourceID, fullURL string
	var metadata map[string]interface{}

	// Try to parse as JSON first
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Not JSON - fallback to HTML/text processing
		s.logger.Debug().Str("url", url).Msg("Response is not JSON, treating as HTML/text")

		bodyStr := string(body)

		// Try to extract <title> from HTML
		title = extractHTMLTitle(bodyStr)
		if title == "" {
			// Use URL-derived title as fallback
			title = deriveTitleFromURL(url)
		}

		// Strip HTML tags for content
		content = cleanText(stripHTML(bodyStr))

		// Extract source ID from URL
		sourceID = extractSourceID(nil, url)

		// Use request URL as full URL
		fullURL = url
		if sourceConfig != nil && !strings.HasPrefix(url, "http") {
			fullURL = constructURL(sourceConfig, url, nil)
		}

		// Store raw body in metadata with basic context
		metadata = map[string]interface{}{
			"raw_body":     bodyStr,
			"content_type": "html/text",
			"source_type":  sourceType,
			"request_url":  url,
		}
	} else {
		// JSON response - use structured extraction
		// Find common content fields
		title, content = s.findContentFields(data)

		// Extract source ID from JSON or URL
		sourceID = extractSourceID(data, url)

		// Construct full URL if needed
		fullURL = constructURL(sourceConfig, url, data)

		// Extract all text from JSON for additional content
		allText := extractAllText(data)
		if content == "" && allText != "" {
			content = allText
		}

		// Clean content
		content = cleanText(stripHTML(content))

		// Store raw JSON in metadata with basic context
		metadata = map[string]interface{}{
			"raw_json":     string(body),
			"content_type": "json",
			"source_type":  sourceType,
			"request_url":  url,
		}

		// Add links if available in response
		if links, ok := data["_links"].(map[string]interface{}); ok && len(links) > 0 {
			metadata["links"] = links
		} else if links, ok := data["links"].(map[string]interface{}); ok && len(links) > 0 {
			metadata["links"] = links
		}
	}

	// Add fallbacks for title if empty
	if title == "" {
		if content != "" {
			// Use first 80 characters of content as title
			title = content
			if len(title) > 80 {
				title = title[:80] + "..."
			}
		} else {
			// Use URL-derived title as final fallback
			title = deriveTitleFromURL(url)
		}
	}

	// Add fallback for content if empty (use plain text body)
	if content == "" && !strings.HasPrefix(metadata["content_type"].(string), "json") {
		// For non-JSON, we've already processed the body above
		// For JSON with no extractable content, use raw JSON as last resort
		if _, ok := metadata["raw_json"]; ok {
			content = string(body)
		}
	}

	// Create document
	now := time.Now()
	doc := &models.Document{
		ID:              generateDocumentID(),
		SourceType:      sourceType,
		SourceID:        sourceID,
		Title:           title,
		ContentMarkdown: content,
		Metadata:        metadata,
		URL:             fullURL,
		DetailLevel:     "full",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return doc, nil
}

// findContentFields searches for common title and content fields in JSON
func (s *Service) findContentFields(data map[string]interface{}) (title string, content string) {
	// Common title field names
	titleFields := []string{"title", "summary", "name", "subject", "heading"}
	for _, field := range titleFields {
		if val, ok := data[field]; ok {
			if str, ok := val.(string); ok && str != "" {
				title = str
				break
			}
		}
	}

	// Common content field names
	contentFields := []string{"body", "content", "description", "text", "markdown", "html"}
	for _, field := range contentFields {
		if val, ok := data[field]; ok {
			// Handle string content
			if str, ok := val.(string); ok && str != "" {
				content = str
				break
			}
			// Handle nested content objects
			if obj, ok := val.(map[string]interface{}); ok {
				// Try common nested patterns
				if storage, ok := obj["storage"].(map[string]interface{}); ok {
					if value, ok := storage["value"].(string); ok && value != "" {
						content = value
						break
					}
				}
				// Try direct value
				if value, ok := obj["value"].(string); ok && value != "" {
					content = value
					break
				}
			}
		}
	}

	// Try nested fields structure (e.g., Jira)
	if fields, ok := data["fields"].(map[string]interface{}); ok {
		if title == "" {
			for _, field := range titleFields {
				if val, ok := fields[field]; ok {
					if str, ok := val.(string); ok && str != "" {
						title = str
						break
					}
				}
			}
		}
		if content == "" {
			for _, field := range contentFields {
				if val, ok := fields[field]; ok {
					if str, ok := val.(string); ok && str != "" {
						content = str
						break
					}
					if obj, ok := val.(map[string]interface{}); ok {
						// Text-first traversal: Extract all text from nested JSON (e.g., Jira ADF)
						// This reduces noise from raw JSON structure
						extractedText := extractAllText(obj)
						if extractedText != "" {
							// Apply HTML stripping and text cleaning
							content = cleanText(stripHTML(extractedText))
							break
						}
						// Fallback to JSON stringification only if text extraction yields nothing
						if jsonBytes, err := json.Marshal(obj); err == nil {
							content = string(jsonBytes)
							break
						}
					}
				}
			}
		}
	}

	return title, content
}
