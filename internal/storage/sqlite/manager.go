package sqlite

import (
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Manager implements the StorageManager interface
type Manager struct {
	db         *SQLiteDB
	jira       interfaces.JiraStorage
	confluence interfaces.ConfluenceStorage
	auth       interfaces.AuthStorage
	document   interfaces.DocumentStorage
	logger     arbor.ILogger
}

// NewManager creates a new SQLite storage manager
func NewManager(logger arbor.ILogger, config *common.SQLiteConfig) (interfaces.StorageManager, error) {
	db, err := NewSQLiteDB(logger, config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		db:         db,
		jira:       NewJiraStorage(db, logger),
		confluence: NewConfluenceStorage(db, logger),
		auth:       NewAuthStorage(db, logger),
		document:   NewDocumentStorage(db, logger),
		logger:     logger,
	}, nil
}

// JiraStorage returns the Jira storage interface
func (m *Manager) JiraStorage() interfaces.JiraStorage {
	return m.jira
}

// ConfluenceStorage returns the Confluence storage interface
func (m *Manager) ConfluenceStorage() interfaces.ConfluenceStorage {
	return m.confluence
}

// AuthStorage returns the Auth storage interface
func (m *Manager) AuthStorage() interfaces.AuthStorage {
	return m.auth
}

// DocumentStorage returns the Document storage interface
func (m *Manager) DocumentStorage() interfaces.DocumentStorage {
	return m.document
}

// DB returns the underlying database connection
func (m *Manager) DB() interface{} {
	if m.db != nil {
		return m.db.DB()
	}
	return nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
