package sqlite

const schemaSQL = `
-- Authentication table
CREATE TABLE IF NOT EXISTS auth_credentials (
	service TEXT PRIMARY KEY,
	data TEXT NOT NULL,
	cookies TEXT,
	tokens TEXT NOT NULL,
	base_url TEXT NOT NULL,
	user_agent TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

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
CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	source_type TEXT NOT NULL,
	source_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	content_markdown TEXT,
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_documents_sync ON documents(force_sync_pending, force_embed_pending);
CREATE INDEX IF NOT EXISTS idx_documents_embedding ON documents(embedding_model) WHERE embedding IS NOT NULL;

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
	return nil
}
