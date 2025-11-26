package storage

import (
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/storage/badger"
)

// NewStorageManager creates a new storage manager based on config
// Note: Only Badger storage is supported
func NewStorageManager(logger arbor.ILogger, config *common.Config) (interfaces.StorageManager, error) {
	return badger.NewManager(logger, &config.Storage.Badger)
}
