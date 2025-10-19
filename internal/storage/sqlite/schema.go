package sqlite

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

-- Crawler job history with configuration snapshots for re-runnable jobs
-- Inspired by Firecrawl's async job model
CREATE TABLE IF NOT EXISTS crawl_jobs (
	id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	config_json TEXT NOT NULL,
	source_config_snapshot TEXT,
	auth_snapshot TEXT,
	refresh_source INTEGER DEFAULT 0,
	status TEXT NOT NULL,
	progress_json TEXT,
	created_at INTEGER NOT NULL,
	started_at INTEGER,
	completed_at INTEGER,
	error TEXT,
	result_count INTEGER DEFAULT 0,
	failed_count INTEGER DEFAULT 0,
	logs TEXT DEFAULT '[]'
);

-- Crawler job indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status ON crawl_jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_source ON crawl_jobs(source_type, entity_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON crawl_jobs(created_at DESC);

-- Source configurations table
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	base_url TEXT NOT NULL,
	enabled INTEGER DEFAULT 1,
	auth_id TEXT,
	auth_domain TEXT,
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
	enabled INTEGER DEFAULT 1,
	last_run INTEGER,
	updated_at INTEGER NOT NULL
);

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

	// MIGRATION 5: Add logs column to crawl_jobs
	if err := s.migrateAddJobLogsColumn(); err != nil {
		return err
	}

	return nil
}

// migrateCrawlJobsColumns adds missing columns to crawl_jobs table
func (s *SQLiteDB) migrateCrawlJobsColumns() error {
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
func (s *SQLiteDB) migrateAddHeartbeatColumn() error {
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

// migrateAddJobLogsColumn adds logs column to crawl_jobs table
func (s *SQLiteDB) migrateAddJobLogsColumn() error {
	columnsQuery := `PRAGMA table_info(crawl_jobs)`
	rows, err := s.db.Query(columnsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasLogs := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "logs" {
			hasLogs = true
			break
		}
	}

	// If column already exists, migration already completed
	if hasLogs {
		return nil
	}

	s.logger.Info().Msg("Running migration: Adding logs column to crawl_jobs")

	// Add the logs column
	if _, err := s.db.Exec(`ALTER TABLE crawl_jobs ADD COLUMN logs TEXT`); err != nil {
		return err
	}

	// Set default value to empty JSON array for existing rows
	s.logger.Info().Msg("Setting default logs values for existing jobs")
	if _, err := s.db.Exec(`UPDATE crawl_jobs SET logs = '[]' WHERE logs IS NULL`); err != nil {
		return err
	}

	s.logger.Info().Msg("Migration: logs column added successfully")
	return nil
}
