package common

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration
type Config struct {
	App     AppConfig     `toml:"app"`
	Server  ServerConfig  `toml:"server"`
	Sources SourcesConfig `toml:"sources"`
	Storage StorageConfig `toml:"storage"`
	LLM     LLMConfig     `toml:"llm"`
	Logging LoggingConfig `toml:"logging"`
}

type AppConfig struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type SourcesConfig struct {
	Confluence ConfluenceConfig `toml:"confluence"`
	Jira       JiraConfig       `toml:"jira"`
	GitHub     GitHubConfig     `toml:"github"`
	Slack      SlackConfig      `toml:"slack"`
	Linear     LinearConfig     `toml:"linear"`
	Notion     NotionConfig     `toml:"notion"`
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

type SlackConfig struct {
	Enabled bool `toml:"enabled"`
}

type LinearConfig struct {
	Enabled bool `toml:"enabled"`
}

type NotionConfig struct {
	Enabled bool `toml:"enabled"`
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
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}

// LoadFromFile loads configuration from a TOML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = toml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
