package atlassian

import (
	"context"
)

// GetJiraData returns all Jira data (projects and issues)
func (s *JiraScraperService) GetJiraData() (map[string]interface{}, error) {
	ctx := context.Background()

	projects, err := s.jiraStorage.GetAllProjects(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"projects": projects,
		"issues":   make([]interface{}, 0),
	}

	for _, project := range projects {
		issues, err := s.jiraStorage.GetIssuesByProject(ctx, project.Key)
		if err != nil {
			s.logger.Warn().Err(err).Str("project", project.Key).Msg("Failed to get issues for project")
			continue
		}
		// Append each issue individually, not the whole array
		for _, issue := range issues {
			result["issues"] = append(result["issues"].([]interface{}), issue)
		}
	}

	return result, nil
}

// ClearAllData deletes all data from all buckets (projects, issues)
func (s *JiraScraperService) ClearAllData() error {
	s.logger.Info().Msg("Clearing all Jira data from database")

	ctx := context.Background()
	if err := s.jiraStorage.ClearAll(ctx); err != nil {
		return err
	}

	s.logger.Info().Msg("All Jira data cleared successfully")
	return nil
}
