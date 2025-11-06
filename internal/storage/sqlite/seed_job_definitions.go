// -----------------------------------------------------------------------
// Seed Job Definitions - Default job definitions for the system
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// SeedJobDefinitions creates default job definitions if they don't exist
// and loads user-defined job definitions from the specified directory
func (m *Manager) SeedJobDefinitions(ctx context.Context, jobDefsDir string) error {
	m.logger.Info().Msg("Seeding default job definitions")

	// Database Maintenance job definition
	dbMaintenanceDef := &models.JobDefinition{
		ID:          "database-maintenance",
		Name:        "Database Maintenance",
		Type:        models.JobDefinitionTypeCustom,
		Description: "Performs database maintenance operations including VACUUM, ANALYZE, and REINDEX",
		Sources:     []string{}, // No sources needed
		Steps: []models.JobStep{
			{
				Name:   "maintenance",
				Action: "database_maintenance",
				Config: map[string]interface{}{
					"operations": []string{"vacuum", "analyze", "reindex"},
				},
				OnError: models.ErrorStrategyFail,
			},
		},
		Schedule:  "", // Manual execution only
		Timeout:   "30m",
		Enabled:   true,
		AutoStart: false,
		Config:    map[string]interface{}{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.upsertJobDefinition(ctx, dbMaintenanceDef); err != nil {
		return err
	}

	// Stockhead Crawler job definition
	stockheadCrawlerDef := &models.JobDefinition{
		ID:          "stockhead-crawler",
		Name:        "Stockhead News Crawler",
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Crawls Stockhead 'Just In' news section, filtering for 'rise-and-shine' articles with 1-level depth",
		Sources:     []string{}, // No pre-configured source needed - uses start_urls
		Steps: []models.JobStep{
			{
				Name:   "crawl_stockhead",
				Action: "crawl",
				Config: map[string]interface{}{
					"start_urls":       []string{"https://stockhead.com.au/just-in/"},
					"include_patterns": []string{"rise-and-shine"},
					"exclude_patterns": []string{},
					"max_depth":        1,
					"max_pages":        100,
					"concurrency":      5,
					"follow_links":     true,
				},
				OnError: models.ErrorStrategyContinue,
			},
		},
		Schedule:  "", // Manual execution only (no automatic schedule)
		Timeout:   "30m",
		Enabled:   true,
		AutoStart: false,
		Config:    map[string]interface{}{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.upsertJobDefinition(ctx, stockheadCrawlerDef); err != nil {
		return err
	}

	// Load user-defined crawler jobs from files
	// This allows users to define custom crawler jobs without modifying code
	if jobDefsDir != "" {
		if err := m.LoadJobDefinitionsFromFiles(ctx, jobDefsDir); err != nil {
			m.logger.Warn().Err(err).Msg("Failed to load job definitions from files")
			// Don't fail startup - file loading is optional
		}
	}

	m.logger.Info().Msg("Default job definitions seeded successfully")
	return nil
}

// upsertJobDefinition inserts or updates a job definition
func (m *Manager) upsertJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	// Check if job definition exists
	var exists bool
	err := m.db.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM job_definitions WHERE id = ?)", jobDef.ID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		m.logger.Debug().
			Str("job_def_id", jobDef.ID).
			Msg("Job definition already exists, skipping")
		return nil
	}

	// Serialize fields
	sourcesJSON, err := jobDef.MarshalSources()
	if err != nil {
		return err
	}

	stepsJSON, err := jobDef.MarshalSteps()
	if err != nil {
		return err
	}

	configJSON, err := jobDef.MarshalConfig()
	if err != nil {
		return err
	}

	postJobsJSON, err := jobDef.MarshalPostJobs()
	if err != nil {
		return err
	}

	errorToleranceJSON, err := jobDef.MarshalErrorTolerance()
	if err != nil {
		return err
	}

	// Insert job definition
	_, err = m.db.DB().ExecContext(ctx, `
		INSERT INTO job_definitions (
			id, name, type, description, sources, steps, schedule, timeout,
			enabled, auto_start, config, post_jobs, error_tolerance,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		jobDef.ID,
		jobDef.Name,
		jobDef.Type,
		jobDef.Description,
		sourcesJSON,
		stepsJSON,
		jobDef.Schedule,
		jobDef.Timeout,
		boolToInt(jobDef.Enabled),
		boolToInt(jobDef.AutoStart),
		configJSON,
		postJobsJSON,
		errorToleranceJSON,
		timeToUnix(jobDef.CreatedAt),
		timeToUnix(jobDef.UpdatedAt),
	)

	if err != nil {
		return err
	}

	m.logger.Info().
		Str("job_def_id", jobDef.ID).
		Str("job_name", jobDef.Name).
		Msg("Job definition created")

	return nil
}

// boolToInt converts bool to int for SQLite storage
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// timeToUnix converts time.Time to Unix timestamp for SQLite storage
func timeToUnix(t time.Time) int64 {
	return t.Unix()
}
