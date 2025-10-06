package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/ternarybob/quaero/internal/models"
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

	if err := s.storeProjects(projects); err != nil {
		return err
	}

	s.logger.Info().Int("count", len(projects)).Msg("Projects stored successfully")
	return nil
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
	ctx := context.Background()

	storedCount := 0
	for _, project := range projects {
		key, ok := project["key"].(string)
		if !ok {
			s.logger.Warn().Msg("Project missing key field")
			continue
		}

		name, _ := project["name"].(string)
		id, _ := project["id"].(string)
		issueCount, _ := project["issueCount"].(int)

		jiraProject := &models.JiraProject{
			Key:        key,
			Name:       name,
			ID:         id,
			IssueCount: issueCount,
		}

		s.logger.Debug().Str("key", key).Str("name", name).Int("issueCount", issueCount).Msg("Storing project")

		// Store to jira_projects table
		if err := s.jiraStorage.StoreProject(ctx, jiraProject); err != nil {
			s.logger.Error().Err(err).Str("project", key).Msg("Failed to store project")
			continue
		}

		// NOTE: Projects are metadata only - actual searchable content comes from issues
		// Issues contain project_key in metadata and are the source of truth for search
		s.logger.Debug().Str("project", key).Msg("Stored project metadata (issues will be indexed separately)")
		storedCount++
	}

	s.logger.Info().Int("stored", storedCount).Int("total", len(projects)).Msg("Finished storing projects")
	return nil
}

// ClearProjectsCache deletes all projects from the database
func (s *JiraScraperService) ClearProjectsCache() error {
	s.logger.Info().Msg("Clearing projects cache...")

	ctx := context.Background()
	projects, err := s.jiraStorage.GetAllProjects(ctx)
	if err != nil {
		return err
	}

	for _, project := range projects {
		if err := s.jiraStorage.DeleteProject(ctx, project.Key); err != nil {
			s.logger.Error().Err(err).Str("project", project.Key).Msg("Failed to delete project")
		}
	}

	return nil
}

// GetProjectCount returns the count of projects in the database
func (s *JiraScraperService) GetProjectCount() int {
	ctx := context.Background()
	count, err := s.jiraStorage.CountProjects(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to count projects")
		return 0
	}
	return count
}
