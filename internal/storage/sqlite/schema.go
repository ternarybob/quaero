// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 6:44:16 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaSQL = `
-- Authentication table
-- Site-based authentication for multiple service instances
CREATE TABLE IF NOT EXISTS auth_credentials (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	site_domain TEXT NOT NULL,
	service_type TEXT NOT NULL,
	data TEXT,
	cookies TEXT,
	tokens TEXT NOT NULL,
	base_url TEXT NOT NULL,
	user_agent TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- Indexes for efficient lookup
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_site_domain ON auth_credentials(site_domain);
CREATE INDEX IF NOT EXISTS idx_auth_service_type ON auth_credentials(service_type, site_domain);

-- Jira tables
CREATE TABLE IF NOT EXISTS jira_projects (
	key TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	id TEXT NOT NULL,
	issue_count INTEGER DEFAULT 0,
	data TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS jira_issues (
	key TEXT PRIMARY KEY,
	project_key TEXT NOT NULL,
	id TEXT NOT NULL,
	summary TEXT,
	description TEXT,
	fields TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

-- Confluence tables
CREATE TABLE IF NOT EXISTS confluence_spaces (
	key TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	id TEXT NOT NULL,
	page_count INTEGER DEFAULT 0,
	data TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS confluence_pages (
	id TEXT PRIMARY KEY,
	space_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT,
	body TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- Documents table (normalized from all sources)
-- Supports Firecrawl-style layered crawling with detail_level
-- PRIMARY CONTENT FORMAT: Markdown (content_markdown field)
CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL,
	source_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content_markdown TEXT,
	detail_level TEXT DEFAULT 'full',
	metadata TEXT,
	url TEXT,
	embedding BLOB,
	embedding_model TEXT,
	last_synced INTEGER,
	source_version TEXT,
	force_sync_pending INTEGER DEFAULT 0,
	force_embed_pending INTEGER DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- LLM audit log table for compliance and debugging
CREATE TABLE IF NOT EXISTS llm_audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp INTEGER NOT NULL,
	mode TEXT NOT NULL,
	operation TEXT NOT NULL,
	success INTEGER NOT NULL,
	error TEXT,
	duration INTEGER NOT NULL,
	query_text TEXT
);

-- Unified jobs table - executor-agnostic job model
-- Stores all job types with flexible configuration as key-value pairs
-- Replaces the old crawl_jobs table with a simpler, more flexible structure
-- Configuration is stored as JSON map[string]interface{} for executor-agnostic design
CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	parent_id TEXT,
	job_type TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT DEFAULT '',
	config_json TEXT NOT NULL,
	metadata_json TEXT,
	status TEXT NOT NULL,
	progress_json TEXT,
	created_at INTEGER NOT NULL,
	started_at INTEGER,
	completed_at INTEGER,
	finished_at INTEGER,
	last_heartbeat INTEGER,
	error TEXT,
	result_count INTEGER DEFAULT 0,
	failed_count INTEGER DEFAULT 0,
	depth INTEGER DEFAULT 0,
	FOREIGN KEY (parent_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- Job indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_parent_id ON jobs(parent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(job_type, status, created_at DESC);

-- Job seen URLs table for concurrency-safe URL deduplication (VERIFICATION COMMENT 1)
-- Tracks URLs that have been enqueued for each job to prevent duplicate processing
-- Uses composite primary key (job_id, url) for atomic INSERT OR IGNORE operations
CREATE TABLE IF NOT EXISTS job_seen_urls (
	job_id TEXT NOT NULL,
	url TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (job_id, url),
	FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- Index for efficient cleanup when jobs are deleted
CREATE INDEX IF NOT EXISTS idx_job_seen_urls_job_id ON job_seen_urls(job_id);

-- Job logs table for structured log storage
-- Provides unlimited log history with indexed queries and automatic CASCADE DELETE
CREATE TABLE IF NOT EXISTS job_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT NOT NULL,
	timestamp TEXT NOT NULL,
	level TEXT NOT NULL,
	message TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_logs_job_id ON job_logs(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_logs_level ON job_logs(level, created_at DESC);

-- Source configurations table
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	base_url TEXT NOT NULL,
	enabled INTEGER DEFAULT 1,
	auth_id TEXT,
	crawl_config TEXT NOT NULL,
	filters TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sources_type ON sources(type, enabled);
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sources_auth ON sources(auth_id);

-- Job settings table for persisting scheduler job configurations
CREATE TABLE IF NOT EXISTS job_settings (
	job_name TEXT PRIMARY KEY,
	schedule TEXT NOT NULL,
	description TEXT DEFAULT '',
	enabled INTEGER DEFAULT 1,
	last_run INTEGER,
	updated_at INTEGER NOT NULL
);

-- Job definitions table for configurable, database-persisted job definitions
CREATE TABLE IF NOT EXISTS job_definitions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	description TEXT,
	sources TEXT NOT NULL,
	steps TEXT NOT NULL,
	schedule TEXT NOT NULL,
	timeout TEXT,
	enabled INTEGER DEFAULT 1,
	auto_start INTEGER DEFAULT 0,
	config TEXT,
	post_jobs TEXT,
	error_tolerance TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- Job definitions indexes
CREATE INDEX IF NOT EXISTS idx_job_definitions_type ON job_definitions(type, enabled);
CREATE INDEX IF NOT EXISTS idx_job_definitions_enabled ON job_definitions(enabled, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_definitions_schedule ON job_definitions(schedule);

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_documents_sync ON documents(force_sync_pending, force_embed_pending);
CREATE INDEX IF NOT EXISTS idx_documents_embedding ON documents(embedding_model) WHERE embedding IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_documents_detail_level ON documents(detail_level, source_type);

-- FTS5 index for full-text search on markdown content
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
	title,
	content_markdown,
	content=documents,
	content_rowid=rowid
);

-- Triggers to keep FTS index in sync with markdown content
CREATE TRIGGER IF NOT EXISTS documents_fts_insert AFTER INSERT ON documents BEGIN
	INSERT INTO documents_fts(rowid, title, content_markdown)
	VALUES (new.rowid, new.title, new.content_markdown);
END;

CREATE TRIGGER IF NOT EXISTS documents_fts_update AFTER UPDATE ON documents BEGIN
	UPDATE documents_fts
	SET title = new.title, content_markdown = new.content_markdown
	WHERE rowid = new.rowid;
END;

CREATE TRIGGER IF NOT EXISTS documents_fts_delete AFTER DELETE ON documents BEGIN
	DELETE FROM documents_fts WHERE rowid = old.rowid;
END;
`

// InitSchema initializes the database schema
func (s *SQLiteDB) InitSchema() error {
	_, err := s.db.Exec(schemaSQL)
	if err != nil {
		return err
	}
	s.logger.Info().Msg("Database schema initialized")

	// Run migrations for schema evolution
	if err := s.runMigrations(); err != nil {
		return err
	}

	// Create default job definitions after schema and migrations are complete
	// This ensures the job_definitions table exists and has the correct schema
	ctx := context.Background()
	jobDefStorage := NewJobDefinitionStorage(s, s.logger)
	if jds, ok := jobDefStorage.(*JobDefinitionStorage); ok {
		if err := jds.CreateDefaultJobDefinitions(ctx); err != nil {
			// Log warning but don't fail startup - default job definitions are a convenience feature
			s.logger.Warn().Err(err).Msg("Failed to create default job definitions")
		} else {
			s.logger.Debug().Msg("Default job definitions initialized")
		}
	}

	return nil
}

// runMigrations checks for and applies schema migrations for existing databases
func (s *SQLiteDB) runMigrations() error {
	// MIGRATION 1: Add missing crawl_jobs columns
	if err := s.migrateCrawlJobsColumns(); err != nil {
		return err
	}

	// MIGRATION 2: Remove content column and migrate to content_markdown only
	if err := s.migrateToMarkdownOnly(); err != nil {
		return err
	}

	// MIGRATION 3: Add last_heartbeat column to crawl_jobs
	if err := s.migrateAddHeartbeatColumn(); err != nil {
		return err
	}

	// MIGRATION 4: Add last_run column to job_settings
	if err := s.migrateAddLastRunColumn(); err != nil {
		return err
	}

	// MIGRATION 5: (deprecated) Add logs column to crawl_jobs
	// This migration is deprecated - the logs column has been replaced by the job_logs table
	// Migration kept for backward compatibility but does nothing on new installations
	if err := s.migrateAddJobLogsColumn(); err != nil {
		return err
	}

	// MIGRATION 6: Add name and description columns to crawl_jobs
	if err := s.migrateAddJobNameDescriptionColumns(); err != nil {
		return err
	}

	// MIGRATION 7: Add description column to job_settings
	if err := s.migrateAddJobSettingsDescriptionColumn(); err != nil {
		return err
	}

	// MIGRATION 8: Add job_definitions table
	if err := s.migrateAddJobDefinitionsTable(); err != nil {
		return err
	}

	// MIGRATION 9: (deprecated) Add seed_urls column to sources table
	// This migration is no longer needed as seed URLs are job-level configuration
	// Kept commented for historical reference
	// if err := s.migrateAddSourcesSeedURLsColumn(); err != nil {
	// 	return err
	// }

	// MIGRATION 10: Remove filters and seed_urls columns from sources table
	if err := s.migrateRemoveSourcesFilteringColumns(); err != nil {
		return err
	}

	// MIGRATION 11: Add timeout column to job_definitions table
	if err := s.migrateAddJobDefinitionsTimeoutColumn(); err != nil {
		return err
	}

	// MIGRATION 12: Add back filters column to sources table
	if err := s.migrateAddBackSourcesFiltersColumn(); err != nil {
		return err
	}

	// MIGRATION 13: Remove deprecated logs column from crawl_jobs table
	// Logs are now stored in the dedicated job_logs table with unlimited history
	// and better query performance. This migration recreates the crawl_jobs table
	// without the logs column while preserving all other data.
	if err := s.migrateRemoveLogsColumn(); err != nil {
		return err
	}

	// MIGRATION 14: Add post_jobs column to job_definitions table
	if err := s.migrateAddPostJobsColumn(); err != nil {
		return err
	}

	// MIGRATION 15: Add parent_id column to crawl_jobs table
	// Enables parent-child job hierarchy tracking for the Queue UI
	if err := s.migrateAddParentIdColumn(); err != nil {
		return err
	}

	// MIGRATION 16: Enable foreign keys and add parent_id CASCADE constraint
	// Ensures referential integrity and automatic cascade deletion of child jobs
	if err := s.migrateEnableForeignKeysAndAddParentConstraint(); err != nil {
		return err
	}

	// MIGRATION 17: Add pre_jobs column to job_definitions table
	if err := s.migrateAddPreJobsColumn(); err != nil {
		return err
	}

	// MIGRATION 18: Add job_type column to crawl_jobs table
	if err := s.migrateAddJobTypeColumn(); err != nil {
		return err
	}

	// MIGRATION 19: Rename idx_jobs_parent_id to idx_crawl_jobs_parent_id
	if err := s.migrateRenameParentIdIndex(); err != nil {
		return err
	}

	// MIGRATION 20: Add error_tolerance column to job_definitions table
	if err := s.migrateAddErrorToleranceColumn(); err != nil {
		return err
	}

	// MIGRATION 21: Add metadata column to crawl_jobs table if it doesn't exist
	if err := s.migrateAddMetadataColumn(); err != nil {
		return err
	}

	// MIGRATION 22: Cleanup orphaned orchestration jobs
	// Removes jobs created by old ExecuteJobDefinitionHandler pattern that have been replaced
	if err := s.migrateCleanupOrphanedOrchestrationJobs(); err != nil {
		return err
	}

	// MIGRATION 23: Cleanup orphaned job_settings entries
	// Removes job_settings rows whose job names no longer exist in job_definitions table
	if err := s.migrateCleanupOrphanedJobSettings(); err != nil {
		return err
	}

	// MIGRATION 24: Add finished_at column to crawl_jobs table
	// Tracks when job AND all spawned children complete or timeout
	if err := s.migrateAddFinishedAtColumn(); err != nil {
		return err
	}

	// MIGRATION 25: Migrate from crawl_jobs to unified jobs table
	// This is a BREAKING CHANGE that consolidates the schema to use executor-agnostic JobModel
	// Removes crawler-specific columns and uses flexible config_json for all job types
	if err := s.migrateCrawlJobsToUnifiedJobs(); err != nil {
		return err
	}

	return nil
}

// migrateAddJobDefinitionsTimeoutColumn adds timeout column to job_definitions table
func (s *SQLiteDB) migrateAddJobDefinitionsTimeoutColumn() error {
	columnsQuery := `PRAGMA table_info(job_definitions)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasTimeout := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "timeout" {
			hasTimeout = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasTimeout {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding timeout column to job_definitions")

	// Add the timeout column
	if _, err := s.db.Exec(`ALTER TABLE job_definitions ADD COLUMN timeout TEXT`); err != nil {
		return err
	}

	// Backfill existing rows with empty string
	s.logger.Info().Msg("Backfilling existing rows with empty timeout")
	if _, err := s.db.Exec(`UPDATE job_definitions SET timeout = '' WHERE timeout IS NULL`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: timeout column added successfully")
	return nil
}

// migrateCrawlJobsColumns adds missing columns to crawl_jobs table
// NOTE: This migration is deprecated - crawl_jobs table has been replaced by jobs table
// Kept for backward compatibility with existing databases
func (s *SQLiteDB) migrateCrawlJobsColumns() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		// Table doesn't exist - skip migration (new unified jobs table is used instead)
		s.logger.Debug().Msg("Skipping crawl_jobs migration - table does not exist (using unified jobs table)")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasSourceConfigSnapshot := false
	hasAuthSnapshot := false
	hasRefreshSource := false
	hasSeedURLs := false

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		switch name {
		case "source_config_snapshot":
			hasSourceConfigSnapshot = true
		case "auth_snapshot":
			hasAuthSnapshot = true
		case "refresh_source":
			hasRefreshSource = true
		case "seed_urls":
			hasSeedURLs = true
		}
	}

	// Add missing columns
	if !hasSourceConfigSnapshot {
		s.logger.Info().Msg("Running migration: Adding source_config_snapshot column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN source_config_snapshot TEXT`); err != nil {
			return err
		}
	}

	if !hasAuthSnapshot {
		s.logger.Info().Msg("Running migration: Adding auth_snapshot column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN auth_snapshot TEXT`); err != nil {
			return err
		}
	}

	if !hasRefreshSource {
		s.logger.Info().Msg("Running migration: Adding refresh_source column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN refresh_source INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}

	if !hasSeedURLs {
		s.logger.Info().Msg("Running migration: Adding seed_urls column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN seed_urls TEXT`); err != nil {
			return err
		}
	}

	return nil
}

// migrateToMarkdownOnly migrates documents table from dual content/content_markdown to markdown-only
func (s *SQLiteDB) migrateToMarkdownOnly() error {
	// Check if content column exists in documents table
	columnsQuery := `PRAGMA table_info(documents)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasContentColumn := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "content" {
			hasContentColumn = true
			break
		}
	}

	// If content column doesn't exist, migration already completed
	if !hasContentColumn {
		return nil
	}

	s.logger.Info().Msg("Running migration: Migrating documents table to markdown-only storage")

	// Step 1: Copy content to content_markdown where content_markdown is NULL or empty
	s.logger.Info().Msg("Step 1: Copying content to content_markdown where needed")
	_, err = s.db.Exec(`
		UPDATE documents
		SET content_markdown = content
		WHERE content_markdown IS NULL OR content_markdown = ''
	`)
	if err != nil {
		return err
	}

	// Step 2: Drop and recreate FTS5 table with new schema
	s.logger.Info().Msg("Step 2: Recreating FTS5 table with content_markdown")
	_, err = s.db.Exec(`DROP TABLE IF EXISTS documents_fts`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE documents_fts USING fts5(
			title,
			content_markdown,
			content=documents,
			content_rowid=rowid
		)
	`)
	if err != nil {
		return err
	}

	// Step 3: Create new documents table without content column
	s.logger.Info().Msg("Step 3: Creating new documents table without content column")
	_, err = s.db.Exec(`
		CREATE TABLE documents_new (
			id TEXT PRIMARY KEY,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content_markdown TEXT,
			detail_level TEXT DEFAULT 'full',
			metadata TEXT,
			url TEXT,
			embedding BLOB,
			embedding_model TEXT,
			last_synced INTEGER,
			source_version TEXT,
			force_sync_pending INTEGER DEFAULT 0,
			force_embed_pending INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Step 4: Copy all data to new table (excluding content column)
	s.logger.Info().Msg("Step 4: Copying data to new table")
	_, err = s.db.Exec(`
		INSERT INTO documents_new
		SELECT
			id, source_type, source_id, title, content_markdown, detail_level,
			metadata, url, embedding, embedding_model, last_synced, source_version,
			force_sync_pending, force_embed_pending, created_at, updated_at
		FROM documents
	`)
	if err != nil {
		return err
	}

	// Step 5: Drop old table and rename new table
	s.logger.Info().Msg("Step 5: Replacing old table with new table")
	_, err = s.db.Exec(`DROP TABLE documents`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`ALTER TABLE documents_new RENAME TO documents`)
	if err != nil {
		return err
	}

	// Step 6: Recreate indexes
	s.logger.Info().Msg("Step 6: Recreating indexes")
	_, err = s.db.Exec(`CREATE UNIQUE INDEX idx_documents_source ON documents(source_type, source_id)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX idx_documents_sync ON documents(force_sync_pending, force_embed_pending)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX idx_documents_embedding ON documents(embedding_model) WHERE embedding IS NOT NULL`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX idx_documents_detail_level ON documents(detail_level, source_type)`)
	if err != nil {
		return err
	}

	// Step 7: Recreate FTS5 triggers
	s.logger.Info().Msg("Step 7: Recreating FTS5 triggers")

	// Drop existing triggers first to avoid "trigger already exists" errors
	_, err = s.db.Exec(`DROP TRIGGER IF EXISTS documents_fts_insert`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`DROP TRIGGER IF EXISTS documents_fts_update`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`DROP TRIGGER IF EXISTS documents_fts_delete`)
	if err != nil {
		return err
	}

	// Create new triggers
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS documents_fts_insert AFTER INSERT ON documents BEGIN
			INSERT INTO documents_fts(rowid, title, content_markdown)
			VALUES (new.rowid, new.title, new.content_markdown);
		END
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS documents_fts_update AFTER UPDATE ON documents BEGIN
			UPDATE documents_fts
			SET title = new.title, content_markdown = new.content_markdown
			WHERE rowid = new.rowid;
		END
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS documents_fts_delete AFTER DELETE ON documents BEGIN
			DELETE FROM documents_fts WHERE rowid = old.rowid;
		END
	`)
	if err != nil {
		return err
	}

	// Step 8: Rebuild FTS5 index with existing data
	s.logger.Info().Msg("Step 8: Rebuilding FTS5 index")
	_, err = s.db.Exec(`INSERT INTO documents_fts(documents_fts) VALUES('rebuild')`)
	if err != nil {
		return err
	}

	s.logger.Info().Msg("Migration to markdown-only storage completed successfully")
	return nil
}

// migrateAddHeartbeatColumn adds last_heartbeat column to crawl_jobs table
// NOTE: This migration is deprecated - crawl_jobs table has been replaced by jobs table
// Kept for backward compatibility with existing databases
func (s *SQLiteDB) migrateAddHeartbeatColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		// Table doesn't exist - skip migration (new unified jobs table is used instead)
		s.logger.Debug().Msg("Skipping heartbeat migration - crawl_jobs table does not exist (using unified jobs table)")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasLastHeartbeat := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "last_heartbeat" {
			hasLastHeartbeat = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasLastHeartbeat {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding last_heartbeat column to crawl_jobs")

	// Add the last_heartbeat column
	if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN last_heartbeat INTEGER`); err != nil {
		return err
	}

	// Set default value to created_at for existing rows
	s.logger.Info().Msg("Setting default last_heartbeat values for existing jobs")
	if _, err := s.db.Exec(`UPDATE crawl_jobs SET last_heartbeat = created_at WHERE last_heartbeat IS NULL`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: last_heartbeat column added successfully")
	return nil
}

// migrateAddLastRunColumn adds last_run column to job_settings table
func (s *SQLiteDB) migrateAddLastRunColumn() error {
	columnsQuery := `PRAGMA table_info(job_settings)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasLastRun := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "last_run" {
			hasLastRun = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasLastRun {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding last_run column to job_settings")

	// Add the last_run column
	if _, err := s.db.Exec(`ALTER TABLE job_settings ADD COLUMN last_run INTEGER`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: last_run column added successfully")
	return nil
}

// migrateAddJobLogsColumn is deprecated and does nothing
// DEPRECATED: This migration has been superseded by MIGRATION 13 (migrateRemoveLogsColumn)
// which removes the logs column entirely in favor of the dedicated job_logs table.
// This function is kept for API compatibility but always returns nil without doing anything.
func (s *SQLiteDB) migrateAddJobLogsColumn() error {
	// Migration deprecated - logs column replaced by job_logs table
	// Do nothing to avoid conflict with MIGRATION 13
	return nil
}

// migrateAddJobNameDescriptionColumns adds name and description columns to crawl_jobs table
// NOTE: This migration is deprecated - crawl_jobs table has been replaced by jobs table
func (s *SQLiteDB) migrateAddJobNameDescriptionColumns() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping job name/description migration - crawl_jobs table does not exist")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasName := false
	hasDescription := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		switch name {
		case "name":
			hasName = true
		case "description":
			hasDescription = true
		}
	}

	// Add missing columns
	if !hasName {
		s.logger.Info().Msg("Running migration: Adding name column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN name TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	if !hasDescription {
		s.logger.Info().Msg("Running migration: Adding description column to crawl_jobs")
		if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN description TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	s.logger.Info().Msg("Migration: name and description columns added successfully")
	return nil
}

// migrateAddJobSettingsDescriptionColumn adds description column to job_settings table
func (s *SQLiteDB) migrateAddJobSettingsDescriptionColumn() error {
	columnsQuery := `PRAGMA table_info(job_settings)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasDescription := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "description" {
			hasDescription = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasDescription {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding description column to job_settings")

	// Add the description column
	if _, err := s.db.Exec(`ALTER TABLE job_settings ADD COLUMN description TEXT DEFAULT ''`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: description column added successfully")
	return nil
}

// migrateAddJobDefinitionsTable adds job_definitions table if it doesn't exist
func (s *SQLiteDB) migrateAddJobDefinitionsTable() error {
	// Check if table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='job_definitions'`).Scan(&tableName)
	if err == nil {
		// Table already exists
		return nil
	}

	s.logger.Info().Msg("Running migration: Creating job_definitions table")

	// Create the table
	_, err = s.db.Exec(`
		CREATE TABLE job_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			sources TEXT NOT NULL,
			steps TEXT NOT NULL,
			schedule TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			auto_start INTEGER DEFAULT 0,
			config TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes
	s.logger.Info().Msg("Creating indexes for job_definitions table")

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_job_definitions_type ON job_definitions(type, enabled)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_job_definitions_enabled ON job_definitions(enabled, created_at DESC)`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_job_definitions_schedule ON job_definitions(schedule)`)
	if err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: job_definitions table and indexes created successfully")
	return nil
}

// NOTE: migrateAddSourcesSeedURLsColumn is deprecated - seed URLs are now job-level configuration
// This function is kept for historical reference but is no longer called
/*
func (s *SQLiteDB) migrateAddSourcesSeedURLsColumn() error {
	// This migration has been superseded by migrateRemoveSourcesFilteringColumns
	// which removes both filters and seed_urls columns
	return nil
}
*/

// migrateRemoveSourcesFilteringColumns removes ONLY the seed_urls column from sources table
// IMPORTANT: This migration preserves the filters column to prevent data loss
func (s *SQLiteDB) migrateRemoveSourcesFilteringColumns() error {
	// Check if seed_urls column exists (only column we want to remove)
	columnsQuery := `PRAGMA table_info(sources)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasFilters := false
	hasSeedURLs := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "filters" {
			hasFilters = true
		}
		if name == "seed_urls" {
			hasSeedURLs = true
		}
	}

	// If seed_urls column doesn't exist, migration already completed
	if !hasSeedURLs {
		return nil
	}

	s.logger.Info().Msg("Running migration: Removing seed_urls column from sources table (preserving filters)")

	// Begin transaction
	s.logger.Info().Msg("Beginning migration transaction")
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			s.logger.Warn().Msg("Rolling back migration transaction due to error")
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("Failed to rollback transaction")
			}
		}
	}()

	// Step 1: Create new sources table without seed_urls but WITH filters (if it existed)
	s.logger.Info().Msg("Step 1: Creating new sources table without seed_urls column")
	createTableSQL := `
		CREATE TABLE sources_new (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			base_url TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			auth_id TEXT,
			crawl_config TEXT NOT NULL,`

	// Include filters column if it existed in original table
	if hasFilters {
		createTableSQL += `
			filters TEXT,`
		s.logger.Info().Msg("Including filters column in new table schema")
	}

	createTableSQL += `
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL
		)
	`

	_, err = tx.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create new sources table: %w", err)
	}

	// Step 2: Copy data from old table to new table (excluding only seed_urls)
	s.logger.Info().Msg("Step 2: Copying data to new table")
	var copyDataSQL string
	if hasFilters {
		// Include filters column in copy
		copyDataSQL = `
			INSERT INTO sources_new
			SELECT id, name, type, base_url, enabled, auth_id, crawl_config, filters, created_at, updated_at
			FROM sources
		`
		s.logger.Info().Msg("Copying data including filters column")
	} else {
		// No filters column to copy
		copyDataSQL = `
			INSERT INTO sources_new
			SELECT id, name, type, base_url, enabled, auth_id, crawl_config, created_at, updated_at
			FROM sources
		`
	}

	_, err = tx.Exec(copyDataSQL)
	if err != nil {
		return fmt.Errorf("failed to copy data to new table: %w", err)
	}

	// Step 3: Drop old table
	s.logger.Info().Msg("Step 3: Dropping old sources table")
	_, err = tx.Exec(`DROP TABLE sources`)
	if err != nil {
		return fmt.Errorf("failed to drop old sources table: %w", err)
	}

	// Step 4: Rename new table to sources
	s.logger.Info().Msg("Step 4: Renaming sources_new to sources")
	_, err = tx.Exec(`ALTER TABLE sources_new RENAME TO sources`)
	if err != nil {
		return fmt.Errorf("failed to rename sources_new to sources: %w", err)
	}

	// Step 5: Recreate indexes
	s.logger.Info().Msg("Step 5: Recreating indexes")
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_sources_type ON sources(type, enabled)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_sources_type: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_sources_enabled: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_sources_auth ON sources(auth_id)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_sources_auth: %w", err)
	}

	// Commit transaction
	s.logger.Info().Msg("Committing migration transaction")
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if hasFilters {
		s.logger.Info().Msg("Migration: seed_urls column removed successfully (filters column preserved)")
	} else {
		s.logger.Info().Msg("Migration: seed_urls column removed successfully")
	}
	return nil
}

// migrateAddBackSourcesFiltersColumn adds back the filters column to sources table
func (s *SQLiteDB) migrateAddBackSourcesFiltersColumn() error {
	columnsQuery := `PRAGMA table_info(sources)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasFilters := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "filters" {
			hasFilters = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasFilters {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding back filters column to sources table")

	// Add the filters column
	if _, err := s.db.Exec(`ALTER TABLE sources ADD COLUMN filters TEXT`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: filters column added back successfully")
	return nil
}

// migrateRemoveLogsColumn removes the deprecated logs column from crawl_jobs table.
// The logs column stored job logs as a JSON array with a 100-entry limit.
// Logs are now stored in the dedicated job_logs table (see lines 145-158) which provides:
// - Unlimited log history (no truncation)
// - Better query performance with indexes
// - Automatic CASCADE DELETE when jobs are deleted
// - Batched writes via LogService for efficiency
// NOTE: This migration is deprecated - crawl_jobs table has been replaced by jobs table
func (s *SQLiteDB) migrateRemoveLogsColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping logs column removal migration - crawl_jobs table does not exist")
		return nil
	}

	// Check if logs column exists
	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasLogs := false
	hasParentID := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "logs" {
			hasLogs = true
		}
		if name == "parent_id" {
			hasParentID = true
		}
	}

	// If logs column doesn't exist, migration already completed
	if !hasLogs {
		return nil
	}

	s.logger.Info().Msg("Running migration: Removing deprecated logs column from crawl_jobs table")

	// Begin transaction
	s.logger.Info().Msg("Beginning migration transaction")
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			s.logger.Warn().Msg("Rolling back migration transaction due to error")
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("Failed to rollback transaction")
			}
		}
	}()

	// Step 1: Create new crawl_jobs table without logs column
	if hasParentID {
		s.logger.Info().Msg("Step 1: Creating new crawl_jobs table without logs column (with parent_id)")
		_, err = tx.Exec(`
			CREATE TABLE crawl_jobs_new (
				id TEXT PRIMARY KEY,
				parent_id TEXT,
				name TEXT DEFAULT '',
				description TEXT DEFAULT '',
				source_type TEXT NOT NULL,
				entity_type TEXT NOT NULL,
				config_json TEXT NOT NULL,
				source_config_snapshot TEXT,
				auth_snapshot TEXT,
				refresh_source INTEGER DEFAULT 0,
				seed_urls TEXT,
				status TEXT NOT NULL,
				progress_json TEXT,
				created_at INTEGER NOT NULL,
				started_at INTEGER,
				completed_at INTEGER,
				last_heartbeat INTEGER,
				error TEXT,
				result_count INTEGER DEFAULT 0,
				failed_count INTEGER DEFAULT 0
			)
		`)
	} else {
		s.logger.Info().Msg("Step 1: Creating new crawl_jobs table without logs column (without parent_id)")
		_, err = tx.Exec(`
			CREATE TABLE crawl_jobs_new (
				id TEXT PRIMARY KEY,
				name TEXT DEFAULT '',
				description TEXT DEFAULT '',
				source_type TEXT NOT NULL,
				entity_type TEXT NOT NULL,
				config_json TEXT NOT NULL,
				source_config_snapshot TEXT,
				auth_snapshot TEXT,
				refresh_source INTEGER DEFAULT 0,
				seed_urls TEXT,
				status TEXT NOT NULL,
				progress_json TEXT,
				created_at INTEGER NOT NULL,
				started_at INTEGER,
				completed_at INTEGER,
				last_heartbeat INTEGER,
				error TEXT,
				result_count INTEGER DEFAULT 0,
				failed_count INTEGER DEFAULT 0
			)
		`)
	}
	if err != nil {
		return fmt.Errorf("failed to create new crawl_jobs table: %w", err)
	}

	// Step 2: Copy all data to new table (excluding logs column)
	s.logger.Info().Msg("Step 2: Copying data to new table")
	if hasParentID {
		_, err = tx.Exec(`
			INSERT INTO crawl_jobs_new
			SELECT id, parent_id, name, description, source_type, entity_type, config_json,
				   source_config_snapshot, auth_snapshot, refresh_source, seed_urls,
				   status, progress_json, created_at, started_at, completed_at,
				   last_heartbeat, error, result_count, failed_count
			FROM crawl_jobs
		`)
	} else {
		_, err = tx.Exec(`
			INSERT INTO crawl_jobs_new
			SELECT id, name, description, source_type, entity_type, config_json,
				   source_config_snapshot, auth_snapshot, refresh_source, seed_urls,
				   status, progress_json, created_at, started_at, completed_at,
				   last_heartbeat, error, result_count, failed_count
			FROM crawl_jobs
		`)
	}
	if err != nil {
		return fmt.Errorf("failed to copy data to new table: %w", err)
	}

	// Step 3: Drop old table
	s.logger.Info().Msg("Step 3: Dropping old crawl_jobs table")
	_, err = tx.Exec(`DROP TABLE crawl_jobs`)
	if err != nil {
		return fmt.Errorf("failed to drop old crawl_jobs table: %w", err)
	}

	// Step 4: Rename new table to crawl_jobs
	s.logger.Info().Msg("Step 4: Renaming crawl_jobs_new to crawl_jobs")
	_, err = tx.Exec(`ALTER TABLE crawl_jobs_new RENAME TO crawl_jobs`)
	if err != nil {
		return fmt.Errorf("failed to rename crawl_jobs_new to crawl_jobs: %w", err)
	}

	// Step 5: Recreate indexes
	s.logger.Info().Msg("Step 5: Recreating indexes")
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_status ON crawl_jobs(status, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_status: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_source ON crawl_jobs(source_type, entity_type, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_source: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_created ON crawl_jobs(created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_created: %w", err)
	}

	if hasParentID {
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_parent_id ON crawl_jobs(parent_id, created_at DESC)`)
		if err != nil {
			return fmt.Errorf("failed to create idx_jobs_parent_id: %w", err)
		}
	}

	// Commit transaction
	s.logger.Info().Msg("Committing migration transaction")
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Msg("Migration: logs column removed successfully (logs now in job_logs table)")
	return nil
}

// migrateAddPostJobsColumn adds post_jobs column to job_definitions table
func (s *SQLiteDB) migrateAddPostJobsColumn() error {
	columnsQuery := `PRAGMA table_info(job_definitions)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasPostJobs := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "post_jobs" {
			hasPostJobs = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasPostJobs {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding post_jobs column to job_definitions")

	// Add the post_jobs column
	if _, err := s.db.Exec(`ALTER TABLE job_definitions ADD COLUMN post_jobs TEXT`); err != nil {
		return err
	}

	// Backfill existing rows with empty JSON array
	s.logger.Info().Msg("Backfilling existing rows with empty post_jobs array")
	if _, err := s.db.Exec(`UPDATE job_definitions SET post_jobs = '[]' WHERE post_jobs IS NULL`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: post_jobs column added successfully")
	return nil
}

// migrateAddParentIdColumn adds parent_id column to crawl_jobs table
func (s *SQLiteDB) migrateAddParentIdColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping parent_id migration - crawl_jobs table does not exist")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasParentID := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "parent_id" {
			hasParentID = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasParentID {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding parent_id column to crawl_jobs")

	// Add the parent_id column
	if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN parent_id TEXT`); err != nil {
		return err
	}

	// Create the index
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_parent_id ON crawl_jobs(parent_id, created_at DESC)`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: parent_id column and index added successfully")
	return nil
}

// migrateEnableForeignKeysAndAddParentConstraint enables foreign key enforcement
// and adds CASCADE constraint to parent_id column in crawl_jobs table
func (s *SQLiteDB) migrateEnableForeignKeysAndAddParentConstraint() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping foreign key migration - crawl_jobs table does not exist")
		return nil
	}

	// Check if foreign key constraint already exists on parent_id
	fkQuery := `PRAGMA foreign_key_list(crawl_jobs)`
	rows, err := s.db.Query(fkQuery)
	if err != nil {
		return fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	hasParentIDConstraint := false
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return fmt.Errorf("failed to scan foreign key: %w", err)
		}
		if from == "parent_id" && onDelete == "CASCADE" {
			hasParentIDConstraint = true
			s.logger.Info().
				Str("from", from).
				Str("on_delete", onDelete).
				Msg("Foreign key constraint on parent_id already exists, migration skipped")
			break
		}
	}

	// If constraint already exists, migration already completed
	if hasParentIDConstraint {
		return nil
	}

	s.logger.Info().Msg("Running migration: Enable foreign keys and add parent_id CASCADE constraint")

	// Begin transaction
	s.logger.Info().Msg("Beginning migration transaction")
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			s.logger.Warn().Msg("Rolling back migration transaction due to error")
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("Failed to rollback transaction")
			}
		}
	}()

	// Step 1: Clean up orphaned child jobs (children without parents)
	s.logger.Info().Msg("Step 1: Cleaning up orphaned child jobs")
	result, err := tx.Exec(`
		DELETE FROM crawl_jobs
		WHERE parent_id IS NOT NULL
		  AND parent_id != ''
		  AND parent_id NOT IN (SELECT id FROM crawl_jobs)
	`)
	if err != nil {
		return fmt.Errorf("failed to clean up orphaned jobs: %w", err)
	}

	orphanedCount, _ := result.RowsAffected()
	s.logger.Info().Int64("orphaned_jobs_deleted", orphanedCount).Msg("Orphaned child jobs cleaned up")

	// Step 2: Create new crawl_jobs table with foreign key constraint
	s.logger.Info().Msg("Step 2: Creating new crawl_jobs table with foreign key constraint")
	_, err = tx.Exec(`
		CREATE TABLE crawl_jobs_new (
			id TEXT PRIMARY KEY,
			parent_id TEXT,
			name TEXT DEFAULT '',
			description TEXT DEFAULT '',
			source_type TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			config_json TEXT NOT NULL,
			source_config_snapshot TEXT,
			auth_snapshot TEXT,
			refresh_source INTEGER DEFAULT 0,
			seed_urls TEXT,
			status TEXT NOT NULL,
			progress_json TEXT,
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			completed_at INTEGER,
			last_heartbeat INTEGER,
			error TEXT,
			result_count INTEGER DEFAULT 0,
			failed_count INTEGER DEFAULT 0,
			FOREIGN KEY (parent_id) REFERENCES crawl_jobs_new(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new crawl_jobs table: %w", err)
	}

	// Step 3: Copy all data from old table to new table
	s.logger.Info().Msg("Step 3: Copying data to new table")
	_, err = tx.Exec(`
		INSERT INTO crawl_jobs_new
		SELECT id, parent_id, name, description, source_type, entity_type, config_json,
			   source_config_snapshot, auth_snapshot, refresh_source, seed_urls,
			   status, progress_json, created_at, started_at, completed_at,
			   last_heartbeat, error, result_count, failed_count
		FROM crawl_jobs
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data to new table: %w", err)
	}

	// Step 4: Drop old table
	s.logger.Info().Msg("Step 4: Dropping old crawl_jobs table")
	_, err = tx.Exec(`DROP TABLE crawl_jobs`)
	if err != nil {
		return fmt.Errorf("failed to drop old crawl_jobs table: %w", err)
	}

	// Step 5: Rename new table to crawl_jobs
	s.logger.Info().Msg("Step 5: Renaming crawl_jobs_new to crawl_jobs")
	_, err = tx.Exec(`ALTER TABLE crawl_jobs_new RENAME TO crawl_jobs`)
	if err != nil {
		return fmt.Errorf("failed to rename crawl_jobs_new to crawl_jobs: %w", err)
	}

	// Step 6: Recreate all indexes
	s.logger.Info().Msg("Step 6: Recreating indexes")
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_status ON crawl_jobs(status, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_status: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_source ON crawl_jobs(source_type, entity_type, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_source: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_created ON crawl_jobs(created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_created: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_parent_id ON crawl_jobs(parent_id, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_parent_id: %w", err)
	}

	// Commit transaction
	s.logger.Info().Msg("Committing migration transaction")
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Step 7: Verify foreign key constraint was added
	s.logger.Info().Msg("Step 7: Verifying foreign key constraint")
	verifyRows, err := s.db.Query(`PRAGMA foreign_key_list(crawl_jobs)`)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Failed to verify foreign key constraint")
	} else {
		defer verifyRows.Close()
		constraintFound := false
		for verifyRows.Next() {
			var id, seq int
			var table, from, to, onUpdate, onDelete, match string
			if err := verifyRows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err == nil {
				if from == "parent_id" {
					constraintFound = true
					s.logger.Info().
						Str("from", from).
						Str("to", to).
						Str("on_delete", onDelete).
						Msg("Foreign key constraint verified")
				}
			}
		}
		if !constraintFound {
			s.logger.Warn().Msg("Foreign key constraint not found after migration")
		}
	}

	s.logger.Info().Msg("Migration: Foreign key constraint on parent_id added successfully")
	return nil
}

// migrateAddPreJobsColumn adds pre_jobs column to job_definitions table
func (s *SQLiteDB) migrateAddPreJobsColumn() error {
	columnsQuery := `PRAGMA table_info(job_definitions)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasPreJobs := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "pre_jobs" {
			hasPreJobs = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasPreJobs {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding pre_jobs column to job_definitions")

	// Add the pre_jobs column
	if _, err := s.db.Exec(`ALTER TABLE job_definitions ADD COLUMN pre_jobs TEXT`); err != nil {
		return err
	}

	// Backfill existing rows with empty JSON array
	s.logger.Info().Msg("Backfilling existing rows with empty pre_jobs array")
	if _, err := s.db.Exec(`UPDATE job_definitions SET pre_jobs = '[]' WHERE pre_jobs IS NULL`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: pre_jobs column added successfully")
	return nil
}

// migrateAddJobTypeColumn adds job_type column to crawl_jobs table
func (s *SQLiteDB) migrateAddJobTypeColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping job_type migration - crawl_jobs table does not exist")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasJobType := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "job_type" {
			hasJobType = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasJobType {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding job_type column to crawl_jobs")

	// Add the job_type column
	if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN job_type TEXT DEFAULT 'parent'`); err != nil {
		return err
	}

	// Backfill existing rows
	s.logger.Info().Msg("Backfilling existing rows with default job_type")
	if _, err := s.db.Exec(`UPDATE crawl_jobs SET job_type = 'parent' WHERE job_type IS NULL`); err != nil {
		return err
	}

	// Create the index
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_crawl_jobs_type_status ON crawl_jobs(job_type, status, created_at DESC)`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: job_type column added successfully")
	return nil
}

// migrateRenameParentIdIndex renames idx_jobs_parent_id to idx_crawl_jobs_parent_id
func (s *SQLiteDB) migrateRenameParentIdIndex() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping index rename migration - crawl_jobs table does not exist")
		return nil
	}

	// Check if old index exists
	indexQuery := `SELECT name FROM sqlite_master WHERE type='index' AND name='idx_jobs_parent_id'`
	var indexName string
	err = s.db.QueryRow(indexQuery).Scan(&indexName)
	if err != nil {
		// Index doesn't exist, migration already completed or not needed
		return nil
	}

	s.logger.Info().Msg("Running migration: Renaming idx_jobs_parent_id to idx_crawl_jobs_parent_id")

	// Drop the old index
	if _, err := s.db.Exec(`DROP INDEX IF EXISTS idx_jobs_parent_id`); err != nil {
		return err
	}

	// Create the new index with correct name
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_crawl_jobs_parent_id ON crawl_jobs(parent_id, created_at DESC)`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: idx_jobs_parent_id renamed to idx_crawl_jobs_parent_id successfully")
	return nil
}

// migrateAddErrorToleranceColumn adds error_tolerance column to job_definitions table
func (s *SQLiteDB) migrateAddErrorToleranceColumn() error {
	columnsQuery := `PRAGMA table_info(job_definitions)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasErrorTolerance := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "error_tolerance" {
			hasErrorTolerance = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasErrorTolerance {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding error_tolerance column to job_definitions")

	// Add the error_tolerance column
	if _, err := s.db.Exec(`ALTER TABLE job_definitions ADD COLUMN error_tolerance TEXT`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: error_tolerance column added successfully")
	return nil
}

// migrateAddMetadataColumn adds metadata column to crawl_jobs table if it doesn't exist
func (s *SQLiteDB) migrateAddMetadataColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping metadata migration - crawl_jobs table does not exist")
		return nil
	}

	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasMetadata := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "metadata" {
			hasMetadata = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasMetadata {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding metadata column to crawl_jobs")

	// Add the metadata column
	if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN metadata TEXT`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: metadata column added successfully")
	return nil
}

// migrateCleanupOrphanedOrchestrationJobs removes orphaned orchestration jobs
// from the old dual-job creation pattern where ExecuteJobDefinitionHandler
// created a parent job before JobExecutor created the actual crawler job.
// After the refactoring, ExecuteJobDefinitionHandler no longer creates jobs,
// so any remaining orchestration wrapper jobs can be safely cleaned up.
// This migration uses narrow criteria to avoid deleting legitimate jobs.
func (s *SQLiteDB) migrateCleanupOrphanedOrchestrationJobs() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping orphaned jobs cleanup - crawl_jobs table does not exist")
		return nil
	}

	s.logger.Info().Msg("Running migration: Cleaning up orphaned orchestration jobs")

	// First, verify required columns exist before attempting deletion
	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Could not verify crawl_jobs schema (non-critical)")
		return nil
	}

	hasMetadata := false
	hasJobType := false
	hasSourceType := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			rows.Close()
			s.logger.Warn().Err(err).Msg("Could not scan column info (non-critical)")
			return nil
		}
		if name == "metadata" {
			hasMetadata = true
		}
		if name == "job_type" {
			hasJobType = true
		}
		if name == "source_type" {
			hasSourceType = true
		}
	}
	rows.Close()

	// If required columns don't exist, skip migration
	if !hasMetadata || !hasJobType || !hasSourceType {
		s.logger.Debug().
			Bool("has_metadata", hasMetadata).
			Bool("has_job_type", hasJobType).
			Bool("has_source_type", hasSourceType).
			Msg("Required columns not present, skipping orphaned job cleanup")
		return nil
	}

	// Query for orphaned orchestration wrapper jobs using narrow criteria:
	// 1. job_type='parent' (orchestration wrapper type)
	// 2. source_type is empty or NULL (orchestration wrappers don't have actual source data)
	// 3. No job_definition_id in metadata (orphaned from old pattern)
	// 4. Has no child jobs (safe to delete)
	orphanedJobsQuery := `
		SELECT id FROM crawl_jobs
		WHERE job_type = 'parent'
		  AND (source_type = '' OR source_type IS NULL)
		  AND (metadata IS NULL
		       OR json_extract(metadata, '$.job_definition_id') IS NULL
		       OR json_extract(metadata, '$.job_definition_id') = '')
		  AND (SELECT COUNT(1) FROM crawl_jobs c WHERE c.parent_id = crawl_jobs.id) = 0
		LIMIT 1000
	`

	orphanedRows, err := s.db.Query(orphanedJobsQuery)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Could not query orphaned jobs (non-critical)")
		return nil
	}
	defer orphanedRows.Close()

	var orphanedIDs []string
	for orphanedRows.Next() {
		var id string
		if err := orphanedRows.Scan(&id); err != nil {
			s.logger.Warn().Err(err).Msg("Could not scan orphaned job ID")
			continue
		}
		orphanedIDs = append(orphanedIDs, id)
	}

	if len(orphanedIDs) == 0 {
		s.logger.Debug().Msg("No orphaned orchestration jobs found")
		return nil
	}

	s.logger.Info().
		Int("orphaned_count", len(orphanedIDs)).
		Strs("orphaned_ids", orphanedIDs).
		Msg("Found orphaned orchestration jobs to delete")

	// Delete the identified orphaned jobs
	result, err := s.db.Exec(`
		DELETE FROM crawl_jobs
		WHERE id IN (
			SELECT id FROM crawl_jobs
			WHERE job_type = 'parent'
			  AND (source_type = '' OR source_type IS NULL)
			  AND (metadata IS NULL
			       OR json_extract(metadata, '$.job_definition_id') IS NULL
			       OR json_extract(metadata, '$.job_definition_id') = '')
			  AND (SELECT COUNT(1) FROM crawl_jobs c WHERE c.parent_id = crawl_jobs.id) = 0
			LIMIT 1000
		)
	`)
	if err != nil {
		s.logger.Warn().Err(err).Msg("Could not delete orphaned jobs (non-critical)")
		return nil
	}

	orphanedCount, _ := result.RowsAffected()
	if orphanedCount > 0 {
		s.logger.Info().
			Int64("orphaned_jobs_deleted", orphanedCount).
			Strs("deleted_ids", orphanedIDs).
			Msg("Orphaned orchestration jobs cleaned up successfully")
	}

	return nil
}

// migrateCleanupOrphanedJobSettings deletes orphaned job_settings entries
// for job names that no longer exist in the job_definitions table.
// This migration ensures that startup warnings about missing job definitions
// don't occur when job definitions have been deleted but their settings remain.
func (s *SQLiteDB) migrateCleanupOrphanedJobSettings() error {
	// Check if the job_settings table exists
	tableQuery := `SELECT name FROM sqlite_master WHERE type='table' AND name='job_settings'`
	var tableName string
	err := s.db.QueryRow(tableQuery).Scan(&tableName)
	if err != nil {
		// Table doesn't exist, migration already completed
		return nil
	}

	s.logger.Info().Msg("Running migration: Cleaning up orphaned job_settings entries")

	// Check if job_definitions table exists
	tableQuery = `SELECT name FROM sqlite_master WHERE type='table' AND name='job_definitions'`
	err = s.db.QueryRow(tableQuery).Scan(&tableName)
	if err != nil {
		// job_definitions table doesn't exist, skip migration
		s.logger.Debug().Msg("job_definitions table not found, skipping orphaned job_settings cleanup")
		return nil
	}

	// Delete orphaned job_settings where job_name doesn't match any job_definitions.id
	result, err := s.db.Exec(`
		DELETE FROM job_settings
		WHERE job_name NOT IN (SELECT id FROM job_definitions)
	`)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned job settings: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	if deletedCount > 0 {
		s.logger.Info().
			Int64("deleted_count", deletedCount).
			Msg("Migration: Cleaned up orphaned job_settings entries")
	} else {
		s.logger.Info().Msg("Migration: No orphaned job_settings entries found")
	}

	return nil
}

// migrateAddFinishedAtColumn adds finished_at column to crawl_jobs table
// This column tracks when a job AND all its spawned children complete or timeout
func (s *SQLiteDB) migrateAddFinishedAtColumn() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err != nil {
		s.logger.Debug().Msg("Skipping finished_at migration - crawl_jobs table does not exist")
		return nil
	}

	// Check if column already exists
	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return fmt.Errorf("failed to query crawl_jobs columns: %w", err)
	}
	defer rows.Close()

	hasFinishedAt := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		if name == "finished_at" {
			hasFinishedAt = true
			break
		}
	}

	if hasFinishedAt {
		s.logger.Debug().Msg("Migration: finished_at column already exists in crawl_jobs")
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding finished_at column to crawl_jobs table")

	// Add the column
	_, err = s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN finished_at INTEGER`)
	if err != nil {
		return fmt.Errorf("failed to add finished_at column: %w", err)
	}

	s.logger.Info().Msg("Migration: Successfully added finished_at column to crawl_jobs")
	return nil
}

// migrateCrawlJobsToUnifiedJobs migrates from crawl_jobs table to unified jobs table
// This is a BREAKING CHANGE that consolidates the schema to use executor-agnostic JobModel
// Removes crawler-specific columns (source_type, entity_type, source_config_snapshot, auth_snapshot, refresh_source, seed_urls)
// and uses flexible config_json and metadata_json for all job types
func (s *SQLiteDB) migrateCrawlJobsToUnifiedJobs() error {
	// Check if crawl_jobs table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='crawl_jobs'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		// crawl_jobs doesn't exist, check if jobs table exists
		err = s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&tableName)
		if err == sql.ErrNoRows {
			// Neither table exists, this is a fresh install - skip migration
			s.logger.Debug().Msg("Migration: Neither crawl_jobs nor jobs table exists (fresh install)")
			return nil
		}
		// jobs table exists, migration already complete
		s.logger.Debug().Msg("Migration: jobs table already exists, crawl_jobs migration complete")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check for crawl_jobs table: %w", err)
	}

	s.logger.Info().Msg("Running migration: Migrating from crawl_jobs to unified jobs table")

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Step 1: Create new jobs table with unified schema
	s.logger.Info().Msg("Step 1: Creating new jobs table with unified schema")
	_, err = tx.Exec(`
		CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			parent_id TEXT,
			job_type TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			config_json TEXT NOT NULL,
			metadata_json TEXT,
			status TEXT NOT NULL,
			progress_json TEXT,
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			completed_at INTEGER,
			finished_at INTEGER,
			last_heartbeat INTEGER,
			error TEXT,
			result_count INTEGER DEFAULT 0,
			failed_count INTEGER DEFAULT 0,
			depth INTEGER DEFAULT 0,
			FOREIGN KEY (parent_id) REFERENCES jobs(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Step 2: Migrate data from crawl_jobs to jobs
	// Note: source_type, entity_type, source_config_snapshot, auth_snapshot, refresh_source, seed_urls are dropped
	// These crawler-specific fields should be moved into config_json or metadata_json if needed
	s.logger.Info().Msg("Step 2: Migrating data from crawl_jobs to jobs")
	_, err = tx.Exec(`
		INSERT INTO jobs (
			id, parent_id, job_type, name, description, config_json, metadata_json,
			status, progress_json, created_at, started_at, completed_at, finished_at,
			last_heartbeat, error, result_count, failed_count, depth
		)
		SELECT
			id, parent_id, job_type, name, description, config_json, metadata,
			status, progress_json, created_at, started_at, completed_at, finished_at,
			last_heartbeat, error, result_count, failed_count, 0
		FROM crawl_jobs
	`)
	if err != nil {
		return fmt.Errorf("failed to migrate data from crawl_jobs to jobs: %w", err)
	}

	// Step 3: Drop old crawl_jobs table
	s.logger.Info().Msg("Step 3: Dropping old crawl_jobs table")
	_, err = tx.Exec(`DROP TABLE crawl_jobs`)
	if err != nil {
		return fmt.Errorf("failed to drop crawl_jobs table: %w", err)
	}

	// Step 4: Create indexes on jobs table
	s.logger.Info().Msg("Step 4: Creating indexes on jobs table")
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_status: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_created: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_parent_id ON jobs(parent_id, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_parent_id: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(job_type, status, created_at DESC)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_jobs_type_status: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Msg("Migration: Successfully migrated from crawl_jobs to unified jobs table")
	return nil
}
