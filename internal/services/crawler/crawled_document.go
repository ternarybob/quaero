// -----------------------------------------------------------------------
// Crawled Document - Document model for crawler-specific documents
// -----------------------------------------------------------------------

package crawler

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/quaero/internal/models"
)

// CrawledDocument represents a document created by the web crawler
type CrawledDocument struct {
	// Core document fields
	ID          string `json:"id"`
	JobID       string `json:"job_id"`        // ID of the crawler job that created this document
	ParentJobID string `json:"parent_job_id"` // ID of the parent crawler job
	SourceURL   string `json:"source_url"`    // URL that was crawled

	// Content fields
	Title           string `json:"title"`
	ContentMarkdown string `json:"content_markdown"` // Primary content in markdown format
	ContentHTML     string `json:"content_html"`     // Original HTML content
	ContentSize     int    `json:"content_size"`     // Size of content in bytes

	// Processing metadata
	ProcessTime time.Duration          `json:"process_time"` // Time taken to process the content
	Metadata    map[string]interface{} `json:"metadata"`     // Extracted metadata

	// Crawler-specific metadata
	CrawlerMetadata *CrawlerDocumentMetadata `json:"crawler_metadata,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CrawlerDocumentMetadata contains crawler-specific metadata
type CrawlerDocumentMetadata struct {
	// Crawling context
	Depth        int       `json:"depth"`         // Depth at which this URL was discovered
	DiscoveredBy string    `json:"discovered_by"` // Job ID that discovered this URL
	CrawledAt    time.Time `json:"crawled_at"`    // When the URL was crawled

	// HTTP response metadata
	StatusCode   int               `json:"status_code"`
	ResponseTime time.Duration     `json:"response_time"`
	Headers      map[string]string `json:"headers,omitempty"`

	// Link processing
	LinksFound    int `json:"links_found"`    // Number of links found on this page
	LinksFiltered int `json:"links_filtered"` // Number of links after filtering
	LinksFollowed int `json:"links_followed"` // Number of links that were followed
	LinksSkipped  int `json:"links_skipped"`  // Number of links skipped due to depth limits

	// Content processing
	WordCount   int    `json:"word_count"`   // Number of words in the content
	Language    string `json:"language"`     // Detected language
	ContentType string `json:"content_type"` // MIME type of the response

	// Error information
	Error       string     `json:"error,omitempty"`         // Error message if crawling failed
	RetryCount  int        `json:"retry_count,omitempty"`   // Number of times this URL was retried
	LastRetryAt *time.Time `json:"last_retry_at,omitempty"` // When the last retry occurred
}

// NewCrawledDocument creates a new crawled document from processed content
func NewCrawledDocument(jobID, parentJobID, sourceURL string, processedContent *ProcessedContent) *CrawledDocument {
	doc := &CrawledDocument{
		ID:              fmt.Sprintf("crawl_%s", uuid.New().String()),
		JobID:           jobID,
		ParentJobID:     parentJobID,
		SourceURL:       sourceURL,
		Title:           processedContent.Title,
		ContentMarkdown: processedContent.Markdown,
		ContentHTML:     processedContent.Content,
		ContentSize:     processedContent.ContentSize,
		ProcessTime:     processedContent.ProcessTime,
		Metadata:        processedContent.Metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	return doc
}

// ToDocument converts a CrawledDocument to the standard Document model for storage
func (cd *CrawledDocument) ToDocument() *models.Document {
	// Prepare metadata map
	metadata := make(map[string]interface{})

	// Copy original metadata
	if cd.Metadata != nil {
		for k, v := range cd.Metadata {
			metadata[k] = v
		}
	}

	// Add crawler-specific metadata
	metadata["job_id"] = cd.JobID
	metadata["parent_job_id"] = cd.ParentJobID
	metadata["source_url"] = cd.SourceURL
	metadata["content_size"] = cd.ContentSize
	metadata["process_time_ms"] = cd.ProcessTime.Milliseconds()

	// Add crawler metadata if available
	if cd.CrawlerMetadata != nil {
		crawlerMeta := map[string]interface{}{
			"depth":          cd.CrawlerMetadata.Depth,
			"discovered_by":  cd.CrawlerMetadata.DiscoveredBy,
			"crawled_at":     cd.CrawlerMetadata.CrawledAt,
			"status_code":    cd.CrawlerMetadata.StatusCode,
			"response_time":  cd.CrawlerMetadata.ResponseTime.Milliseconds(),
			"links_found":    cd.CrawlerMetadata.LinksFound,
			"links_filtered": cd.CrawlerMetadata.LinksFiltered,
			"links_followed": cd.CrawlerMetadata.LinksFollowed,
			"links_skipped":  cd.CrawlerMetadata.LinksSkipped,
			"word_count":     cd.CrawlerMetadata.WordCount,
			"language":       cd.CrawlerMetadata.Language,
			"content_type":   cd.CrawlerMetadata.ContentType,
		}

		if cd.CrawlerMetadata.Headers != nil {
			crawlerMeta["headers"] = cd.CrawlerMetadata.Headers
		}

		if cd.CrawlerMetadata.Error != "" {
			crawlerMeta["error"] = cd.CrawlerMetadata.Error
			crawlerMeta["retry_count"] = cd.CrawlerMetadata.RetryCount
			if cd.CrawlerMetadata.LastRetryAt != nil {
				crawlerMeta["last_retry_at"] = cd.CrawlerMetadata.LastRetryAt
			}
		}

		metadata["crawler"] = crawlerMeta
	}

	// Create the standard document
	doc := &models.Document{
		ID:              cd.ID,
		SourceType:      "crawler",    // Source type for crawler documents
		SourceID:        cd.SourceURL, // Use URL as source ID
		Title:           cd.Title,
		ContentMarkdown: cd.ContentMarkdown,
		DetailLevel:     models.DetailLevelFull, // Crawler always provides full content
		Metadata:        metadata,
		URL:             cd.SourceURL,
		CreatedAt:       cd.CreatedAt,
		UpdatedAt:       cd.UpdatedAt,
	}

	return doc
}

// FromDocument converts a standard Document back to a CrawledDocument
func FromDocument(doc *models.Document) (*CrawledDocument, error) {
	if doc.SourceType != "crawler" {
		return nil, fmt.Errorf("document is not a crawler document: source_type=%s", doc.SourceType)
	}

	cd := &CrawledDocument{
		ID:              doc.ID,
		SourceURL:       doc.SourceID, // URL is stored as SourceID
		Title:           doc.Title,
		ContentMarkdown: doc.ContentMarkdown,
		Metadata:        make(map[string]interface{}),
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}

	// Extract crawler-specific metadata
	if doc.Metadata != nil {
		// Copy non-crawler metadata
		for k, v := range doc.Metadata {
			if k != "crawler" && k != "job_id" && k != "parent_job_id" && k != "source_url" &&
				k != "content_size" && k != "process_time_ms" {
				cd.Metadata[k] = v
			}
		}

		// Extract basic crawler fields
		if jobID, ok := doc.Metadata["job_id"].(string); ok {
			cd.JobID = jobID
		}
		if parentJobID, ok := doc.Metadata["parent_job_id"].(string); ok {
			cd.ParentJobID = parentJobID
		}
		if contentSize, ok := doc.Metadata["content_size"].(float64); ok {
			cd.ContentSize = int(contentSize)
		} else if contentSize, ok := doc.Metadata["content_size"].(int); ok {
			cd.ContentSize = contentSize
		}
		if processTimeMs, ok := doc.Metadata["process_time_ms"].(float64); ok {
			cd.ProcessTime = time.Duration(processTimeMs) * time.Millisecond
		} else if processTimeMs, ok := doc.Metadata["process_time_ms"].(int); ok {
			cd.ProcessTime = time.Duration(processTimeMs) * time.Millisecond
		}

		// Extract crawler metadata
		if crawlerMeta, ok := doc.Metadata["crawler"].(map[string]interface{}); ok {
			cd.CrawlerMetadata = &CrawlerDocumentMetadata{}

			if depth, ok := crawlerMeta["depth"].(float64); ok {
				cd.CrawlerMetadata.Depth = int(depth)
			} else if depth, ok := crawlerMeta["depth"].(int); ok {
				cd.CrawlerMetadata.Depth = depth
			}

			if discoveredBy, ok := crawlerMeta["discovered_by"].(string); ok {
				cd.CrawlerMetadata.DiscoveredBy = discoveredBy
			}

			if crawledAtStr, ok := crawlerMeta["crawled_at"].(string); ok {
				if crawledAt, err := time.Parse(time.RFC3339, crawledAtStr); err == nil {
					cd.CrawlerMetadata.CrawledAt = crawledAt
				}
			}

			if statusCode, ok := crawlerMeta["status_code"].(float64); ok {
				cd.CrawlerMetadata.StatusCode = int(statusCode)
			} else if statusCode, ok := crawlerMeta["status_code"].(int); ok {
				cd.CrawlerMetadata.StatusCode = statusCode
			}

			if responseTimeMs, ok := crawlerMeta["response_time"].(float64); ok {
				cd.CrawlerMetadata.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond
			} else if responseTimeMs, ok := crawlerMeta["response_time"].(int); ok {
				cd.CrawlerMetadata.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond
			}

			// Extract link statistics
			if linksFound, ok := crawlerMeta["links_found"].(float64); ok {
				cd.CrawlerMetadata.LinksFound = int(linksFound)
			} else if linksFound, ok := crawlerMeta["links_found"].(int); ok {
				cd.CrawlerMetadata.LinksFound = linksFound
			}

			if linksFiltered, ok := crawlerMeta["links_filtered"].(float64); ok {
				cd.CrawlerMetadata.LinksFiltered = int(linksFiltered)
			} else if linksFiltered, ok := crawlerMeta["links_filtered"].(int); ok {
				cd.CrawlerMetadata.LinksFiltered = linksFiltered
			}

			if linksFollowed, ok := crawlerMeta["links_followed"].(float64); ok {
				cd.CrawlerMetadata.LinksFollowed = int(linksFollowed)
			} else if linksFollowed, ok := crawlerMeta["links_followed"].(int); ok {
				cd.CrawlerMetadata.LinksFollowed = linksFollowed
			}

			if linksSkipped, ok := crawlerMeta["links_skipped"].(float64); ok {
				cd.CrawlerMetadata.LinksSkipped = int(linksSkipped)
			} else if linksSkipped, ok := crawlerMeta["links_skipped"].(int); ok {
				cd.CrawlerMetadata.LinksSkipped = linksSkipped
			}

			// Extract content metadata
			if wordCount, ok := crawlerMeta["word_count"].(float64); ok {
				cd.CrawlerMetadata.WordCount = int(wordCount)
			} else if wordCount, ok := crawlerMeta["word_count"].(int); ok {
				cd.CrawlerMetadata.WordCount = wordCount
			}

			if language, ok := crawlerMeta["language"].(string); ok {
				cd.CrawlerMetadata.Language = language
			}

			if contentType, ok := crawlerMeta["content_type"].(string); ok {
				cd.CrawlerMetadata.ContentType = contentType
			}

			// Extract error information
			if errorMsg, ok := crawlerMeta["error"].(string); ok {
				cd.CrawlerMetadata.Error = errorMsg
			}

			if retryCount, ok := crawlerMeta["retry_count"].(float64); ok {
				cd.CrawlerMetadata.RetryCount = int(retryCount)
			} else if retryCount, ok := crawlerMeta["retry_count"].(int); ok {
				cd.CrawlerMetadata.RetryCount = retryCount
			}

			if lastRetryAtStr, ok := crawlerMeta["last_retry_at"].(string); ok {
				if lastRetryAt, err := time.Parse(time.RFC3339, lastRetryAtStr); err == nil {
					cd.CrawlerMetadata.LastRetryAt = &lastRetryAt
				}
			}

			// Extract headers
			if headers, ok := crawlerMeta["headers"].(map[string]interface{}); ok {
				cd.CrawlerMetadata.Headers = make(map[string]string)
				for k, v := range headers {
					if vStr, ok := v.(string); ok {
						cd.CrawlerMetadata.Headers[k] = vStr
					}
				}
			}
		}
	}

	return cd, nil
}

// SetCrawlerMetadata sets the crawler-specific metadata
func (cd *CrawledDocument) SetCrawlerMetadata(depth int, discoveredBy string, statusCode int, responseTime time.Duration,
	linksFound, linksFiltered, linksFollowed, linksSkipped int) {
	cd.CrawlerMetadata = &CrawlerDocumentMetadata{
		Depth:         depth,
		DiscoveredBy:  discoveredBy,
		CrawledAt:     time.Now(),
		StatusCode:    statusCode,
		ResponseTime:  responseTime,
		LinksFound:    linksFound,
		LinksFiltered: linksFiltered,
		LinksFollowed: linksFollowed,
		LinksSkipped:  linksSkipped,
	}

	// Extract additional metadata from content
	if cd.Metadata != nil {
		if wordCount, ok := cd.Metadata["word_count"].(int); ok {
			cd.CrawlerMetadata.WordCount = wordCount
		}
		if language, ok := cd.Metadata["language"].(string); ok {
			cd.CrawlerMetadata.Language = language
		}
	}
}

// SetError sets error information for failed crawls
func (cd *CrawledDocument) SetError(errorMsg string, retryCount int) {
	if cd.CrawlerMetadata == nil {
		cd.CrawlerMetadata = &CrawlerDocumentMetadata{}
	}

	cd.CrawlerMetadata.Error = errorMsg
	cd.CrawlerMetadata.RetryCount = retryCount
	now := time.Now()
	cd.CrawlerMetadata.LastRetryAt = &now
}

// IsSuccessful returns true if the document was successfully crawled
func (cd *CrawledDocument) IsSuccessful() bool {
	return cd.CrawlerMetadata == nil || cd.CrawlerMetadata.Error == ""
}

// GetSummary returns a summary of the crawled document
func (cd *CrawledDocument) GetSummary() string {
	if cd.CrawlerMetadata != nil {
		return fmt.Sprintf("Crawled %s (depth %d, %d words, %d links found)",
			cd.SourceURL, cd.CrawlerMetadata.Depth, cd.CrawlerMetadata.WordCount, cd.CrawlerMetadata.LinksFound)
	}
	return fmt.Sprintf("Crawled %s (%d bytes)", cd.SourceURL, cd.ContentSize)
}
