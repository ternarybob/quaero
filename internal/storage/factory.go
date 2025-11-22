package storage

import (
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/storage/badger"
)

// NewStorageManager creates a new storage manager based on config
func NewStorageManager(logger arbor.ILogger, config *common.Config) (interfaces.StorageManager, error) {
	// Enforce Badger-only storage
	if config.Storage.Type != "badger" && config.Storage.Type != "" {
		return nil, fmt.Errorf("unsupported storage type: %s (only 'badger' is supported)", config.Storage.Type)
	}
	return badger.NewManager(logger, &config.Storage.Badger)
}
