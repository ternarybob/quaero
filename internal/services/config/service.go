package config

import (
	"fmt"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service implements the ConfigService interface
type Service struct {
	config *common.Config
}

// NewService creates a new configuration service
// The config parameter should already be loaded with defaults, file, env, and CLI overrides applied
func NewService(config *common.Config) interfaces.ConfigService {
	return &Service{
		config: config,
	}
}

// GetConfig returns the complete configuration
func (s *Service) GetConfig() *common.Config {
	return s.config
}

// Server configuration accessors
func (s *Service) GetServerPort() int {
	return s.config.Server.Port
}

func (s *Service) GetServerHost() string {
	return s.config.Server.Host
}

func (s *Service) GetServerURL() string {
	return fmt.Sprintf("http://%s:%d", s.config.Server.Host, s.config.Server.Port)
}

// Storage configuration accessors
func (s *Service) GetStorageType() string {
	return s.config.Storage.Type
}

func (s *Service) GetSQLitePath() string {
	return s.config.Storage.SQLite.Path
}

// LLM configuration accessors
func (s *Service) GetLLMMode() string {
	return s.config.LLM.Mode
}

func (s *Service) GetOfflineLLMConfig() common.OfflineLLMConfig {
	return s.config.LLM.Offline
}

func (s *Service) GetCloudLLMConfig() common.CloudLLMConfig {
	return s.config.LLM.Cloud
}

// RAG configuration accessors
func (s *Service) GetRAGConfig() common.RAGConfig {
	return s.config.RAG
}

// Logging configuration accessors
func (s *Service) GetLoggingLevel() string {
	return s.config.Logging.Level
}

func (s *Service) GetLoggingFormat() string {
	return s.config.Logging.Format
}

func (s *Service) GetLoggingOutput() []string {
	return s.config.Logging.Output
}

// Sources configuration accessors
func (s *Service) IsJiraEnabled() bool {
	return s.config.Sources.Jira.Enabled
}

func (s *Service) IsConfluenceEnabled() bool {
	return s.config.Sources.Confluence.Enabled
}

func (s *Service) IsGitHubEnabled() bool {
	return s.config.Sources.GitHub.Enabled
}
