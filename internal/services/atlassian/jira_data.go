package atlassian

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// GetJiraData returns all Jira data (projects and issues)
func (s *JiraScraperService) GetJiraData() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"projects": make([]map[string]interface{}, 0),
		"issues":   make([]map[string]interface{}, 0),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		if err := s.loadProjectsData(tx, result); err != nil {
			return err
		}

		if err := s.loadIssuesData(tx, result); err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func (s *JiraScraperService) loadProjectsData(tx *bolt.Tx, result map[string]interface{}) error {
	projectBucket := tx.Bucket([]byte(projectsBucket))
	if projectBucket == nil {
		return nil
	}

	return projectBucket.ForEach(func(k, v []byte) error {
		var project map[string]interface{}
		if err := json.Unmarshal(v, &project); err == nil {
			result["projects"] = append(result["projects"].([]map[string]interface{}), project)
		}
		return nil
	})
}

func (s *JiraScraperService) loadIssuesData(tx *bolt.Tx, result map[string]interface{}) error {
	issueBucket := tx.Bucket([]byte(issuesBucket))
	if issueBucket == nil {
		return nil
	}

	return issueBucket.ForEach(func(k, v []byte) error {
		var issue map[string]interface{}
		if err := json.Unmarshal(v, &issue); err == nil {
			result["issues"] = append(result["issues"].([]map[string]interface{}), issue)
		}
		return nil
	})
}

// ClearAllData deletes all data from all buckets (projects, issues)
func (s *JiraScraperService) ClearAllData() error {
	s.logger.Info().Msg("Clearing all Jira data from database")

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := s.recreateBucket(tx, projectsBucket); err != nil {
			return err
		}

		if err := s.recreateBucket(tx, issuesBucket); err != nil {
			return err
		}

		s.logger.Info().Msg("All Jira data cleared successfully")
		return nil
	})
}

func (s *JiraScraperService) recreateBucket(tx *bolt.Tx, bucketName string) error {
	if err := tx.DeleteBucket([]byte(bucketName)); err != nil && err != bolt.ErrBucketNotFound {
		return fmt.Errorf("failed to delete %s bucket: %w", bucketName, err)
	}

	if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
		return fmt.Errorf("failed to recreate %s bucket: %w", bucketName, err)
	}

	return nil
}
