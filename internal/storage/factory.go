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

// NewJiraStorage creates a new Jira storage instance
func NewJiraStorage(logger arbor.ILogger, config *common.Config) (interfaces.JiraStorage, error) {
	manager, err := NewStorageManager(logger, config)
	if err != nil {
		return nil, err
	}
	return manager.JiraStorage(), nil
}

// NewConfluenceStorage creates a new Confluence storage instance
func NewConfluenceStorage(logger arbor.ILogger, config *common.Config) (interfaces.ConfluenceStorage, error) {
	manager, err := NewStorageManager(logger, config)
	if err != nil {
		return nil, err
	}
	return manager.ConfluenceStorage(), nil
}

// NewAuthStorage creates a new Auth storage instance
func NewAuthStorage(logger arbor.ILogger, config *common.Config) (interfaces.AuthStorage, error) {
	manager, err := NewStorageManager(logger, config)
	if err != nil {
		return nil, err
	}
	return manager.AuthStorage(), nil
}
