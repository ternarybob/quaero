package llm

import (
	"database/sql"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/llm/offline"
)

// NewLLMService creates the appropriate LLM service implementation based on configuration
func NewLLMService(
	cfg *common.Config,
	db interface{},
	logger arbor.ILogger,
) (interfaces.LLMService, AuditLogger, error) {
	// Cast db to *sql.DB
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return nil, nil, fmt.Errorf("database must be *sql.DB, got %T", db)
	}

	// Validate LLM mode
	if cfg.LLM.Mode != "offline" && cfg.LLM.Mode != "cloud" {
		return nil, nil, fmt.Errorf("invalid LLM mode '%s': must be 'offline' or 'cloud'", cfg.LLM.Mode)
	}

	logger.Info().Str("mode", cfg.LLM.Mode).Msg("Initializing LLM service")

	// Create audit logger from database and audit config
	var auditLogger AuditLogger
	if cfg.LLM.Audit.Enabled {
		auditLogger = NewSQLiteAuditLogger(sqlDB, cfg.LLM.Audit.LogQueries, logger)
		logger.Info().Msg("LLM audit logging enabled")
	} else {
		auditLogger = NewNullAuditLogger()
		logger.Info().Msg("LLM audit logging disabled (using NullAuditLogger)")
	}

	// Create appropriate service based on mode
	switch cfg.LLM.Mode {
	case "offline":
		return createOfflineService(cfg, auditLogger, logger)

	case "cloud":
		return nil, nil, fmt.Errorf("cloud mode not yet implemented")

	default:
		return nil, nil, fmt.Errorf("unsupported LLM mode: %s", cfg.LLM.Mode)
	}
}

// createOfflineService creates and validates the offline LLM service
func createOfflineService(
	cfg *common.Config,
	auditLogger AuditLogger,
	logger arbor.ILogger,
) (interfaces.LLMService, AuditLogger, error) {
	// Check for mock mode (used in tests)
	if cfg.LLM.Offline.MockMode {
		logger.Warn().Msg("Creating offline LLM service in MOCK mode (for testing only)")
		service := offline.NewMockOfflineLLMService(logger)
		return service, auditLogger, nil
	}

	// Validate offline configuration
	if err := validateOfflineConfig(&cfg.LLM.Offline); err != nil {
		return nil, nil, fmt.Errorf("invalid offline configuration: %w", err)
	}

	logger.Debug().
		Str("model_dir", cfg.LLM.Offline.ModelDir).
		Str("embed_model", cfg.LLM.Offline.EmbedModel).
		Str("chat_model", cfg.LLM.Offline.ChatModel).
		Int("context_size", cfg.LLM.Offline.ContextSize).
		Int("thread_count", cfg.LLM.Offline.ThreadCount).
		Msg("Creating offline LLM service")

	// Create offline service
	service, err := offline.NewOfflineLLMService(
		cfg.Server.LlamaDir,
		cfg.LLM.Offline.ModelDir,
		cfg.LLM.Offline.EmbedModel,
		cfg.LLM.Offline.ChatModel,
		cfg.LLM.Offline.ContextSize,
		cfg.LLM.Offline.ThreadCount,
		cfg.LLM.Offline.GPULayers,
		logger,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create offline LLM service, falling back to MOCK mode. Please install llama-cli for full functionality.")
		mockService := offline.NewMockOfflineLLMService(logger)
		return mockService, auditLogger, nil
	}

	return service, auditLogger, nil
}

// validateOfflineConfig validates the offline LLM configuration
func validateOfflineConfig(cfg *common.OfflineLLMConfig) error {
	if cfg.ModelDir == "" {
		return fmt.Errorf("ModelDir is required for offline mode")
	}

	if cfg.EmbedModel == "" {
		return fmt.Errorf("EmbedModel is required for offline mode")
	}

	if cfg.ChatModel == "" {
		return fmt.Errorf("ChatModel is required for offline mode")
	}

	if cfg.ContextSize <= 0 {
		return fmt.Errorf("ContextSize must be greater than 0, got %d", cfg.ContextSize)
	}

	if cfg.ThreadCount <= 0 {
		return fmt.Errorf("ThreadCount must be greater than 0, got %d", cfg.ThreadCount)
	}

	return nil
}

// validateCloudConfig validates the cloud LLM configuration
func validateCloudConfig(cfg *common.CloudLLMConfig) error {
	if cfg.Provider == "" {
		return fmt.Errorf("Provider is required for cloud mode")
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("APIKey is required for cloud mode (security violation: cannot use cloud mode without API key)")
	}

	return nil
}
