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
	job           interfaces.QueueStorage
	log           interfaces.LogStorage
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
		job:           NewQueueStorage(db, logger),
		log:           NewLogStorage(db, logger),
		jobDefinition: NewJobDefinitionStorage(db, logger),
		kv:            NewKVStorage(db, logger),
		connector:     NewConnectorStorage(db, logger),
		logger:        logger,
	}

	logger.Debug().Msg("Badger storage manager initialized")

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

// QueueStorage returns the Queue storage interface
func (m *Manager) QueueStorage() interfaces.QueueStorage {
	return m.job
}

// LogStorage returns the Log storage interface
func (m *Manager) LogStorage() interfaces.LogStorage {
	return m.log
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
	m.logger.Debug().Msg("MigrateAPIKeysToKVStore is no-op (Phase 4: API key migration completed)")
	return nil
}

// LoadJobDefinitionsFromFiles loads job definitions from TOML files
func (m *Manager) LoadJobDefinitionsFromFiles(ctx context.Context, dirPath string) error {
	return LoadJobDefinitionsFromFiles(ctx, m.jobDefinition, m.kv, dirPath, m.logger)
}

// LoadConnectorsFromFiles loads connectors from TOML files
func (m *Manager) LoadConnectorsFromFiles(ctx context.Context, dirPath string) error {
	return LoadConnectorsFromFiles(ctx, m.connector, m.kv, dirPath, m.logger)
}

// LoadEmailFromFile loads email configuration from email.toml file
func (m *Manager) LoadEmailFromFile(ctx context.Context, dirPath string) error {
	return LoadEmailFromFile(ctx, m.kv, dirPath, m.logger)
}
