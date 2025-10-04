package common

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `toml:"server"`
	Sources SourcesConfig `toml:"sources"`
	Storage StorageConfig `toml:"storage"`
	LLM     LLMConfig     `toml:"llm"`
	Logging LoggingConfig `toml:"logging"`
}

type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type SourcesConfig struct {
	Confluence ConfluenceConfig `toml:"confluence"`
	Jira       JiraConfig       `toml:"jira"`
	GitHub     GitHubConfig     `toml:"github"`
}

type ConfluenceConfig struct {
	Enabled bool     `toml:"enabled"`
	Spaces  []string `toml:"spaces"`
}

type JiraConfig struct {
	Enabled  bool     `toml:"enabled"`
	Projects []string `toml:"projects"`
}

type GitHubConfig struct {
	Enabled bool     `toml:"enabled"`
	Token   string   `toml:"token"`
	Repos   []string `toml:"repos"`
}

type StorageConfig struct {
	RavenDB    RavenDBConfig    `toml:"ravendb"`
	Filesystem FilesystemConfig `toml:"filesystem"`
}

type RavenDBConfig struct {
	URLs     []string `toml:"urls"`
	Database string   `toml:"database"`
}

type FilesystemConfig struct {
	Images      string `toml:"images"`
	Attachments string `toml:"attachments"`
}

type LLMConfig struct {
	Ollama OllamaConfig `toml:"ollama"`
}

type OllamaConfig struct {
	URL         string `toml:"url"`
	TextModel   string `toml:"text_model"`
	VisionModel string `toml:"vision_model"`
}

type LoggingConfig struct {
	Level  string   `toml:"level"`
	Format string   `toml:"format"`
	Output []string `toml:"output"`
}

// NewDefaultConfig creates a configuration with default values
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: []string{"stdout", "file"},
		},
	}
}

// LoadFromFile loads configuration with priority: default -> file -> env -> CLI
// Priority system: CLI flags > Environment variables > Config file > Defaults
func LoadFromFile(path string) (*Config, error) {
	// Start with defaults
	config := NewDefaultConfig()

	// Load from file if exists (overrides defaults)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		err = toml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Apply environment variables (overrides file config)
	applyEnvOverrides(config)

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *Config) {
	// Server configuration
	if port := os.Getenv("QUAERO_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := os.Getenv("QUAERO_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}

	// Logging configuration
	if level := os.Getenv("QUAERO_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("QUAERO_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
	if output := os.Getenv("QUAERO_LOG_OUTPUT"); output != "" {
		// Split comma-separated output types
		outputs := []string{}
		for _, o := range splitString(output, ",") {
			trimmed := trimSpace(o)
			if trimmed != "" {
				outputs = append(outputs, trimmed)
			}
		}
		if len(outputs) > 0 {
			config.Logging.Output = outputs
		}
	}
}

// ApplyCLIOverrides applies CLI flag overrides to config
func ApplyCLIOverrides(config *Config, port int, host string) {
	// CLI flags have highest priority
	if port > 0 {
		config.Server.Port = port
	}
	if host != "" {
		config.Server.Host = host
	}
}

// Helper functions for string manipulation
func splitString(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
