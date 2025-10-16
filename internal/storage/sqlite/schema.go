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
CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL,
	source_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
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
	failed_count INTEGER DEFAULT 0
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_documents_sync ON documents(force_sync_pending, force_embed_pending);
CREATE INDEX IF NOT EXISTS idx_documents_embedding ON documents(embedding_model) WHERE embedding IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_documents_detail_level ON documents(detail_level, source_type);

-- FTS5 index for full-text search
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
	title,
	content,
	content=documents,
	content_rowid=rowid
);

-- Triggers to keep FTS index in sync
CREATE TRIGGER IF NOT EXISTS documents_fts_insert AFTER INSERT ON documents BEGIN
	INSERT INTO documents_fts(rowid, title, content)
	VALUES (new.rowid, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_fts_update AFTER UPDATE ON documents BEGIN
	UPDATE documents_fts
	SET title = new.title, content = new.content
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
	// Check if new crawl_jobs columns exist
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

	if !hasSourceConfigSnapshot || !hasAuthSnapshot || !hasRefreshSource || !hasSeedURLs {
		s.logger.Info().Msg("Schema migrations completed successfully")
	}

	return nil
}
