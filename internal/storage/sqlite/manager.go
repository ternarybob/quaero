package sqlite

import (
	"context"

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
	kv            interfaces.KeyValueStorage
	logger        arbor.ILogger
}

// NewManager creates a new SQLite storage manager
func NewManager(logger arbor.ILogger, config *common.SQLiteConfig) (interfaces.StorageManager, error) {
	db, err := NewSQLiteDB(logger, config)
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
		logger:        logger,
	}

	logger.Info().Msg("Storage manager initialized (auth, document, job, jobLog, jobDefinition, kv)")

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

// MigrateAPIKeysToKVStore is a no-op function retained for backward compatibility.
//
// This migration was used in Phase 3 to move API keys from auth_credentials to key_value_store.
// As of Phase 4, the api_key and auth_type columns have been removed from auth_credentials,
// so this function no longer performs any migration.
//
// The function is kept to avoid breaking any code that may still call it, but it simply
// returns nil immediately. All API keys should now be managed directly through the KV store.
func (m *Manager) MigrateAPIKeysToKVStore(ctx context.Context) error {
	m.logger.Info().Msg("MigrateAPIKeysToKVStore is no-op (Phase 4: API key migration completed)")
	return nil
}
