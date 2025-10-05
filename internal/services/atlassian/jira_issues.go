package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// DeleteProjectIssues deletes all issues for a given project
func (s *JiraScraperService) DeleteProjectIssues(projectKey string) error {
	s.logger.Info().Str("project", projectKey).Msg("Deleting issues for project")

	ctx := context.Background()
	if err := s.jiraStorage.DeleteIssuesByProject(ctx, projectKey); err != nil {
		return err
	}

	s.logger.Info().Str("project", projectKey).Msg("Deleted project issues")
	return nil
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

		startAt += maxResults
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
	ctx := context.Background()
	jiraIssues := make([]*models.JiraIssue, 0, len(issues))

	for _, issue := range issues {
		key, ok := issue["key"].(string)
		if !ok {
			continue
		}

		id, _ := issue["id"].(string)
		fields, _ := issue["fields"].(map[string]interface{})

		jiraIssue := &models.JiraIssue{
			Key:    key,
			ID:     id,
			Fields: fields,
		}
		jiraIssues = append(jiraIssues, jiraIssue)
	}

	return s.jiraStorage.StoreIssues(ctx, jiraIssues)
}

// GetIssueCount returns the count of issues in the database
func (s *JiraScraperService) GetIssueCount() int {
	ctx := context.Background()
	count, err := s.jiraStorage.CountIssues(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to count issues")
		return 0
	}
	return count
}
