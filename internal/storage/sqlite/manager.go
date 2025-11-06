package sqlite

import (
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Manager implements the StorageManager interface
type Manager struct {
	db            *SQLiteDB
	auth          interfaces.AuthStorage
	document      interfaces.DocumentStorage
	job           interfaces.JobStorage
	jobLog        interfaces.JobLogStorage
	jobDefinition interfaces.JobDefinitionStorage
	logger        arbor.ILogger
}

// NewManager creates a new SQLite storage manager
func NewManager(logger arbor.ILogger, config *common.SQLiteConfig, environment string) (interfaces.StorageManager, error) {
	db, err := NewSQLiteDB(logger, config, environment)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		db:            db,
		auth:          NewAuthStorage(db, logger),
		document:      NewDocumentStorage(db, logger),
		job:           NewJobStorage(db, logger),
		jobLog:        NewJobLogStorage(db, logger),
		jobDefinition: NewJobDefinitionStorage(db, logger),
		logger:        logger,
	}

	logger.Info().Msg("Storage manager initialized")

	return manager, nil
}

// AuthStorage returns the Auth storage interface
func (m *Manager) AuthStorage() interfaces.AuthStorage {
	return m.auth
}

// DocumentStorage returns the Document storage interface
func (m *Manager) DocumentStorage() interfaces.DocumentStorage {
	return m.document
}

// JobStorage returns the Job storage interface
func (m *Manager) JobStorage() interfaces.JobStorage {
	return m.job
}

// JobLogStorage returns the JobLog storage interface
func (m *Manager) JobLogStorage() interfaces.JobLogStorage {
	return m.jobLog
}

// JobDefinitionStorage returns the JobDefinition storage interface
func (m *Manager) JobDefinitionStorage() interfaces.JobDefinitionStorage {
	return m.jobDefinition
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
