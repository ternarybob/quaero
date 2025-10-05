package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// GetSpacePages fetches pages for a specific Confluence space
func (s *ConfluenceScraperService) GetSpacePages(spaceKey string) error {
	return s.scrapeSpacePages(spaceKey)
}

func (s *ConfluenceScraperService) scrapeSpacePages(spaceKey string) error {
	s.logger.Info().Str("spaceKey", spaceKey).Msg("Starting to fetch Confluence pages from space")

	pageCount, err := s.GetSpacePageCount(spaceKey)
	if err != nil {
		s.logger.Warn().Err(err).Str("spaceKey", spaceKey).Msg("Could not get page count")
		pageCount = -1
	}

	limit := 25
	batchSize := 5
	totalPages := 0
	start := 0

	for {
		pages, hasMore, err := s.fetchPagesBatch(spaceKey, start, limit, batchSize)
		if err != nil {
			return err
		}

		if len(pages) == 0 {
			break
		}

		if err := s.storePages(pages); err != nil {
			return err
		}

		totalPages += len(pages)

		if !hasMore || (pageCount > 0 && totalPages >= pageCount) {
			break
		}

		start += len(pages)
	}

	s.logger.Info().
		Str("spaceKey", spaceKey).
		Int("totalPages", totalPages).
		Msg("Completed page scraping for space")

	return s.updateSpacePageCount(spaceKey, totalPages)
}

func (s *ConfluenceScraperService) fetchPagesBatch(spaceKey string, start, limit, batchSize int) ([]map[string]interface{}, bool, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPages []map[string]interface{}
	hasMore := false

	for i := 0; i < batchSize; i++ {
		wg.Add(1)
		batchStart := start + (i * limit)

		go func(bStart int) {
			defer wg.Done()

			path := fmt.Sprintf("/wiki/rest/api/content?spaceKey=%s&start=%d&limit=%d&expand=body.storage,space",
				spaceKey, bStart, limit)

			data, err := s.makeRequest("GET", path)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to fetch pages batch")
				return
			}

			var result struct {
				Results []map[string]interface{} `json:"results"`
				Size    int                      `json:"size"`
			}

			if err := json.Unmarshal(data, &result); err != nil {
				s.logger.Error().Err(err).Msg("Failed to parse pages")
				return
			}

			mu.Lock()
			defer mu.Unlock()

			allPages = append(allPages, result.Results...)
			if len(result.Results) >= limit {
				hasMore = true
			}

			time.Sleep(100 * time.Millisecond)
		}(batchStart)
	}

	wg.Wait()
	return allPages, hasMore, nil
}

func (s *ConfluenceScraperService) storePages(pages []map[string]interface{}) error {
	ctx := context.Background()
	confluencePages := make([]*models.ConfluencePage, 0, len(pages))

	for _, page := range pages {
		id, ok := page["id"].(string)
		if !ok {
			continue
		}

		title, _ := page["title"].(string)
		body, _ := page["body"].(map[string]interface{})

		var spaceID string
		if space, ok := page["space"].(map[string]interface{}); ok {
			// Try id first, fallback to key (same as we do for spaces)
			spaceID, _ = space["id"].(string)
			if spaceID == "" {
				spaceID, _ = space["key"].(string)
			}
		}

		confluencePage := &models.ConfluencePage{
			ID:      id,
			Title:   title,
			SpaceID: spaceID,
			Body:    body,
		}
		confluencePages = append(confluencePages, confluencePage)
	}

	return s.confluenceStorage.StorePages(ctx, confluencePages)
}

func (s *ConfluenceScraperService) updateSpacePageCount(spaceKey string, totalPages int) error {
	ctx := context.Background()

	space, err := s.confluenceStorage.GetSpace(ctx, spaceKey)
	if err != nil {
		return err
	}

	space.PageCount = totalPages
	return s.confluenceStorage.StoreSpace(ctx, space)
}

// GetPageCount returns the count of Confluence pages in the database
func (s *ConfluenceScraperService) GetPageCount() int {
	ctx := context.Background()
	count, err := s.confluenceStorage.CountPages(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to count pages")
		return 0
	}
	return count
}
