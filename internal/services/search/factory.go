package search

import (
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// NewSearchService creates a search service based on configuration
// Supported modes:
//   - "fts5": Basic FTS5 full-text search
//   - "advanced": Google-style query parsing with FTS5 (default)
//   - "disabled": No-op service (returns 503 Service Unavailable)
//
// The factory also checks if FTS5 is enabled in the database config.
// If FTS5 is disabled, it automatically uses DisabledSearchService regardless of mode.
func NewSearchService(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
	config *common.Config,
) (interfaces.SearchService, error) {
	// Check if FTS5 is enabled first - this overrides mode selection
	if !config.Storage.SQLite.EnableFTS5 {
		logger.Warn().
			Bool("fts5_enabled", false).
			Str("requested_mode", config.Search.Mode).
			Msg("FTS5 is disabled: using DisabledSearchService regardless of configured mode")
		return NewDisabledSearchService(logger), nil
	}

	// FTS5 is enabled - select service based on mode
	mode := strings.ToLower(strings.TrimSpace(config.Search.Mode))

	switch mode {
	case "fts5":
		logger.Info().
			Str("mode", "fts5").
			Msg("Initializing FTS5 search service")
		return NewFTS5SearchService(storage, logger), nil

	case "advanced", "": // Default to advanced if empty
		logger.Info().
			Str("mode", "advanced").
			Msg("Initializing advanced search service with Google-style query parsing")
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
