package atlassian

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// GetSpacePageCount returns the total count of pages for a Confluence space
func (s *ConfluenceScraperService) GetSpacePageCount(spaceKey string) (int, error) {
	path := fmt.Sprintf("/wiki/rest/api/content?spaceKey=%s&limit=0", spaceKey)

	s.logger.Debug().
		Str("spaceKey", spaceKey).
		Str("path", path).
		Msg("Fetching page count")

	data, err := s.makeRequest("GET", path)
	if err != nil {
		s.logger.Error().
			Str("spaceKey", spaceKey).
			Err(err).
			Msg("Failed to fetch page count from API")
		return -1, err
	}

	var result struct {
		Size  int `json:"size"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		s.logger.Error().
			Str("spaceKey", spaceKey).
			Err(err).
			Msg("Failed to parse page count response")
		return -1, fmt.Errorf("failed to parse response: %w", err)
	}

	s.logger.Debug().
		Str("spaceKey", spaceKey).
		Int("total", result.Total).
		Msg("Retrieved page count from API")

	return result.Total, nil
}

// ScrapeSpaces scrapes all Confluence spaces and page counts
func (s *ConfluenceScraperService) ScrapeSpaces() error {
	s.logger.Info().Msg("Scraping Confluence spaces...")

	allSpaces := []map[string]interface{}{}
	start := 0
	limit := 25

	for {
		spaces, hasMore, err := s.fetchSpacesBatch(start, limit)
		if err != nil {
			return err
		}

		if len(spaces) == 0 {
			break
		}

		allSpaces = append(allSpaces, spaces...)

		if !hasMore {
			break
		}

		start += limit
		time.Sleep(500 * time.Millisecond)
	}

	s.logger.Info().Int("total", len(allSpaces)).Msg("Fetched all Confluence spaces")

	s.enrichSpacesWithPageCounts(allSpaces)

	return s.storeSpaces(allSpaces)
}

func (s *ConfluenceScraperService) fetchSpacesBatch(start, limit int) ([]map[string]interface{}, bool, error) {
	path := fmt.Sprintf("/wiki/rest/api/space?start=%d&limit=%d", start, limit)
	data, err := s.makeRequest("GET", path)
	if err != nil {
		return nil, false, err
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
		Size    int                      `json:"size"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false, fmt.Errorf("failed to parse spaces: %w", err)
	}

	hasMore := len(result.Results) >= limit
	return result.Results, hasMore, nil
}

func (s *ConfluenceScraperService) enrichSpacesWithPageCounts(spaces []map[string]interface{}) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range spaces {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			mu.Lock()
			spaceKey, ok := spaces[index]["key"].(string)
			mu.Unlock()

			if !ok {
				return
			}

			pageCount, err := s.GetSpacePageCount(spaceKey)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.logger.Warn().Str("space", spaceKey).Err(err).Msg("Failed to get page count")
				spaces[index]["pageCount"] = -1
			} else {
				spaces[index]["pageCount"] = pageCount
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	s.logger.Info().Msg("Completed counting pages for all spaces")
}

func (s *ConfluenceScraperService) storeSpaces(spaces []map[string]interface{}) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(spacesBucket))
		for _, space := range spaces {
			spaceKey, ok := space["key"].(string)
			if !ok {
				continue
			}
			value, err := json.Marshal(space)
			if err != nil {
				continue
			}
			if err := bucket.Put([]byte(spaceKey), value); err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearSpacesCache deletes all Confluence spaces from the database
func (s *ConfluenceScraperService) ClearSpacesCache() error {
	s.logger.Info().Msg("Clearing Confluence spaces cache...")

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(spacesBucket)); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket([]byte(spacesBucket))
		return err
	})
}

// GetSpaceCount returns the count of Confluence spaces in the database
func (s *ConfluenceScraperService) GetSpaceCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(spacesBucket))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}
