package badger

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/timshannon/badgerhold/v4"
)

// BadgerDB manages the Badger database connection
type BadgerDB struct {
	store  *badgerhold.Store
	logger arbor.ILogger
	config *common.BadgerConfig
}

// NewBadgerDB creates a new Badger database connection
func NewBadgerDB(logger arbor.ILogger, config *common.BadgerConfig) (*BadgerDB, error) {
	// If reset_on_startup is enabled, delete the existing database
	if config.ResetOnStartup {
		if _, err := os.Stat(config.Path); err == nil {
			logger.Debug().Str("path", config.Path).Msg("Deleting existing database (reset_on_startup=true)")
			if err := os.RemoveAll(config.Path); err != nil {
				logger.Warn().Err(err).Str("path", config.Path).Msg("Failed to delete database directory")
			}
		}
	}

	// Ensure the directory exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	logger.Debug().Str("path", config.Path).Msg("Opening Badger database connection")

	options := badgerhold.DefaultOptions
	options.Dir = config.Path
	options.ValueDir = config.Path
	options.Logger = nil // Disable default badger logger to use arbor

	store, err := badgerhold.Open(options)
	if err != nil {
		logger.Fatal().Err(err).Str("path", config.Path).Msg("BadgerDB: Failed to open database")
		return nil, fmt.Errorf("failed to open badger database: %w", err)
	}

	logger.Debug().Str("path", config.Path).Msg("Badger database initialized")

	return &BadgerDB{
		store:  store,
		logger: logger,
		config: config,
	}, nil
}

// Store returns the underlying badgerhold store
func (b *BadgerDB) Store() *badgerhold.Store {
	return b.store
}

// Close closes the database connection
func (b *BadgerDB) Close() error {
	if b.store != nil {
		return b.store.Close()
	}
	return nil
}
