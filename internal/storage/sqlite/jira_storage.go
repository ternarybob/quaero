// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:10:47 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// JiraStorage implements the JiraStorage interface for SQLite
type JiraStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
	mu     sync.Mutex // Serializes write operations to prevent SQLITE_BUSY errors
}

// NewJiraStorage creates a new JiraStorage instance
func NewJiraStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.JiraStorage {
	return &JiraStorage{
		db:     db,
		logger: logger,
	}
}

// StoreProject stores a Jira project
func (s *JiraStorage) StoreProject(ctx context.Context, project *models.JiraProject) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(project)
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}

	query := "INSERT INTO jira_projects (key, name, id, issue_count, data, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET name = excluded.name, id = excluded.id, issue_count = excluded.issue_count, data = excluded.data, updated_at = excluded.updated_at"

	_, err = s.db.DB().ExecContext(ctx, query,
		project.Key, project.Name, project.ID, project.IssueCount, data, time.Now().Unix())

	if err != nil {
		return fmt.Errorf("failed to store project: %w", err)
	}

	s.logger.Debug().Str("key", project.Key).Msg("Stored Jira project")
	return nil
}

// GetProject retrieves a Jira project by key
func (s *JiraStorage) GetProject(ctx context.Context, key string) (*models.JiraProject, error) {
	var data []byte
	query := "SELECT data FROM jira_projects WHERE key = ?"

	err := s.db.DB().QueryRowContext(ctx, query, key).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	var project models.JiraProject
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, nil
}

// GetAllProjects retrieves all Jira projects
func (s *JiraStorage) GetAllProjects(ctx context.Context) ([]*models.JiraProject, error) {
	query := "SELECT data FROM jira_projects ORDER BY key"

	s.logger.Info().Msg("Querying all projects from database")

	rows, err := s.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.JiraProject
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}

		var project models.JiraProject
		if err := json.Unmarshal(data, &project); err != nil {
			return nil, fmt.Errorf("failed to unmarshal project: %w", err)
		}
		projects = append(projects, &project)
	}

	s.logger.Info().Int("count", len(projects)).Msg("Retrieved projects from database")
	return projects, rows.Err()
}

// DeleteProject deletes a Jira project
func (s *JiraStorage) DeleteProject(ctx context.Context, key string) error {
	query := "DELETE FROM jira_projects WHERE key = ?"
	_, err := s.db.DB().ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	s.logger.Debug().Str("key", key).Msg("Deleted Jira project")
	return nil
}

// CountProjects returns the number of projects
func (s *JiraStorage) CountProjects(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM jira_projects"
	err := s.db.DB().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count projects: %w", err)
	}
	return count, nil
}

// GetMostRecentProject returns the most recently updated project with its timestamp
func (s *JiraStorage) GetMostRecentProject(ctx context.Context) (*models.JiraProject, int64, error) {
	var data []byte
	var updatedAt int64
	query := "SELECT data, updated_at FROM jira_projects ORDER BY updated_at DESC LIMIT 1"

	err := s.db.DB().QueryRowContext(ctx, query).Scan(&data, &updatedAt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get most recent project: %w", err)
	}

	var project models.JiraProject
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal project: %w", err)
	}

	return &project, updatedAt, nil
}

// StoreIssue stores a Jira issue
func (s *JiraStorage) StoreIssue(ctx context.Context, issue *models.JiraIssue) error {
	fields, err := json.Marshal(issue.Fields)
	if err != nil {
		return fmt.Errorf("failed to marshal issue fields: %w", err)
	}

	// Extract summary and description for FTS
	summary := ""
	description := ""
	if issue.Fields != nil {
		if s, ok := issue.Fields["summary"].(string); ok {
			summary = s
		}
		if d, ok := issue.Fields["description"].(string); ok {
			description = d
		}
	}

	// Extract project key from issue key (format: PROJECT-123)
	projectKey := ""
	if idx := strings.Index(issue.Key, "-"); idx > 0 {
		projectKey = issue.Key[:idx]
	}

	query := "INSERT INTO jira_issues (key, project_key, id, summary, description, fields, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET project_key = excluded.project_key, id = excluded.id, summary = excluded.summary, description = excluded.description, fields = excluded.fields, updated_at = excluded.updated_at"

	_, err = s.db.DB().ExecContext(ctx, query,
		issue.Key, projectKey, issue.ID, summary, description, fields, time.Now().Unix())

	if err != nil {
		return fmt.Errorf("failed to store issue: %w", err)
	}

	s.logger.Debug().Str("key", issue.Key).Msg("Stored Jira issue")
	return nil
}

// StoreIssues stores multiple Jira issues in a transaction
func (s *JiraStorage) StoreIssues(ctx context.Context, issues []*models.JiraIssue) error {
	// Serialize write operations to prevent SQLITE_BUSY errors when multiple
	// goroutines try to write simultaneously
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO jira_issues (key, project_key, id, summary, description, fields, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET project_key = excluded.project_key, id = excluded.id, summary = excluded.summary, description = excluded.description, fields = excluded.fields, updated_at = excluded.updated_at")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	storedCount := 0
	for _, issue := range issues {
		fields, err := json.Marshal(issue.Fields)
		if err != nil {
			s.logger.Warn().Str("key", issue.Key).Err(err).Msg("Failed to marshal issue fields")
			continue
		}

		// Extract summary and description for FTS
		summary := ""
		description := ""
		if issue.Fields != nil {
			if s, ok := issue.Fields["summary"].(string); ok {
				summary = s
			}
			if d, ok := issue.Fields["description"].(string); ok {
				description = d
			}
		}

		// Extract project key from issue key (format: PROJECT-123)
		projectKey := ""
		if idx := strings.Index(issue.Key, "-"); idx > 0 {
			projectKey = issue.Key[:idx]
		}

		if projectKey == "" {
			s.logger.Warn().Str("key", issue.Key).Msg("Failed to extract project key from issue key")
			continue
		}

		_, err = stmt.ExecContext(ctx, issue.Key, projectKey, issue.ID, summary, description, fields, now)
		if err != nil {
			s.logger.Error().Str("key", issue.Key).Str("project", projectKey).Err(err).Msg("Failed to execute insert for issue")
			return fmt.Errorf("failed to store issue %s: %w", issue.Key, err)
		}

		storedCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Int("stored", storedCount).Int("total", len(issues)).Msg("Stored Jira issues batch")
	return nil
}

// Remaining methods omitted for brevity - implement similar pattern
// GetIssue, GetIssuesByProject, DeleteIssue, DeleteIssuesByProject,
// CountIssues, CountIssuesByProject, SearchIssues, ClearAll

func (s *JiraStorage) GetIssue(ctx context.Context, key string) (*models.JiraIssue, error) {
	var fields []byte
	query := "SELECT key, id, fields FROM jira_issues WHERE key = ?"

	var issue models.JiraIssue
	err := s.db.DB().QueryRowContext(ctx, query, key).Scan(&issue.Key, &issue.ID, &fields)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	if err := json.Unmarshal(fields, &issue.Fields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issue fields: %w", err)
	}

	return &issue, nil
}

func (s *JiraStorage) GetIssuesByProject(ctx context.Context, projectKey string) ([]*models.JiraIssue, error) {
	query := "SELECT key, id, fields FROM jira_issues WHERE project_key = ? ORDER BY key"

	rows, err := s.db.DB().QueryContext(ctx, query, projectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to query issues: %w", err)
	}
	defer rows.Close()

	var issues []*models.JiraIssue
	for rows.Next() {
		var fields []byte
		var issue models.JiraIssue

		if err := rows.Scan(&issue.Key, &issue.ID, &fields); err != nil {
			return nil, fmt.Errorf("failed to scan issue: %w", err)
		}

		if err := json.Unmarshal(fields, &issue.Fields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal issue fields: %w", err)
		}

		issues = append(issues, &issue)
	}

	return issues, rows.Err()
}

func (s *JiraStorage) DeleteIssue(ctx context.Context, key string) error {
	query := "DELETE FROM jira_issues WHERE key = ?"
	_, err := s.db.DB().ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("failed to delete issue: %w", err)
	}

	s.logger.Debug().Str("key", key).Msg("Deleted Jira issue")
	return nil
}

func (s *JiraStorage) DeleteIssuesByProject(ctx context.Context, projectKey string) error {
	query := "DELETE FROM jira_issues WHERE project_key = ?"
	result, err := s.db.DB().ExecContext(ctx, query, projectKey)
	if err != nil {
		return fmt.Errorf("failed to delete issues: %w", err)
	}

	rows, _ := result.RowsAffected()
	s.logger.Debug().Str("project", projectKey).Int64("deleted", rows).Msg("Deleted project issues")
	return nil
}

func (s *JiraStorage) CountIssues(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM jira_issues"
	err := s.db.DB().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count issues: %w", err)
	}
	return count, nil
}

func (s *JiraStorage) CountIssuesByProject(ctx context.Context, projectKey string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM jira_issues WHERE project_key = ?"
	err := s.db.DB().QueryRowContext(ctx, query, projectKey).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count issues: %w", err)
	}
	return count, nil
}

// GetMostRecentIssue returns the most recently updated issue with its timestamp
func (s *JiraStorage) GetMostRecentIssue(ctx context.Context) (*models.JiraIssue, int64, error) {
	var fields []byte
	var updatedAt int64
	var issue models.JiraIssue

	query := "SELECT key, id, fields, updated_at FROM jira_issues ORDER BY updated_at DESC LIMIT 1"

	err := s.db.DB().QueryRowContext(ctx, query).Scan(&issue.Key, &issue.ID, &fields, &updatedAt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get most recent issue: %w", err)
	}

	if err := json.Unmarshal(fields, &issue.Fields); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal issue fields: %w", err)
	}

	return &issue, updatedAt, nil
}

func (s *JiraStorage) SearchIssues(ctx context.Context, query string) ([]*models.JiraIssue, error) {
	// Use FTS5 table for full-text search
	sqlQuery := `
		SELECT ji.key, ji.id, ji.fields
		FROM jira_issues ji
		JOIN jira_issues_fts fts ON ji.key = fts.key
		WHERE jira_issues_fts MATCH ?
		ORDER BY rank
		LIMIT 100
	`

	rows, err := s.db.DB().QueryContext(ctx, sqlQuery, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}
	defer rows.Close()

	var issues []*models.JiraIssue
	for rows.Next() {
		var fields []byte
		var issue models.JiraIssue

		if err := rows.Scan(&issue.Key, &issue.ID, &fields); err != nil {
			return nil, fmt.Errorf("failed to scan issue: %w", err)
		}

		if err := json.Unmarshal(fields, &issue.Fields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal issue fields: %w", err)
		}

		issues = append(issues, &issue)
	}

	return issues, rows.Err()
}

func (s *JiraStorage) ClearAll(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM jira_issues"); err != nil {
		return fmt.Errorf("failed to clear issues: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM jira_projects"); err != nil {
		return fmt.Errorf("failed to clear projects: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Msg("Cleared all Jira data")
	return nil
}
