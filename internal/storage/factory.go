package storage

import (
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

// NewStorageManager creates a new storage manager based on config
func NewStorageManager(logger arbor.ILogger, config *common.Config) (interfaces.StorageManager, error) {
	switch config.Storage.Type {
	case "sqlite", "":
		return sqlite.NewManager(logger, &config.Storage.SQLite)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}
}

// NewAuthStorage creates a new Auth storage instance
func NewAuthStorage(logger arbor.ILogger, config *common.Config) (interfaces.AuthStorage, error) {
	manager, err := NewStorageManager(logger, config)
	if err != nil {
		return nil, err
	}
	return manager.AuthStorage(), nil
}
