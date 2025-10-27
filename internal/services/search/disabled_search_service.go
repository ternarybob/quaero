// -----------------------------------------------------------------------
// Last Modified: Monday, 27th January 2025 8:00:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package search

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ErrSearchDisabled is returned when search functionality is unavailable
var ErrSearchDisabled = fmt.Errorf("search service is disabled: FTS5 is required but not enabled in configuration")

// DisabledSearchService is a no-op implementation used when FTS5 is disabled.
// It implements interfaces.SearchService but returns ErrSearchDisabled for all operations.
type DisabledSearchService struct {
	logger arbor.ILogger
}

// NewDisabledSearchService creates a no-op search service for when FTS5 is disabled
func NewDisabledSearchService(logger arbor.ILogger) interfaces.SearchService {
	return &DisabledSearchService{
		logger: logger,
	}
}

// Search returns ErrSearchDisabled
func (s *DisabledSearchService) Search(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	s.logger.Warn().
		Str("query", query).
		Msg("Search attempted but service is disabled (FTS5 not enabled)")
	return nil, ErrSearchDisabled
}

// GetByID returns ErrSearchDisabled
func (s *DisabledSearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	s.logger.Warn().
		Str("id", id).
		Msg("GetByID attempted but service is disabled (FTS5 not enabled)")
	return nil, ErrSearchDisabled
}

// SearchByReference returns ErrSearchDisabled
func (s *DisabledSearchService) SearchByReference(ctx context.Context, reference string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	s.logger.Warn().
		Str("reference", reference).
		Msg("SearchByReference attempted but service is disabled (FTS5 not enabled)")
	return nil, ErrSearchDisabled
}
