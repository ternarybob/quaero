package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"maragu.dev/goqite"
	_ "modernc.org/sqlite"
)

// SQLiteDB manages the SQLite database connection
type SQLiteDB struct {
	db     *sql.DB
	logger arbor.ILogger
	config *common.SQLiteConfig
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(logger arbor.ILogger, config *common.SQLiteConfig) (*SQLiteDB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	// modernc.org/sqlite uses "sqlite" driver name (not "sqlite3")
	db, err := sql.Open("sqlite", config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool to prevent SQLITE_BUSY errors
	// SQLite doesn't handle concurrent writes well, so limit to 1 connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteDB{
		db:     db,
		logger: logger,
		config: config,
	}

	// Initialize goqite queue schema
	if err := goqite.Setup(context.Background(), db); err != nil {
		// Check if error is about table already existing (which is safe to ignore)
		errMsg := err.Error()
		if strings.Contains(errMsg, "table goqite already exists") {
			logger.Debug().Msg("goqite queue schema already exists (skipping initialization)")
		} else {
			db.Close()
			return nil, fmt.Errorf("failed to initialize goqite schema: %w", err)
		}
	} else {
		logger.Info().Msg("goqite queue schema initialized")
	}

	// Configure database
	if err := s.configure(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	// Initialize schema
	if err := s.InitSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info().Str("path", config.Path).Msg("SQLite database initialized")
	return s, nil
}

// configure sets up SQLite pragmas and settings
func (s *SQLiteDB) configure() error {
	pragmas := []string{
		fmt.Sprintf("PRAGMA cache_size = -%d", s.config.CacheSizeMB*1024), // Negative for KB
		fmt.Sprintf("PRAGMA busy_timeout = %d", s.config.BusyTimeoutMS),
		"PRAGMA foreign_keys = ON", // Enabled for referential integrity (CASCADE constraints for jobs, logs, URLs)
		"PRAGMA synchronous = NORMAL",
	}

	if s.config.WALMode {
		pragmas = append(pragmas, "PRAGMA journal_mode = WAL")
	}

	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	// Verify WAL mode is active
	if s.config.WALMode {
		var journalMode string
		if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to verify journal mode")
		} else {
			s.logger.Info().
				Str("journal_mode", journalMode).
				Int("busy_timeout_ms", s.config.BusyTimeoutMS).
				Int("cache_size_mb", s.config.CacheSizeMB).
				Msg("SQLite configuration applied")
		}
	}

	return nil
}

// DB returns the underlying database connection
func (s *SQLiteDB) DB() *sql.DB {
	return s.db
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// BeginTx starts a new transaction
func (s *SQLiteDB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

// Ping verifies the database connection
func (s *SQLiteDB) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
