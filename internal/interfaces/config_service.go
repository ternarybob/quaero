package interfaces

import "github.com/ternarybob/quaero/internal/common"

// ConfigService provides access to application configuration
// Priority system: CLI flags > Environment variables > Config file > Defaults
type ConfigService interface {
	// GetConfig returns the complete configuration
	GetConfig() *common.Config

	// Server configuration accessors
	GetServerPort() int
	GetServerHost() string
	GetServerURL() string

	// Storage configuration accessors
	GetStorageType() string
	GetSQLitePath() string

	// LLM configuration accessors
	GetLLMMode() string
	GetOfflineLLMConfig() common.OfflineLLMConfig
	GetCloudLLMConfig() common.CloudLLMConfig

	// RAG configuration accessors
	GetRAGConfig() common.RAGConfig

	// Logging configuration accessors
	GetLoggingLevel() string
	GetLoggingFormat() string
	GetLoggingOutput() []string
}
