package config

import (
	"testing"

	"github.com/ternarybob/quaero/internal/common"
)

func TestNewService(t *testing.T) {
	config := common.NewDefaultConfig()
	service := NewService(config)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestGetConfig(t *testing.T) {
	config := common.NewDefaultConfig()
	service := NewService(config)

	result := service.GetConfig()
	if result != config {
		t.Error("GetConfig() did not return the same config instance")
	}
}

func TestServerConfigAccessors(t *testing.T) {
	config := common.NewDefaultConfig()
	config.Server.Port = 9090
	config.Server.Host = "0.0.0.0"

	service := NewService(config)

	if service.GetServerPort() != 9090 {
		t.Errorf("GetServerPort() = %d, want 9090", service.GetServerPort())
	}

	if service.GetServerHost() != "0.0.0.0" {
		t.Errorf("GetServerHost() = %s, want 0.0.0.0", service.GetServerHost())
	}

	expectedURL := "http://0.0.0.0:9090"
	if service.GetServerURL() != expectedURL {
		t.Errorf("GetServerURL() = %s, want %s", service.GetServerURL(), expectedURL)
	}
}

func TestStorageConfigAccessors(t *testing.T) {
	config := common.NewDefaultConfig()
	config.Storage.Type = "sqlite"
	config.Storage.SQLite.Path = "/tmp/test.db"

	service := NewService(config)

	if service.GetStorageType() != "sqlite" {
		t.Errorf("GetStorageType() = %s, want sqlite", service.GetStorageType())
	}

	if service.GetSQLitePath() != "/tmp/test.db" {
		t.Errorf("GetSQLitePath() = %s, want /tmp/test.db", service.GetSQLitePath())
	}
}

func TestLLMConfigAccessors(t *testing.T) {
	config := common.NewDefaultConfig()
	config.LLM.Mode = "offline"

	service := NewService(config)

	if service.GetLLMMode() != "offline" {
		t.Errorf("GetLLMMode() = %s, want offline", service.GetLLMMode())
	}

	offlineConfig := service.GetOfflineLLMConfig()
	if offlineConfig.ModelDir != config.LLM.Offline.ModelDir {
		t.Errorf("GetOfflineLLMConfig() returned incorrect ModelDir")
	}

	cloudConfig := service.GetCloudLLMConfig()
	if cloudConfig.Provider != config.LLM.Cloud.Provider {
		t.Errorf("GetCloudLLMConfig() returned incorrect Provider")
	}
}

func TestRAGConfigAccessors(t *testing.T) {
	config := common.NewDefaultConfig()
	config.RAG.MaxDocuments = 10
	config.RAG.MinSimilarity = 0.8
	config.RAG.SearchMode = "vector"

	service := NewService(config)

	ragConfig := service.GetRAGConfig()
	if ragConfig.MaxDocuments != 10 {
		t.Errorf("GetRAGConfig().MaxDocuments = %d, want 10", ragConfig.MaxDocuments)
	}
	if ragConfig.MinSimilarity != 0.8 {
		t.Errorf("GetRAGConfig().MinSimilarity = %f, want 0.8", ragConfig.MinSimilarity)
	}
	if ragConfig.SearchMode != "vector" {
		t.Errorf("GetRAGConfig().SearchMode = %s, want vector", ragConfig.SearchMode)
	}
}

func TestLoggingConfigAccessors(t *testing.T) {
	config := common.NewDefaultConfig()
	config.Logging.Level = "debug"
	config.Logging.Format = "json"
	config.Logging.Output = []string{"stdout", "file"}

	service := NewService(config)

	if service.GetLoggingLevel() != "debug" {
		t.Errorf("GetLoggingLevel() = %s, want debug", service.GetLoggingLevel())
	}

	if service.GetLoggingFormat() != "json" {
		t.Errorf("GetLoggingFormat() = %s, want json", service.GetLoggingFormat())
	}

	output := service.GetLoggingOutput()
	if len(output) != 2 {
		t.Errorf("GetLoggingOutput() length = %d, want 2", len(output))
	}
}

func TestSourcesConfigAccessors(t *testing.T) {
	tests := []struct {
		name              string
		jiraEnabled       bool
		confluenceEnabled bool
		githubEnabled     bool
	}{
		{"all disabled", false, false, false},
		{"jira only", true, false, false},
		{"confluence only", false, true, false},
		{"github only", false, false, true},
		{"all enabled", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := common.NewDefaultConfig()
			config.Sources.Jira.Enabled = tt.jiraEnabled
			config.Sources.Confluence.Enabled = tt.confluenceEnabled
			config.Sources.GitHub.Enabled = tt.githubEnabled

			service := NewService(config)

			if service.IsJiraEnabled() != tt.jiraEnabled {
				t.Errorf("IsJiraEnabled() = %v, want %v", service.IsJiraEnabled(), tt.jiraEnabled)
			}

			if service.IsConfluenceEnabled() != tt.confluenceEnabled {
				t.Errorf("IsConfluenceEnabled() = %v, want %v", service.IsConfluenceEnabled(), tt.confluenceEnabled)
			}

			if service.IsGitHubEnabled() != tt.githubEnabled {
				t.Errorf("IsGitHubEnabled() = %v, want %v", service.IsGitHubEnabled(), tt.githubEnabled)
			}
		})
	}
}
