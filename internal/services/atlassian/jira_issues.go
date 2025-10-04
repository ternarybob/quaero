package atlassian

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	bolt "go.etcd.io/bbolt"
)

// DeleteProjectIssues deletes all issues for a given project
func (s *JiraScraperService) DeleteProjectIssues(projectKey string) error {
	s.logger.Info().Str("project", projectKey).Msg("Deleting issues for project")

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(issuesBucket))
		if bucket == nil {
			return nil
		}

		keysToDelete := s.findProjectIssueKeys(bucket, projectKey)

		for _, k := range keysToDelete {
			if err := bucket.Delete(k); err != nil {
				return err
			}
		}

		s.logger.Info().
			Str("project", projectKey).
			Int("deleted", len(keysToDelete)).
			Msg("Deleted project issues")

		return nil
	})
}

func (s *JiraScraperService) findProjectIssueKeys(bucket *bolt.Bucket, projectKey string) [][]byte {
	var keysToDelete [][]byte
	c := bucket.Cursor()

	for k, v := c.First(); k != nil; k, v = c.Next() {
		var issue map[string]interface{}
		if err := json.Unmarshal(v, &issue); err != nil {
			continue
		}

		if s.issueMatchesProject(issue, projectKey) {
			keysToDelete = append(keysToDelete, k)
		}
	}

	return keysToDelete
}

func (s *JiraScraperService) issueMatchesProject(issue map[string]interface{}, projectKey string) bool {
	fields, ok := issue["fields"].(map[string]interface{})
	if !ok {
		return false
	}

	project, ok := fields["project"].(map[string]interface{})
	if !ok {
		return false
	}

	key, ok := project["key"].(string)
	return ok && key == projectKey
}

// GetProjectIssues retrieves all issues for a given project
func (s *JiraScraperService) GetProjectIssues(projectKey string) error {
	if err := s.DeleteProjectIssues(projectKey); err != nil {
		s.logger.Error().Err(err).Str("project", projectKey).Msg("Failed to delete old issues")
		return err
	}

	return s.scrapeProjectIssues(projectKey)
}

func (s *JiraScraperService) scrapeProjectIssues(projectKey string) error {
	s.logger.Info().Str("project", projectKey).Msg("Scraping issues for project")

	startAt := 0
	maxResults := 100
	totalFetched := 0
	maxIterations := 200

	for iteration := 0; iteration < maxIterations; iteration++ {
		issues, isLast, err := s.fetchIssuesBatch(projectKey, startAt, maxResults)
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			break
		}

		if err := s.storeIssues(issues); err != nil {
			return err
		}

		totalFetched += len(issues)

		if isLast || len(issues) < maxResults {
			break
		}

		startAt += len(issues)
		time.Sleep(300 * time.Millisecond)
	}

	s.logger.Info().
		Str("project", projectKey).
		Int("totalIssues", totalFetched).
		Msg("Completed fetching issues")

	return nil
}

func (s *JiraScraperService) fetchIssuesBatch(projectKey string, startAt, maxResults int) ([]map[string]interface{}, bool, error) {
	jql := fmt.Sprintf("project=\"%s\"", projectKey)
	encodedJQL := url.QueryEscape(jql)
	path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&startAt=%d&maxResults=%d&fields=key,summary,status,issuetype,project",
		encodedJQL, startAt, maxResults)

	data, err := s.makeRequest("GET", path)
	if err != nil {
		return nil, false, err
	}

	var result struct {
		Issues []map[string]interface{} `json:"issues"`
		IsLast bool                     `json:"isLast"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false, fmt.Errorf("failed to parse issues: %w", err)
	}

	return result.Issues, result.IsLast, nil
}

func (s *JiraScraperService) storeIssues(issues []map[string]interface{}) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(issuesBucket))
		if bucket == nil {
			return fmt.Errorf("issues bucket not found")
		}

		for _, issue := range issues {
			key, ok := issue["key"].(string)
			if !ok {
				continue
			}

			value, err := json.Marshal(issue)
			if err != nil {
				continue
			}

			if err := bucket.Put([]byte(key), value); err != nil {
				return fmt.Errorf("failed to store issue %s: %w", key, err)
			}
		}

		return nil
	})
}

// GetIssueCount returns the count of issues in the database
func (s *JiraScraperService) GetIssueCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(issuesBucket))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}
