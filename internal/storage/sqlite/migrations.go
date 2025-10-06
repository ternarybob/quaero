package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// migrate runs database migrations
func (s *SQLiteDB) migrate() error {
	ctx := context.Background()

	// Create migrations table
	if err := s.createMigrationsTable(ctx); err != nil {
		return err
	}

	// Run migrations
	migrations := []migration{
		{version: 1, name: "initial_schema", up: migrateV1},
		{version: 2, name: "fts5_indexes", up: migrateV2},
		{version: 3, name: "documents_table", up: migrateV3},
		{version: 4, name: "llm_audit_log", up: migrateV4},
		{version: 5, name: "document_sync_fields", up: migrateV5},
	}

	for _, m := range migrations {
		if err := s.runMigration(ctx, m); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.name, err)
		}
	}

	return nil
}

type migration struct {
	version int
	name    string
	up      func(context.Context, *sql.Tx) error
}

func (s *SQLiteDB) createMigrationsTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at INTEGER NOT NULL
	)`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *SQLiteDB) runMigration(ctx context.Context, m migration) error {
	// Check if migration already applied
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Already applied
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Run migration
	if err := m.up(ctx, tx); err != nil {
		return err
	}

	// Record migration
	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, strftime('%s', 'now'))",
		m.version, m.name)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// migrateV1 creates the initial schema
func migrateV1(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		// Jira Projects
		`CREATE TABLE IF NOT EXISTS jira_projects (
			key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			id TEXT NOT NULL,
			issue_count INTEGER DEFAULT 0,
			data JSON,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,

		// Jira Issues
		`CREATE TABLE IF NOT EXISTS jira_issues (
			key TEXT PRIMARY KEY,
			project_key TEXT NOT NULL,
			id TEXT NOT NULL,
			summary TEXT,
			description TEXT,
			fields JSON,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now')),
			FOREIGN KEY (project_key) REFERENCES jira_projects(key) ON DELETE CASCADE
		)`,

		// Confluence Spaces
		`CREATE TABLE IF NOT EXISTS confluence_spaces (
			key TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			id TEXT NOT NULL,
			page_count INTEGER DEFAULT 0,
			data JSON,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,

		// Confluence Pages
		`CREATE TABLE IF NOT EXISTS confluence_pages (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT,
			body JSON,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now')),
			FOREIGN KEY (space_id) REFERENCES confluence_spaces(id) ON DELETE CASCADE
		)`,

		// Auth Credentials
		`CREATE TABLE IF NOT EXISTS auth_credentials (
			service TEXT PRIMARY KEY,
			data JSON,
			cookies BLOB,
			tokens JSON,
			base_url TEXT,
			user_agent TEXT,
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_jira_issues_project ON jira_issues(project_key)`,
		`CREATE INDEX IF NOT EXISTS idx_confluence_pages_space ON confluence_pages(space_id)`,
		`CREATE INDEX IF NOT EXISTS idx_jira_issues_updated ON jira_issues(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_confluence_pages_updated ON confluence_pages(updated_at)`,
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	return nil
}

// migrateV2 creates FTS5 indexes
func migrateV2(ctx context.Context, tx *sql.Tx) error {
	// Only create FTS5 tables if enabled
	var fts5Enabled bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pragma_compile_options WHERE compile_options LIKE '%ENABLE_FTS5%')").Scan(&fts5Enabled)
	if err != nil || !fts5Enabled {
		// FTS5 not available, skip
		return nil
	}

	queries := []string{
		// FTS5 for Jira issues
		`CREATE VIRTUAL TABLE IF NOT EXISTS jira_issues_fts USING fts5(
			key UNINDEXED,
			summary,
			description,
			content=jira_issues,
			content_rowid=rowid
		)`,

		// Triggers to keep FTS in sync with jira_issues
		`CREATE TRIGGER IF NOT EXISTS jira_issues_ai AFTER INSERT ON jira_issues BEGIN
			INSERT INTO jira_issues_fts(rowid, key, summary, description)
			VALUES (new.rowid, new.key, new.summary, new.description);
		END`,

		`CREATE TRIGGER IF NOT EXISTS jira_issues_ad AFTER DELETE ON jira_issues BEGIN
			DELETE FROM jira_issues_fts WHERE rowid = old.rowid;
		END`,

		`CREATE TRIGGER IF NOT EXISTS jira_issues_au AFTER UPDATE ON jira_issues BEGIN
			DELETE FROM jira_issues_fts WHERE rowid = old.rowid;
			INSERT INTO jira_issues_fts(rowid, key, summary, description)
			VALUES (new.rowid, new.key, new.summary, new.description);
		END`,

		// FTS5 for Confluence pages
		`CREATE VIRTUAL TABLE IF NOT EXISTS confluence_pages_fts USING fts5(
			id UNINDEXED,
			title,
			content,
			content=confluence_pages,
			content_rowid=rowid
		)`,

		// Triggers to keep FTS in sync with confluence_pages
		`CREATE TRIGGER IF NOT EXISTS confluence_pages_ai AFTER INSERT ON confluence_pages BEGIN
			INSERT INTO confluence_pages_fts(rowid, id, title, content)
			VALUES (new.rowid, new.id, new.title, new.content);
		END`,

		`CREATE TRIGGER IF NOT EXISTS confluence_pages_ad AFTER DELETE ON confluence_pages BEGIN
			DELETE FROM confluence_pages_fts WHERE rowid = old.rowid;
		END`,

		`CREATE TRIGGER IF NOT EXISTS confluence_pages_au AFTER UPDATE ON confluence_pages BEGIN
			DELETE FROM confluence_pages_fts WHERE rowid = old.rowid;
			INSERT INTO confluence_pages_fts(rowid, id, title, content)
			VALUES (new.rowid, new.id, new.title, new.content);
		END`,
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			// FTS5 creation might fail if not supported, log but don't fail migration
			return nil
		}
	}

	return nil
}

// migrateV3 creates normalized documents table with vector support
func migrateV3(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		// Documents table
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			content_markdown TEXT,
			embedding BLOB,
			embedding_model TEXT,
			metadata TEXT,
			url TEXT,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now')),
			UNIQUE(source_type, source_id)
		)`,

		// Indexes for documents
		`CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_source_id ON documents(source_type, source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_updated ON documents(updated_at)`,

		// Document chunks table
		`CREATE TABLE IF NOT EXISTS document_chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			embedding BLOB,
			token_count INTEGER,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
			UNIQUE(document_id, chunk_index)
		)`,

		// Index for chunks
		`CREATE INDEX IF NOT EXISTS idx_chunks_document ON document_chunks(document_id)`,

		// FTS5 for documents (if available)
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			title,
			content,
			content=documents,
			content_rowid=rowid
		)`,

		// Triggers to keep FTS in sync
		`CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
			INSERT INTO documents_fts(rowid, title, content)
			VALUES (new.rowid, new.title, new.content);
		END`,

		`CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
			DELETE FROM documents_fts WHERE rowid = old.rowid;
		END`,

		`CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
			DELETE FROM documents_fts WHERE rowid = old.rowid;
			INSERT INTO documents_fts(rowid, title, content)
			VALUES (new.rowid, new.title, new.content);
		END`,
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	return nil
}

// migrateV4 creates llm_audit_log table for LLM operation tracking
func migrateV4(ctx context.Context, tx *sql.Tx) error {
	query := `
	CREATE TABLE IF NOT EXISTS llm_audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		mode TEXT NOT NULL,
		operation TEXT NOT NULL,
		success INTEGER NOT NULL,
		error TEXT,
		duration INTEGER NOT NULL,
		query_text TEXT,
		created_at INTEGER DEFAULT (strftime('%s', 'now'))
	)`

	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create llm_audit_log table: %w", err)
	}

	// Create index for timestamp queries
	indexQuery := `CREATE INDEX IF NOT EXISTS idx_llm_audit_timestamp ON llm_audit_log(timestamp DESC)`
	if _, err := tx.ExecContext(ctx, indexQuery); err != nil {
		return fmt.Errorf("failed to create llm_audit_log index: %w", err)
	}

	return nil
}

// migrateV5 adds sync tracking fields to documents table
func migrateV5(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`ALTER TABLE documents ADD COLUMN last_synced INTEGER`,
		`ALTER TABLE documents ADD COLUMN source_version TEXT`,
		`ALTER TABLE documents ADD COLUMN force_sync_pending INTEGER DEFAULT 0`,
		`ALTER TABLE documents ADD COLUMN force_embed_pending INTEGER DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_documents_force_sync ON documents(force_sync_pending) WHERE force_sync_pending = 1`,
		`CREATE INDEX IF NOT EXISTS idx_documents_force_embed ON documents(force_embed_pending) WHERE force_embed_pending = 1`,
		`CREATE INDEX IF NOT EXISTS idx_documents_last_synced ON documents(last_synced)`,
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	return nil
}
