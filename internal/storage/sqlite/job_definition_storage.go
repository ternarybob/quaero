// -----------------------------------------------------------------------
// Last Modified: Monday, 20th October 2025 5:35:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ErrJobDefinitionNotFound is returned when a job definition is not found
var ErrJobDefinitionNotFound = errors.New("job definition not found")

// JobDefinitionStorage implements interfaces.JobDefinitionStorage for SQLite
type JobDefinitionStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
	mu     sync.Mutex
}

// NewJobDefinitionStorage creates a new JobDefinitionStorage instance
func NewJobDefinitionStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.JobDefinitionStorage {
	return &JobDefinitionStorage{
		db:     db,
		logger: logger,
	}
}

// SaveJobDefinition creates or updates a job definition
func (s *JobDefinitionStorage) SaveJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate job definition before persisting
	if err := jobDef.Validate(); err != nil {
		return fmt.Errorf("job definition validation failed: %w", err)
	}

	// Set timestamps
	if jobDef.CreatedAt.IsZero() {
		jobDef.CreatedAt = time.Now()
	}
	jobDef.UpdatedAt = time.Now()

	// Serialize Sources array to JSON using model helper
	sourcesJSON, err := jobDef.MarshalSources()
	if err != nil {
		return err
	}

	// Serialize Steps array to JSON using model helper
	stepsJSON, err := jobDef.MarshalSteps()
	if err != nil {
		return err
	}

	// Serialize Config map to JSON using model helper
	configJSON, err := jobDef.MarshalConfig()
	if err != nil {
		return err
	}

	// Convert bools to integers
	enabled := 0
	if jobDef.Enabled {
		enabled = 1
	}
	autoStart := 0
	if jobDef.AutoStart {
		autoStart = 1
	}

	// Convert timestamps to Unix integers
	createdAt := jobDef.CreatedAt.Unix()
	updatedAt := jobDef.UpdatedAt.Unix()

	// Insert or update using ON CONFLICT
	query := `
		INSERT INTO job_definitions (
			id, name, type, description, sources, steps, schedule, timeout,
			enabled, auto_start, config, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			description = excluded.description,
			sources = excluded.sources,
			steps = excluded.steps,
			schedule = excluded.schedule,
			timeout = excluded.timeout,
			enabled = excluded.enabled,
			auto_start = excluded.auto_start,
			config = excluded.config,
			updated_at = excluded.updated_at
	`

	_, err = s.db.DB().ExecContext(ctx, query,
		jobDef.ID, jobDef.Name, string(jobDef.Type), jobDef.Description,
		sourcesJSON, stepsJSON, jobDef.Schedule, jobDef.Timeout,
		enabled, autoStart, configJSON, createdAt, updatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save job definition: %w", err)
	}

	s.logger.Info().
		Str("job_def_id", jobDef.ID).
		Str("job_def_name", jobDef.Name).
		Msg("Job definition saved successfully")

	return nil
}

// UpdateJobDefinition updates an existing job definition
func (s *JobDefinitionStorage) UpdateJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate job definition before updating
	if err := jobDef.Validate(); err != nil {
		return fmt.Errorf("job definition validation failed: %w", err)
	}

	// Check if job definition exists
	var exists int
	checkQuery := `SELECT COUNT(*) FROM job_definitions WHERE id = ?`
	err := s.db.DB().QueryRowContext(ctx, checkQuery, jobDef.ID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check job definition existence: %w", err)
	}
	if exists == 0 {
		return ErrJobDefinitionNotFound
	}

	// Set UpdatedAt timestamp
	jobDef.UpdatedAt = time.Now()

	// Serialize Sources array to JSON using model helper
	sourcesJSON, err := jobDef.MarshalSources()
	if err != nil {
		return err
	}

	// Serialize Steps array to JSON using model helper
	stepsJSON, err := jobDef.MarshalSteps()
	if err != nil {
		return err
	}

	// Serialize Config map to JSON using model helper
	configJSON, err := jobDef.MarshalConfig()
	if err != nil {
		return err
	}

	// Convert bools to integers
	enabled := 0
	if jobDef.Enabled {
		enabled = 1
	}
	autoStart := 0
	if jobDef.AutoStart {
		autoStart = 1
	}

	// Convert timestamps to Unix integers
	updatedAt := jobDef.UpdatedAt.Unix()

	// Update query
	query := `
		UPDATE job_definitions SET
			name = ?,
			type = ?,
			description = ?,
			sources = ?,
			steps = ?,
			schedule = ?,
			timeout = ?,
			enabled = ?,
			auto_start = ?,
			config = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err = s.db.DB().ExecContext(ctx, query,
		jobDef.Name, string(jobDef.Type), jobDef.Description,
		sourcesJSON, stepsJSON, jobDef.Schedule, jobDef.Timeout,
		enabled, autoStart, configJSON, updatedAt, jobDef.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job definition: %w", err)
	}

	s.logger.Info().
		Str("job_def_id", jobDef.ID).
		Str("job_def_name", jobDef.Name).
		Msg("Job definition updated successfully")

	return nil
}

// GetJobDefinition retrieves a job definition by ID
func (s *JobDefinitionStorage) GetJobDefinition(ctx context.Context, id string) (*models.JobDefinition, error) {
	query := `
		SELECT id, name, type, description, sources, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, created_at, updated_at
		FROM job_definitions
		WHERE id = ?
	`

	row := s.db.DB().QueryRowContext(ctx, query, id)
	jobDef, err := s.scanJobDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrJobDefinitionNotFound
		}
		return nil, fmt.Errorf("failed to get job definition: %w", err)
	}

	return jobDef, nil
}

// ListJobDefinitions lists job definitions with optional filtering and pagination
func (s *JobDefinitionStorage) ListJobDefinitions(ctx context.Context, opts *interfaces.JobDefinitionListOptions) ([]*models.JobDefinition, error) {
	query := `
		SELECT id, name, type, description, sources, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, created_at, updated_at
		FROM job_definitions
		WHERE 1=1
	`
	args := []interface{}{}

	// Apply filters if provided
	if opts != nil {
		// Filter by type
		if opts.Type != "" {
			query += " AND type = ?"
			args = append(args, opts.Type)
		}

		// Filter by enabled status
		if opts.Enabled != nil {
			if *opts.Enabled {
				query += " AND enabled = 1"
			} else {
				query += " AND enabled = 0"
			}
		}

		// Apply ordering
		orderBy := "created_at DESC"
		if opts.OrderBy != "" {
			switch opts.OrderBy {
			case "created_at", "updated_at", "name":
				orderDir := "DESC"
				if opts.OrderDir == "ASC" {
					orderDir = "ASC"
				}
				orderBy = fmt.Sprintf("%s %s", opts.OrderBy, orderDir)
			}
		}
		query += fmt.Sprintf(" ORDER BY %s", orderBy)

		// Apply pagination
		if opts.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, opts.Limit)

			if opts.Offset > 0 {
				query += " OFFSET ?"
				args = append(args, opts.Offset)
			}
		}
	} else {
		// Default ordering
		query += " ORDER BY created_at DESC"
	}

	rows, err := s.db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list job definitions: %w", err)
	}
	defer rows.Close()

	return s.scanJobDefinitions(rows)
}

// GetJobDefinitionsByType retrieves all job definitions of a specific type
func (s *JobDefinitionStorage) GetJobDefinitionsByType(ctx context.Context, jobType string) ([]*models.JobDefinition, error) {
	query := `
		SELECT id, name, type, description, sources, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, created_at, updated_at
		FROM job_definitions
		WHERE type = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.DB().QueryContext(ctx, query, jobType)
	if err != nil {
		return nil, fmt.Errorf("failed to get job definitions by type: %w", err)
	}
	defer rows.Close()

	return s.scanJobDefinitions(rows)
}

// GetEnabledJobDefinitions retrieves all enabled job definitions
func (s *JobDefinitionStorage) GetEnabledJobDefinitions(ctx context.Context) ([]*models.JobDefinition, error) {
	query := `
		SELECT id, name, type, description, sources, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, created_at, updated_at
		FROM job_definitions
		WHERE enabled = 1
		ORDER BY created_at DESC
	`

	rows, err := s.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled job definitions: %w", err)
	}
	defer rows.Close()

	return s.scanJobDefinitions(rows)
}

// DeleteJobDefinition deletes a job definition by ID
func (s *JobDefinitionStorage) DeleteJobDefinition(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM job_definitions WHERE id = ?`
	result, err := s.db.DB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete job definition: %w", err)
	}

	// Check if any row was deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrJobDefinitionNotFound
	}

	s.logger.Info().
		Str("job_def_id", id).
		Msg("Job definition deleted successfully")

	return nil
}

// CountJobDefinitions returns the total count of job definitions
func (s *JobDefinitionStorage) CountJobDefinitions(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM job_definitions`
	err := s.db.DB().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count job definitions: %w", err)
	}
	return count, nil
}

// scanJobDefinition scans a single row into a JobDefinition
func (s *JobDefinitionStorage) scanJobDefinition(row *sql.Row) (*models.JobDefinition, error) {
	var (
		id, name, jobType, description, sourcesJSON, stepsJSON, schedule, timeout, configJSON string
		enabled, autoStart                                                                    int
		createdAt, updatedAt                                                                  int64
	)

	err := row.Scan(
		&id, &name, &jobType, &description, &sourcesJSON, &stepsJSON, &schedule, &timeout,
		&enabled, &autoStart, &configJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Construct JobDefinition
	jobDef := &models.JobDefinition{
		ID:          id,
		Name:        name,
		Type:        models.JobType(jobType),
		Description: description,
		Schedule:    schedule,
		Timeout:     timeout,
		Enabled:     enabled == 1,
		AutoStart:   autoStart == 1,
		CreatedAt:   time.Unix(createdAt, 0),
		UpdatedAt:   time.Unix(updatedAt, 0),
	}

	// Deserialize Sources JSON using model helper
	if err := jobDef.UnmarshalSources(sourcesJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal sources JSON")
		jobDef.Sources = []string{}
	}

	// Deserialize Steps JSON using model helper
	if err := jobDef.UnmarshalSteps(stepsJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal steps JSON")
		jobDef.Steps = []models.JobStep{}
	}

	// Deserialize Config JSON using model helper
	if err := jobDef.UnmarshalConfig(configJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal config JSON")
		jobDef.Config = make(map[string]interface{})
	}

	return jobDef, nil
}

// scanJobDefinitions scans multiple rows into JobDefinition slice
func (s *JobDefinitionStorage) scanJobDefinitions(rows *sql.Rows) ([]*models.JobDefinition, error) {
	var jobDefs []*models.JobDefinition

	for rows.Next() {
		var (
			id, name, jobType, description, sourcesJSON, stepsJSON, schedule, timeout, configJSON string
			enabled, autoStart                                                                    int
			createdAt, updatedAt                                                                  int64
		)

		err := rows.Scan(
			&id, &name, &jobType, &description, &sourcesJSON, &stepsJSON, &schedule, &timeout,
			&enabled, &autoStart, &configJSON, &createdAt, &updatedAt,
		)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Msg("Failed to scan job definition row, skipping")
			continue
		}

		// Construct JobDefinition
		jobDef := &models.JobDefinition{
			ID:          id,
			Name:        name,
			Type:        models.JobType(jobType),
			Description: description,
			Schedule:    schedule,
			Timeout:     timeout,
			Enabled:     enabled == 1,
			AutoStart:   autoStart == 1,
			CreatedAt:   time.Unix(createdAt, 0),
			UpdatedAt:   time.Unix(updatedAt, 0),
		}

		// Deserialize Sources JSON using model helper
		if err := jobDef.UnmarshalSources(sourcesJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal sources JSON")
			jobDef.Sources = []string{}
		}

		// Deserialize Steps JSON using model helper
		if err := jobDef.UnmarshalSteps(stepsJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal steps JSON")
			jobDef.Steps = []models.JobStep{}
		}

		// Deserialize Config JSON using model helper
		if err := jobDef.UnmarshalConfig(configJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal config JSON")
			jobDef.Config = make(map[string]interface{})
		}

		jobDefs = append(jobDefs, jobDef)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job definition rows: %w", err)
	}

	return jobDefs, nil
}
