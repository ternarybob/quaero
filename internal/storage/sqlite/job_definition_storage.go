// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 11:27:50 am
// Modified By: Bob McAllan
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

	// Serialize PostJobs array to JSON using model helper
	postJobsJSON, err := jobDef.MarshalPostJobs()
	if err != nil {
		return err
	}

	// Serialize PreJobs array to JSON using model helper
	preJobsJSON, err := jobDef.MarshalPreJobs()
	if err != nil {
		return err
	}

	// Serialize ErrorTolerance to JSON using model helper
	errorToleranceJSON, err := jobDef.MarshalErrorTolerance()
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
	// Convert auth_id to NULL if empty (to satisfy foreign key constraint)
	var authID interface{}
	if jobDef.AuthID == "" {
		authID = nil
	} else {
		authID = jobDef.AuthID
	}

	query := `
		INSERT INTO job_definitions (
			id, name, type, job_type, description, source_type, base_url, auth_id, steps, schedule, timeout,
			enabled, auto_start, config, pre_jobs, post_jobs, error_tolerance, toml, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			job_type = excluded.job_type,
			description = excluded.description,
			source_type = excluded.source_type,
			base_url = excluded.base_url,
			auth_id = excluded.auth_id,
			steps = excluded.steps,
			schedule = excluded.schedule,
			timeout = excluded.timeout,
			enabled = excluded.enabled,
			auto_start = excluded.auto_start,
			config = excluded.config,
			pre_jobs = excluded.pre_jobs,
			post_jobs = excluded.post_jobs,
			error_tolerance = excluded.error_tolerance,
			toml = excluded.toml,
			updated_at = excluded.updated_at
	`

	_, err = s.db.DB().ExecContext(ctx, query,
		jobDef.ID, jobDef.Name, string(jobDef.Type), string(jobDef.JobType), jobDef.Description,
		jobDef.SourceType, jobDef.BaseURL, authID, stepsJSON, jobDef.Schedule, jobDef.Timeout,
		enabled, autoStart, configJSON, preJobsJSON, postJobsJSON, errorToleranceJSON, jobDef.TOML, createdAt, updatedAt,
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

	// Serialize PostJobs array to JSON using model helper
	postJobsJSON, err := jobDef.MarshalPostJobs()
	if err != nil {
		return err
	}

	// Serialize PreJobs array to JSON using model helper
	preJobsJSON, err := jobDef.MarshalPreJobs()
	if err != nil {
		return err
	}

	// Serialize ErrorTolerance to JSON using model helper
	errorToleranceJSON, err := jobDef.MarshalErrorTolerance()
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

	// Convert auth_id to NULL if empty (to satisfy foreign key constraint)
	var authID interface{}
	if jobDef.AuthID == "" {
		authID = nil
	} else {
		authID = jobDef.AuthID
	}

	// Update query
	query := `
		UPDATE job_definitions SET
			name = ?,
			type = ?,
			job_type = ?,
			description = ?,
			source_type = ?,
			base_url = ?,
			auth_id = ?,
			steps = ?,
			schedule = ?,
			timeout = ?,
			enabled = ?,
			auto_start = ?,
			config = ?,
			pre_jobs = ?,
			post_jobs = ?,
			error_tolerance = ?,
			toml = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err = s.db.DB().ExecContext(ctx, query,
		jobDef.Name, string(jobDef.Type), string(jobDef.JobType), jobDef.Description,
		jobDef.SourceType, jobDef.BaseURL, authID, stepsJSON, jobDef.Schedule, jobDef.Timeout,
		enabled, autoStart, configJSON, preJobsJSON, postJobsJSON, errorToleranceJSON, jobDef.TOML, updatedAt, jobDef.ID,
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
		SELECT id, name, type, COALESCE(job_type, 'user') AS job_type, description, COALESCE(source_type, '') AS source_type, COALESCE(base_url, '') AS base_url, COALESCE(auth_id, '') AS auth_id, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, COALESCE(pre_jobs, '[]') AS pre_jobs, COALESCE(post_jobs, '[]') AS post_jobs, COALESCE(error_tolerance, '{}') AS error_tolerance, COALESCE(toml, '') AS toml, created_at, updated_at
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
		SELECT id, name, type, COALESCE(job_type, 'user') AS job_type, description, COALESCE(source_type, '') AS source_type, COALESCE(base_url, '') AS base_url, COALESCE(auth_id, '') AS auth_id, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, COALESCE(pre_jobs, '[]') AS pre_jobs, COALESCE(post_jobs, '[]') AS post_jobs, COALESCE(error_tolerance, '{}') AS error_tolerance, COALESCE(toml, '') AS toml, created_at, updated_at
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
		SELECT id, name, type, COALESCE(job_type, 'user') AS job_type, description, COALESCE(source_type, '') AS source_type, COALESCE(base_url, '') AS base_url, COALESCE(auth_id, '') AS auth_id, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, COALESCE(pre_jobs, '[]') AS pre_jobs, COALESCE(post_jobs, '[]') AS post_jobs, COALESCE(error_tolerance, '{}') AS error_tolerance, COALESCE(toml, '') AS toml, created_at, updated_at
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
		SELECT id, name, type, COALESCE(job_type, 'user') AS job_type, description, COALESCE(source_type, '') AS source_type, COALESCE(base_url, '') AS base_url, COALESCE(auth_id, '') AS auth_id, steps, schedule, COALESCE(timeout, '') AS timeout,
		       enabled, auto_start, config, COALESCE(pre_jobs, '[]') AS pre_jobs, COALESCE(post_jobs, '[]') AS post_jobs, COALESCE(error_tolerance, '{}') AS error_tolerance, COALESCE(toml, '') AS toml, created_at, updated_at
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

// CreateDefaultJobDefinitions creates default job definitions that ship with Quaero.
// This method is idempotent and safe to call multiple times - it will only create
// missing job definitions using ON CONFLICT DO NOTHING to preserve user customizations.
func (s *JobDefinitionStorage) CreateDefaultJobDefinitions(ctx context.Context) error {
	// Database Maintenance Job - Manual execution only
	dbMaintenanceJob := &models.JobDefinition{
		ID:          "database-maintenance",
		Name:        "Database Maintenance",
		Type:        models.JobDefinitionTypeCustom,
		JobType:     models.JobOwnerTypeSystem, // System job - readonly
		Description: "Performs database maintenance operations including VACUUM, ANALYZE, and REINDEX to optimize database performance and reclaim disk space.",
		Steps: []models.JobStep{
			{
				Name:   "database_maintenance",
				Action: "database_maintenance",
				Config: map[string]interface{}{
					"operations": []string{"vacuum", "analyze", "reindex"},
				},
				OnError: models.ErrorStrategyFail,
			},
		},
		Schedule:  "", // Manual execution only (no automatic schedule)
		Timeout:   "30m",
		Enabled:   true,
		AutoStart: false,
		Config:    make(map[string]interface{}),
		PostJobs:  []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dbMaintenanceStepsJSON, err := dbMaintenanceJob.MarshalSteps()
	if err != nil {
		return fmt.Errorf("failed to marshal database maintenance steps: %w", err)
	}

	dbMaintenanceConfigJSON, err := dbMaintenanceJob.MarshalConfig()
	if err != nil {
		return fmt.Errorf("failed to marshal database maintenance config: %w", err)
	}

	dbMaintenancePreJobsJSON, err := dbMaintenanceJob.MarshalPreJobs()
	if err != nil {
		return fmt.Errorf("failed to marshal database maintenance pre_jobs: %w", err)
	}

	dbMaintenancePostJobsJSON, err := dbMaintenanceJob.MarshalPostJobs()
	if err != nil {
		return fmt.Errorf("failed to marshal database maintenance post_jobs: %w", err)
	}

	// Convert bools to integers for database maintenance job
	dbMaintenanceEnabled := 0
	if dbMaintenanceJob.Enabled {
		dbMaintenanceEnabled = 1
	}
	dbMaintenanceAutoStart := 0
	if dbMaintenanceJob.AutoStart {
		dbMaintenanceAutoStart = 1
	}

	// Convert timestamps to Unix integers for database maintenance job
	dbMaintenanceCreatedAt := dbMaintenanceJob.CreatedAt.Unix()
	dbMaintenanceUpdatedAt := dbMaintenanceJob.UpdatedAt.Unix()

	// Serialize ErrorTolerance to JSON using model helper for database maintenance job
	dbMaintenanceErrorToleranceJSON, err := dbMaintenanceJob.MarshalErrorTolerance()
	if err != nil {
		return fmt.Errorf("failed to marshal database maintenance error_tolerance: %w", err)
	}

	// Convert auth_id to NULL if empty (to satisfy foreign key constraint)
	var dbMaintenanceAuthID interface{}
	if dbMaintenanceJob.AuthID == "" {
		dbMaintenanceAuthID = nil
	} else {
		dbMaintenanceAuthID = dbMaintenanceJob.AuthID
	}

	// Insert database maintenance job definition using ON CONFLICT DO NOTHING to preserve user customizations
	dbMaintenanceQuery := `
		INSERT INTO job_definitions (
			id, name, type, job_type, description, source_type, base_url, auth_id, steps, schedule, timeout,
			enabled, auto_start, config, pre_jobs, post_jobs, error_tolerance, toml, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`

	dbMaintenanceResult, err := s.db.DB().ExecContext(ctx, dbMaintenanceQuery,
		dbMaintenanceJob.ID, dbMaintenanceJob.Name, string(dbMaintenanceJob.Type), string(dbMaintenanceJob.JobType), dbMaintenanceJob.Description,
		dbMaintenanceJob.SourceType, dbMaintenanceJob.BaseURL, dbMaintenanceAuthID, dbMaintenanceStepsJSON, dbMaintenanceJob.Schedule, dbMaintenanceJob.Timeout,
		dbMaintenanceEnabled, dbMaintenanceAutoStart, dbMaintenanceConfigJSON, dbMaintenancePreJobsJSON, dbMaintenancePostJobsJSON, dbMaintenanceErrorToleranceJSON, dbMaintenanceJob.TOML, dbMaintenanceCreatedAt, dbMaintenanceUpdatedAt,
	)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_def_id", dbMaintenanceJob.ID).
			Msg("Failed to create default database maintenance job")
		return fmt.Errorf("failed to create default database maintenance job: %w", err)
	}

	// Check if a row was actually inserted for database maintenance job
	dbMaintenanceRowsAffected, err := dbMaintenanceResult.RowsAffected()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to get rows affected for database maintenance job creation")
	} else if dbMaintenanceRowsAffected > 0 {
		s.logger.Info().
			Str("job_def_id", dbMaintenanceJob.ID).
			Str("job_def_name", dbMaintenanceJob.Name).
			Msg("Default job definition created")
	} else {
		s.logger.Debug().
			Str("job_def_id", dbMaintenanceJob.ID).
			Msg("Default job definition already exists, preserving user customizations")
	}

	// Corpus Summary Job - Generates corpus statistics hourly
	corpusSummaryJob := &models.JobDefinition{
		ID:          "default-corpus-summary",
		Name:        "Corpus Summary Generation",
		Type:        models.JobDefinitionTypeCustom,
		JobType:     models.JobOwnerTypeSystem, // System job - readonly
		Description: "Generates a summary document containing statistics about the document corpus (total documents, documents by source type). This summary is searchable and enables queries like 'how many documents are in the system'. Runs hourly to keep statistics current.",
		Steps: []models.JobStep{
			{
				Name:    "corpus_summary",
				Action:  "corpus_summary",
				Config:  map[string]interface{}{},
				OnError: models.ErrorStrategyFail,
			},
		},
		Schedule:  "0 * * * *", // Hourly at minute 0
		Timeout:   "5m",        // 5 minutes should be sufficient for counting
		Enabled:   true,        // Enable by default
		AutoStart: false,       // Only run on schedule, not on startup
		Config:    make(map[string]interface{}),
		PostJobs:  []string{}, // No post jobs for corpus summary job
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	corpusStepsJSON, err := corpusSummaryJob.MarshalSteps()
	if err != nil {
		return fmt.Errorf("failed to marshal corpus summary steps: %w", err)
	}

	corpusConfigJSON, err := corpusSummaryJob.MarshalConfig()
	if err != nil {
		return fmt.Errorf("failed to marshal corpus summary config: %w", err)
	}

	corpusPreJobsJSON, err := corpusSummaryJob.MarshalPreJobs()
	if err != nil {
		return fmt.Errorf("failed to marshal corpus summary pre_jobs: %w", err)
	}

	corpusPostJobsJSON, err := corpusSummaryJob.MarshalPostJobs()
	if err != nil {
		return fmt.Errorf("failed to marshal corpus summary post_jobs: %w", err)
	}

	// Convert bools to integers for corpus summary job
	corpusEnabled := 0
	if corpusSummaryJob.Enabled {
		corpusEnabled = 1
	}
	corpusAutoStart := 0
	if corpusSummaryJob.AutoStart {
		corpusAutoStart = 1
	}

	// Convert timestamps to Unix integers for corpus summary job
	corpusCreatedAt := corpusSummaryJob.CreatedAt.Unix()
	corpusUpdatedAt := corpusSummaryJob.UpdatedAt.Unix()

	// Serialize ErrorTolerance to JSON using model helper for corpus summary job
	corpusErrorToleranceJSON, err := corpusSummaryJob.MarshalErrorTolerance()
	if err != nil {
		return fmt.Errorf("failed to marshal corpus summary error_tolerance: %w", err)
	}

	// Convert auth_id to NULL if empty (to satisfy foreign key constraint)
	var corpusAuthID interface{}
	if corpusSummaryJob.AuthID == "" {
		corpusAuthID = nil
	} else {
		corpusAuthID = corpusSummaryJob.AuthID
	}

	// Insert corpus summary job definition using ON CONFLICT DO NOTHING to preserve user customizations
	corpusQuery := `
		INSERT INTO job_definitions (
			id, name, type, job_type, description, source_type, base_url, auth_id, steps, schedule, timeout,
			enabled, auto_start, config, pre_jobs, post_jobs, error_tolerance, toml, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`

	corpusResult, err := s.db.DB().ExecContext(ctx, corpusQuery,
		corpusSummaryJob.ID, corpusSummaryJob.Name, string(corpusSummaryJob.Type), string(corpusSummaryJob.JobType), corpusSummaryJob.Description,
		corpusSummaryJob.SourceType, corpusSummaryJob.BaseURL, corpusAuthID, corpusStepsJSON, corpusSummaryJob.Schedule, corpusSummaryJob.Timeout,
		corpusEnabled, corpusAutoStart, corpusConfigJSON, corpusPreJobsJSON, corpusPostJobsJSON, corpusErrorToleranceJSON, corpusSummaryJob.TOML, corpusCreatedAt, corpusUpdatedAt,
	)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_def_id", corpusSummaryJob.ID).
			Msg("Failed to create default corpus summary job")
		return fmt.Errorf("failed to create default corpus summary job: %w", err)
	}

	// Check if a row was actually inserted for corpus summary job
	corpusRowsAffected, err := corpusResult.RowsAffected()
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to get rows affected for corpus summary job creation")
	} else if corpusRowsAffected > 0 {
		s.logger.Info().
			Str("job_def_id", corpusSummaryJob.ID).
			Str("job_def_name", corpusSummaryJob.Name).
			Msg("Default job definition created")
	} else {
		s.logger.Debug().
			Str("job_def_id", corpusSummaryJob.ID).
			Msg("Default job definition already exists, preserving user customizations")
	}

	return nil
}

// scanJobDefinition scans a single row into a JobDefinition
func (s *JobDefinitionStorage) scanJobDefinition(row *sql.Row) (*models.JobDefinition, error) {
	var (
		id, name, jobDefType, jobOwnerType, description, sourceType, baseURL, authID, stepsJSON, schedule, timeout, configJSON, preJobsJSON, postJobsJSON, errorToleranceJSON, toml string
		enabled, autoStart                                                                                                                                                                int
		createdAt, updatedAt                                                                                                                                                              int64
	)

	err := row.Scan(
		&id, &name, &jobDefType, &jobOwnerType, &description, &sourceType, &baseURL, &authID, &stepsJSON, &schedule, &timeout,
		&enabled, &autoStart, &configJSON, &preJobsJSON, &postJobsJSON, &errorToleranceJSON, &toml, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Construct JobDefinition
	jobDef := &models.JobDefinition{
		ID:          id,
		Name:        name,
		Type:        models.JobDefinitionType(jobDefType),
		JobType:     models.JobOwnerType(jobOwnerType),
		Description: description,
		SourceType:  sourceType,
		BaseURL:     baseURL,
		AuthID:      authID,
		Schedule:    schedule,
		Timeout:     timeout,
		Enabled:     enabled == 1,
		AutoStart:   autoStart == 1,
		TOML:        toml,
		CreatedAt:   time.Unix(createdAt, 0),
		UpdatedAt:   time.Unix(updatedAt, 0),
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

	// Deserialize PreJobs JSON using model helper
	if err := jobDef.UnmarshalPreJobs(preJobsJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal pre_jobs JSON")
		jobDef.PreJobs = []string{}
	}

	// Deserialize PostJobs JSON using model helper
	if err := jobDef.UnmarshalPostJobs(postJobsJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal post_jobs JSON")
		jobDef.PostJobs = []string{}
	}

	// Deserialize ErrorTolerance JSON using model helper
	if err := jobDef.UnmarshalErrorTolerance(errorToleranceJSON); err != nil {
		s.logger.Warn().
			Str("job_def_id", id).
			Err(err).
			Msg("Failed to unmarshal error_tolerance JSON")
		jobDef.ErrorTolerance = nil
	}

	return jobDef, nil
}

// scanJobDefinitions scans multiple rows into JobDefinition slice
func (s *JobDefinitionStorage) scanJobDefinitions(rows *sql.Rows) ([]*models.JobDefinition, error) {
	var jobDefs []*models.JobDefinition

	for rows.Next() {
		var (
			id, name, jobDefType, jobOwnerType, description, sourceType, baseURL, authID, stepsJSON, schedule, timeout, configJSON, preJobsJSON, postJobsJSON, errorToleranceJSON, toml string
			enabled, autoStart                                                                                                                                                                int
			createdAt, updatedAt                                                                                                                                                              int64
		)

		err := rows.Scan(
			&id, &name, &jobDefType, &jobOwnerType, &description, &sourceType, &baseURL, &authID, &stepsJSON, &schedule, &timeout,
			&enabled, &autoStart, &configJSON, &preJobsJSON, &postJobsJSON, &errorToleranceJSON, &toml, &createdAt, &updatedAt,
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
			Type:        models.JobDefinitionType(jobDefType),
			JobType:     models.JobOwnerType(jobOwnerType),
			Description: description,
			SourceType:  sourceType,
			BaseURL:     baseURL,
			AuthID:      authID,
			Schedule:    schedule,
			Timeout:     timeout,
			Enabled:     enabled == 1,
			AutoStart:   autoStart == 1,
			TOML:        toml,
			CreatedAt:   time.Unix(createdAt, 0),
			UpdatedAt:   time.Unix(updatedAt, 0),
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

		// Deserialize PreJobs JSON using model helper
		if err := jobDef.UnmarshalPreJobs(preJobsJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal pre_jobs JSON")
			jobDef.PreJobs = []string{}
		}

		// Deserialize PostJobs JSON using model helper
		if err := jobDef.UnmarshalPostJobs(postJobsJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal post_jobs JSON")
			jobDef.PostJobs = []string{}
		}

		// Deserialize ErrorTolerance JSON using model helper
		if err := jobDef.UnmarshalErrorTolerance(errorToleranceJSON); err != nil {
			s.logger.Warn().
				Str("job_def_id", id).
				Err(err).
				Msg("Failed to unmarshal error_tolerance JSON")
			jobDef.ErrorTolerance = nil
		}

		jobDefs = append(jobDefs, jobDef)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job definition rows: %w", err)
	}

	return jobDefs, nil
}
