// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 6:44:16 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
)

const schemaSQL = `
-- Authentication table
-- Site-based authentication for cookie-based web services
CREATE TABLE IF NOT EXISTS auth_credentials (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	site_domain TEXT,
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
CREATE INDEX IF NOT EXISTS idx_auth_service_type ON auth_credentials(service_type, site_domain);

-- Key/Value store table for generic key/value storage
-- Provides simple string-based storage with optional descriptions
CREATE TABLE IF NOT EXISTS key_value_store (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	description TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- Index for efficient listing by recency
CREATE INDEX IF NOT EXISTS idx_kv_updated ON key_value_store(updated_at DESC);

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
	tags TEXT,
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
	job_type TEXT NOT NULL DEFAULT 'user',
	description TEXT,
	source_type TEXT,
	base_url TEXT,
	auth_id TEXT,
	steps TEXT NOT NULL,
	schedule TEXT NOT NULL,
	timeout TEXT,
	enabled INTEGER DEFAULT 1,
	auto_start INTEGER DEFAULT 0,
	config TEXT,
	pre_jobs TEXT,
	post_jobs TEXT,
	error_tolerance TEXT,
	tags TEXT,
	toml TEXT,
	validation_status TEXT DEFAULT 'unknown',
	validation_error TEXT DEFAULT '',
	validated_at INTEGER,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL,
	CHECK (job_type IN ('system', 'user'))
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
// Database is built from scratch on each startup - no migrations needed
func (s *SQLiteDB) InitSchema() error {
	// Execute schema SQL to create all tables
	_, err := s.db.Exec(schemaSQL)
	if err != nil {
		return err
	}
	s.logger.Info().Msg("Database schema initialized")

	// Create default job definitions
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

// runMigrations is deprecated - database is rebuilt from scratch on each startup
// Schema is fully defined in schemaSQL constant above
// This function is kept for backward compatibility but does nothing
func (s *SQLiteDB) runMigrations() error {
	s.logger.Debug().Msg("Migration system disabled - database built from scratch")
	return nil
}
