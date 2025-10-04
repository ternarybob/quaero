package atlassian

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// GetConfluenceData returns all Confluence data (spaces and pages)
func (s *ConfluenceScraperService) GetConfluenceData() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"spaces": make([]map[string]interface{}, 0),
		"pages":  make([]map[string]interface{}, 0),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		if err := s.loadSpacesData(tx, result); err != nil {
			return err
		}

		if err := s.loadPagesData(tx, result); err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func (s *ConfluenceScraperService) loadSpacesData(tx *bolt.Tx, result map[string]interface{}) error {
	spaceBucket := tx.Bucket([]byte(spacesBucket))
	if spaceBucket == nil {
		return nil
	}

	return spaceBucket.ForEach(func(k, v []byte) error {
		var space map[string]interface{}
		if err := json.Unmarshal(v, &space); err == nil {
			result["spaces"] = append(result["spaces"].([]map[string]interface{}), space)
		}
		return nil
	})
}

func (s *ConfluenceScraperService) loadPagesData(tx *bolt.Tx, result map[string]interface{}) error {
	pageBucket := tx.Bucket([]byte(pagesBucket))
	if pageBucket == nil {
		return nil
	}

	return pageBucket.ForEach(func(k, v []byte) error {
		var page map[string]interface{}
		if err := json.Unmarshal(v, &page); err == nil {
			result["pages"] = append(result["pages"].([]map[string]interface{}), page)
		}
		return nil
	})
}

// ClearAllData deletes all Confluence data from all buckets
func (s *ConfluenceScraperService) ClearAllData() error {
	s.logger.Info().Msg("Clearing all Confluence data from database")

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := s.recreateBucket(tx, spacesBucket); err != nil {
			return err
		}

		if err := s.recreateBucket(tx, pagesBucket); err != nil {
			return err
		}

		s.logger.Info().Msg("All Confluence data cleared successfully")
		return nil
	})
}

func (s *ConfluenceScraperService) recreateBucket(tx *bolt.Tx, bucketName string) error {
	if err := tx.DeleteBucket([]byte(bucketName)); err != nil && err != bolt.ErrBucketNotFound {
		return fmt.Errorf("failed to delete %s bucket: %w", bucketName, err)
	}

	if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
		return fmt.Errorf("failed to recreate %s bucket: %w", bucketName, err)
	}

	return nil
}
