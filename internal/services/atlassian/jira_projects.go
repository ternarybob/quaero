package atlassian

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// GetProjectIssueCount returns the total count of issues for a project
func (s *JiraScraperService) GetProjectIssueCount(projectKey string) (int, error) {
	jql := fmt.Sprintf("project=\"%s\"", projectKey)
	encodedJQL := url.QueryEscape(jql)
	path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&maxResults=5000&fields=-all", encodedJQL)

	s.logger.Debug().
		Str("project", projectKey).
		Str("jql", jql).
		Msg("Fetching issue count")

	data, err := s.makeRequest("GET", path)
	if err != nil {
		s.logger.Error().
			Str("project", projectKey).
			Err(err).
			Msg("Failed to fetch issue count from API")
		return 0, err
	}

	var result struct {
		Issues []interface{} `json:"issues"`
		IsLast bool          `json:"isLast"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		s.logger.Error().
			Str("project", projectKey).
			Err(err).
			Msg("Failed to parse issue count response")
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	count := len(result.Issues)
	s.logger.Info().
		Str("project", projectKey).
		Int("count", count).
		Msg("Retrieved issue count")

	return count, nil
}

// ScrapeProjects scrapes all Jira projects and their issue counts
func (s *JiraScraperService) ScrapeProjects() error {
	s.logger.Info().Msg("Scraping projects...")

	data, err := s.makeRequest("GET", "/rest/api/3/project")
	if err != nil {
		return err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("failed to parse projects: %w", err)
	}

	s.logger.Info().Int("count", len(projects)).Msg("Found projects")

	s.enrichProjectsWithIssueCounts(projects)

	return s.storeProjects(projects)
}

func (s *JiraScraperService) enrichProjectsWithIssueCounts(projects []map[string]interface{}) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range projects {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			mu.Lock()
			projectKey := projects[index]["key"].(string)
			mu.Unlock()

			issueCount, err := s.GetProjectIssueCount(projectKey)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.logger.Warn().Str("project", projectKey).Err(err).Msg("Failed to get issue count")
				projects[index]["issueCount"] = 0
			} else {
				projects[index]["issueCount"] = issueCount
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	s.logger.Info().Msg("Completed counting issues for all projects")
}

func (s *JiraScraperService) storeProjects(projects []map[string]interface{}) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(projectsBucket))
		for _, project := range projects {
			key := project["key"].(string)
			value, err := json.Marshal(project)
			if err != nil {
				continue
			}
			if err := bucket.Put([]byte(key), value); err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearProjectsCache deletes all projects from the database
func (s *JiraScraperService) ClearProjectsCache() error {
	s.logger.Info().Msg("Clearing projects cache...")

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(projectsBucket)); err != nil {
			return err
		}
		_, err := tx.CreateBucket([]byte(projectsBucket))
		return err
	})
}

// GetProjectCount returns the count of projects in the database
func (s *JiraScraperService) GetProjectCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(projectsBucket))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}
