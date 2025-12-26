// Package cache provides document caching services for the queue system.
// It determines if cached documents can be reused based on cache configuration.
package cache

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service provides document cache freshness checking.
type Service struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
}

// NewService creates a new cache service.
func NewService(documentStorage interfaces.DocumentStorage, logger arbor.ILogger) *Service {
	return &Service{
		documentStorage: documentStorage,
		logger:          logger,
	}
}

// IsFresh checks if a document is still fresh based on cache config.
// Returns true if the document can be reused from cache.
func (s *Service) IsFresh(doc *models.Document, config models.CacheConfig) bool {
	if !config.Enabled || config.Type == models.CacheTypeNone {
		return false
	}

	if doc == nil || doc.LastSynced == nil {
		return false
	}

	switch config.Type {
	case models.CacheTypeRollingTime:
		return s.isRollingTimeFresh(doc, config.Hours)
	case models.CacheTypeHardTime:
		return s.isHardTimeFresh(doc, config.Hours)
	case models.CacheTypeAuto:
		return s.isAutoFresh(doc)
	default:
		return false
	}
}

// isRollingTimeFresh checks if document is fresh within rolling time window.
// Document is fresh if LastSynced is within N hours from now.
func (s *Service) isRollingTimeFresh(doc *models.Document, hours int) bool {
	cacheWindow := time.Duration(hours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// isHardTimeFresh checks if document is fresh within hard time boundary.
// Document is fresh if LastSynced is after today's 00:00 UTC.
func (s *Service) isHardTimeFresh(doc *models.Document, hours int) bool {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return doc.LastSynced.After(todayStart) || doc.LastSynced.Equal(todayStart)
}

// isAutoFresh uses AI-based assessment for cache freshness.
// Currently implemented as rolling_time with 24h window (stub for future AI implementation).
func (s *Service) isAutoFresh(doc *models.Document) bool {
	// Stub implementation - treat as 24h rolling window
	// Future: Use LLM to assess if content type/source typically changes frequently
	return time.Since(*doc.LastSynced) < 24*time.Hour
}

// GetFreshDocument retrieves the most recent fresh document matching cache tags.
// Tags should include jobdef and step tags for precise matching.
func (s *Service) GetFreshDocument(ctx context.Context, tags []string, config models.CacheConfig) (*models.Document, bool) {
	if !config.Enabled || config.Type == models.CacheTypeNone {
		return nil, false
	}

	// Extract jobdef and step from cache tags
	info := models.ParseCacheTags(tags)
	if info.JobDefID == "" || info.StepName == "" {
		s.logger.Debug().
			Strs("tags", tags).
			Msg("Cannot lookup cache: missing jobdef or step tags")
		return nil, false
	}

	// Query documents with matching cache tags
	docs, err := s.getDocumentsByCacheTags(info.JobDefID, info.StepName)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("jobdef_id", info.JobDefID).
			Str("step_name", info.StepName).
			Msg("Failed to query cached documents")
		return nil, false
	}

	if len(docs) == 0 {
		return nil, false
	}

	// Sort by LastSynced descending to get most recent first
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].LastSynced == nil {
			return false
		}
		if docs[j].LastSynced == nil {
			return true
		}
		return docs[i].LastSynced.After(*docs[j].LastSynced)
	})

	// Extract requested content hash for cache invalidation
	requestedHash := info.ContentHash

	// Find the first fresh document (revision 1 is preferred)
	for _, doc := range docs {
		docInfo := models.ParseCacheTags(doc.Tags)

		// If a content hash was requested, verify the document has the same hash
		// This ensures that when prompt/template content changes, old cached docs are skipped
		if requestedHash != "" && docInfo.ContentHash != requestedHash {
			s.logger.Debug().
				Str("doc_id", doc.ID).
				Str("doc_hash", docInfo.ContentHash).
				Str("requested_hash", requestedHash).
				Msg("Cache hash mismatch - skipping document")
			continue
		}

		if docInfo.Revision == 1 && s.IsFresh(doc, config) {
			s.logger.Debug().
				Str("doc_id", doc.ID).
				Str("last_synced", doc.LastSynced.Format("2006-01-02 15:04")).
				Str("cache_type", string(config.Type)).
				Msg("Found fresh cached document")
			return doc, true
		}
	}

	// No fresh document found
	return nil, false
}

// CleanupRevisions removes excess revisions beyond the configured limit.
func (s *Service) CleanupRevisions(ctx context.Context, jobDefID, stepName string, keepCount int) error {
	if keepCount < 1 {
		keepCount = 1
	}

	docs, err := s.getDocumentsByCacheTags(jobDefID, stepName)
	if err != nil {
		return fmt.Errorf("failed to query documents for cleanup: %w", err)
	}

	if len(docs) <= keepCount {
		return nil // Nothing to clean up
	}

	// Group by revision number
	byRevision := make(map[int][]*models.Document)
	for _, doc := range docs {
		info := models.ParseCacheTags(doc.Tags)
		byRevision[info.Revision] = append(byRevision[info.Revision], doc)
	}

	// Find revisions to delete (keep lowest revision numbers)
	var revisions []int
	for rev := range byRevision {
		revisions = append(revisions, rev)
	}
	sort.Ints(revisions)

	// Delete excess revisions (higher revision numbers = older)
	deletedCount := 0
	for i := keepCount; i < len(revisions); i++ {
		rev := revisions[i]
		for _, doc := range byRevision[rev] {
			if err := s.documentStorage.DeleteDocument(doc.ID); err != nil {
				s.logger.Warn().
					Err(err).
					Str("doc_id", doc.ID).
					Int("revision", rev).
					Msg("Failed to delete old revision")
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		s.logger.Info().
			Str("jobdef_id", jobDefID).
			Str("step_name", stepName).
			Int("deleted_count", deletedCount).
			Int("keep_count", keepCount).
			Msg("Cleaned up old document revisions")
	}

	return nil
}

// GetCurrentRevision returns the current revision number for a job/step.
// Returns 0 if no documents exist for this job/step.
func (s *Service) GetCurrentRevision(ctx context.Context, jobDefID, stepName string) (int, error) {
	docs, err := s.getDocumentsByCacheTags(jobDefID, stepName)
	if err != nil {
		return 0, fmt.Errorf("failed to query documents: %w", err)
	}

	if len(docs) == 0 {
		return 0, nil
	}

	// Find highest revision number
	maxRevision := 0
	for _, doc := range docs {
		info := models.ParseCacheTags(doc.Tags)
		if info.Revision > maxRevision {
			maxRevision = info.Revision
		}
	}

	return maxRevision, nil
}

// CleanupByJobDefID removes all documents associated with a job definition.
// Used when job definition content changes to force document regeneration.
// Returns the number of documents deleted.
func (s *Service) CleanupByJobDefID(ctx context.Context, jobDefID string) (int, error) {
	// Query all documents with the jobdef tag
	jobdefTag := "jobdef:" + sanitizeTagValue(jobDefID)

	opts := &interfaces.ListOptions{
		Tags:  []string{jobdefTag},
		Limit: 10000, // High limit to get all documents for this job
	}

	docs, err := s.documentStorage.ListDocuments(opts)
	if err != nil {
		return 0, fmt.Errorf("failed to query documents for job definition cleanup: %w", err)
	}

	if len(docs) == 0 {
		return 0, nil
	}

	deletedCount := 0
	for _, doc := range docs {
		if err := s.documentStorage.DeleteDocument(doc.ID); err != nil {
			s.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Str("jobdef_id", jobDefID).
				Msg("Failed to delete document during job definition cleanup")
		} else {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		s.logger.Info().
			Str("jobdef_id", jobDefID).
			Int("deleted_count", deletedCount).
			Int("total_found", len(docs)).
			Msg("Cleaned up documents for updated job definition")
	}

	return deletedCount, nil
}

// getDocumentsByCacheTags retrieves documents matching jobdef and step cache tags.
func (s *Service) getDocumentsByCacheTags(jobDefID, stepName string) ([]*models.Document, error) {
	// Use ListDocuments with tag filtering
	// We need to find documents that have both jobdef:<id> and step:<name> tags
	jobdefTag := fmt.Sprintf("jobdef:%s", models.MergeTags([]string{jobDefID})[0])
	stepTag := fmt.Sprintf("step:%s", models.MergeTags([]string{stepName})[0])

	// Sanitize tags like GenerateCacheTags does
	jobdefTag = "jobdef:" + sanitizeTagValue(jobDefID)
	stepTag = "step:" + sanitizeTagValue(stepName)

	// Query with both tags (AND logic)
	opts := &interfaces.ListOptions{
		Tags:  []string{jobdefTag, stepTag},
		Limit: 100, // Reasonable limit for revision management
	}

	docs, err := s.documentStorage.ListDocuments(opts)
	if err != nil {
		return nil, err
	}

	return docs, nil
}

// sanitizeTagValue mirrors the sanitizeTag function in cache_config.go
func sanitizeTagValue(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Keep only alphanumeric, hyphen, underscore
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// Ensure Service implements CacheService interface
var _ interfaces.CacheService = (*Service)(nil)
