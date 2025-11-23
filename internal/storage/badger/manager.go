package badger

import (
	"context"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Manager implements the StorageManager interface for Badger
type Manager struct {
	db            *BadgerDB
	auth          interfaces.AuthStorage
	document      interfaces.DocumentStorage
	job           interfaces.JobStorage
	jobLog        interfaces.JobLogStorage
	jobDefinition interfaces.JobDefinitionStorage
	kv            interfaces.KeyValueStorage
	connector     interfaces.ConnectorStorage
	logger        arbor.ILogger
}

// NewManager creates a new Badger storage manager
func NewManager(logger arbor.ILogger, config *common.BadgerConfig) (interfaces.StorageManager, error) {
	db, err := NewBadgerDB(logger, config)
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
		kv:            NewKVStorage(db, logger),
		connector:     NewConnectorStorage(db, logger),
		logger:        logger,
	}

	logger.Info().Msg("Badger storage manager initialized")

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

// KeyValueStorage returns the KeyValue storage interface
func (m *Manager) KeyValueStorage() interfaces.KeyValueStorage {
	return m.kv
}

// ConnectorStorage returns the Connector storage interface
func (m *Manager) ConnectorStorage() interfaces.ConnectorStorage {
	return m.connector
}

// DB returns the underlying database connection
func (m *Manager) DB() interface{} {
	if m.db != nil {
		return m.db.Store()
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

// MigrateAPIKeysToKVStore is a no-op function retained for backward compatibility.
func (m *Manager) MigrateAPIKeysToKVStore(ctx context.Context) error {
	m.logger.Info().Msg("MigrateAPIKeysToKVStore is no-op (Phase 4: API key migration completed)")
	return nil
}
