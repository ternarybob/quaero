package atlassian

import (
	"context"
)

// GetConfluenceData returns all Confluence data (spaces and pages)
func (s *ConfluenceScraperService) GetConfluenceData() (map[string]interface{}, error) {
	ctx := context.Background()

	spaces, err := s.confluenceStorage.GetAllSpaces(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Debug().Int("spaceCount", len(spaces)).Msg("Retrieved spaces from storage")

	result := map[string]interface{}{
		"spaces": spaces,
		"pages":  make([]interface{}, 0),
	}

	for _, space := range spaces {
		s.logger.Debug().Str("spaceKey", space.Key).Str("spaceID", space.ID).Msg("Looking up pages for space")
		pages, err := s.confluenceStorage.GetPagesBySpace(ctx, space.ID)
		if err != nil {
			s.logger.Warn().Err(err).Str("space", space.Key).Msg("Failed to get pages for space")
			continue
		}
		s.logger.Debug().Str("spaceKey", space.Key).Int("pageCount", len(pages)).Msg("Retrieved pages for space")
		// Append each page individually, not the whole array
		for _, page := range pages {
			result["pages"] = append(result["pages"].([]interface{}), page)
		}
	}

	s.logger.Info().Int("totalPages", len(result["pages"].([]interface{}))).Msg("Total pages loaded from database")
	return result, nil
}

// ClearAllData deletes all Confluence data from all buckets
func (s *ConfluenceScraperService) ClearAllData() error {
	s.logger.Info().Msg("Clearing all Confluence data from database")

	ctx := context.Background()
	if err := s.confluenceStorage.ClearAll(ctx); err != nil {
		return err
	}

	s.logger.Info().Msg("All Confluence data cleared successfully")
	return nil
}
