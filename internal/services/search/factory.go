package search

import (
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// NewSearchService creates a search service based on configuration
// Supported modes:
//   - "advanced": Google-style query parsing with regex-based search (default)
//   - "disabled": No-op service (returns 503 Service Unavailable)
func NewSearchService(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
	config *common.Config,
) (interfaces.SearchService, error) {
	// Select service based on mode
	mode := strings.ToLower(strings.TrimSpace(config.Search.Mode))

	switch mode {
	case "advanced", "": // Default to advanced if empty
		logger.Debug().
			Str("mode", "advanced").
			Msg("Initializing advanced search service with regex-based search")
		return NewAdvancedSearchService(storage, logger, config), nil

	case "disabled":
		logger.Warn().
			Str("mode", "disabled").
			Msg("Search service explicitly disabled via configuration")
		return NewDisabledSearchService(logger), nil

	default:
		logger.Warn().
			Str("mode", mode).
			Str("fallback", "advanced").
			Msg("Unknown search mode, falling back to advanced search")
		return NewAdvancedSearchService(storage, logger, config), nil
	}
}
